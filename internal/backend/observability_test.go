package backend

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runsvc "github.com/fvrv17/mvp/internal/runner"
)

type readyRunnerStub struct {
	err error
}

func (s readyRunnerStub) Run(context.Context, runsvc.RunRequest) (runsvc.RunResult, error) {
	return runsvc.RunResult{}, nil
}

func (s readyRunnerStub) Ready(context.Context) error {
	return s.err
}

func TestLivenessAndReadinessEndpoints(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	app.SetChallengeRunner(readyRunnerStub{})
	router := app.Router()

	for _, path := range []string{"/livez", "/readyz", "/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, recorder.Code)
		}
	}
}

func TestReadinessFailsWithoutRunner(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when runner is missing, got %d", recorder.Code)
	}
}

func TestReadinessFailsWhenRunnerIsUnavailable(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	app.SetChallengeRunner(readyRunnerStub{err: errors.New("runner offline")})
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when runner readiness fails, got %d", recorder.Code)
	}
}

func TestMetricsEndpointExposesRequestCounters(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	app.SetChallengeRunner(readyRunnerStub{})
	router := app.Router()

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for metrics, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "backend_requests_total") {
		t.Fatalf("expected backend_requests_total in metrics output")
	}
	if !strings.Contains(body, `route="/livez"`) {
		t.Fatalf("expected livez route metrics in output")
	}
}
