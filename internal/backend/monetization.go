package backend

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/fvrv17/mvp/internal/platform/httpx"
	"github.com/fvrv17/mvp/internal/platform/id"
)

const (
	planCodeDeveloperFree = "dev_free"
	planCodeDeveloperPlus = "dev_plus"
	planCodeHRFree        = "hr_free"
	planCodeHRPro         = "hr_pro"
	planCodeHRBusiness    = "hr_business"
	planCodeInternalAdmin = "internal_admin"
)

func (a *App) handleMonetizationSummary(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.monetizationSummary(user.ID))
}

func (a *App) handleListCosmeticCatalog(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if !roleSupportsDeveloperRoom(user.Role) {
		httpx.WriteError(w, http.StatusForbidden, "developer room cosmetics are unavailable for this role")
		return
	}
	a.mu.RLock()
	catalog := a.listCosmeticCatalogLocked()
	a.mu.RUnlock()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"catalog": catalog})
}

func (a *App) handleCosmeticInventory(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if !roleSupportsDeveloperRoom(user.Role) {
		httpx.WriteError(w, http.StatusForbidden, "developer room cosmetics are unavailable for this role")
		return
	}
	a.mu.RLock()
	inventory := a.cosmeticInventoryLocked(user.ID)
	a.mu.RUnlock()
	httpx.WriteJSON(w, http.StatusOK, inventory)
}

func (a *App) monetizationSummary(userID string) MonetizationSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.monetizationSummaryLocked(userID)
}

func (a *App) monetizationSummaryLocked(userID string) MonetizationSummary {
	subscription := a.subscriptionForUserLocked(userID)
	plan := a.planForSubscriptionLocked(subscription)
	usage := a.usageForSubscriptionPeriodLocked(userID, subscription)
	return MonetizationSummary{
		Plan:         plan,
		Subscription: subscription,
		Entitlements: plan.Entitlements,
		Usage:        usage,
		FeatureFlags: append([]string(nil), plan.Features...),
	}
}

func (a *App) seedMonetization() {
	for _, plan := range defaultPlans() {
		a.plans[plan.Code] = plan
	}
	for _, item := range defaultCosmeticCatalog() {
		a.cosmeticCatalog[item.Code] = item
	}
}

