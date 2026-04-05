package backend

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAIHintLimitAndExplanationFlow(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	token := registerAndLogin(t, router, RegisterRequest{
		Email:    "ai-user@example.com",
		Username: "ai-user",
		Password: "password123",
		Country:  "US",
	})

	instanceResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances", StartChallengeRequest{TemplateID: "react_feature_search"}, token)
	if instanceResp.Code != http.StatusCreated {
		t.Fatalf("start challenge: %d", instanceResp.Code)
	}
	var view ChallengeInstanceView
	if err := json.NewDecoder(instanceResp.Body).Decode(&view); err != nil {
		t.Fatalf("decode instance: %v", err)
	}

	firstHintResp := performJSON(t, router, http.MethodPost, "/v1/ai/challenges/"+view.Instance.ID+"/hint", AIHintRequest{FocusArea: "filtering"}, token)
	if firstHintResp.Code != http.StatusOK {
		t.Fatalf("first hint status: %d", firstHintResp.Code)
	}
	var firstHint AIHintResponse
	if err := json.NewDecoder(firstHintResp.Body).Decode(&firstHint); err != nil {
		t.Fatalf("decode first hint: %v", err)
	}
	if firstHint.Provider != "deterministic" || firstHint.RemainingHints != 1 || firstHint.UsedHints != 1 {
		t.Fatalf("unexpected first hint response: %+v", firstHint)
	}

	secondHintResp := performJSON(t, router, http.MethodPost, "/v1/ai/challenges/"+view.Instance.ID+"/hint", AIHintRequest{Question: "what should I validate first?"}, token)
	if secondHintResp.Code != http.StatusOK {
		t.Fatalf("second hint status: %d", secondHintResp.Code)
	}
	var secondHint AIHintResponse
	if err := json.NewDecoder(secondHintResp.Body).Decode(&secondHint); err != nil {
		t.Fatalf("decode second hint: %v", err)
	}
	if secondHint.RemainingHints != 0 || secondHint.UsedHints != 2 {
		t.Fatalf("unexpected second hint response: %+v", secondHint)
	}

	thirdHintResp := performJSON(t, router, http.MethodPost, "/v1/ai/challenges/"+view.Instance.ID+"/hint", nil, token)
	if thirdHintResp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected hint limit response, got %d", thirdHintResp.Code)
	}

	submitResp := performJSON(t, router, http.MethodPost, "/v1/challenges/instances/"+view.Instance.ID+"/submissions", SubmitChallengeRequest{
		Language: "jsx",
		SourceFiles: map[string]string{
			"src/App.jsx": searchSolutionCode(),
		},
	}, token)
	if submitResp.Code != http.StatusCreated {
		t.Fatalf("submit status: %d", submitResp.Code)
	}

	explainResp := performJSON(t, router, http.MethodPost, "/v1/ai/challenges/"+view.Instance.ID+"/explain", nil, token)
	if explainResp.Code != http.StatusOK {
		t.Fatalf("explain status: %d", explainResp.Code)
	}
	var explanation AIExplanationResponse
	if err := json.NewDecoder(explainResp.Body).Decode(&explanation); err != nil {
		t.Fatalf("decode explanation: %v", err)
	}
	if explanation.Provider != "deterministic" || explanation.Summary == "" {
		t.Fatalf("unexpected explanation response: %+v", explanation)
	}
	if len(explanation.Strengths) == 0 || len(explanation.Improvements) == 0 {
		t.Fatalf("expected strengths and improvements in explanation: %+v", explanation)
	}

	if len(app.aiInteractions) != 3 {
		t.Fatalf("expected 3 persisted ai interactions, got %d", len(app.aiInteractions))
	}
}

func TestAIMutationPreviewRequiresHR(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	userToken := registerAndLogin(t, router, RegisterRequest{
		Email:    "candidate@example.com",
		Username: "candidate",
		Password: "password123",
		Country:  "US",
	})
	hrToken := registerAndLogin(t, router, RegisterRequest{
		Email:    "recruiter@example.com",
		Username: "recruiter",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})

	forbiddenResp := performJSON(t, router, http.MethodPost, "/v1/hr/ai/templates/react_feature_search/mutation-preview", AIMutationPreviewRequest{Seed: 1234}, userToken)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-HR preview, got %d", forbiddenResp.Code)
	}

	previewResp := performJSON(t, router, http.MethodPost, "/v1/hr/ai/templates/react_feature_search/mutation-preview", AIMutationPreviewRequest{Seed: 1234}, hrToken)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview status: %d", previewResp.Code)
	}
	var preview AIMutationPreviewResponse
	if err := json.NewDecoder(previewResp.Body).Decode(&preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Provider != "deterministic" || preview.Title == "" || preview.Description == "" {
		t.Fatalf("unexpected preview response: %+v", preview)
	}
	if preview.Seed != 1234 {
		t.Fatalf("expected preview seed to round-trip, got %d", preview.Seed)
	}
}
