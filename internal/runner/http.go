package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func NewHandler(engine Engine) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "runner"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if engine == nil {
			writeError(w, http.StatusServiceUnavailable, "runner engine is not configured")
			return
		}
		type readinessChecker interface {
			Ready(context.Context) error
		}
		if checker, ok := engine.(readinessChecker); ok {
			if err := checker.Ready(r.Context()); err != nil {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "runner"})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "runner"})
	})
	mux.HandleFunc("/v1/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if engine == nil {
			writeError(w, http.StatusServiceUnavailable, "runner engine is not configured")
			return
		}
		var req RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		result, err := engine.Run(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, RunResponse{
			Status: "ok",
			Result: result,
		})
	})
	return requestLogger(mux)
}

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *HTTPClient) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if c == nil || c.baseURL == "" {
		return RunResult{}, errors.New("runner base url is required")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return RunResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/run", bytes.NewReader(body))
	if err != nil {
		return RunResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return RunResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var payload map[string]any
		if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr == nil {
			if message, ok := payload["error"].(string); ok && message != "" {
				return RunResult{}, errors.New(message)
			}
		}
		return RunResult{}, errors.New(resp.Status)
	}

	var payload RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return RunResult{}, err
	}
	return payload.Result, nil
}

func (c *HTTPClient) Ready(ctx context.Context) error {
	if c == nil || c.baseURL == "" {
		return errors.New("runner base url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func requestLogger(next http.Handler) http.Handler {
	return next
}
