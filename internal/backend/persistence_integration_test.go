package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	runsvc "github.com/fvrv17/mvp/internal/runner"
)

func TestPersistentAppRestoresStateAcrossRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres-backed integration test in short mode")
	}

	dockerBinary := requireDockerDaemon(t)
	postgresContainer := startDockerContainer(t, dockerBinary,
		"postgres:16-alpine",
		"-e", "POSTGRES_DB=mvp",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-P",
	)
	postgresPort := dockerMappedPort(t, dockerBinary, postgresContainer, "5432/tcp")
	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/mvp?sslmode=disable", postgresPort)
	waitFor(t, 30*time.Second, func() error {
		store, err := OpenSQLStore(context.Background(), dsn)
		if err != nil {
			return err
		}
		return store.Close()
	})

	app := newPersistentTestApp(t, dsn)
	router := app.Router()

	auth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "persistent@example.com",
		Username: "persistent",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})
	result := submitSolvedChallenge(t, router, auth.AccessToken, "react_feature_search", searchSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 12},
		{EventType: "snapshot", OffsetSeconds: 37},
	})
	equipResp := performJSON(t, router, http.MethodPost, "/v1/dev/cosmetics/equip", EquipCosmeticRequest{
		CosmeticCode: "window_sunset_default",
	}, auth.AccessToken)
	if equipResp.Code != http.StatusOK {
		t.Fatalf("equip cosmetic before restart: %d", equipResp.Code)
	}
	instanceID := result.Submission.ChallengeInstanceID

	if err := app.Close(); err != nil {
		t.Fatalf("close first persistent app: %v", err)
	}

	reloaded := newPersistentTestApp(t, dsn)
	reloadedRouter := reloaded.Router()

	refreshResp := performJSON(t, reloadedRouter, http.MethodPost, "/v1/auth/refresh", RefreshRequest{
		RefreshToken: auth.RefreshToken,
	}, "")
	if refreshResp.Code != http.StatusOK {
		t.Fatalf("refresh after restart: %d", refreshResp.Code)
	}
	var refreshed AuthResponse
	if err := json.NewDecoder(refreshResp.Body).Decode(&refreshed); err != nil {
		t.Fatalf("decode refreshed auth: %v", err)
	}

	profileResp := performJSON(t, reloadedRouter, http.MethodGet, "/v1/profile", nil, refreshed.AccessToken)
	if profileResp.Code != http.StatusOK {
		t.Fatalf("get profile after restart: %d", profileResp.Code)
	}
	var profile UserProfile
	if err := json.NewDecoder(profileResp.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.CompletedChallenges != 1 {
		t.Fatalf("expected completed challenge count to persist, got %d", profile.CompletedChallenges)
	}
	if profile.CurrentSkillScore <= 0 {
		t.Fatalf("expected persisted skill score to be positive, got %.2f", profile.CurrentSkillScore)
	}

	monetizationResp := performJSON(t, reloadedRouter, http.MethodGet, "/v1/monetization/summary", nil, refreshed.AccessToken)
	if monetizationResp.Code != http.StatusOK {
		t.Fatalf("get monetization summary after restart: %d", monetizationResp.Code)
	}
	var monetization MonetizationSummary
	if err := json.NewDecoder(monetizationResp.Body).Decode(&monetization); err != nil {
		t.Fatalf("decode monetization summary: %v", err)
	}
	if monetization.Plan.Code != planCodeDeveloperFree {
		t.Fatalf("expected persisted developer free plan, got %s", monetization.Plan.Code)
	}

	roomResp := performJSON(t, reloadedRouter, http.MethodGet, "/v1/room", nil, refreshed.AccessToken)
	if roomResp.Code != http.StatusOK {
		t.Fatalf("get room after restart: %d", roomResp.Code)
	}
	var roomPayload struct {
		Items []UserRoomItem `json:"items"`
	}
	if err := json.NewDecoder(roomResp.Body).Decode(&roomPayload); err != nil {
		t.Fatalf("decode room payload: %v", err)
	}
	if len(roomPayload.Items) != 6 {
		t.Fatalf("expected persisted room items, got %d", len(roomPayload.Items))
	}

	skillsResp := performJSON(t, reloadedRouter, http.MethodGet, "/v1/skills", nil, refreshed.AccessToken)
	if skillsResp.Code != http.StatusOK {
		t.Fatalf("get skills after restart: %d", skillsResp.Code)
	}
	var skillsPayload struct {
		Skills []UserSkill `json:"skills"`
	}
	if err := json.NewDecoder(skillsResp.Body).Decode(&skillsPayload); err != nil {
		t.Fatalf("decode skills payload: %v", err)
	}
	if len(skillsPayload.Skills) == 0 {
		t.Fatal("expected persisted skill rows after restart")
	}

	inventoryResp := performJSON(t, reloadedRouter, http.MethodGet, "/v1/dev/cosmetics/inventory", nil, refreshed.AccessToken)
	if inventoryResp.Code != http.StatusOK {
		t.Fatalf("get cosmetic inventory after restart: %d", inventoryResp.Code)
	}
	var inventory CosmeticInventoryResponse
	if err := json.NewDecoder(inventoryResp.Body).Decode(&inventory); err != nil {
		t.Fatalf("decode cosmetic inventory after restart: %v", err)
	}
	if len(inventory.Owned) != 6 || len(inventory.Equipped) != 3 {
		t.Fatalf("expected persisted default cosmetics after restart, owned=%d equipped=%d", len(inventory.Owned), len(inventory.Equipped))
	}
	if equippedCosmeticForSlot(inventory.Equipped, "window_scene") != "window_sunset_default" {
		t.Fatalf("expected sunset window to persist across restart, got %s", equippedCosmeticForSlot(inventory.Equipped, "window_scene"))
	}

	instanceResp := performJSON(t, reloadedRouter, http.MethodGet, "/v1/challenges/instances/"+instanceID, nil, refreshed.AccessToken)
	if instanceResp.Code != http.StatusOK {
		t.Fatalf("get persisted instance after restart: %d", instanceResp.Code)
	}
	var instanceView ChallengeInstanceView
	if err := json.NewDecoder(instanceResp.Body).Decode(&instanceView); err != nil {
		t.Fatalf("decode persisted instance: %v", err)
	}
	if instanceView.Instance.ID != instanceID {
		t.Fatalf("expected persisted instance id %s, got %s", instanceID, instanceView.Instance.ID)
	}
	if instanceView.TemplateID != "react_feature_search" {
		t.Fatalf("expected persisted template react_feature_search, got %s", instanceView.TemplateID)
	}
}

