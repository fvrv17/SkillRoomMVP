package runnerproxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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

func TestValidateDockerRequestAllowsExpectedContainerCreate(t *testing.T) {
	proxy := NewWithConfig(Config{
		AllowedImage:    "deploy-runner:latest",
		AllowedCommands: []string{"node /opt/skillroom-runtime/run-evaluation.mjs"},
	})
	body := []byte(`{
		"Image":"deploy-runner:latest",
		"WorkingDir":"/workspace",
		"User":"1000:1000",
		"Entrypoint":["sh"],
		"Cmd":["-lc","node /opt/skillroom-runtime/run-evaluation.mjs"],
		"HostConfig":{
			"NetworkMode":"none",
			"NanoCpus":500000000,
			"Memory":268435456,
			"PidsLimit":512,
			"CapDrop":["ALL"],
			"SecurityOpt":["no-new-privileges"],
			"Ulimits":[{"Name":"nproc","Soft":512,"Hard":512}]
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1.45/containers/create", bytes.NewReader(body))

	if err := proxy.validateDockerRequest(req); err != nil {
		t.Fatalf("expected create body to be allowed, got %v", err)
	}
}

func TestValidateDockerRequestRejectsUnexpectedContainerCreateBody(t *testing.T) {
	proxy := NewWithConfig(Config{
		AllowedImage:    "deploy-runner:latest",
		AllowedCommands: []string{"node /opt/skillroom-runtime/run-evaluation.mjs"},
	})
	tests := []struct {
		name string
		body string
	}{
		{
			name: "wrong image",
			body: `{"Image":"evil:latest","WorkingDir":"/workspace","User":"1000:1000","Entrypoint":["sh"],"Cmd":["-lc","node /opt/skillroom-runtime/run-evaluation.mjs"],"HostConfig":{"NetworkMode":"none","NanoCpus":500000000,"Memory":268435456,"PidsLimit":512,"CapDrop":["ALL"],"SecurityOpt":["no-new-privileges"],"Ulimits":[{"Name":"nproc","Soft":512,"Hard":512}]}}`,
		},
		{
			name: "privileged via unknown field",
			body: `{"Image":"deploy-runner:latest","WorkingDir":"/workspace","User":"1000:1000","Entrypoint":["sh"],"Cmd":["-lc","node /opt/skillroom-runtime/run-evaluation.mjs"],"HostConfig":{"NetworkMode":"none","NanoCpus":500000000,"Memory":268435456,"PidsLimit":512,"CapDrop":["ALL"],"SecurityOpt":["no-new-privileges"],"Ulimits":[{"Name":"nproc","Soft":512,"Hard":512}],"Privileged":true}}`,
		},
		{
			name: "unexpected command",
			body: `{"Image":"deploy-runner:latest","WorkingDir":"/workspace","User":"1000:1000","Entrypoint":["sh"],"Cmd":["-lc","apk add curl"],"HostConfig":{"NetworkMode":"none","NanoCpus":500000000,"Memory":268435456,"PidsLimit":512,"CapDrop":["ALL"],"SecurityOpt":["no-new-privileges"],"Ulimits":[{"Name":"nproc","Soft":512,"Hard":512}]}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1.45/containers/create", bytes.NewBufferString(tc.body))
			if err := proxy.validateDockerRequest(req); err == nil {
				t.Fatalf("expected create body to be rejected")
			}
		})
	}
}

func TestValidateDockerRequestRejectsUnexpectedArchiveAndLogQueries(t *testing.T) {
	proxy := NewWithConfig(Config{})

	archiveReq := httptest.NewRequest(http.MethodPut, "/v1.45/containers/test/archive?path=/etc", nil)
	if err := proxy.validateDockerRequest(archiveReq); err == nil {
		t.Fatalf("expected archive path to be rejected")
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1.45/containers/test/logs?stdout=1&stderr=0", nil)
	if err := proxy.validateDockerRequest(logReq); err == nil {
		t.Fatalf("expected invalid log query to be rejected")
	}
}
