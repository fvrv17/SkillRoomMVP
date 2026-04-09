package runnerproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	versionPrefixPattern = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)
	allowedDockerRoutes  = []dockerRouteRule{
		{method: http.MethodGet, pattern: regexp.MustCompile(`^/_ping$`)},
		{method: http.MethodHead, pattern: regexp.MustCompile(`^/_ping$`)},
		{method: http.MethodGet, pattern: regexp.MustCompile(`^/version$`)},
		{method: http.MethodGet, pattern: regexp.MustCompile(`^/images/.+/json$`)},
		{method: http.MethodPost, pattern: regexp.MustCompile(`^/containers/create$`)},
		{method: http.MethodPut, pattern: regexp.MustCompile(`^/containers/.+/archive$`)},
		{method: http.MethodPost, pattern: regexp.MustCompile(`^/containers/.+/(start|wait)$`)},
		{method: http.MethodGet, pattern: regexp.MustCompile(`^/containers/.+/logs$`)},
		{method: http.MethodDelete, pattern: regexp.MustCompile(`^/containers/.+$`)},
	}
)

const (
	defaultAllowedWorkingDir  = "/workspace"
	defaultAllowedUser        = "1000:1000"
	defaultAllowedArchivePath = "/workspace"
	defaultMaxDockerBodyBytes = 64 * 1024
	defaultMaxNanoCPUs        = int64(1_000_000_000)
	defaultMaxMemoryBytes     = int64(512 * 1024 * 1024)
	defaultMaxPidsLimit       = int64(512)
	defaultSandboxCommand     = "node /opt/skillroom-runtime/run-evaluation.mjs"
	defaultReadinessCommand   = "test -f /opt/skillroom-runtime/run-evaluation.mjs && node --version >/dev/null"
)

type dockerRouteRule struct {
	method  string
	pattern *regexp.Regexp
}

type Config struct {
	SocketPath         string
	AllowedImage       string
	AllowedCommands    []string
	AllowedWorkingDir  string
	AllowedUser        string
	AllowedArchivePath string
	MaxBodyBytes       int64
	MaxNanoCPUs        int64
	MaxMemoryBytes     int64
	MaxPidsLimit       int64
}

type dockerContainerCreateRequest struct {
	Image      string                 `json:"Image"`
	WorkingDir string                 `json:"WorkingDir"`
	User       string                 `json:"User"`
	Entrypoint []string               `json:"Entrypoint"`
	Cmd        []string               `json:"Cmd"`
	HostConfig dockerContainerHostCfg `json:"HostConfig"`
}

type dockerContainerHostCfg struct {
	NetworkMode string         `json:"NetworkMode"`
	NanoCpus    int64          `json:"NanoCpus"`
	Memory      int64          `json:"Memory"`
	PidsLimit   int64          `json:"PidsLimit"`
	CapDrop     []string       `json:"CapDrop"`
	SecurityOpt []string       `json:"SecurityOpt"`
	Ulimits     []dockerUlimit `json:"Ulimits"`
}

type dockerUlimit struct {
	Name string `json:"Name"`
	Soft int64  `json:"Soft"`
	Hard int64  `json:"Hard"`
}

type Proxy struct {
	socketPath string
	config     Config
	client     *http.Client
	proxy      *httputil.ReverseProxy
}

func New(socketPath string) *Proxy {
	return NewWithConfig(Config{SocketPath: socketPath})
}

func NewWithConfig(config Config) *Proxy {
	config = normalizeConfig(config)
	socketPath := config.SocketPath
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	target, _ := url.Parse("http://docker")
	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	reverseProxy.Transport = transport
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "docker proxy upstream unavailable", http.StatusBadGateway)
	}

	return &Proxy{
		socketPath: socketPath,
		config:     config,
		client: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
		proxy: reverseProxy,
	}
}

func (p *Proxy) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "runner-docker-proxy"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := p.Ready(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "runner-docker-proxy"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !allowedDockerRequest(r.Method, r.URL.Path) {
			writeError(w, http.StatusForbidden, "docker route is not allowed")
			return
		}
		if err := p.validateDockerRequest(r); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		p.proxy.ServeHTTP(w, r)
	})
	return mux
}

func (p *Proxy) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("docker daemon is not ready")
	}
	return nil
}

func allowedDockerRequest(method, path string) bool {
	normalized := normalizeDockerPath(path)
	for _, route := range allowedDockerRoutes {
		if route.method == method && route.pattern.MatchString(normalized) {
			return true
		}
	}
	return false
}

func normalizeDockerPath(path string) string {
	if path == "" {
		return "/"
	}
	return versionPrefixPattern.ReplaceAllString(path, "")
}

func normalizeConfig(config Config) Config {
	if strings.TrimSpace(config.SocketPath) == "" {
		config.SocketPath = "/var/run/docker.sock"
	}
	if strings.TrimSpace(config.AllowedWorkingDir) == "" {
		config.AllowedWorkingDir = defaultAllowedWorkingDir
	}
	if strings.TrimSpace(config.AllowedUser) == "" {
		config.AllowedUser = defaultAllowedUser
	}
	if strings.TrimSpace(config.AllowedArchivePath) == "" {
		config.AllowedArchivePath = defaultAllowedArchivePath
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = defaultMaxDockerBodyBytes
	}
	if config.MaxNanoCPUs <= 0 {
		config.MaxNanoCPUs = defaultMaxNanoCPUs
	}
	if config.MaxMemoryBytes <= 0 {
		config.MaxMemoryBytes = defaultMaxMemoryBytes
	}
	if config.MaxPidsLimit <= 0 {
		config.MaxPidsLimit = defaultMaxPidsLimit
	}
	if len(config.AllowedCommands) == 0 {
		config.AllowedCommands = []string{defaultSandboxCommand}
	}
	if !containsString(config.AllowedCommands, defaultReadinessCommand) {
		config.AllowedCommands = append(config.AllowedCommands, defaultReadinessCommand)
	}
	return config
}