func TestRedisOpsStoreExercisesCacheAndRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Redis-backed integration test in short mode")
	}

	dockerBinary := requireDockerDaemon(t)
	redisContainer := startDockerContainer(t, dockerBinary, "redis:7-alpine", "-P", "--", "--save", "", "--appendonly", "no")
	redisAddr := net.JoinHostPort("127.0.0.1", dockerMappedPort(t, dockerBinary, redisContainer, "6379/tcp"))
	store := NewRedisOpsStore(redisAddr, "", 0)

	waitFor(t, 20*time.Second, func() error {
		return store.Ping(context.Background())
	})

	ctx := context.Background()
	first, err := store.Allow(ctx, "integration:rate", 2, time.Minute)
	if err != nil {
		t.Fatalf("allow first: %v", err)
	}
	second, err := store.Allow(ctx, "integration:rate", 2, time.Minute)
	if err != nil {
		t.Fatalf("allow second: %v", err)
	}
	third, err := store.Allow(ctx, "integration:rate", 2, time.Minute)
	if err != nil {
		t.Fatalf("allow third: %v", err)
	}
	if !first.Allowed || !second.Allowed || third.Allowed {
		t.Fatalf("unexpected redis rate-limit decisions: first=%+v second=%+v third=%+v", first, second, third)
	}

	if err := store.Set(ctx, "integration:cache", []byte(`{"score":91}`), time.Minute); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	value, ok, err := store.Get(ctx, "integration:cache")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if !ok || string(value) != `{"score":91}` {
		t.Fatalf("unexpected cache state: ok=%t value=%s", ok, string(value))
	}
	if err := store.Delete(ctx, "integration:cache"); err != nil {
		t.Fatalf("delete cache: %v", err)
	}
	if _, ok, err := store.Get(ctx, "integration:cache"); err != nil || ok {
		t.Fatalf("expected cache entry to be deleted: ok=%t err=%v", ok, err)
	}
}

func TestRedisRateLimitIsSharedAcrossAppInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Redis-backed app integration test in short mode")
	}

	dockerBinary := requireDockerDaemon(t)
	redisContainer := startDockerContainer(t, dockerBinary, "redis:7-alpine", "-P", "--", "--save", "", "--appendonly", "no")
	redisAddr := net.JoinHostPort("127.0.0.1", dockerMappedPort(t, dockerBinary, redisContainer, "6379/tcp"))

	storeOne := NewRedisOpsStore(redisAddr, "", 0)
	waitFor(t, 20*time.Second, func() error {
		return storeOne.Ping(context.Background())
	})

	appOne := NewApp("redis-secret", "redis-issuer")
	appOne.SetOpsStore(storeOne)
	routerOne := appOne.Router()

	for i := 0; i < 10; i++ {
		recorder := performRegisterFromIP(t, routerOne, fmt.Sprintf("shared-%02d@example.com", i), fmt.Sprintf("shared-%02d", i), "203.0.113.25:12345")
		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected register %d to succeed, got %d", i+1, recorder.Code)
		}
	}
	blocked := performRegisterFromIP(t, routerOne, "shared-blocked@example.com", "shared-blocked", "203.0.113.25:12345")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected first app to hit redis-backed rate limit, got %d", blocked.Code)
	}

	appTwo := NewApp("redis-secret", "redis-issuer")
	appTwo.SetOpsStore(NewRedisOpsStore(redisAddr, "", 0))
	routerTwo := appTwo.Router()

	sharedBlocked := performRegisterFromIP(t, routerTwo, "shared-blocked-2@example.com", "shared-blocked-2", "203.0.113.25:12345")
	if sharedBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second app instance to observe shared redis rate limit, got %d", sharedBlocked.Code)
	}
}

