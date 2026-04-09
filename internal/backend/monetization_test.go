package backend

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMonetizationSummarySeedsDefaultPlansByRole(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	devAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "dev-m1@example.com",
		Username: "dev-m1",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})
	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-m1@example.com",
		Username: "hr-m1",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})

	devSummaryResp := performJSON(t, router, http.MethodGet, "/v1/monetization/summary", nil, devAuth.AccessToken)
	if devSummaryResp.Code != http.StatusOK {
		t.Fatalf("developer summary status: %d", devSummaryResp.Code)
	}
	var devSummary MonetizationSummary
	if err := json.NewDecoder(devSummaryResp.Body).Decode(&devSummary); err != nil {
		t.Fatalf("decode developer summary: %v", err)
	}
	if devSummary.Plan.Code != planCodeDeveloperFree {
		t.Fatalf("expected developer free plan, got %s", devSummary.Plan.Code)
	}
	if !devSummary.Entitlements.CosmeticCustomization {
		t.Fatal("expected developer plan to enable cosmetic customization")
	}

	hrSummaryResp := performJSON(t, router, http.MethodGet, "/v1/monetization/summary", nil, hrAuth.AccessToken)
	if hrSummaryResp.Code != http.StatusOK {
		t.Fatalf("hr summary status: %d", hrSummaryResp.Code)
	}
	var hrSummary MonetizationSummary
	if err := json.NewDecoder(hrSummaryResp.Body).Decode(&hrSummary); err != nil {
		t.Fatalf("decode hr summary: %v", err)
	}
	if hrSummary.Plan.Code != planCodeHRFree {
		t.Fatalf("expected hr free plan, got %s", hrSummary.Plan.Code)
	}
	if !hrSummary.Entitlements.CandidatePreview {
		t.Fatal("expected hr free plan to include candidate preview")
	}
	if hrSummary.Entitlements.CandidateUnlocksPerMonth != 3 {
		t.Fatalf("expected hr free unlock limit of 3, got %d", hrSummary.Entitlements.CandidateUnlocksPerMonth)
	}

	devInventoryResp := performJSON(t, router, http.MethodGet, "/v1/dev/cosmetics/inventory", nil, devAuth.AccessToken)
	if devInventoryResp.Code != http.StatusOK {
		t.Fatalf("developer inventory status: %d", devInventoryResp.Code)
	}
	var inventory CosmeticInventoryResponse
	if err := json.NewDecoder(devInventoryResp.Body).Decode(&inventory); err != nil {
		t.Fatalf("decode cosmetic inventory: %v", err)
	}
	if len(inventory.Catalog) < 6 {
		t.Fatalf("expected seeded cosmetic catalog, got %d items", len(inventory.Catalog))
	}
	if len(inventory.Owned) != 6 {
		t.Fatalf("expected 6 default owned cosmetics, got %d", len(inventory.Owned))
	}
	if len(inventory.Equipped) != 3 {
		t.Fatalf("expected 3 equipped default cosmetics, got %d", len(inventory.Equipped))
	}

	hrInventoryResp := performJSON(t, router, http.MethodGet, "/v1/dev/cosmetics/inventory", nil, hrAuth.AccessToken)
	if hrInventoryResp.Code != http.StatusForbidden {
		t.Fatalf("expected hr cosmetic inventory to be forbidden, got %d", hrInventoryResp.Code)
	}
}

