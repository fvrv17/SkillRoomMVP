package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
	SandboxImage   string
	SandboxCommand string
	DefaultCPU     string
	DefaultMemory  int
	DefaultTimeout time.Duration
}

type DockerEngine struct {
	config DockerConfig
}

func NewDockerEngine(config DockerConfig) DockerEngine {
	if strings.TrimSpace(config.DockerBinary) == "" {
		config.DockerBinary = "docker"
	}
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
	return DockerEngine{config: config}
}

func (e DockerEngine) Run(ctx context.Context, req RunRequest) (RunResult, error) {
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
	createArgs := []string{
		"create",
		"--name", containerName,
		"--entrypoint", "sh",
		"--network", "none",
		"--cpus", firstNonEmpty(req.CPUFraction, e.config.DefaultCPU),
		"--memory", fmt.Sprintf("%dm", maxInt(req.MemoryMB, e.config.DefaultMemory)),
		"--pids-limit", "512",
		"--ulimit", "nproc=512:512",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--workdir", "/workspace",
		e.config.SandboxImage,
		"-lc", e.config.SandboxCommand,
	}
	if _, _, err := e.runDocker(runCtx, createArgs...); err != nil {
		return RunResult{}, err
	}
	defer func() {
		_, _, _ = e.runDocker(context.Background(), "rm", "-f", containerName)
	}()

	if _, _, err := e.runDocker(runCtx, "cp", workspaceDir+"/.", containerName+":/workspace"); err != nil {
		return RunResult{}, err
	}

	stdout, stderr, err := e.runDocker(runCtx, "start", "-a", containerName)
	if runCtx.Err() == context.DeadlineExceeded {
		return RunResult{Errors: []string{"sandbox execution timed out"}}, errors.New("sandbox execution timed out")
	}
	if err != nil {
		return RunResult{}, fmt.Errorf("sandbox execution failed: %w: %s", err, strings.TrimSpace(stderr))
	}

	result, err := parseRunResult(stdout)
	if err != nil {
		return RunResult{}, fmt.Errorf("parse sandbox output: %w: %s", err, strings.TrimSpace(stderr))
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		result.Errors = append(result.Errors, trimmed)
	}
	return result, nil
}

func (e DockerEngine) runDocker(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, e.config.DockerBinary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
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
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0o644)
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
