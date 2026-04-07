package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	runsvc "github.com/fvrv17/mvp/internal/runner"
)

type submissionResponse struct {
	Submission Submission          `json:"submission"`
	Evaluation EvaluationResult    `json:"evaluation"`
	Execution  RunnerReport        `json:"execution"`
	AntiCheat  AntiCheatAssessment `json:"anti_cheat"`
	Profile    UserProfile         `json:"profile"`
	Skills     []UserSkill         `json:"skills"`
	Room       []UserRoomItem      `json:"room"`
	Telemetry  TelemetrySummary    `json:"telemetry"`
	TemplateID string              `json:"template_id"`
}

type runPreviewResponse struct {
	Status     string           `json:"status"`
	Execution  RunnerReport     `json:"execution"`
	Telemetry  TelemetrySummary `json:"telemetry"`
	InstanceID string           `json:"instance_id"`
	TemplateID string           `json:"template_id"`
}

type stubRunner struct {
	result runsvc.RunResult
}

func (s stubRunner) Run(context.Context, runsvc.RunRequest) (runsvc.RunResult, error) {
	return s.result, nil
}

func newTestApp() *App {
	app := NewApp("test-secret", "test-issuer")
	app.SetChallengeRunner(stubRunner{
		result: runsvc.RunResult{
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
		},
	})
	return app
}

func TestChallengeFlowUsesTelemetryAndServerEvaluation(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	token := registerAndLogin(t, router, RegisterRequest{
		Email:    "dev1@example.com",
		Username: "dev1",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	result := submitSolvedChallenge(t, router, token, "react_feature_search", searchSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 12},
		{EventType: "snapshot", OffsetSeconds: 35, Payload: map[string]any{"line_count": 18}},
	})

	if result.TemplateID != "react_feature_search" {
		t.Fatalf("expected template id react_feature_search, got %s", result.TemplateID)
	}
	if result.Execution.TestsTotal != 5 {
		t.Fatalf("expected 5 checks, got %d", result.Execution.TestsTotal)
	}
	if result.Execution.TestsPassed < 4 {
		t.Fatalf("expected at least 4 checks to pass, got %d", result.Execution.TestsPassed)
	}
	if result.AntiCheat.Level != "low" {
		t.Fatalf("expected low anti-cheat level, got %s", result.AntiCheat.Level)
	}
	if result.Telemetry.InputEvents != 1 || result.Telemetry.SnapshotEvents != 1 {
		t.Fatalf("unexpected telemetry summary: %+v", result.Telemetry)
	}
	if result.Profile.CurrentSkillScore <= 0 {
		t.Fatalf("expected skill score to increase")
	}
	if result.Profile.CompletedChallenges != 1 {
		t.Fatalf("expected completed challenge count to increment")
	}
	if len(result.Room) != 6 {
		t.Fatalf("expected 6 room items, got %d", len(result.Room))
	}
}

