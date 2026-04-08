package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stubEngine struct {
	result RunResult
	err    error
}

func (s stubEngine) Run(context.Context, RunRequest) (RunResult, error) {
	return s.result, s.err
}

func TestHandlerServesRunEndpoint(t *testing.T) {
	body, err := json.Marshal(RunRequest{
		Language: "jsx",
		Files: map[string]string{
			"src/App.jsx": "export default function App() { return null; }",
			"tests/visible.spec.jsx": `import { it, expect } from "vitest";
it("evaluates a real assertion", () => expect(Math.max(1, 2)).toBe(2));`,
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	NewHandler(stubEngine{result: RunResult{Passed: 1}}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var payload RunResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if payload.Result.Passed != 1 {
		t.Fatalf("expected stub result to be returned")
	}
}

func TestHandlerRejectsInvalidPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewBufferString(`{"files":`))
	recorder := httptest.NewRecorder()

	NewHandler(stubEngine{}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestWriteWorkspaceAddsRuntimeFiles(t *testing.T) {
	root := t.TempDir()
	req := RunRequest{
		Language: "jsx",
		Files: map[string]string{
			"src/App.jsx":            "export default function App() { return null; }",
			"tests/visible.spec.jsx": "export {}",
			"tests/hidden.spec.jsx":  "export {}",
		},
		EditableFiles: []string{"src/App.jsx"},
	}

	if err := writeWorkspace(root, req); err != nil {
		t.Fatalf("write workspace: %v", err)
	}

	for _, path := range []string{
		"src/App.jsx",
		"tests/visible.spec.jsx",
		"tests/hidden.spec.jsx",
		"skillroom-run.json",
		"vitest.config.mjs",
		"eslint.config.mjs",
		".skillroom/setup.js",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestParseRunResult(t *testing.T) {
	result, err := parseRunResult(`{"passed":2,"failed":1,"test_results":[{"name":"a","passed":true}]}`)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.Passed != 2 || result.Failed != 1 {
		t.Fatalf("unexpected parsed result: %+v", result)
	}
}

func TestHTTPClientReadyChecksRunnerReadiness(t *testing.T) {
	client := NewHTTPClient("http://runner", 2*time.Second)
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/readyz" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
			Header:     make(http.Header),
		}, nil
	})
	if err := client.Ready(context.Background()); err != nil {
		t.Fatalf("expected runner ready check to pass: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
