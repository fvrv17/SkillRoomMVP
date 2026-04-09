package backend

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHRCandidatePreviewAndUnlockFlow(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-unlock@example.com",
		Username: "hr-unlock",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})
	candidateAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "candidate-unlock@example.com",
		Username: "candidate-unlock",
		Password: "password123",
		Country:  "US",
		Role:     RoleUser,
	})
	updateProfileResp := performJSON(t, router, http.MethodPatch, "/v1/profile", UpdateProfileRequest{
		LinkedInURL: "linkedin.com/in/candidate-unlock",
	}, candidateAuth.AccessToken)
	if updateProfileResp.Code != http.StatusOK {
		t.Fatalf("update profile status: %d", updateProfileResp.Code)
	}

	submitSolvedChallenge(t, router, candidateAuth.AccessToken, "react_feature_search", searchSolutionCode(), []TelemetryEventRequest{
		{EventType: "input", OffsetSeconds: 12},
		{EventType: "snapshot", OffsetSeconds: 35},
	})

	searchResp := performJSON(t, router, http.MethodGet, "/v1/hr/candidates?min_score=0&active_days=30", nil, hrAuth.AccessToken)
	if searchResp.Code != http.StatusOK {
		t.Fatalf("search candidates status: %d", searchResp.Code)
	}
	var searchPayload struct {
		Candidates   []CandidateView     `json:"candidates"`
		Pagination   PaginationInfo      `json:"pagination"`
		Monetization MonetizationSummary `json:"monetization"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&searchPayload); err != nil {
		t.Fatalf("decode search payload: %v", err)
	}
	if len(searchPayload.Candidates) == 0 {
		t.Fatal("expected at least one candidate in HR search")
	}
	if searchPayload.Pagination.Total == 0 {
		t.Fatal("expected pagination metadata in HR search payload")
	}
	candidatePreview := searchPayload.Candidates[0]
	if candidatePreview.Access.IsUnlocked {
		t.Fatal("expected preview candidate to start locked")
	}
	if candidatePreview.Access.RemainingUnlocks != 3 {
		t.Fatalf("expected 3 remaining unlocks on hr free plan, got %d", candidatePreview.Access.RemainingUnlocks)
	}

	detailResp := performJSON(t, router, http.MethodGet, "/v1/hr/candidates/"+candidatePreview.UserID, nil, hrAuth.AccessToken)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail before unlock status: %d", detailResp.Code)
	}
	var lockedDetail CandidateDetailView
	if err := json.NewDecoder(detailResp.Body).Decode(&lockedDetail); err != nil {
		t.Fatalf("decode locked detail: %v", err)
	}
	if lockedDetail.Contact != nil || lockedDetail.Profile != nil || len(lockedDetail.Skills) != 0 || len(lockedDetail.Room) != 0 {
		t.Fatal("expected locked detail to hide contact, profile, skills, and room")
	}
	if len(lockedDetail.LockedFields) == 0 {
		t.Fatal("expected locked fields metadata before unlock")
	}

	unlockResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+candidatePreview.UserID+"/unlock", nil, hrAuth.AccessToken)
	if unlockResp.Code != http.StatusCreated {
		t.Fatalf("unlock status: %d", unlockResp.Code)
	}
	var unlocked CandidateDetailView
	if err := json.NewDecoder(unlockResp.Body).Decode(&unlocked); err != nil {
		t.Fatalf("decode unlocked detail: %v", err)
	}
	if !unlocked.Candidate.Access.IsUnlocked {
		t.Fatal("expected candidate to be unlocked after unlock call")
	}
	if unlocked.Contact == nil || unlocked.Contact.Email != "candidate-unlock@example.com" {
		t.Fatalf("expected unlocked contact email, got %#v", unlocked.Contact)
	}
	if unlocked.Contact.LinkedInURL != "https://linkedin.com/in/candidate-unlock" {
		t.Fatalf("expected unlocked linkedin url, got %#v", unlocked.Contact)
	}
	if unlocked.Profile == nil || unlocked.Profile.SelectedTrack == "" {
		t.Fatalf("expected unlocked profile, got %#v", unlocked.Profile)
	}
	if unlocked.Profile.LinkedInURL != "https://linkedin.com/in/candidate-unlock" {
		t.Fatalf("expected unlocked profile linkedin, got %#v", unlocked.Profile)
	}
	if len(unlocked.Skills) == 0 {
		t.Fatal("expected unlocked skills after candidate unlock")
	}
	if len(unlocked.Room) == 0 {
		t.Fatal("expected unlocked room after candidate unlock")
	}
	if len(unlocked.RoomCustomization.Equipped) != 3 {
		t.Fatalf("expected candidate room customization to be included after unlock, got %d equipped cosmetics", len(unlocked.RoomCustomization.Equipped))
	}
	if len(unlocked.RecentSubmissions) == 0 {
		t.Fatal("expected recent submissions after candidate unlock")
	}
	if unlocked.Monetization.Usage.CandidateUnlocksUsed != 1 {
		t.Fatalf("expected unlock usage to increment, got %d", unlocked.Monetization.Usage.CandidateUnlocksUsed)
	}
}

func TestHRUnlockLimitEnforcedByFreePlan(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-limit@example.com",
		Username: "hr-limit",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})

	candidateIDs := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		auth := registerAndCaptureAuth(t, router, RegisterRequest{
			Email:    "candidate-limit-" + string(rune('a'+i)) + "@example.com",
			Username: "candidate-limit-" + string(rune('a'+i)),
			Password: "password123",
			Country:  "US",
			Role:     RoleUser,
		})
		candidateIDs = append(candidateIDs, auth.User.ID)
	}

	for i, userID := range candidateIDs[:3] {
		resp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+userID+"/unlock", nil, hrAuth.AccessToken)
		if resp.Code != http.StatusCreated {
			t.Fatalf("unlock %d status: %d", i+1, resp.Code)
		}
	}

	fourthResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+candidateIDs[3]+"/unlock", nil, hrAuth.AccessToken)
	if fourthResp.Code != http.StatusBadRequest {
		t.Fatalf("expected fourth unlock to fail with 400, got %d", fourthResp.Code)
	}
	if !strings.Contains(fourthResp.Body.String(), "unlock limit") {
		t.Fatalf("expected unlock limit error, got %s", fourthResp.Body.String())
	}
}

func TestHRInviteRequiresUnlockAndHonorsPlanLimit(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-invite@example.com",
		Username: "hr-invite",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})

	candidateIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		auth := registerAndCaptureAuth(t, router, RegisterRequest{
			Email:    "candidate-invite-" + string(rune('a'+i)) + "@example.com",
			Username: "candidate-invite-" + string(rune('a'+i)),
			Password: "password123",
			Country:  "US",
			Role:     RoleUser,
		})
		candidateIDs = append(candidateIDs, auth.User.ID)
	}

	lockedInviteResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+candidateIDs[0]+"/invite", nil, hrAuth.AccessToken)
	if lockedInviteResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invite before unlock to fail with 400, got %d", lockedInviteResp.Code)
	}
	if !strings.Contains(lockedInviteResp.Body.String(), "must be unlocked") {
		t.Fatalf("expected unlock requirement error, got %s", lockedInviteResp.Body.String())
	}

	for i, userID := range candidateIDs {
		unlockResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+userID+"/unlock", nil, hrAuth.AccessToken)
		if unlockResp.Code != http.StatusCreated {
			t.Fatalf("unlock %d status: %d", i+1, unlockResp.Code)
		}
	}

	for i, userID := range candidateIDs[:2] {
		inviteResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+userID+"/invite", nil, hrAuth.AccessToken)
		if inviteResp.Code != http.StatusCreated {
			t.Fatalf("invite %d status: %d", i+1, inviteResp.Code)
		}
		var invited CandidateDetailView
		if err := json.NewDecoder(inviteResp.Body).Decode(&invited); err != nil {
			t.Fatalf("decode invite response: %v", err)
		}
		if !invited.Candidate.Access.IsInvited {
			t.Fatalf("expected candidate %d to be invited", i+1)
		}
	}

	thirdInviteResp := performJSON(t, router, http.MethodPost, "/v1/hr/candidates/"+candidateIDs[2]+"/invite", nil, hrAuth.AccessToken)
	if thirdInviteResp.Code != http.StatusBadRequest {
		t.Fatalf("expected third invite to fail with 400, got %d", thirdInviteResp.Code)
	}
	if !strings.Contains(thirdInviteResp.Body.String(), "invite limit") {
		t.Fatalf("expected invite limit error, got %s", thirdInviteResp.Body.String())
	}

	leaderboardResp := performJSON(t, router, http.MethodGet, "/v1/hr/leaderboard", nil, hrAuth.AccessToken)
	if leaderboardResp.Code != http.StatusOK {
		t.Fatalf("leaderboard status: %d", leaderboardResp.Code)
	}
	var leaderboardPayload struct {
		Rankings     []CandidateView     `json:"rankings"`
		Pagination   PaginationInfo      `json:"pagination"`
		Monetization MonetizationSummary `json:"monetization"`
	}
	if err := json.NewDecoder(leaderboardResp.Body).Decode(&leaderboardPayload); err != nil {
		t.Fatalf("decode leaderboard payload: %v", err)
	}
	if leaderboardPayload.Monetization.Usage.CandidateInvitesUsed != 2 {
		t.Fatalf("expected 2 invites used, got %d", leaderboardPayload.Monetization.Usage.CandidateInvitesUsed)
	}
	if leaderboardPayload.Pagination.Total != len(leaderboardPayload.Rankings) {
		t.Fatalf("expected leaderboard pagination total to match visible rankings in this fixture, total=%d visible=%d", leaderboardPayload.Pagination.Total, len(leaderboardPayload.Rankings))
	}
}

func TestHRCandidateEndpointsSupportPagination(t *testing.T) {
	app := newTestApp()
	router := app.Router()

	hrAuth := registerAndCaptureAuth(t, router, RegisterRequest{
		Email:    "hr-pagination@example.com",
		Username: "hr-pagination",
		Password: "password123",
		Country:  "US",
		Role:     RoleHR,
	})

	for i := 0; i < 4; i++ {
		auth := registerAndCaptureAuth(t, router, RegisterRequest{
			Email:    "candidate-pagination-" + string(rune('a'+i)) + "@example.com",
			Username: "candidate-pagination-" + string(rune('a'+i)),
			Password: "password123",
			Country:  "US",
			Role:     RoleUser,
		})
		submitSolvedChallenge(t, router, auth.AccessToken, "react_feature_search", searchSolutionCode(), []TelemetryEventRequest{
			{EventType: "input", OffsetSeconds: 10 + i},
		})
	}

	firstPageResp := performJSON(t, router, http.MethodGet, "/v1/hr/candidates?limit=2&offset=0", nil, hrAuth.AccessToken)
	if firstPageResp.Code != http.StatusOK {
		t.Fatalf("first candidate page status: %d", firstPageResp.Code)
	}
	var firstPage struct {
		Candidates []CandidateView `json:"candidates"`
		Pagination PaginationInfo  `json:"pagination"`
	}
	if err := json.NewDecoder(firstPageResp.Body).Decode(&firstPage); err != nil {
		t.Fatalf("decode first candidate page: %v", err)
	}
	if len(firstPage.Candidates) != 2 {
		t.Fatalf("expected 2 candidate previews on first page, got %d", len(firstPage.Candidates))
	}
	if firstPage.Pagination.Total != 4 || !firstPage.Pagination.HasMore {
		t.Fatalf("unexpected first candidate pagination: %+v", firstPage.Pagination)
	}

	secondPageResp := performJSON(t, router, http.MethodGet, "/v1/hr/candidates?limit=2&offset=2", nil, hrAuth.AccessToken)
	if secondPageResp.Code != http.StatusOK {
		t.Fatalf("second candidate page status: %d", secondPageResp.Code)
	}
	var secondPage struct {
		Candidates []CandidateView `json:"candidates"`
		Pagination PaginationInfo  `json:"pagination"`
	}
	if err := json.NewDecoder(secondPageResp.Body).Decode(&secondPage); err != nil {
		t.Fatalf("decode second candidate page: %v", err)
	}
	if len(secondPage.Candidates) != 2 {
		t.Fatalf("expected 2 candidate previews on second page, got %d", len(secondPage.Candidates))
	}
	if secondPage.Pagination.Total != 4 || secondPage.Pagination.HasMore {
		t.Fatalf("unexpected second candidate pagination: %+v", secondPage.Pagination)
	}
	if firstPage.Candidates[0].UserID == secondPage.Candidates[0].UserID {
		t.Fatal("expected candidate pagination pages to advance through the result set")
	}

	leaderboardResp := performJSON(t, router, http.MethodGet, "/v1/hr/leaderboard?limit=2&offset=2", nil, hrAuth.AccessToken)
	if leaderboardResp.Code != http.StatusOK {
		t.Fatalf("leaderboard pagination status: %d", leaderboardResp.Code)
	}
	var leaderboardPage struct {
		Rankings   []CandidateView `json:"rankings"`
		Pagination PaginationInfo  `json:"pagination"`
	}
	if err := json.NewDecoder(leaderboardResp.Body).Decode(&leaderboardPage); err != nil {
		t.Fatalf("decode paginated leaderboard: %v", err)
	}
	if len(leaderboardPage.Rankings) != 2 {
		t.Fatalf("expected 2 leaderboard entries on paginated page, got %d", len(leaderboardPage.Rankings))
	}
	if leaderboardPage.Pagination.Total != 4 || leaderboardPage.Pagination.HasMore {
		t.Fatalf("unexpected leaderboard pagination: %+v", leaderboardPage.Pagination)
	}
}
