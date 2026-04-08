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

func usageUnits(units int) int {
	if units <= 0 {
		return 1
	}
	return units
}