func newPersistentTestApp(t *testing.T, dsn string) *App {
	t.Helper()

	app, err := NewPersistentApp(context.Background(), "persistent-secret", "persistent-issuer", dsn)
	if err != nil {
		t.Fatalf("create persistent app: %v", err)
	}
	t.Cleanup(func() {
		_ = app.Close()
	})
	app.SetChallengeRunner(stubRunner{
		result: runsvcRunResult(),
	})
	return app
}

func registerAndCaptureAuth(t *testing.T, router http.Handler, req RegisterRequest) AuthResponse {
	t.Helper()

	resp := performJSON(t, router, http.MethodPost, "/v1/auth/register", req, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("register failed: %d", resp.Code)
	}
	var auth AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	return auth
}

func performRegisterFromIP(t *testing.T, router http.Handler, email, username, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	return performJSONWithRemoteAddr(t, router, http.MethodPost, "/v1/auth/register", RegisterRequest{
		Email:    email,
		Username: username,
		Password: "password123",
		Country:  "US",
	}, "", remoteAddr)
}

func performJSONWithRemoteAddr(t *testing.T, router http.Handler, method, path string, body any, token, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	var payload strings.Builder
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		payload.Write(encoded)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(payload.String()))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func requireDockerDaemon(t *testing.T) string {
	t.Helper()

	dockerBinary, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker binary is not available")
	}
	if _, err := runDockerCommand(dockerBinary, "info"); err != nil {
		t.Skip("docker daemon is not available")
	}
	return dockerBinary
}

func startDockerContainer(t *testing.T, dockerBinary string, image string, args ...string) string {
	t.Helper()

	containerName := fmt.Sprintf("skillroom-it-%d", time.Now().UTC().UnixNano())
	commandArgs := []string{"run", "-d", "--rm", "--name", containerName}
	dockerArgs := args
	containerArgs := []string{}
	for index, arg := range args {
		if arg == "--" {
			dockerArgs = args[:index]
			containerArgs = args[index+1:]
			break
		}
	}
	commandArgs = append(commandArgs, dockerArgs...)
	commandArgs = append(commandArgs, image)
	commandArgs = append(commandArgs, containerArgs...)

	output, err := runDockerCommand(dockerBinary, commandArgs...)
	if err != nil {
		t.Fatalf("start container %s: %v\n%s", image, err, string(output))
	}
	t.Cleanup(func() {
		_, _ = runDockerCommand(dockerBinary, "rm", "-f", containerName)
	})
	return containerName
}

func dockerMappedPort(t *testing.T, dockerBinary, containerName, containerPort string) string {
	t.Helper()

	output, err := runDockerCommand(dockerBinary, "port", containerName, containerPort)
	if err != nil {
		t.Fatalf("docker port %s %s: %v\n%s", containerName, containerPort, err, string(output))
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hostPort := line[strings.LastIndex(line, ":")+1:]
		if _, err := strconv.Atoi(hostPort); err == nil {
			return hostPort
		}
	}
	t.Fatalf("unable to parse mapped port for %s from %q", containerName, string(output))
	return ""
}

func waitFor(t *testing.T, timeout time.Duration, fn func() error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := fn(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for integration dependency: %v", lastErr)
}

func runsvcRunResult() runsvc.RunResult {
	return runsvc.RunResult{
		Passed:          4,
		Failed:          1,
		HiddenPassed:    1,
		HiddenFailed:    0,
		ExecutionTimeMS: 32,
		Lint: runsvc.LintResult{
			ErrorCount:   0,
			WarningCount: 1,
		},
		TestResults: []runsvc.TestResult{
			{File: "tests/visible.spec.jsx", Name: "visible one", Passed: true},
			{File: "tests/visible.spec.jsx", Name: "visible two", Passed: true},
			{File: "tests/visible.spec.jsx", Name: "visible three", Passed: true},
			{File: "tests/hidden.spec.jsx", Name: "hidden one", Passed: true, Hidden: true},
			{File: "tests/visible.spec.jsx", Name: "visible four", Passed: false},
		},
	}
}

func runDockerCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
