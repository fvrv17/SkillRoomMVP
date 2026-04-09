package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultSandboxCommand = "node /opt/skillroom-runtime/run-evaluation.mjs"
	defaultCPUFraction    = "0.50"
	defaultMemoryMB       = 256
	defaultTimeout        = 60 * time.Second
)

type DockerConfig struct {
	DockerBinary   string
	DockerHost     string
	SandboxImage   string
	SandboxCommand string
	DefaultCPU     string
	DefaultMemory  int
	DefaultTimeout time.Duration
}

type DockerEngine struct {
	config         DockerConfig
	client         *http.Client
	baseURL        string
	mu             sync.Mutex
	lastReadyCheck time.Time
	lastReadyErr   error
}

func NewDockerEngine(config DockerConfig) *DockerEngine {
	if strings.TrimSpace(config.SandboxCommand) == "" {
		config.SandboxCommand = defaultSandboxCommand
	}
	if strings.TrimSpace(config.DefaultCPU) == "" {
		config.DefaultCPU = defaultCPUFraction
	}
	if config.DefaultMemory <= 0 {
		config.DefaultMemory = defaultMemoryMB
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = defaultTimeout
	}
	config.DockerHost = firstNonEmpty(config.DockerHost, os.Getenv("RUNNER_DOCKER_HOST"), os.Getenv("DOCKER_HOST"), "unix:///var/run/docker.sock")
	client, baseURL := dockerAPIClient(config.DockerHost)
	return &DockerEngine{
		config:  config,
		client:  client,
		baseURL: baseURL,
	}
}

func (e *DockerEngine) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if strings.TrimSpace(req.Language) == "" {
		return RunResult{}, errors.New("language is required")
	}
	if len(req.Files) == 0 {
		return RunResult{}, errors.New("files are required")
	}
	if strings.TrimSpace(e.config.SandboxImage) == "" {
		return RunResult{}, errors.New("sandbox image is required")
	}

	runCtx := ctx
	timeout := e.config.DefaultTimeout
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	workspaceDir, err := os.MkdirTemp("", "skillroom-run-*")
	if err != nil {
		return RunResult{}, err
	}
	defer os.RemoveAll(workspaceDir)

	if err := writeWorkspace(workspaceDir, req); err != nil {
		return RunResult{}, err
	}

	containerName := fmt.Sprintf("skillroom-run-%d", time.Now().UTC().UnixNano())
	containerID, err := e.createContainer(runCtx, containerName, e.config.SandboxCommand, req.CPUFraction, req.MemoryMB)
	if err != nil {
		return RunResult{}, err
	}
	defer func() {
		_ = e.removeContainer(context.Background(), containerID)
	}()

	if err := e.copyWorkspace(runCtx, containerID, workspaceDir); err != nil {
		return RunResult{}, err
	}

	if runCtx.Err() == context.DeadlineExceeded {
		return RunResult{Errors: []string{"sandbox execution timed out"}}, errors.New("sandbox execution timed out")
	}
	if err := e.startContainer(runCtx, containerID); err != nil {
		return RunResult{}, err
	}

	exitStatus, err := e.waitContainer(runCtx, containerID)
	if runCtx.Err() == context.DeadlineExceeded {
		return RunResult{Errors: []string{"sandbox execution timed out"}}, errors.New("sandbox execution timed out")
	}
	if err != nil {
		return RunResult{}, err
	}

	stdout, stderr, err := e.readContainerLogs(runCtx, containerID)
	if err != nil {
		return RunResult{}, err
	}

	result, err := parseRunResult(stdout)
	if err != nil {
		return RunResult{}, fmt.Errorf("parse sandbox output: %w: %s", err, strings.TrimSpace(stderr))
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		result.Errors = append(result.Errors, trimmed)
	}
	if exitStatus != 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("sandbox exited with status %d", exitStatus))
	}
	return result, nil
}

func (e *DockerEngine) Ready(ctx context.Context) error {
	if e == nil {
		return errors.New("runner engine is not configured")
	}
	e.mu.Lock()
	if !e.lastReadyCheck.IsZero() && time.Since(e.lastReadyCheck) < 15*time.Second {
		err := e.lastReadyErr
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()

	readyCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		readyCtx, cancel = context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
	}
	err := e.probeReadiness(readyCtx)

	e.mu.Lock()
	e.lastReadyCheck = time.Now().UTC()
	e.lastReadyErr = err
	e.mu.Unlock()
	return err
}

func (e *DockerEngine) probeReadiness(ctx context.Context) error {
	if strings.TrimSpace(e.config.SandboxImage) == "" {
		return errors.New("sandbox image is required")
	}
	if err := e.inspectSandboxImage(ctx); err != nil {
		return err
	}
	containerID, err := e.createContainer(
		ctx,
		fmt.Sprintf("skillroom-ready-%d", time.Now().UTC().UnixNano()),
		"test -f /opt/skillroom-runtime/run-evaluation.mjs && node --version >/dev/null",
		e.config.DefaultCPU,
		e.config.DefaultMemory,
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = e.removeContainer(context.Background(), containerID)
	}()
	if err := e.startContainer(ctx, containerID); err != nil {
		return err
	}
	status, err := e.waitContainer(ctx, containerID)
	if err != nil {
		return err
	}
	if status != 0 {
		stdout, stderr, _ := e.readContainerLogs(ctx, containerID)
		return fmt.Errorf("start sandbox probe: exited with status %d: %s %s", status, strings.TrimSpace(stdout), strings.TrimSpace(stderr))
	}
	return nil
}