func defaultPlans() []Plan {
	return []Plan{
		{
			ID:                "plan_dev_free",
			Code:              planCodeDeveloperFree,
			Name:              "Developer Free",
			Audience:          "developer",
			Tier:              "free",
			Currency:          "USD",
			MonthlyPriceCents: 0,
			Active:            true,
			Features:          []string{"free_ai_beta", "room_preview", "default_cosmetics"},
			Entitlements: PlanEntitlements{
				DeveloperAIFreeBeta:   true,
				CosmeticCustomization: true,
			},
		},
		{
			ID:                "plan_dev_plus",
			Code:              planCodeDeveloperPlus,
			Name:              "Developer Plus",
			Audience:          "developer",
			Tier:              "plus",
			Currency:          "USD",
			MonthlyPriceCents: 900,
			Active:            true,
			Features:          []string{"free_ai_beta", "premium_cosmetics"},
			Entitlements: PlanEntitlements{
				DeveloperAIFreeBeta:   true,
				CosmeticCustomization: true,
				PremiumCosmetics:      true,
			},
		},
		{
			ID:                "plan_hr_free",
			Code:              planCodeHRFree,
			Name:              "HR Free",
			Audience:          "hr",
			Tier:              "free",
			Currency:          "USD",
			MonthlyPriceCents: 0,
			Active:            true,
			Features:          []string{"candidate_preview", "light_search"},
			Entitlements: PlanEntitlements{
				CandidatePreview:         true,
				CandidateUnlocksPerMonth: 3,
				CandidateInvitesPerMonth: 2,
				HRAIActionsPerMonth:      10,
			},
		},
		{
			ID:                "plan_hr_pro",
			Code:              planCodeHRPro,
			Name:              "HR Pro",
			Audience:          "hr",
			Tier:              "pro",
			Currency:          "USD",
			MonthlyPriceCents: 9900,
			Active:            true,
			Features:          []string{"candidate_preview", "advanced_filters", "ai_shortlist_ready"},
			Entitlements: PlanEntitlements{
				CandidatePreview:         true,
				CandidateUnlocksPerMonth: 40,
				CandidateInvitesPerMonth: 25,
				HRAIActionsPerMonth:      150,
				HRAdvancedFilters:        true,
				BusinessSeats:            1,
			},
		},
		{
			ID:                "plan_hr_business",
			Code:              planCodeHRBusiness,
			Name:              "HR Business",
			Audience:          "hr",
			Tier:              "business",
			Currency:          "USD",
			MonthlyPriceCents: 29900,
			Active:            true,
			Features:          []string{"candidate_preview", "advanced_filters", "ai_shortlist_ready", "team_ready"},
			Entitlements: PlanEntitlements{
				CandidatePreview:         true,
				CandidateUnlocksPerMonth: 250,
				CandidateInvitesPerMonth: 150,
				HRAIActionsPerMonth:      1000,
				HRAdvancedFilters:        true,
				BusinessSeats:            25,
			},
		},
		{
			ID:                "plan_internal_admin",
			Code:              planCodeInternalAdmin,
			Name:              "Internal Admin",
			Audience:          "internal",
			Tier:              "internal",
			Currency:          "USD",
			MonthlyPriceCents: 0,
			Active:            true,
			Features:          []string{"candidate_preview", "advanced_filters", "free_ai_beta", "premium_cosmetics", "internal_override"},
			Entitlements: PlanEntitlements{
				CandidatePreview:         true,
				CandidateUnlocksPerMonth: 10000,
				CandidateInvitesPerMonth: 10000,
				HRAIActionsPerMonth:      10000,
				HRAdvancedFilters:        true,
				BusinessSeats:            999,
				DeveloperAIFreeBeta:      true,
				CosmeticCustomization:    true,
				PremiumCosmetics:         true,
			},
		},
	}
}

func defaultCosmeticCatalog() []CosmeticCatalogItem {
	return []CosmeticCatalogItem{
		{
			ID:          "cos_window_daylight",
			Code:        "window_daylight_default",
			Name:        "Daylight Window",
			Category:    "window_scene",
			SlotCode:    "window_scene",
			Description: "Default daylight scene for the studio window.",
			Audience:    "developer",
			Rarity:      "common",
			Premium:     false,
			AssetRef:    "window/daylight",
			Active:      true,
		},
		{
			ID:          "cos_wall_cream",
			Code:        "wall_cream_default",
			Name:        "Studio Cream Walls",
			Category:    "wall_style",
			SlotCode:    "wall_style",
			Description: "Neutral wall finish for the base SkillRoom.",
			Audience:    "developer",
			Rarity:      "common",
			Premium:     false,
			AssetRef:    "walls/cream",
			Active:      true,
		},
		{
			ID:          "cos_floor_oak",
			Code:        "floor_oak_default",
			Name:        "Light Oak Floor",
			Category:    "floor_style",
			SlotCode:    "floor_style",
			Description: "Base oak floor style for the studio shell.",
			Audience:    "developer",
			Rarity:      "common",
			Premium:     false,
			AssetRef:    "floors/oak-light",
			Active:      true,
		},
		{
			ID:          "cos_decor_books",
			Code:        "decor_books_orange",
			Name:        "Orange Book Stack",
			Category:    "decor",
			SlotCode:    "decor_left",
			Description: "Desk-side decorative stack with a warm orange accent.",
			Audience:    "developer",
			Rarity:      "rare",
			Premium:     true,
			AssetRef:    "decor/books-orange",
			Active:      true,
		},
		{
			ID:          "cos_decor_lamp",
			Code:        "decor_lamp_black",
			Name:        "Black Studio Lamp",
			Category:    "decor",
			SlotCode:    "decor_right",
			Description: "Minimal matte-black lamp for room styling.",
			Audience:    "developer",
			Rarity:      "rare",
			Premium:     true,
			AssetRef:    "decor/lamp-black",
			Active:      true,
		},
		{
			ID:          "cos_decor_poster",
			Code:        "decor_poster_grid",
			Name:        "Grid Poster",
			Category:    "decor",
			SlotCode:    "decor_wall",
			Description: "Wall poster variant for simple visual identity changes.",
			Audience:    "developer",
			Rarity:      "rare",
			Premium:     true,
			AssetRef:    "decor/poster-grid",
			Active:      true,
		},
	}
}

