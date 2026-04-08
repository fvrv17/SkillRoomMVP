package backend

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHRIsForbiddenFromDeveloperWorkspaceEndpoints(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-role@example.com",
		Username: "hr-role",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})

	paths := []string{
		"/v1/profile",
		"/v1/skills",
		"/v1/room",
		"/v1/challenges/templates",
		"/v1/rankings/global",
	}
	for _, path := range paths {
		resp := performJSON(t, router, http.MethodGet, path, nil, hrAuth.AccessToken)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("expected %s to be forbidden for hr, got %d", path, resp.Code)
		}
	}
}

func TestDeveloperIsForbiddenFromHRWorkspaceEndpoints(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	userAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "dev-role@example.com",
		Username: "dev-role",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	resp := performJSON(t, router, http.MethodGet, "/v1/hr/candidates", nil, userAuth.AccessToken)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected hr candidates endpoint to be forbidden for developers, got %d", resp.Code)
	}
}

func TestDeveloperProfileSupportsLinkedInOnlyForLinkedInHosts(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	userAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "linkedin@example.com",
		Username: "linkedin-user",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	updateResp := performJSON(t, router, http.MethodPatch, "/v1/profile", UpdateProfileRequest{
		LinkedInURL: "linkedin.com/in/linkedin-user",
	}, userAuth.AccessToken)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected linkedin profile update to succeed, got %d", updateResp.Code)
	}
	var updated UserProfile
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated profile: %v", err)
	}
	if updated.LinkedInURL != "https://linkedin.com/in/linkedin-user" {
		t.Fatalf("expected normalized linkedin url, got %q", updated.LinkedInURL)
	}

	invalidResp := performJSON(t, router, http.MethodPatch, "/v1/profile", UpdateProfileRequest{
		LinkedInURL: "https://github.com/linkedin-user",
	}, userAuth.AccessToken)
	if invalidResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid linkedin host to fail, got %d", invalidResp.Code)
	}
}
