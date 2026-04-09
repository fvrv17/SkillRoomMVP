package backend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runsvc "github.com/fvrv17/mvp/internal/runner"
)

func TestRealRunnerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker-backed runner e2e in short mode")
	}

	dockerBinary, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker binary is not available")
	}
	if _, err := runCommand(dockerBinary, "info"); err != nil {
		t.Skip("docker daemon is not available")
	}

	sandboxImage := "skillroom-runner-e2e:" + runnerSandboxFingerprint(t)
	ensureRunnerSandboxImage(t, dockerBinary, sandboxImage)

	app := NewApp("e2e-secret", "e2e-issuer")
	app.SetChallengeRunner(runsvc.NewDockerEngine(runsvc.DockerConfig{
		DockerBinary:   dockerBinary,
		SandboxImage:   sandboxImage,
		DefaultTimeout: 90 * time.Second,
	}))
	router := app.Router()

	passing := runRealChallengeSubmission(t, router, RegisterRequest{
		Email:    "runner-pass@example.com",
		Username: "runner-pass",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	}, "react_debug_resize_cleanup", true)

	failing := runRealChallengeSubmission(t, router, RegisterRequest{
		Email:    "runner-fail@example.com",
		Username: "runner-fail",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	}, "react_debug_resize_cleanup", false)

	if passing.Execution.ExecutionCostMS <= 0 && passing.Execution.ExecutionTimeMS <= 0 {
		t.Fatalf("expected real execution cost to be reported: %+v", passing.Execution)
	}
	if passing.Execution.QualityPassed <= failing.Execution.QualityPassed {
		t.Fatalf("expected passing solution to satisfy more quality checks: pass=%d fail=%d", passing.Execution.QualityPassed, failing.Execution.QualityPassed)
	}
	if passing.Execution.QualityFailed >= failing.Execution.QualityFailed {
		t.Fatalf("expected failing solution to leave more quality checks unresolved: pass=%d fail=%d", passing.Execution.QualityFailed, failing.Execution.QualityFailed)
	}
	if passing.Evaluation.FinalScore <= failing.Evaluation.FinalScore {
		t.Fatalf("expected higher score for passing solution: pass=%.2f fail=%.2f", passing.Evaluation.FinalScore, failing.Evaluation.FinalScore)
	}
	if passing.Evaluation.FinalScore < 70 {
		t.Fatalf("expected passing solution to score well, got %.2f", passing.Evaluation.FinalScore)
	}
	if passing.Evaluation.QualityScore <= failing.Evaluation.QualityScore {
		t.Fatalf("expected higher quality score for passing solution: pass=%.2f fail=%.2f", passing.Evaluation.QualityScore, failing.Evaluation.QualityScore)
	}
	if passing.Evaluation.FinalScore-failing.Evaluation.FinalScore < 10 {
		t.Fatalf("expected a meaningful score gap between passing and failing solutions: pass=%.2f fail=%.2f", passing.Evaluation.FinalScore, failing.Evaluation.FinalScore)
	}
}

func runRealChallengeSubmission(t *testing.T, router http.Handler, req RegisterRequest, templateID string, solved bool) submissionResponse {
	t.Helper()

	token := registerAndLogin(t, router, req)
	view := startChallengeInstance(t, router, token, templateID)

	source := view.Instance.VisibleFiles[editablePathForTemplate(templateID)]
	if strings.TrimSpace(source) == "" {
		t.Fatalf("missing visible source for %s", templateID)
	}
	if solved {
		source = patchedResizeSolution(t, source)
	}

	for _, event := range []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 15},
		{EventType: "snapshot", OffsetSeconds: 38},
	} {
		resp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/telemetry", event, token)
		if resp.Code != http.StatusCreated {
			t.Fatalf("record telemetry: %d", resp.Code)
		}
	}

	submitResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/submissions", SubmitChallengeRequest{
		Language: "jsx",
		SourceFiles: map[string]string{
			editablePathForTemplate(templateID): source,
		},
	}, token)
	if submitResp.Code != http.StatusCreated {
		t.Fatalf("submit challenge: %d", submitResp.Code)
	}

	var result submissionResponse
	if err := json.NewDecoder(submitResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	return result
}

func patchedResizeSolution(t *testing.T, starter string) string {
	t.Helper()

	oldBlock := `  useEffect(() => {
    window.addEventListener("resize", () => {
      setWidth(window.innerWidth);
    });
  }, [width]);
`
	newBlock := `  useEffect(() => {
    function handleResize() {
      setWidth(window.innerWidth);
    }

    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
    };
  }, []);
`
	patched := strings.Replace(starter, oldBlock, newBlock, 1)
	if patched == starter {
		t.Fatalf("unable to patch starter code into a passing solution")
	}
	return patched
}

func ensureRunnerSandboxImage(t *testing.T, dockerBinary, image string) {
	t.Helper()

	if _, err := runCommand(dockerBinary, "image", "inspect", image); err == nil {
		return
	}

	rootDir := filepath.Clean(filepath.Join("..", ".."))
	build := exec.Command(dockerBinary, "build", "-f", "deploy/runner.Dockerfile", "-t", image, ".")
	build.Dir = rootDir
	output, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build runner sandbox image: %v\n%s", err, string(output))
	}
}

func runnerSandboxFingerprint(t *testing.T) string {
	t.Helper()

	rootDir := filepath.Clean(filepath.Join("..", ".."))
	hash := sha256.New()
	for _, path := range []string{
		filepath.Join("deploy", "runner.Dockerfile"),
		filepath.Join("deploy", "runner-runtime", "package.json"),
		filepath.Join("deploy", "runner-runtime", "run-evaluation.mjs"),
	} {
		content, err := os.ReadFile(filepath.Join(rootDir, path))
		if err != nil {
			t.Fatalf("read sandbox input %s: %v", path, err)
		}
		if _, err := hash.Write(content); err != nil {
			t.Fatalf("hash sandbox input %s: %v", path, err)
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil))[:12]
}

func runCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