func (a *App) initUserMonetizationLocked(userID string, role Role, now time.Time) {
	if _, ok := a.subscriptions[userID]; !ok {
		planCode := defaultPlanCodeForRole(role)
		plan := a.plans[planCode]
		a.subscriptions[userID] = Subscription{
			ID:                 id.New("sub"),
			UserID:             userID,
			PlanID:             plan.ID,
			PlanCode:           plan.Code,
			Status:             "active",
			Source:             "system_default",
			AutoRenew:          false,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}

	if !roleSupportsDeveloperRoom(role) {
		return
	}
	if _, ok := a.userCosmetics[userID]; !ok {
		a.userCosmetics[userID] = map[string]UserCosmetic{}
	}
	if _, ok := a.equippedCosmetics[userID]; !ok {
		a.equippedCosmetics[userID] = map[string]EquippedCosmetic{}
	}
	for slotCode, cosmeticCode := range defaultEquippedCosmetics() {
		if _, owned := a.userCosmetics[userID][cosmeticCode]; !owned {
			cosmetic, ok := a.cosmeticCatalog[cosmeticCode]
			if !ok {
				continue
			}
			a.userCosmetics[userID][cosmeticCode] = UserCosmetic{
				ID:           id.New("uco"),
				UserID:       userID,
				CosmeticID:   cosmetic.ID,
				CosmeticCode: cosmetic.Code,
				Source:       "default_grant",
				OwnedAt:      now,
			}
		}
		if _, equipped := a.equippedCosmetics[userID][slotCode]; !equipped {
			a.equippedCosmetics[userID][slotCode] = EquippedCosmetic{
				UserID:       userID,
				SlotCode:     slotCode,
				CosmeticCode: cosmeticCode,
				UpdatedAt:    now,
			}
		}
	}
}

func (a *App) normalizeUserMonetizationLocked(userID string, role Role, fallbackTime time.Time) {
	a.initUserMonetizationLocked(userID, role, fallbackTime)
	subscription := a.subscriptions[userID]
	if subscription.PlanCode == "" {
		subscription.PlanCode = defaultPlanCodeForRole(role)
	}
	if subscription.PlanID == "" {
		subscription.PlanID = a.plans[subscription.PlanCode].ID
	}
	if subscription.Status == "" {
		subscription.Status = "active"
	}
	if subscription.CurrentPeriodStart.IsZero() {
		subscription.CurrentPeriodStart = fallbackTime
	}
	if subscription.CurrentPeriodEnd.IsZero() || !subscription.CurrentPeriodEnd.After(subscription.CurrentPeriodStart) {
		subscription.CurrentPeriodEnd = subscription.CurrentPeriodStart.Add(30 * 24 * time.Hour)
	}
	if subscription.CreatedAt.IsZero() {
		subscription.CreatedAt = fallbackTime
	}
	if subscription.UpdatedAt.IsZero() {
		subscription.UpdatedAt = fallbackTime
	}
	a.subscriptions[userID] = subscription
}

func defaultEquippedCosmetics() map[string]string {
	return map[string]string{
		"window_scene": "window_daylight_default",
		"wall_style":   "wall_cream_default",
		"floor_style":  "floor_oak_default",
	}
}

func defaultPlanCodeForRole(role Role) string {
	switch role {
	case RoleHR:
		return planCodeHRFree
	case RoleAdmin:
		return planCodeInternalAdmin
	default:
		return planCodeDeveloperFree
	}
}

func roleSupportsDeveloperRoom(role Role) bool {
	return role == RoleUser || role == RoleAdmin
}

func (a *App) subscriptionForUserLocked(userID string) Subscription {
	subscription, ok := a.subscriptions[userID]
	if ok {
		return subscription
	}
	user, ok := a.users[userID]
	if !ok {
		return Subscription{}
	}
	now := time.Now().UTC()
	a.initUserMonetizationLocked(userID, user.Role, now)
	return a.subscriptions[userID]
}

func (a *App) planForSubscriptionLocked(subscription Subscription) Plan {
	if plan, ok := a.plans[subscription.PlanCode]; ok {
		return plan
	}
	return a.plans[defaultPlanCodeForRole(RoleUser)]
}

func (a *App) usageForSubscriptionPeriodLocked(userID string, subscription Subscription) MonetizationUsage {
	usage := MonetizationUsage{
		PeriodStart: subscription.CurrentPeriodStart,
		PeriodEnd:   subscription.CurrentPeriodEnd,
	}
	for _, unlock := range a.candidateUnlocks[userID] {
		if !unlock.CreatedAt.Before(subscription.CurrentPeriodStart) && unlock.CreatedAt.Before(subscription.CurrentPeriodEnd) {
			usage.CandidateUnlocksUsed++
		}
	}
	for _, invite := range a.candidateInvites[userID] {
		if !invite.CreatedAt.Before(subscription.CurrentPeriodStart) && invite.CreatedAt.Before(subscription.CurrentPeriodEnd) {
			usage.CandidateInvitesUsed++
		}
	}
	for _, event := range a.aiUsageEvents[userID] {
		if event.CreatedAt.Before(subscription.CurrentPeriodStart) || !event.CreatedAt.Before(subscription.CurrentPeriodEnd) {
			continue
		}
		switch event.Action {
		case "developer_hint":
			usage.DeveloperHintsUsed += usageUnits(event.Units)
		case "developer_explain":
			usage.DeveloperExplainsUsed += usageUnits(event.Units)
		default:
			if event.Scope == "hr" {
				usage.HRAIActionsUsed += usageUnits(event.Units)
			}
		}
	}
	return usage
}

func (a *App) listCosmeticCatalogLocked() []CosmeticCatalogItem {
	items := make([]CosmeticCatalogItem, 0, len(a.cosmeticCatalog))
	for _, item := range a.cosmeticCatalog {
		if item.Active {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			return items[i].Name < items[j].Name
		}
		return items[i].Category < items[j].Category
	})
	return items
}

