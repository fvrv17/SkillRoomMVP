package backend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLivenessAndReadinessEndpoints(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
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

func TestMetricsEndpointExposesRequestCounters(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
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