func TestTrophyCaseUsesAchievementState(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	token := registerAndLogin(t, router, RegisterRequest{
		Email:    "trophy@example.com",
		Username: "trophy",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	app.mu.Lock()
	for userID, user := range app.users {
		if user.Email != "trophy@example.com" {
			continue
		}
		user.Profile.CompletedChallenges = 12
		user.Profile.ConfidenceScore = 86
		user.Profile.PercentileGlobal = 93
		user.Profile.StreakDays = 7
		user.Profile.UpdatedAt = time.Now().UTC()
		app.users[userID] = user
		app.scoreHistory[userID] = []float64{78, 80, 82, 81, 79}
		app.updateAllRoomTrophiesLocked()
		break
	}
	app.mu.Unlock()

	roomResp := performJSON(t, router, http.MethodGet, "/v1/room", nil, token)
	if roomResp.Code != http.StatusOK {
		t.Fatalf("get room: %d", roomResp.Code)
	}

	var payload struct {
		Items []UserRoomItem `json:"items"`
	}
	if err := json.NewDecoder(roomResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode room payload: %v", err)
	}

	var trophy UserRoomItem
	found := false
	for _, item := range payload.Items {
		if item.RoomItemCode == "trophy_case" {
			trophy = item
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected trophy_case in room payload")
	}
	if trophy.CurrentLevel != "static" {
		t.Fatalf("expected trophy_case current_level static, got %s", trophy.CurrentLevel)
	}
	if trophy.CurrentVariant != "trophy_case_default" {
		t.Fatalf("expected trophy_case default variant, got %s", trophy.CurrentVariant)
	}
	if trophy.State["presentation_mode"] != "achievement_case" {
		t.Fatalf("expected achievement_case presentation mode, got %#v", trophy.State["presentation_mode"])
	}
	if _, ok := trophy.State["level"]; ok {
		t.Fatal("expected trophy_case state to omit tier level")
	}
	achievementCount, ok := trophy.State["achievement_count"].(float64)
	if !ok || achievementCount < 4 {
		t.Fatalf("expected achievement_count >= 4, got %#v", trophy.State["achievement_count"])
	}

	rawAchievements, ok := trophy.State["achievements"].([]any)
	if !ok || len(rawAchievements) == 0 {
		t.Fatalf("expected achievements array, got %#v", trophy.State["achievements"])
	}
	foundTopTen := false
	for _, raw := range rawAchievements {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entry["code"] == "top_percentile_10" {
			foundTopTen = true
			break
		}
	}
	if !foundTopTen {
		t.Fatalf("expected top_percentile_10 achievement, got %#v", rawAchievements)
	}
}

func TestRunPreviewDoesNotFinalizeChallenge(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	token := registerAndLogin(t, router, RegisterRequest{
		Email:    "preview@example.com",
		Username: "preview",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	view := startChallengeInstance(t, router, token, "react_feature_search")
	for _, event := range []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 15},
		{EventType: "snapshot", OffsetSeconds: 37},
	} {
		telemetryResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/telemetry", event, token)
		if telemetryResp.Code != http.StatusCreated {
			t.Fatalf("record telemetry: %d", telemetryResp.Code)
		}
	}

	runResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/runs", SubmitChallengeRequest{
		Language: "jsx",
		SourceFiles: map[string]string{
			"src/App.jsx": searchSolutionCode(),
		},
	}, token)
	if runResp.Code != http.StatusOK {
		t.Fatalf("run preview: %d", runResp.Code)
	}

	var preview runPreviewResponse
	if err := json.NewDecoder(runResp.Body).Decode(&preview); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if preview.Status != "preview" {
		t.Fatalf("expected preview status, got %s", preview.Status)
	}
	if preview.Execution.TestsPassed < 4 {
		t.Fatalf("expected preview to pass checks, got %d", preview.Execution.TestsPassed)
	}
	if len(preview.Execution.Checks) != preview.Execution.TestsTotal {
		t.Fatalf("expected detailed checks in preview response")
	}
	if preview.Telemetry.InputEvents != 1 || preview.Telemetry.SnapshotEvents != 1 {
		t.Fatalf("unexpected telemetry summary: %+v", preview.Telemetry)
	}

	instanceResp := performJSON(t, router, http.MethodGet, "/v1/challenges/instances/"+view.Instance.ID, nil, token)
	if instanceResp.Code != http.StatusOK {
		t.Fatalf("get challenge instance: %d", instanceResp.Code)
	}
	var refreshed ChallengeInstanceView
	if err := json.NewDecoder(instanceResp.Body).Decode(&refreshed); err != nil {
		t.Fatalf("decode refreshed instance: %v", err)
	}
	if refreshed.Instance.Status != "assigned" {
		t.Fatalf("expected run preview to keep assigned status, got %s", refreshed.Instance.Status)
	}

	profileResp := performJSON(t, router, http.MethodGet, "/v1/profile", nil, token)
	if profileResp.Code != http.StatusOK {
		t.Fatalf("get profile: %d", profileResp.Code)
	}
	var profile UserProfile
	if err := json.NewDecoder(profileResp.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.CompletedChallenges != 0 {
		t.Fatalf("expected preview not to increment completed challenges")
	}
}

func TestRankingsFriendChatAndHRSearch(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	user1 := registerAndLogin(t, router, RegisterRequest{Email: "u1@example.com", Username: "u1", Password: "password123", Country: "US"})
	user2 := registerAndLogin(t, router, RegisterRequest{Email: "u2@example.com", Username: "u2", Password: "password123", Country: "US"})
	user3 := registerAndLogin(t, router, RegisterRequest{Email: "u3@example.com", Username: "u3", Password: "password123", Country: "CA"})
	hrToken := registerAndLogin(t, router, RegisterRequest{Email: "hr@example.com", Username: "hr", Password: "password123", Country: "US", Role: RoleHR})

	scoreChallenge(t, router, user1, "react_performance_virtual_list", performanceSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 18},
		{EventType: "snapshot", OffsetSeconds: 42},
	})
	scoreChallenge(t, router, user1, "react_feature_search", searchSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 16},
		{EventType: "snapshot", OffsetSeconds: 39},
	})
	scoreChallenge(t, router, user2, "react_logic_selection_state", hookSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 25},
		{EventType: "snapshot", OffsetSeconds: 50},
	})
	scoreChallenge(t, router, user3, "react_feature_search", basicSearchSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 30},
	})

	globalResp := performJSON(t, router, http.MethodGet, "/v1/rankings/global", nil, user1)
	if globalResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", globalResp.Code)
	}
	var globalRankings map[string][]RankingEntry
	if err := json.NewDecoder(globalResp.Body).Decode(&globalRankings); err != nil {
		t.Fatalf("decode global rankings: %v", err)
	}
	if len(globalRankings["rankings"]) != 3 {
		t.Fatalf("expected 3 ranked users, got %d", len(globalRankings["rankings"]))
	}
	if globalRankings["rankings"][0].Username != "u1" {
		t.Fatalf("expected u1 to lead rankings, got %s", globalRankings["rankings"][0].Username)
	}

	friendReq := performJSON(t, router, http.MethodPost, "/v1/friends/"+userIDFromToken(t, app, user2)+"/request", nil, user1)
	if friendReq.Code != http.StatusCreated {
		t.Fatalf("expected 201 friend request, got %d", friendReq.Code)
	}
	friendAccept := performJSON(t, router, http.MethodPost, "/v1/friends/"+userIDFromToken(t, app, user1)+"/accept", nil, user2)
	if friendAccept.Code != http.StatusOK {
		t.Fatalf("expected 200 friend accept, got %d", friendAccept.Code)
	}

	msgResp := performJSON(t, router, http.MethodPost, "/v1/chat/direct/"+userIDFromToken(t, app, user2)+"/messages", ChatRequest{Body: "let's discuss the challenge"}, user1)
	if msgResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 chat message, got %d", msgResp.Code)
	}

	friendsResp := performJSON(t, router, http.MethodGet, "/v1/rankings/friends", nil, user1)
	if friendsResp.Code != http.StatusOK {
		t.Fatalf("expected 200 friends ranking, got %d", friendsResp.Code)
	}
	var friendRankings map[string][]RankingEntry
	if err := json.NewDecoder(friendsResp.Body).Decode(&friendRankings); err != nil {
		t.Fatalf("decode rankings: %v", err)
	}
	if len(friendRankings["rankings"]) != 2 {
		t.Fatalf("expected self + accepted friend in ranking, got %d", len(friendRankings["rankings"]))
	}

	companyResp := performJSON(t, router, http.MethodPost, "/v1/hr/companies", CompanyRequest{Name: "React Corp", Description: "Hiring React developers"}, hrToken)
	if companyResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 company, got %d", companyResp.Code)
	}
	var company Company
	if err := json.NewDecoder(companyResp.Body).Decode(&company); err != nil {
		t.Fatalf("decode company: %v", err)
	}

	jobResp := performJSON(t, router, http.MethodPost, "/v1/hr/companies/"+company.ID+"/jobs", JobRequest{
		Title:         "Senior React Developer",
		Description:   "Build product UI",
		RequiredScore: 50,
		RequiredSkills: map[string]float64{
			"react": 60,
		},
	}, hrToken)
	if jobResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 job, got %d", jobResp.Code)
	}

	searchResp := performJSON(t, router, http.MethodGet, "/v1/hr/candidates?min_score=20&top_percent=100&active_days=14", nil, hrToken)
	if searchResp.Code != http.StatusOK {
		t.Fatalf("expected 200 search, got %d", searchResp.Code)
	}
	var candidates map[string][]CandidateView
	if err := json.NewDecoder(searchResp.Body).Decode(&candidates); err != nil {
		t.Fatalf("decode candidates: %v", err)
	}
	if len(candidates["candidates"]) < 3 {
		t.Fatalf("expected candidates in HR search, got %d", len(candidates["candidates"]))
	}
}

