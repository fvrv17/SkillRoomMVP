package backend

import "time"

type Plan struct {
	ID                string           `json:"id"`
	Code              string           `json:"code"`
	Name              string           `json:"name"`
	Audience          string           `json:"audience"`
	Tier              string           `json:"tier"`
	Currency          string           `json:"currency"`
	MonthlyPriceCents int              `json:"monthly_price_cents"`
	Active            bool             `json:"active"`
	Features          []string         `json:"features,omitempty"`
	Entitlements      PlanEntitlements `json:"entitlements"`
	Metadata          map[string]any   `json:"metadata,omitempty"`
}

type PlanEntitlements struct {
	CandidatePreview          bool `json:"candidate_preview"`
	CandidateUnlocksPerMonth  int  `json:"candidate_unlocks_per_month"`
	HRAIActionsPerMonth       int  `json:"hr_ai_actions_per_month"`
	HRAdvancedFilters         bool `json:"hr_advanced_filters"`
	BusinessSeats             int  `json:"business_seats"`
	DeveloperHintsPerMonth    int  `json:"developer_hints_per_month"`
	DeveloperExplainsPerMonth int  `json:"developer_explains_per_month"`
	DeveloperAIFreeBeta       bool `json:"developer_ai_free_beta"`
	CosmeticCustomization     bool `json:"cosmetic_customization"`
	PremiumCosmetics          bool `json:"premium_cosmetics"`
}

type Subscription struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"user_id"`
	PlanID             string    `json:"plan_id"`
	PlanCode           string    `json:"plan_code"`
	Status             string    `json:"status"`
	Source             string    `json:"source"`
	AutoRenew          bool      `json:"auto_renew"`
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CandidateUnlock struct {
	ID              string    `json:"id"`
	RecruiterUserID string    `json:"recruiter_user_id"`
	CandidateUserID string    `json:"candidate_user_id"`
	Source          string    `json:"source"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type AIUsageEvent struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Scope     string         `json:"scope"`
	Action    string         `json:"action"`
	Units     int            `json:"units"`
	Context   map[string]any `json:"context_json,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type CosmeticCatalogItem struct {
	ID          string         `json:"id"`
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	SlotCode    string         `json:"slot_code"`
	Description string         `json:"description"`
	Audience    string         `json:"audience"`
	Rarity      string         `json:"rarity"`
	Premium     bool           `json:"premium"`
	AssetRef    string         `json:"asset_ref"`
	Active      bool           `json:"active"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UserCosmetic struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	CosmeticID   string    `json:"cosmetic_id"`
	CosmeticCode string    `json:"cosmetic_code"`
	Source       string    `json:"source"`
	OwnedAt      time.Time `json:"owned_at"`
}

type EquippedCosmetic struct {
	UserID       string    `json:"user_id"`
	SlotCode     string    `json:"slot_code"`
	CosmeticCode string    `json:"cosmetic_code"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MonetizationUsage struct {
	PeriodStart           time.Time `json:"period_start"`
	PeriodEnd             time.Time `json:"period_end"`
	CandidateUnlocksUsed  int       `json:"candidate_unlocks_used"`
	HRAIActionsUsed       int       `json:"hr_ai_actions_used"`
	DeveloperHintsUsed    int       `json:"developer_hints_used"`
	DeveloperExplainsUsed int       `json:"developer_explains_used"`
}

type MonetizationSummary struct {
	Plan         Plan              `json:"plan"`
	Subscription Subscription      `json:"subscription"`
	Entitlements PlanEntitlements  `json:"entitlements"`
	Usage        MonetizationUsage `json:"usage"`
	FeatureFlags []string          `json:"feature_flags,omitempty"`
}

type CosmeticInventoryResponse struct {
	Catalog  []CosmeticCatalogItem `json:"catalog"`
	Owned    []UserCosmetic        `json:"owned"`
	Equipped []EquippedCosmetic    `json:"equipped"`
}
