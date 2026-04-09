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

func TestMetricsEndpointExposesBusinessCounters(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	refreshResp := performRequestWithOptions(t, router, requestOptions{
		Method: http.MethodPost,
		Path:   "/v1/auth/refresh",
		Cookies: []*http.Cookie{
			{Name: refreshTokenCookieName, Value: "rfr_invalid"},
		},
	})
	if refreshResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid refresh to return 401, got %d", refreshResp.Code)
	}

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "metrics-hr@example.com",
		Username: "metrics-hr",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})
	firstCandidate := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "metrics-candidate-a@example.com",
		Username: "metrics-candidate-a",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})
	secondCandidate := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "metrics-candidate-b@example.com",
		Username: "metrics-candidate-b",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	unlockResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+firstCandidate.User.ID+"/unlock", nil, hrAuth.AccessToken)
	if unlockResp.Code != http.StatusCreated {
		t.Fatalf("expected unlock to succeed, got %d", unlockResp.Code)
	}
	inviteDeniedResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+secondCandidate.User.ID+"/invite", nil, hrAuth.AccessToken)
	if inviteDeniedResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invite before unlock to fail, got %d", inviteDeniedResp.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	body := recorder.Body.String()

	for _, event := range []string{"refresh_failed", "candidate_unlock_succeeded", "candidate_invite_denied"} {
		if !strings.Contains(body, `event="`+event+`"`) {
			t.Fatalf("expected %s counter in metrics output", event)
		}
	}
}

func TestMetricsCaptureReadinessAndRunnerFailures(t *testing.T) {
	noRunnerApp := NewApp("test-secret", "test-issuer")
	noRunnerRouter := noRunnerApp.Router()

	readinessResp := httptest.NewRecorder()
	noRunnerRouter.ServeHTTP(readinessResp, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readinessResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readiness to fail without runner, got %d", readinessResp.Code)
	}
	readinessMetrics := httptest.NewRecorder()
	noRunnerRouter.ServeHTTP(readinessMetrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(readinessMetrics.Body.String(), `event="readiness_failed"`) {
		t.Fatalf("expected readiness_failed counter in metrics output")
	}

	unavailableApp := NewApp("test-secret", "test-issuer")
	unavailableApp.SetChallengeRunner(stubRunner{err: errors.New("runner offline")})
	unavailableRouter := unavailableApp.Router()
	userToken := registerAndLogin(t, unavailableRouter, RegisterRequest{
		Email:    "runner-unavailable@example.com",
		Username: "runner-unavailable",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})
	instance := startChallengeInstance(t, unavailableRouter, userToken, "react_feature_search")
	runResp := performJSON(t, unavailableRouter, http.MethodPost, "/v1/challenges/instances/"+instance.Instance.ID+"/runs", SubmitChallengeRequest{
		Language:    "jsx",
		RawCodeText: searchSolutionCode(),
	}, userToken)
	if runResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected runner unavailable status, got %d", runResp.Code)
	}
	unavailableMetrics := httptest.NewRecorder()
	unavailableRouter.ServeHTTP(unavailableMetrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(unavailableMetrics.Body.String(), `event="runner_unavailable"`) {
		t.Fatalf("expected runner_unavailable counter in metrics output")
	}

	timeoutApp := NewApp("test-secret", "test-issuer")
	timeoutApp.SetChallengeRunner(stubRunner{err: context.DeadlineExceeded})
	timeoutRouter := timeoutApp.Router()
	timeoutToken := registerAndLogin(t, timeoutRouter, RegisterRequest{
		Email:    "runner-timeout@example.com",
		Username: "runner-timeout",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})
	timeoutInstance := startChallengeInstance(t, timeoutRouter, timeoutToken, "react_feature_search")
	timeoutResp := performJSON(t, timeoutRouter, http.MethodPost, "/v1/challenges/instances/"+timeoutInstance.Instance.ID+"/runs", SubmitChallengeRequest{
		Language:    "jsx",
		RawCodeText: searchSolutionCode(),
	}, timeoutToken)
	if timeoutResp.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected runner timeout status, got %d", timeoutResp.Code)
	}
	timeoutMetrics := httptest.NewRecorder()
	timeoutRouter.ServeHTTP(timeoutMetrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(timeoutMetrics.Body.String(), `event="runner_timeout"`) {
		t.Fatalf("expected runner_timeout counter in metrics output")
	}
}
