package backend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendRootServesShell(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from root, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Signal Room") {
		t.Fatalf("expected frontend shell in root response")
	}
}

func TestFrontendJavaScriptServed(t *testing.T) {
	app := NewApp("test-secret", "test-issuer")
	router := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from app.js, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "hydrateWorkspace") {
		t.Fatalf("expected frontend script payload")
	}
}