func (e *DockerEngine) inspectSandboxImage(ctx context.Context) error {
	path := fmt.Sprintf("/images/%s/json", dockerPathEscape(e.config.SandboxImage))
	resp, err := e.doDockerRequest(ctx, http.MethodGet, path, nil, nil, "")
	if err != nil {
		return fmt.Errorf("inspect sandbox image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("inspect sandbox image: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (e *DockerEngine) createContainer(ctx context.Context, name, command, cpuFraction string, memoryMB int) (string, error) {
	payload := map[string]any{
		"Image":      e.config.SandboxImage,
		"WorkingDir": "/workspace",
		"User":       "0:0",
		"Entrypoint": []string{"sh"},
		"Cmd":        []string{"-lc", command},
		"HostConfig": map[string]any{
			"NetworkMode": "none",
			"NanoCpus":    cpuFractionToNano(firstNonEmpty(cpuFraction, e.config.DefaultCPU)),
			"Memory":      int64(maxInt(memoryMB, e.config.DefaultMemory)) * 1024 * 1024,
			"PidsLimit":   int64(512),
			"CapDrop":     []string{"ALL"},
			"SecurityOpt": []string{"no-new-privileges"},
			"Ulimits": []map[string]any{
				{"Name": "nproc", "Soft": 512, "Hard": 512},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	resp, err := e.doDockerRequest(ctx, http.MethodPost, "/containers/create", map[string]string{"name": name}, bytes.NewReader(body), "application/json")
	if err != nil {
		return "", fmt.Errorf("create sandbox container: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("create sandbox container: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode created container: %w", err)
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", errors.New("docker did not return a container id")
	}
	return result.ID, nil
}

func (e *DockerEngine) copyWorkspace(ctx context.Context, containerID, workspaceDir string) error {
	archive, err := tarWorkspace(workspaceDir)
	if err != nil {
		return err
	}
	resp, err := e.doDockerRequest(ctx, http.MethodPut, "/containers/"+containerID+"/archive", map[string]string{"path": "/workspace"}, bytesReader(archive), "application/x-tar")
	if err != nil {
		return fmt.Errorf("copy workspace into sandbox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("copy workspace into sandbox: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (e *DockerEngine) startContainer(ctx context.Context, containerID string) error {
	resp, err := e.doDockerRequest(ctx, http.MethodPost, "/containers/"+containerID+"/start", nil, nil, "")
	if err != nil {
		return fmt.Errorf("start sandbox container: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("start sandbox container: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (e *DockerEngine) waitContainer(ctx context.Context, containerID string) (int64, error) {
	resp, err := e.doDockerRequest(ctx, http.MethodPost, "/containers/"+containerID+"/wait", map[string]string{"condition": "not-running"}, nil, "")
	if err != nil {
		return 0, fmt.Errorf("wait for sandbox container: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("wait for sandbox container: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		StatusCode int64 `json:"StatusCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode wait response: %w", err)
	}
	return result.StatusCode, nil
}

func (e *DockerEngine) readContainerLogs(ctx context.Context, containerID string) (string, string, error) {
	resp, err := e.doDockerRequest(ctx, http.MethodGet, "/containers/"+containerID+"/logs", map[string]string{
		"stdout": "1",
		"stderr": "1",
	}, nil, "")
	if err != nil {
		return "", "", fmt.Errorf("read sandbox logs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("read sandbox logs: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return demuxDockerStream(resp.Body)
}

func (e *DockerEngine) removeContainer(ctx context.Context, containerID string) error {
	resp, err := e.doDockerRequest(ctx, http.MethodDelete, "/containers/"+containerID, map[string]string{"force": "1"}, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("remove sandbox container: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (e *DockerEngine) doDockerRequest(ctx context.Context, method, path string, query map[string]string, body io.Reader, contentType string) (*http.Response, error) {
	reqURL := e.baseURL + path
	if len(query) > 0 {
		values := make([]string, 0, len(query))
		for key, value := range query {
			values = append(values, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
		sort.Strings(values)
		reqURL += "?" + strings.Join(values, "&")
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return e.client.Do(req)
}

func dockerAPIClient(host string) (*http.Client, string) {
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "unix://") {
		socketPath := strings.TrimPrefix(firstNonEmpty(host, "unix:///var/run/docker.sock"), "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		}
		return &http.Client{Transport: transport}, "http://docker"
	}
	if strings.HasPrefix(host, "tcp://") {
		host = "http://" + strings.TrimPrefix(host, "tcp://")
	}
	return &http.Client{}, strings.TrimRight(host, "/")
}

func tarWorkspace(root string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)

	err := filepath.Walk(root, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, currentPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

func demuxDockerStream(body io.Reader) (string, string, error) {
	payload, err := io.ReadAll(body)
	if err != nil {
		return "", "", err
	}
	if len(payload) < 8 {
		return string(payload), "", nil
	}
	reader := bytes.NewReader(payload)
	var stdout strings.Builder
	var stderr strings.Builder
	for reader.Len() > 0 {
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			return string(payload), "", nil
		}
		size := binary.BigEndian.Uint32(header[4:])
		if size == 0 {
			continue
		}
		chunk := make([]byte, size)
		if _, err := io.ReadFull(reader, chunk); err != nil {
			return string(payload), "", nil
		}
		switch header[0] {
		case 1:
			stdout.Write(chunk)
		case 2:
			stderr.Write(chunk)
		default:
			stdout.Write(chunk)
		}
	}
	return stdout.String(), stderr.String(), nil
}

func dockerPathEscape(value string) string {
	return url.PathEscape(value)
}

func cpuFractionToNano(value string) int64 {
	value = firstNonEmpty(value, defaultCPUFraction)
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		parsed = 0.5
	}
	return int64(math.Round(parsed * 1_000_000_000))
}

type workspaceManifest struct {
	LintFiles []string `json:"lint_files"`
}

func writeWorkspace(root string, req RunRequest) error {
	for path, content := range req.Files {
		if err := writeWorkspaceFile(root, path, content); err != nil {
			return err
		}
	}

	lintFiles := append([]string(nil), req.EditableFiles...)
	if len(lintFiles) == 0 {
		for path := range req.Files {
			if strings.HasPrefix(path, "src/") && (strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx")) {
				lintFiles = append(lintFiles, path)
			}
		}
		sort.Strings(lintFiles)
	}

	manifestBytes, err := json.Marshal(workspaceManifest{LintFiles: lintFiles})
	if err != nil {
		return err
	}
	if err := writeWorkspaceFile(root, "skillroom-run.json", string(manifestBytes)); err != nil {
		return err
	}
	if err := writeWorkspaceFile(root, "vitest.config.mjs", vitestConfigFile); err != nil {
		return err
	}
	if err := writeWorkspaceFile(root, ".skillroom/setup.js", vitestSetupFile); err != nil {
		return err
	}
	if err := writeWorkspaceFile(root, "eslint.config.mjs", eslintConfigFile); err != nil {
		return err
	}
	if err := makeWorkspaceDirsWritable(root); err != nil {
		return err
	}
	return nil
}

func writeWorkspaceFile(root, name, content string) error {
	cleaned := filepath.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("invalid workspace path %q", name)
	}
	fullPath := filepath.Join(root, cleaned)
	if !strings.HasPrefix(fullPath, root) {
		return fmt.Errorf("invalid workspace path %q", name)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o777); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0o644)
}

func makeWorkspaceDirsWritable(root string) error {
	// Docker copies host ownership into the container on some platforms, while
	// the sandbox process runs without capability overrides. Explicit chmod keeps
	// report and cache directories writable regardless of the host umask.
	return filepath.Walk(root, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		return os.Chmod(currentPath, 0o777)
	})
}

func parseRunResult(stdout string) (RunResult, error) {
	var result RunResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		return RunResult{}, err
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

const vitestConfigFile = `import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "jsdom",
    globals: true,
    include: ["tests/**/*.spec.jsx", "tests/**/*.spec.js"],
    setupFiles: ["./.skillroom/setup.js"],
    restoreMocks: true,
    clearMocks: true
  }
});
`

const vitestSetupFile = `import "@testing-library/jest-dom/vitest";
import { act } from "react";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

const originalAdvanceTimersByTime = vi.advanceTimersByTime.bind(vi);
const originalAdvanceTimersByTimeAsync = vi.advanceTimersByTimeAsync.bind(vi);
const originalRunAllTimers = vi.runAllTimers.bind(vi);
const originalRunOnlyPendingTimers = vi.runOnlyPendingTimers.bind(vi);

vi.advanceTimersByTime = (milliseconds) => act(() => originalAdvanceTimersByTime(milliseconds));
vi.runAllTimers = () => act(() => originalRunAllTimers());
vi.runOnlyPendingTimers = () => act(() => originalRunOnlyPendingTimers());
vi.advanceTimersByTimeAsync = async (milliseconds) => {
  let result;
  await act(async () => {
    result = await originalAdvanceTimersByTimeAsync(milliseconds);
  });
  return result;
};

afterEach(() => {
  cleanup();
});
`

const eslintConfigFile = `export default [
  {
    files: ["src/**/*.{js,jsx}"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      parserOptions: {
        ecmaFeatures: {
          jsx: true
        }
      },
      globals: {
        window: "readonly",
        document: "readonly",
        console: "readonly",
        setTimeout: "readonly",
        clearTimeout: "readonly",
        Event: "readonly"
      }
    },
    rules: {
      "no-debugger": "error",
      "no-var": "error",
      "prefer-const": "warn",
      "no-console": ["warn", { allow: ["warn", "error"] }]
    }
  }
];
`