func (p *Proxy) validateDockerRequest(r *http.Request) error {
	if r == nil {
		return errors.New("request is required")
	}
	normalized := normalizeDockerPath(r.URL.Path)
	switch {
	case r.Method == http.MethodPost && normalized == "/containers/create":
		return p.validateContainerCreate(r)
	case r.Method == http.MethodGet && strings.HasPrefix(normalized, "/images/") && strings.HasSuffix(normalized, "/json"):
		return p.validateImageInspect(normalized)
	case r.Method == http.MethodPut && strings.HasSuffix(normalized, "/archive"):
		return p.validateArchivePut(r)
	case r.Method == http.MethodPost && strings.HasSuffix(normalized, "/wait"):
		return p.validateContainerWait(r)
	case r.Method == http.MethodGet && strings.HasSuffix(normalized, "/logs"):
		return p.validateContainerLogs(r)
	case r.Method == http.MethodDelete && strings.HasPrefix(normalized, "/containers/"):
		return p.validateContainerDelete(r)
	default:
		return nil
	}
}

func (p *Proxy) validateImageInspect(normalizedPath string) error {
	if strings.TrimSpace(p.config.AllowedImage) == "" {
		return nil
	}
	expected := "/images/" + url.PathEscape(p.config.AllowedImage) + "/json"
	if normalizedPath != expected {
		return errors.New("docker image inspect target is not allowed")
	}
	return nil
}

func (p *Proxy) validateArchivePut(r *http.Request) error {
	if got := strings.TrimSpace(r.URL.Query().Get("path")); got != p.config.AllowedArchivePath {
		return fmt.Errorf("docker archive path %q is not allowed", got)
	}
	return nil
}

func (p *Proxy) validateContainerWait(r *http.Request) error {
	condition := strings.TrimSpace(r.URL.Query().Get("condition"))
	if condition != "" && condition != "not-running" {
		return fmt.Errorf("docker wait condition %q is not allowed", condition)
	}
	return nil
}

func (p *Proxy) validateContainerLogs(r *http.Request) error {
	query := r.URL.Query()
	if query.Get("stdout") != "1" || query.Get("stderr") != "1" {
		return errors.New("docker logs must request stdout and stderr")
	}
	if follow := strings.TrimSpace(query.Get("follow")); follow != "" && follow != "0" {
		return errors.New("docker log streaming is not allowed")
	}
	return nil
}

func (p *Proxy) validateContainerDelete(r *http.Request) error {
	force := strings.TrimSpace(r.URL.Query().Get("force"))
	if force != "" && force != "1" {
		return errors.New("docker delete force value is not allowed")
	}
	return nil
}

func (p *Proxy) validateContainerCreate(r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, p.config.MaxBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read docker create body: %w", err)
	}
	if int64(len(body)) > p.config.MaxBodyBytes {
		return errors.New("docker create body exceeds maximum size")
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload dockerContainerCreateRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("invalid docker create body: %w", err)
	}
	if decoder.More() {
		return errors.New("invalid docker create body: unexpected trailing data")
	}
	if strings.TrimSpace(payload.Image) == "" {
		return errors.New("docker create image is required")
	}
	if strings.TrimSpace(p.config.AllowedImage) != "" && payload.Image != p.config.AllowedImage {
		return errors.New("docker create image is not allowed")
	}
	if payload.WorkingDir != p.config.AllowedWorkingDir {
		return errors.New("docker create working directory is not allowed")
	}
	if payload.User != p.config.AllowedUser {
		return errors.New("docker create user is not allowed")
	}
	if len(payload.Entrypoint) != 1 || payload.Entrypoint[0] != "sh" {
		return errors.New("docker create entrypoint is not allowed")
	}
	if len(payload.Cmd) != 2 || payload.Cmd[0] != "-lc" || !containsString(p.config.AllowedCommands, payload.Cmd[1]) {
		return errors.New("docker create command is not allowed")
	}
	if payload.HostConfig.NetworkMode != "none" {
		return errors.New("docker create network mode is not allowed")
	}
	if payload.HostConfig.NanoCpus <= 0 || payload.HostConfig.NanoCpus > p.config.MaxNanoCPUs {
		return errors.New("docker create cpu limit is not allowed")
	}
	if payload.HostConfig.Memory <= 0 || payload.HostConfig.Memory > p.config.MaxMemoryBytes {
		return errors.New("docker create memory limit is not allowed")
	}
	if payload.HostConfig.PidsLimit <= 0 || payload.HostConfig.PidsLimit > p.config.MaxPidsLimit {
		return errors.New("docker create pids limit is not allowed")
	}
	if len(payload.HostConfig.CapDrop) != 1 || payload.HostConfig.CapDrop[0] != "ALL" {
		return errors.New("docker create capabilities are not allowed")
	}
	if len(payload.HostConfig.SecurityOpt) != 1 || payload.HostConfig.SecurityOpt[0] != "no-new-privileges" {
		return errors.New("docker create security options are not allowed")
	}
	if len(payload.HostConfig.Ulimits) != 1 {
		return errors.New("docker create ulimits are not allowed")
	}
	ulimit := payload.HostConfig.Ulimits[0]
	if ulimit.Name != "nproc" || ulimit.Soft <= 0 || ulimit.Hard <= 0 || ulimit.Soft > p.config.MaxPidsLimit || ulimit.Hard > p.config.MaxPidsLimit {
		return errors.New("docker create nproc ulimit is not allowed")
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