func TestDeveloperCosmeticEquipRejectsPremiumWithoutPlusAndAllowsEntitledEquip(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	devAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "dev-equip@example.com",
		Username: "dev-equip",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	premiumResp := performJSON(t, router, http.MethodPost, "/v1/dev/cosmetics/equip", EquipCosmeticRequest{
		CosmeticCode: "window_night_plus",
	}, devAuth.AccessToken)
	if premiumResp.Code != http.StatusForbidden {
		t.Fatalf("expected premium equip to be forbidden on free plan, got %d", premiumResp.Code)
	}

	freeResp := performJSON(t, router, http.MethodPost, "/v1/dev/cosmetics/equip", EquipCosmeticRequest{
		CosmeticCode: "window_sunset_default",
	}, devAuth.AccessToken)
	if freeResp.Code != http.StatusOK {
		t.Fatalf("expected free cosmetic equip to succeed, got %d", freeResp.Code)
	}
	var inventory CosmeticInventoryResponse
	if err := json.NewDecoder(freeResp.Body).Decode(&inventory); err != nil {
		t.Fatalf("decode free equip inventory: %v", err)
	}
	if equippedCosmeticForSlot(inventory.Equipped, "window_scene") != "window_sunset_default" {
		t.Fatalf("expected sunset window to be equipped, got %s", equippedCosmeticForSlot(inventory.Equipped, "window_scene"))
	}

	userID := app.emailIndex["dev-equip@example.com"]
	subscription := app.subscriptions[userID]
	subscription.PlanCode = planCodeDeveloperPlus
	subscription.PlanID = app.plans[planCodeDeveloperPlus].ID
	app.subscriptions[userID] = subscription

	plusResp := performJSON(t, router, http.MethodPost, "/v1/dev/cosmetics/equip", EquipCosmeticRequest{
		CosmeticCode: "window_night_plus",
	}, devAuth.AccessToken)
	if plusResp.Code != http.StatusOK {
		t.Fatalf("expected premium equip to succeed for plus plan, got %d", plusResp.Code)
	}
	if err := json.NewDecoder(plusResp.Body).Decode(&inventory); err != nil {
		t.Fatalf("decode plus equip inventory: %v", err)
	}
	if equippedCosmeticForSlot(inventory.Equipped, "window_scene") != "window_night_plus" {
		t.Fatalf("expected night window to be equipped, got %s", equippedCosmeticForSlot(inventory.Equipped, "window_scene"))
	}
}

func equippedCosmeticForSlot(items []EquippedCosmetic, slotCode string) string {
	for _, item := range items {
		if item.SlotCode == slotCode {
			return item.CosmeticCode
		}
	}
	return ""
}

func TestMonetizationUsageMetersDeveloperAndHRAI(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	devAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "dev-ai@example.com",
		Username: "dev-ai",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})

	instance := startChallengeInstance(t, router, devAuth.AccessToken, "react_feature_search")

	hintResp := performJSON(t, router, http.MethodPost, "/v1/ai/challenges/"+instance.Instance.ID+"/hint", AIHintRequest{
		FocusArea: "state",
	}, devAuth.AccessToken)
	if hintResp.Code != http.StatusOK {
		t.Fatalf("hint status: %d", hintResp.Code)
	}

	submission := submitSolvedChallenge(t, router, devAuth.AccessToken, "react_feature_search", searchSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 11},
		{EventType: "snapshot", OffsetSeconds: 29},
	})
	explainResp := performJSON(t, router, http.MethodPost, "/v1/ai/challenges/"+submission.Submission.ChallengeInstanceID+"/explain", AIExplanationRequest{
		SubmissionID: submission.Submission.ID,
	}, devAuth.AccessToken)
	if explainResp.Code != http.StatusOK {
		t.Fatalf("explain status: %d", explainResp.Code)
	}

	devSummaryResp := performJSON(t, router, http.MethodGet, "/v1/monetization/summary", nil, devAuth.AccessToken)
	if devSummaryResp.Code != http.StatusOK {
		t.Fatalf("developer summary status: %d", devSummaryResp.Code)
	}
	var devSummary MonetizationSummary
	if err := json.NewDecoder(devSummaryResp.Body).Decode(&devSummary); err != nil {
		t.Fatalf("decode developer summary: %v", err)
	}
	if devSummary.Usage.DeveloperHintsUsed != 1 {
		t.Fatalf("expected 1 developer hint usage, got %d", devSummary.Usage.DeveloperHintsUsed)
	}
	if devSummary.Usage.DeveloperExplainsUsed != 1 {
		t.Fatalf("expected 1 developer explain usage, got %d", devSummary.Usage.DeveloperExplainsUsed)
	}

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-ai@example.com",
		Username: "hr-ai",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})
	mutationResp := performJSON(t, router, http.MethodPost, "/v1/hr/ai/templates/react_feature_search/mutation-preview", AIMutationPreviewRequest{
		Seed: 44,
	}, hrAuth.AccessToken)
	if mutationResp.Code != http.StatusOK {
		t.Fatalf("mutation preview status: %d", mutationResp.Code)
	}

	hrSummaryResp := performJSON(t, router, http.MethodGet, "/v1/monetization/summary", nil, hrAuth.AccessToken)
	if hrSummaryResp.Code != http.StatusOK {
		t.Fatalf("hr summary status: %d", hrSummaryResp.Code)
	}
	var hrSummary MonetizationSummary
	if err := json.NewDecoder(hrSummaryResp.Body).Decode(&hrSummary); err != nil {
		t.Fatalf("decode hr summary: %v", err)
	}
	if hrSummary.Usage.HRAIActionsUsed != 1 {
		t.Fatalf("expected 1 hr ai usage event, got %d", hrSummary.Usage.HRAIActionsUsed)
	}
}