func TestSubmissionFlagsHighSuspicionForCopiedSolution(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	firstUser := registerAndLogin(t, router, RegisterRequest{Email: "copy-1@example.com", Username: "copy1", Password: "password123", Country: "US"})
	secondUser := registerAndLogin(t, router, RegisterRequest{Email: "copy-2@example.com", Username: "copy2", Password: "password123", Country: "US"})

	code := searchSolutionCode()
	submitSolvedChallenge(t, router, firstUser, "react_feature_search", code, []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 20},
		{EventType: "snapshot", OffsetSeconds: 46},
	})
	result := submitSolvedChallenge(t, router, secondUser, "react_feature_search", code, []TelemetryEventRequest{
		{EventType: "paste", OffsetSeconds: 3},
		{EventType: "focus_lost", OffsetSeconds: 7},
		{EventType: "focus_lost", OffsetSeconds: 11},
	})

	if result.AntiCheat.Level != "high" {
		t.Fatalf("expected high anti-cheat level, got %s", result.AntiCheat.Level)
	}
	if result.AntiCheat.SimilarityScore < 0.9 {
		t.Fatalf("expected near-identical similarity score, got %f", result.AntiCheat.SimilarityScore)
	}
	if result.Execution.PasteEvents != 1 {
		t.Fatalf("expected paste telemetry to be counted")
	}
}

