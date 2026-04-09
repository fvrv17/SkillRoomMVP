package runnerproxy

import (
	"context"
	"encoding/json"
	"errors"
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

type dockerRouteRule struct {
	method  string
	pattern *regexp.Regexp
}

type Proxy struct {
	socketPath string
	client     *http.Client
	proxy      *httputil.ReverseProxy
}

func New(socketPath string) *Proxy {
	if strings.TrimSpace(socketPath) == "" {
		socketPath = "/var/run/docker.sock"
	}
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
