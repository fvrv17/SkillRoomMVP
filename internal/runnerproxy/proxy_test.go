package runnerproxy

import (
	"net/http"
	"testing"
)

func TestAllowedDockerRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		allow  bool
	}{
		{name: "ping", method: http.MethodGet, path: "/_ping", allow: true},
		{name: "versioned image inspect", method: http.MethodGet, path: "/v1.45/images/deploy-runner:latest/json", allow: true},
		{name: "container create", method: http.MethodPost, path: "/v1.45/containers/create", allow: true},
		{name: "container archive put", method: http.MethodPut, path: "/v1.45/containers/abc/archive", allow: true},
		{name: "container start", method: http.MethodPost, path: "/v1.45/containers/abc/start", allow: true},
		{name: "container wait", method: http.MethodPost, path: "/v1.45/containers/abc/wait", allow: true},
		{name: "container logs", method: http.MethodGet, path: "/v1.45/containers/abc/logs", allow: true},
		{name: "container delete", method: http.MethodDelete, path: "/v1.45/containers/abc", allow: true},
		{name: "image build denied", method: http.MethodPost, path: "/v1.45/build", allow: false},
		{name: "network create denied", method: http.MethodPost, path: "/v1.45/networks/create", allow: false},
		{name: "exec denied", method: http.MethodPost, path: "/v1.45/containers/abc/exec", allow: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if allowedDockerRequest(tc.method, tc.path) != tc.allow {
				t.Fatalf("expected allow=%v for %s %s", tc.allow, tc.method, tc.path)
			}
		})
	}
}

func TestNormalizeDockerPathStripsVersionPrefix(t *testing.T) {
	if got := normalizeDockerPath("/v1.44/containers/create"); got != "/containers/create" {
		t.Fatalf("unexpected normalized path: %s", got)
	}
}
