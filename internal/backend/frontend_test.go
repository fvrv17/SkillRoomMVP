package backend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendRootRedirectsToConfiguredFrontend(t *testing.T) {
	t.Setenv("FRONTEND_REDIRECT_URL", "http://localhost:3000")
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 from root, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "http://localhost:3000" {
		t.Fatalf("expected redirect to frontend, got %q", location)
	}
}

func TestFrontendRootServesNoticeWithoutConfiguredFrontend(t *testing.T) {
	t.Setenv("FRONTEND_REDIRECT_URL", "")
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from root, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "SkillRoom backend is running") {
		t.Fatalf("expected backend notice in root response")
	}
}

func TestLegacyFrontendAssetsAreNotServed(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from legacy asset route, got %d", recorder.Code)
	}
}