func scoreChallenge(t *testing.T, router http.Handler, token, templateID, code string, telemetry []TelemetryEventRequest) submissionResponse {
	t.Helper()
	return submitSolvedChallenge(t, router, token, templateID, code, telemetry)
}

func submitSolvedChallenge(t *testing.T, router http.Handler, token, templateID, code string, telemetry []TelemetryEventRequest) submissionResponse {
	t.Helper()
	view := startChallengeInstance(t, router, token, templateID)
	for _, event := range telemetry {
		telemetryResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/telemetry", event, token)
		if telemetryResp.Code != http.StatusCreated {
			t.Fatalf("record telemetry: %d", telemetryResp.Code)
		}
	}
	submitResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/submissions", SubmitChallengeRequest{
		Language: "jsx",
		SourceFiles: map[string]string{
			editablePathForTemplate(templateID): code,
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

func startChallengeInstance(t *testing.T, router http.Handler, token, templateID string) ChallengeInstanceView {
	t.Helper()
	instanceResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances", StartChallengeRequest{TemplateID: templateID}, token)
	if instanceResp.Code != http.StatusCreated {
		t.Fatalf("start challenge: %d", instanceResp.Code)
	}
	var view ChallengeInstanceView
	if err := json.NewDecoder(instanceResp.Body).Decode(&view); err != nil {
		t.Fatalf("decode instance: %v", err)
	}
	return view
}

func registerAndLogin(t *testing.T, router http.Handler, req RegisterRequest) string {
	t.Helper()
	registerResp := performJSON(t, router, http.MethodPost, "/v1/auth/register", req, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register failed: %d", registerResp.Code)
	}
	var authResp AuthResponse
	if err := json.NewDecoder(registerResp.Body).Decode(&authResp); err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	return authResp.AccessToken
}

func performJSON(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func userIDFromToken(t *testing.T, app *App, token string) string {
	t.Helper()
	claims, err := app.tokens.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	return claims.Subject
}

func editablePathForTemplate(templateID string) string {
	switch templateID {
	case "react_logic_selection_state":
		return "src/useSelectionState.js"
	default:
		return "src/App.jsx"
	}
}

func searchSolutionCode() string {
	return `import React, { useEffect, useMemo, useState } from "react";

export function SearchResults({ items }) {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");

  useEffect(() => {
    const timeoutId = setTimeout(() => setDebouncedQuery(query.trim().toLowerCase()), 150);
    return () => clearTimeout(timeoutId);
  }, [query]);

  const visibleItems = useMemo(() => {
    return items.filter((item) => item.label.toLowerCase().includes(debouncedQuery));
  }, [items, debouncedQuery]);

  return (
    <section>
      <input value={query} onChange={(event) => setQuery(event.target.value)} />
      {visibleItems.length === 0 ? (
        <p>No results found</p>
      ) : (
        visibleItems.map((item) => <div key={item.id}>{item.label}</div>)
      )}
    </section>
  );
}`
}

func basicSearchSolutionCode() string {
	return `import React, { useState } from "react";

export function SearchResults({ items }) {
  const [query, setQuery] = useState("");
  const visibleItems = items.filter((item) => item.label.toLowerCase().includes(query.toLowerCase()));

  return (
    <section>
      <input value={query} onChange={(event) => setQuery(event.target.value)} />
      {visibleItems.map((item) => <div key={item.id}>{item.label}</div>)}
    </section>
  );
}`
}

func hookSolutionCode() string {
	return `import React, { useRef, useState } from "react";

export function usePaginatedSearch(fetchPage) {
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const latestRequest = useRef(0);

  async function loadPage(cursor) {
    setLoading(true);
    const requestId = latestRequest.current + 1;
    latestRequest.current = requestId;
    const page = await fetchPage(cursor);
    if (requestId !== latestRequest.current) {
      return { results, loading };
    }
    setResults((prev) => [...prev, ...page.items]);
    setLoading(false);
    return page;
  }

  return { results, loading, loadPage };
}`
}

func performanceSolutionCode() string {
	return `import React, { memo, useCallback, useDeferredValue, useMemo } from "react";

const CandidateRow = memo(function CandidateRow({ item, onSelect }) {
  return <button onClick={onSelect}>{item.name}</button>;
});

export function CandidateGrid({ items, onSelect, query }) {
  const deferredQuery = useDeferredValue(query);
  const visibleItems = useMemo(() => {
    return items.filter((item) => item.name.toLowerCase().includes(deferredQuery.toLowerCase()));
  }, [items, deferredQuery]);

  const buildSelectHandler = useCallback((id) => {
    return () => onSelect(id);
  }, [onSelect]);

  return visibleItems.map((item) => (
    <CandidateRow key={item.id} item={item} onSelect={buildSelectHandler(item.id)} />
  ));
}`
}