func (a *App) cosmeticInventoryLocked(userID string) CosmeticInventoryResponse {
	ownedMap := a.userCosmetics[userID]
	equippedMap := a.equippedCosmetics[userID]

	owned := make([]UserCosmetic, 0, len(ownedMap))
	for _, item := range ownedMap {
		owned = append(owned, item)
	}
	sort.Slice(owned, func(i, j int) bool {
		if owned[i].OwnedAt.Equal(owned[j].OwnedAt) {
			return owned[i].CosmeticCode < owned[j].CosmeticCode
		}
		return owned[i].OwnedAt.Before(owned[j].OwnedAt)
	})

	equipped := make([]EquippedCosmetic, 0, len(equippedMap))
	for _, item := range equippedMap {
		equipped = append(equipped, item)
	}
	sort.Slice(equipped, func(i, j int) bool {
		return equipped[i].SlotCode < equipped[j].SlotCode
	})

	return CosmeticInventoryResponse{
		Catalog:  a.listCosmeticCatalogLocked(),
		Owned:    owned,
		Equipped: equipped,
	}
}

func (a *App) recordAIUsage(ctx context.Context, event AIUsageEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.users[event.UserID]; !ok {
		return errors.New("user not found")
	}
	if event.ID == "" {
		event.ID = id.New("aiu")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Units <= 0 {
		event.Units = 1
	}
	if event.Context == nil {
		event.Context = map[string]any{}
	}
	a.aiUsageEvents[event.UserID] = append(a.aiUsageEvents[event.UserID], event)
	return a.persistAIUsageLocked(ctx, event)
}

func (a *App) candidateDetail(ctx context.Context, recruiterUserID, candidateUserID string) (CandidateDetailView, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	recruiter, ok := a.users[recruiterUserID]
	if !ok || (recruiter.Role != RoleHR && recruiter.Role != RoleAdmin) {
		return CandidateDetailView{}, errors.New("recruiter not found")
	}
	candidate, ok := a.users[candidateUserID]
	if !ok || candidate.Role != RoleUser {
		return CandidateDetailView{}, errors.New("candidate not found")
	}
	monetization := a.monetizationSummaryLocked(recruiterUserID)
	preview := a.candidatePreviewLocked(recruiterUserID, candidate, monetization)
	detail := CandidateDetailView{
		Candidate:    preview,
		LockedFields: []string{"contact", "linkedin", "profile", "skills", "room", "recent_submissions"},
		Monetization: monetization,
	}
	if !preview.Access.IsUnlocked {
		return detail, nil
	}
	profileCopy := a.profileViewLocked(candidate.ID)
	detail.Contact = &CandidateContact{Email: candidate.Email, LinkedInURL: profileCopy.LinkedInURL}
	detail.Profile = &profileCopy
	detail.Skills = a.listUserSkillsLocked(candidate.ID)
	detail.Room = a.listUserRoomItemsLocked(candidate.ID)
	detail.RecentSubmissions = a.candidateRecentSubmissionsLocked(candidate.ID)
	detail.LockedFields = nil
	return detail, nil
}

func (a *App) unlockCandidate(ctx context.Context, recruiterUserID, candidateUserID string) (CandidateDetailView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	recruiter, ok := a.users[recruiterUserID]
	if !ok || (recruiter.Role != RoleHR && recruiter.Role != RoleAdmin) {
		return CandidateDetailView{}, errors.New("recruiter not found")
	}
	candidate, ok := a.users[candidateUserID]
	if !ok || candidate.Role != RoleUser {
		return CandidateDetailView{}, errors.New("candidate not found")
	}
	if recruiterUserID == candidateUserID {
		return CandidateDetailView{}, errors.New("cannot unlock your own candidate card")
	}

	monetization := a.monetizationSummaryLocked(recruiterUserID)
	access := a.candidateAccessLocked(recruiterUserID, candidateUserID, monetization)
	if access.IsUnlocked {
		return a.candidateDetailLocked(recruiterUserID, candidate, monetization), nil
	}
	if !access.CanUnlock {
		if access.RemainingUnlocks <= 0 {
			return CandidateDetailView{}, errors.New("candidate unlock limit reached for current plan")
		}
		return CandidateDetailView{}, errors.New("candidate unlocks are unavailable for current plan")
	}

	now := time.Now().UTC()
	unlock := CandidateUnlock{
		ID:              id.New("cul"),
		RecruiterUserID: recruiterUserID,
		CandidateUserID: candidateUserID,
		Source:          "plan_credit",
		Status:          "active",
		CreatedAt:       now,
	}
	if _, ok := a.candidateUnlocks[recruiterUserID]; !ok {
		a.candidateUnlocks[recruiterUserID] = map[string]CandidateUnlock{}
	}
	a.candidateUnlocks[recruiterUserID][candidateUserID] = unlock
	if err := a.persistCandidateUnlockLocked(ctx, unlock); err != nil {
		return CandidateDetailView{}, err
	}

	updatedMonetization := a.monetizationSummaryLocked(recruiterUserID)
	return a.candidateDetailLocked(recruiterUserID, candidate, updatedMonetization), nil
}

func (a *App) inviteCandidate(ctx context.Context, recruiterUserID, candidateUserID string) (CandidateDetailView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	recruiter, ok := a.users[recruiterUserID]
	if !ok || (recruiter.Role != RoleHR && recruiter.Role != RoleAdmin) {
		return CandidateDetailView{}, errors.New("recruiter not found")
	}
	candidate, ok := a.users[candidateUserID]
	if !ok || candidate.Role != RoleUser {
		return CandidateDetailView{}, errors.New("candidate not found")
	}
	monetization := a.monetizationSummaryLocked(recruiterUserID)
	access := a.candidateAccessLocked(recruiterUserID, candidateUserID, monetization)
	if !access.IsUnlocked {
		return CandidateDetailView{}, errors.New("candidate must be unlocked before inviting")
	}
	if access.IsInvited {
		return a.candidateDetailLocked(recruiterUserID, candidate, monetization), nil
	}
	if !access.CanInvite {
		if access.RemainingInvites <= 0 {
			return CandidateDetailView{}, errors.New("candidate invite limit reached for current plan")
		}
		return CandidateDetailView{}, errors.New("candidate invites are unavailable for current plan")
	}

	now := time.Now().UTC()
	invite := CandidateInvite{
		ID:              id.New("cin"),
		RecruiterUserID: recruiterUserID,
		CandidateUserID: candidateUserID,
		Source:          "plan_credit",
		Status:          "invited",
		CreatedAt:       now,
	}
	if _, ok := a.candidateInvites[recruiterUserID]; !ok {
		a.candidateInvites[recruiterUserID] = map[string]CandidateInvite{}
	}
	a.candidateInvites[recruiterUserID][candidateUserID] = invite
	if err := a.persistCandidateInviteLocked(ctx, invite); err != nil {
		return CandidateDetailView{}, err
	}

	updatedMonetization := a.monetizationSummaryLocked(recruiterUserID)
	return a.candidateDetailLocked(recruiterUserID, candidate, updatedMonetization), nil
}

func (a *App) candidateDetailLocked(recruiterUserID string, candidate User, monetization MonetizationSummary) CandidateDetailView {
	preview := a.candidatePreviewLocked(recruiterUserID, candidate, monetization)
	detail := CandidateDetailView{
		Candidate:    preview,
		Monetization: monetization,
	}
	if !preview.Access.IsUnlocked {
		detail.LockedFields = []string{"contact", "linkedin", "profile", "skills", "room", "recent_submissions"}
		return detail
	}
	profile := a.profileViewLocked(candidate.ID)
	detail.Contact = &CandidateContact{Email: candidate.Email, LinkedInURL: profile.LinkedInURL}
	detail.Profile = &profile
	detail.Skills = a.listUserSkillsLocked(candidate.ID)
	detail.Room = a.listUserRoomItemsLocked(candidate.ID)
	detail.RecentSubmissions = a.candidateRecentSubmissionsLocked(candidate.ID)
	return detail
}

func (a *App) candidatePreviewLocked(recruiterUserID string, user User, monetization MonetizationSummary) CandidateView {
	profile := a.profileViewLocked(user.ID)
	summary := CandidateSummary{
		Score:           user.Profile.CurrentSkillScore,
		Percentile:      user.Profile.PercentileGlobal,
		ConfidenceScore: profile.ConfidenceScore,
		ConfidenceLevel: profile.ConfidenceLevel,
		LastActiveAt:    user.LastActiveAt,
		TasksCompleted:  user.Profile.CompletedChallenges,
	}
	return CandidateView{
		Summary:           summary,
		UserID:            user.ID,
		Username:          user.Username,
		Country:           user.Country,
		Access:            a.candidateAccessLocked(recruiterUserID, user.ID, monetization),
		CurrentSkillScore: user.Profile.CurrentSkillScore,
		PercentileGlobal:  user.Profile.PercentileGlobal,
		ConfidenceScore:   profile.ConfidenceScore,
		ConfidenceLevel:   profile.ConfidenceLevel,
		ConfidenceReasons: profile.ConfidenceReasons,
		LastActiveAt:      user.LastActiveAt,
		TasksSolved:       user.Profile.CompletedChallenges,
		RecentActivity:    a.recentActivityLocked(user.ID),
		Strengths:         a.skillSummaryLocked(user.ID, true),
		Weaknesses:        a.skillSummaryLocked(user.ID, false),
	}
}

func (a *App) candidateAccessLocked(recruiterUserID, candidateUserID string, monetization MonetizationSummary) CandidateAccess {
	unlock, unlocked := CandidateUnlock{}, false
	if entries, ok := a.candidateUnlocks[recruiterUserID]; ok {
		unlock, unlocked = entries[candidateUserID]
	}
	invite, invited := CandidateInvite{}, false
	if entries, ok := a.candidateInvites[recruiterUserID]; ok {
		invite, invited = entries[candidateUserID]
	}
	remaining := monetization.Entitlements.CandidateUnlocksPerMonth - monetization.Usage.CandidateUnlocksUsed
	if monetization.Entitlements.CandidateUnlocksPerMonth <= 0 {
		remaining = 0
	}
	if remaining < 0 {
		remaining = 0
	}
	remainingInvites := monetization.Entitlements.CandidateInvitesPerMonth - monetization.Usage.CandidateInvitesUsed
	if monetization.Entitlements.CandidateInvitesPerMonth <= 0 {
		remainingInvites = 0
	}
	if remainingInvites < 0 {
		remainingInvites = 0
	}
	var unlockedAt *time.Time
	if unlocked {
		timeCopy := unlock.CreatedAt
		unlockedAt = &timeCopy
	}
	var invitedAt *time.Time
	if invited {
		timeCopy := invite.CreatedAt
		invitedAt = &timeCopy
	}
	return CandidateAccess{
		IsUnlocked:       unlocked,
		UnlockRequired:   !unlocked,
		CanUnlock:        unlocked || remaining > 0,
		UnlockStatus:     firstNonEmpty(unlock.Status, "locked"),
		UnlockSource:     unlock.Source,
		UnlockedAt:       unlockedAt,
		RemainingUnlocks: remaining,
		IsInvited:        invited,
		CanInvite:        invited || (unlocked && remainingInvites > 0),
		InviteStatus:     firstNonEmpty(invite.Status, "not_invited"),
		InviteSource:     invite.Source,
		InvitedAt:        invitedAt,
		RemainingInvites: remainingInvites,
	}
}

func (a *App) candidateRecentSubmissionsLocked(userID string) []CandidateSubmissionSummary {
	summaries := make([]CandidateSubmissionSummary, 0)
	for _, instance := range a.instances {
		if instance.UserID != userID {
			continue
		}
		for _, submission := range a.submissions {
			if submission.ChallengeInstanceID != instance.ID {
				continue
			}
			evaluation := a.evaluations[submission.ID]
			template := a.templates[instance.TemplateID]
			summaries = append(summaries, CandidateSubmissionSummary{
				SubmissionID:        submission.ID,
				ChallengeInstanceID: submission.ChallengeInstanceID,
				TemplateID:          instance.TemplateID,
				TemplateTitle:       template.Title,
				Category:            template.Category,
				SubmittedAt:         submission.SubmittedAt,
				FinalScore:          evaluation.FinalScore,
				QualityScore:        evaluation.QualityScore,
				ExecutionCostScore:  firstNonZeroFloat(evaluation.ExecutionCostScore, evaluation.SpeedScore),
				ExecutionStatus:     submission.ExecutionStatus,
			})
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SubmittedAt.After(summaries[j].SubmittedAt)
	})
	if len(summaries) > 8 {
		summaries = summaries[:8]
	}
	return summaries
}

func (a *App) persistUserMonetizationLocked(ctx context.Context, userID string) error {
	if a.store == nil {
		return nil
	}
	subscription, ok := a.subscriptions[userID]
	if ok {
		if err := a.store.UpsertSubscription(ctx, subscription); err != nil {
			return err
		}
	}
	if cosmetics, ok := a.userCosmetics[userID]; ok {
		for _, item := range cosmetics {
			if err := a.store.UpsertUserCosmetic(ctx, item); err != nil {
				return err
			}
		}
	}
	if equipped, ok := a.equippedCosmetics[userID]; ok {
		for _, item := range equipped {
			if err := a.store.UpsertEquippedCosmetic(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) persistAIUsageLocked(ctx context.Context, event AIUsageEvent) error {
	if a.store == nil {
		return nil
	}
	return a.store.InsertAIUsageEvent(ctx, event)
}

func (a *App) persistCandidateUnlockLocked(ctx context.Context, unlock CandidateUnlock) error {
	if a.store == nil {
		return nil
	}
	return a.store.UpsertCandidateUnlock(ctx, unlock)
}

func (a *App) persistCandidateInviteLocked(ctx context.Context, invite CandidateInvite) error {
	if a.store == nil {
		return nil
	}
	return a.store.UpsertCandidateInvite(ctx, invite)
}

func usageUnits(units int) int {
	if units <= 0 {
		return 1
	}
	return units
}
