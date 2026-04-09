package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fvrv17/mvp/internal/evaluation"
	"github.com/fvrv17/mvp/internal/platform/httpx"
	"github.com/fvrv17/mvp/internal/platform/id"
	"github.com/fvrv17/mvp/internal/platform/security"
	runsvc "github.com/fvrv17/mvp/internal/runner"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type refreshSession struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

const (
	accessTokenCookieName  = "skillroom_access"
	refreshTokenCookieName = "skillroom_refresh"
	proxySecretHeaderName  = "X-SkillRoom-Proxy-Secret"
	rankingSnapshotTTL     = 30 * time.Second
	hrCandidatePageLimit   = 6
	hrLeaderboardPageLimit = 8
	hrPaginationMaxLimit   = 24
)

type rankingSnapshot struct {
	RefreshedAt  time.Time
	Global       []RankingEntry
	ByCountry    map[string][]RankingEntry
	CandidateIDs []string
}

type App struct {
	mu                 sync.RWMutex
	tokens             *security.TokenManager
	accessTTL          time.Duration
	refreshTTL         time.Duration
	store              *SQLStore
	ops                OpsStore
	ai                 AIProvider
	runner             runsvc.Engine
	metrics            *AppMetrics
	users              map[string]User
	emailIndex         map[string]string
	refreshSessions    map[string]refreshSession
	skills             map[string]Skill
	userSkills         map[string]map[string]UserSkill
	roomItems          map[string]RoomItem
	userRoomItems      map[string]map[string]UserRoomItem
	templates          map[string]ChallengeTemplate
	variants           map[string]ChallengeVariant
	instances          map[string]ChallengeInstance
	submissions        map[string]Submission
	evaluations        map[string]EvaluationResult
	telemetryEvents    map[string][]TelemetryEvent
	scoreEvents        map[string][]ScoreEvent
	scoreHistory       map[string][]float64
	friendships        map[string]map[string]Friendship
	chats              map[string]Chat
	directChats        map[string]string
	chatMessages       map[string][]ChatMessage
	companies          map[string]Company
	jobs               map[string]Job
	shortlists         []HRShortlist
	aiInteractions     []AIInteraction
	plans              map[string]Plan
	subscriptions      map[string]Subscription
	candidateUnlocks   map[string]map[string]CandidateUnlock
	candidateInvites   map[string]map[string]CandidateInvite
	aiUsageEvents      map[string][]AIUsageEvent
	cosmeticCatalog    map[string]CosmeticCatalogItem
	userCosmetics      map[string]map[string]UserCosmetic
	equippedCosmetics  map[string]map[string]EquippedCosmetic
	trustedProxyCIDRs  []netip.Prefix
	trustedProxySecret string
	rankingSnapshot    rankingSnapshot
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
	Country  string `json:"country"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	User         User      `json:"user"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"-"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UpdateProfileRequest struct {
	LinkedInURL string `json:"linkedin_url,omitempty"`
}

type StartChallengeRequest struct {
	TemplateID string `json:"template_id,omitempty"`
	Category   string `json:"category,omitempty"`
}

type SubmitChallengeRequest struct {
	Language    string            `json:"language"`
	RawCodeText string            `json:"raw_code_text,omitempty"`
	SourceFiles map[string]string `json:"source_files,omitempty"`
}

type TelemetryEventRequest struct {
	EventType     string         `json:"event_type"`
	OffsetSeconds int            `json:"offset_seconds"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type ChallengeInstanceView struct {
	Instance      ChallengeInstance `json:"instance"`
	TemplateID    string            `json:"template_id"`
	Title         string            `json:"title"`
	Description   string            `json:"description_md"`
	Category      string            `json:"category"`
	Difficulty    int               `json:"difficulty"`
	VisibleTests  map[string]string `json:"visible_tests,omitempty"`
	EditableFiles []string          `json:"editable_files,omitempty"`
	Variant       ChallengeVariant  `json:"variant"`
}

type FriendRequest struct {
	UserID string `json:"user_id"`
}

type ChatRequest struct {
	Body string `json:"body"`
}

type CompanyRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type JobRequest struct {
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	RequiredScore  float64            `json:"required_score"`
	RequiredSkills map[string]float64 `json:"required_skills_json"`
}

type ShortlistRequest struct {
	CompanyID string `json:"company_id"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
}

type EquipCosmeticRequest struct {
	CosmeticCode string `json:"cosmetic_code"`
}

func NewApp(secret, issuer string) *App {
	app := &App{
		tokens:            security.NewTokenManager(secret, issuer),
		accessTTL:         15 * time.Minute,
		refreshTTL:        7 * 24 * time.Hour,
		ops:               NewMemoryOpsStore(),
		ai:                NewDeterministicAIProvider(),
		metrics:           NewAppMetrics(),
		users:             map[string]User{},
		emailIndex:        map[string]string{},
		refreshSessions:   map[string]refreshSession{},
		skills:            map[string]Skill{},
		userSkills:        map[string]map[string]UserSkill{},
		roomItems:         map[string]RoomItem{},
		userRoomItems:     map[string]map[string]UserRoomItem{},
		templates:         map[string]ChallengeTemplate{},
		variants:          map[string]ChallengeVariant{},
		instances:         map[string]ChallengeInstance{},
		submissions:       map[string]Submission{},
		evaluations:       map[string]EvaluationResult{},
		telemetryEvents:   map[string][]TelemetryEvent{},
		scoreEvents:       map[string][]ScoreEvent{},
		scoreHistory:      map[string][]float64{},
		friendships:       map[string]map[string]Friendship{},
		chats:             map[string]Chat{},
		directChats:       map[string]string{},
		chatMessages:      map[string][]ChatMessage{},
		companies:         map[string]Company{},
		jobs:              map[string]Job{},
		aiInteractions:    []AIInteraction{},
		plans:             map[string]Plan{},
		subscriptions:     map[string]Subscription{},
		candidateUnlocks:  map[string]map[string]CandidateUnlock{},
		candidateInvites:  map[string]map[string]CandidateInvite{},
		aiUsageEvents:     map[string][]AIUsageEvent{},
		cosmeticCatalog:   map[string]CosmeticCatalogItem{},
		userCosmetics:     map[string]map[string]UserCosmetic{},
		equippedCosmetics: map[string]map[string]EquippedCosmetic{},
	}

	app.seedSkillsAndRoom()
	app.seedMonetization()
	for _, template := range DefaultChallengeTemplates() {
		app.templates[template.ID] = template
	}
	return app
}

func NewPersistentApp(ctx context.Context, secret, issuer, dsn string) (*App, error) {
	app := NewApp(secret, issuer)
	store, err := OpenSQLStore(ctx, dsn)
	if err != nil {
		return nil, err
	}
	app.store = store
	if err := store.SyncCatalog(ctx, app); err != nil {
		return nil, err
	}
	if err := store.LoadInto(ctx, app); err != nil {
		return nil, err
	}
	if err := store.SyncCatalog(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (a *App) SetOpsStore(store OpsStore) {
	if store == nil {
		a.ops = NewMemoryOpsStore()
		return
	}
	a.ops = store
}

func (a *App) SetAIProvider(provider AIProvider) {
	if provider == nil {
		a.ai = NewDeterministicAIProvider()
		return
	}
	a.ai = provider
}

func (a *App) SetChallengeRunner(engine runsvc.Engine) {
	a.runner = engine
}

func (a *App) SetTrustedProxyPolicy(secret string, cidrs []string) error {
	parsed := make([]netip.Prefix, 0, len(cidrs))
	for _, candidate := range cidrs {
		value := strings.TrimSpace(candidate)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return fmt.Errorf("parse trusted proxy cidr %q: %w", value, err)
			}
			parsed = append(parsed, prefix)
			continue
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("parse trusted proxy address %q: %w", value, err)
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		parsed = append(parsed, netip.PrefixFrom(addr, bits))
	}
	a.trustedProxySecret = strings.TrimSpace(secret)
	a.trustedProxyCIDRs = parsed
	return nil
}

func (a *App) Close() error {
	if a == nil || a.store == nil {
		return nil
	}
	return a.store.Close()
}

func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(a.observeRequests)
	r.Use(middleware.Recoverer)

	r.Get("/livez", a.handleLiveness)
	r.Get("/readyz", a.handleReadiness)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "backend"})
	})
	r.Get("/metrics", a.handleMetrics)
	a.mountFrontend(r)

	r.Route("/v1", func(r chi.Router) {
		r.With(a.rateLimitByIP("auth-register", 10, time.Minute)).Post("/auth/register", a.handleRegister)
		r.With(a.rateLimitByIP("auth-login", 30, time.Minute)).Post("/auth/login", a.handleLogin)
		r.With(a.rateLimitByIP("auth-refresh", 30, time.Minute)).Post("/auth/refresh", a.handleRefresh)
		r.With(a.rateLimitByIP("auth-logout", 60, time.Minute)).Post("/auth/logout", a.handleLogout)

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Use(a.rateLimitByUser("user-api", 240, time.Minute))
			r.Get("/me", a.handleMe)
			r.Get("/monetization/summary", a.handleMonetizationSummary)
		})

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Use(a.requireRoles(RoleUser, RoleAdmin))
			r.Use(a.rateLimitByUser("developer-api", 240, time.Minute))
			r.Get("/profile", a.handleProfile)
			r.Patch("/profile", a.handleUpdateProfile)
			r.Get("/skills", a.handleSkills)
			r.Get("/room", a.handleRoom)
			r.Get("/dev/cosmetics/catalog", a.handleListCosmeticCatalog)
			r.Get("/dev/cosmetics/inventory", a.handleCosmeticInventory)
			r.Post("/dev/cosmetics/equip", a.handleEquipCosmetic)
			r.Get("/challenges/templates", a.handleListTemplates)
			r.Post("/challenges/instances", a.handleStartChallenge)
			r.Get("/challenges/instances/{instanceID}", a.handleGetInstance)
			r.With(a.rateLimitByUser("challenge-telemetry", 600, time.Minute)).Post("/challenges/instances/{instanceID}/telemetry", a.handleRecordTelemetry)
			r.With(a.rateLimitByUser("challenge-run", 120, time.Minute)).Post("/challenges/instances/{instanceID}/runs", a.handleRunChallenge)
			r.With(a.rateLimitByUser("challenge-submit", 40, time.Minute)).Post("/challenges/instances/{instanceID}/submissions", a.handleSubmitChallenge)
			r.With(a.rateLimitByUser("ai-hint", 20, time.Minute)).Post("/ai/challenges/{instanceID}/hint", a.handleAIHint)
			r.With(a.rateLimitByUser("ai-explain", 30, time.Minute)).Post("/ai/challenges/{instanceID}/explain", a.handleAIExplain)
			r.Post("/friends/{userID}/request", a.handleFriendRequest)
			r.Post("/friends/{userID}/accept", a.handleFriendAccept)
			r.Get("/rankings/global", a.handleGlobalRanking)
			r.Get("/rankings/country", a.handleCountryRanking)
			r.Get("/rankings/friends", a.handleFriendsRanking)
			r.Get("/chat/direct/{userID}/messages", a.handleListDirectMessages)
			r.With(a.rateLimitByUser("chat-direct", 120, time.Minute)).Post("/chat/direct/{userID}/messages", a.handleCreateDirectMessage)
		})

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Use(a.requireRoles(RoleHR, RoleAdmin))
			r.Use(a.rateLimitByUser("hr-api", 180, time.Minute))
			r.Post("/hr/companies", a.handleCreateCompany)
			r.Post("/hr/companies/{companyID}/jobs", a.handleCreateJob)
			r.Get("/hr/candidates", a.handleSearchCandidates)
			r.Get("/hr/leaderboard", a.handleHRLeaderboard)
			r.Get("/hr/candidates/{userID}", a.handleGetCandidateDetail)
			r.Post("/hr/candidates/{userID}/unlock", a.handleUnlockCandidate)
			r.Post("/hr/candidates/{userID}/invite", a.handleInviteCandidate)
			r.Post("/hr/shortlists", a.handleShortlistCandidate)
			r.With(a.rateLimitByUser("ai-mutation-preview", 30, time.Minute)).Post("/hr/ai/templates/{templateID}/mutation-preview", a.handleAIMutationPreview)
		})
	})

	return r
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := a.register(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.writeAuthCookies(w, r, resp)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := a.login(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	a.writeAuthCookies(w, r, resp)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := a.refresh(r.Context(), a.refreshTokenFromRequest(r, req.RefreshToken))
	if err != nil {
		a.metrics.IncrementEvent("refresh_failed")
		a.clearAuthCookies(w, r)
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	a.writeAuthCookies(w, r, resp)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.logout(r.Context(), a.refreshTokenFromRequest(r, req.RefreshToken)); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.clearAuthCookies(w, r)
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "signed_out"})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	user.Profile = a.profileView(user.ID)
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.profileView(user.ID))
}

func (a *App) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req UpdateProfileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := a.updateProfile(r.Context(), user.ID, req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, profile)
}

func (a *App) handleSkills(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"skills": a.listUserSkills(user.ID)})
}

func (a *App) handleRoom(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	view, err := a.roomView(r.Context(), user.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a *App) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := a.listTemplates(r.Context(), r.URL.Query().Get("category"))
	if err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"templates": templates})
}

func (a *App) handleStartChallenge(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req StartChallengeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	view, err := a.startChallenge(r.Context(), user.ID, req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a *App) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	view, err := a.challengeInstanceView(r.Context(), user.ID, chi.URLParam(r, "instanceID"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a *App) handleRecordTelemetry(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req TelemetryEventRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := a.recordTelemetry(r.Context(), user.ID, chi.URLParam(r, "instanceID"), req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"status": "accepted", "summary": summary})
}

func (a *App) handleAIHint(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req AIHintRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := a.aiHint(r.Context(), user.ID, chi.URLParam(r, "instanceID"), req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "hint limit") {
			status = http.StatusTooManyRequests
		}
		httpx.WriteError(w, status, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (a *App) handleAIExplain(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req AIExplanationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := a.aiExplain(r.Context(), user.ID, chi.URLParam(r, "instanceID"), req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (a *App) handleSubmitChallenge(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req SubmitChallengeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.submitChallenge(r.Context(), user.ID, chi.URLParam(r, "instanceID"), req)
	if err != nil {
		a.observeChallengeExecutionError(err)
		httpx.WriteError(w, challengeExecutionStatus(err), err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (a *App) handleRunChallenge(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req SubmitChallengeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.runChallenge(r.Context(), user.ID, chi.URLParam(r, "instanceID"), req)
	if err != nil {
		a.observeChallengeExecutionError(err)
		httpx.WriteError(w, challengeExecutionStatus(err), err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func challengeExecutionStatus(err error) int {
	switch {
	case errors.Is(err, errRunnerTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, errRunnerUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

func (a *App) handleFriendRequest(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err := a.requestFriend(r.Context(), user.ID, chi.URLParam(r, "userID")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "pending"})
}

func (a *App) handleFriendAccept(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err := a.acceptFriend(r.Context(), user.ID, chi.URLParam(r, "userID")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (a *App) handleGlobalRanking(w http.ResponseWriter, r *http.Request) {
	rankings, err := a.rankingsCached(r.Context(), "global", "", "")
	if err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rankings": rankings})
}

func (a *App) handleCountryRanking(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	rankings, err := a.rankingsCached(r.Context(), "country", user.Country, "")
	if err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rankings": rankings})
}

func (a *App) handleFriendsRanking(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	rankings, err := a.rankingsCached(r.Context(), "friends", "", user.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rankings": rankings})
}

func (a *App) handleListDirectMessages(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	messages, err := a.directMessages(user.ID, chi.URLParam(r, "userID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (a *App) handleCreateDirectMessage(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req ChatRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	message, err := a.createDirectMessage(r.Context(), user.ID, chi.URLParam(r, "userID"), req.Body)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, message)
}

func (a *App) handleAIMutationPreview(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req AIMutationPreviewRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := a.aiMutationPreview(r.Context(), user.ID, chi.URLParam(r, "templateID"), req)
	if err != nil {
		if errors.Is(err, errHRAIQuotaExceeded) {
			httpx.WriteError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (a *App) handleCreateCompany(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req CompanyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	company, err := a.createCompany(r.Context(), user.ID, req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, company)
}

func (a *App) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req JobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	job, err := a.createJob(r.Context(), user.ID, chi.URLParam(r, "companyID"), req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, job)
}

func (a *App) handleSearchCandidates(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	minScore, _ := strconv.ParseFloat(r.URL.Query().Get("min_score"), 64)
	topPercent, _ := strconv.ParseFloat(r.URL.Query().Get("top_percent"), 64)
	activeDays, _ := strconv.Atoi(r.URL.Query().Get("active_days"))
	minConfidence, _ := strconv.ParseFloat(r.URL.Query().Get("min_confidence"), 64)
	limit, offset := parsePaginationQuery(r, hrCandidatePageLimit)
	results, pagination, monetization, err := a.searchCandidates(r.Context(), user.ID, minScore, minConfidence, topPercent, activeDays, limit, offset)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"candidates":   results,
		"pagination":   pagination,
		"monetization": monetization,
	})
}

func (a *App) handleHRLeaderboard(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	limit, offset := parsePaginationQuery(r, hrLeaderboardPageLimit)
	results, pagination, monetization, err := a.searchCandidates(r.Context(), user.ID, 0, 0, 0, 0, limit, offset)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rankings":     results,
		"pagination":   pagination,
		"monetization": monetization,
	})
}

func (a *App) handleGetCandidateDetail(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	detail, err := a.candidateDetail(r.Context(), user.ID, chi.URLParam(r, "userID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func (a *App) handleUnlockCandidate(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	detail, err := a.unlockCandidate(r.Context(), user.ID, chi.URLParam(r, "userID"))
	if err != nil {
		a.metrics.IncrementEvent("candidate_unlock_denied")
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.metrics.IncrementEvent("candidate_unlock_succeeded")
	httpx.WriteJSON(w, http.StatusCreated, detail)
}

func (a *App) handleInviteCandidate(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	detail, err := a.inviteCandidate(r.Context(), user.ID, chi.URLParam(r, "userID"))
	if err != nil {
		a.metrics.IncrementEvent("candidate_invite_denied")
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.metrics.IncrementEvent("candidate_invite_succeeded")
	httpx.WriteJSON(w, http.StatusCreated, detail)
}

func (a *App) handleShortlistCandidate(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticatedUser(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req ShortlistRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	entry, err := a.shortlistCandidate(r.Context(), user.ID, req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, entry)
}

func (a *App) register(ctx context.Context, req RegisterRequest) (AuthResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return AuthResponse{}, errors.New("email, username, and password are required")
	}
	role := req.Role
	if role == "" {
		role = RoleUser
	}
	if role != RoleUser && role != RoleHR && role != RoleAdmin {
		return AuthResponse{}, errors.New("invalid role")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, exists := a.emailIndex[email]; exists {
		return AuthResponse{}, errors.New("email already exists")
	}
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return AuthResponse{}, err
	}
	now := time.Now().UTC()
	user := User{
		ID:           id.New("usr"),
		Email:        email,
		Username:     strings.TrimSpace(req.Username),
		PasswordHash: hash,
		Role:         role,
		Country:      strings.ToUpper(strings.TrimSpace(req.Country)),
		CreatedAt:    now,
		LastActiveAt: now,
		Profile: UserProfile{
			UserID:          "",
			SelectedTrack:   "react",
			ConfidenceScore: 50,
			UpdatedAt:       now,
		},
	}
	user.Profile.UserID = user.ID
	a.users[user.ID] = user
	a.emailIndex[email] = user.ID
	a.initUserStateLocked(user.ID, now)
	a.initUserMonetizationLocked(user.ID, role, now)
	if err := a.persistUserStateLocked(ctx, user.ID); err != nil {
		return AuthResponse{}, err
	}
	if err := a.persistUserMonetizationLocked(ctx, user.ID); err != nil {
		return AuthResponse{}, err
	}
	a.invalidateRankingCachesLocked(ctx, user.ID)
	return a.mintAuthLocked(ctx, user)
}

func (a *App) login(ctx context.Context, req LoginRequest) (AuthResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	userID, ok := a.emailIndex[strings.ToLower(strings.TrimSpace(req.Email))]
	if !ok {
		return AuthResponse{}, errors.New("invalid credentials")
	}
	user := a.users[userID]
	if err := security.ComparePassword(user.PasswordHash, req.Password); err != nil {
		return AuthResponse{}, errors.New("invalid credentials")
	}
	user.LastActiveAt = time.Now().UTC()
	a.users[userID] = user
	if err := a.persistUserStateLocked(ctx, userID); err != nil {
		return AuthResponse{}, err
	}
	if err := a.persistUserMonetizationLocked(ctx, userID); err != nil {
		return AuthResponse{}, err
	}
	return a.mintAuthLocked(ctx, user)
}

func (a *App) refresh(ctx context.Context, token string) (AuthResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, ok := a.refreshSessions[token]
	if !ok || time.Now().UTC().After(session.ExpiresAt) {
		return AuthResponse{}, errors.New("invalid refresh token")
	}
	delete(a.refreshSessions, token)
	if err := a.deleteRefreshSessionLocked(ctx, token); err != nil {
		return AuthResponse{}, err
	}
	user := a.users[session.UserID]
	return a.mintAuthLocked(ctx, user)
}

func (a *App) logout(ctx context.Context, token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	delete(a.refreshSessions, token)
	return a.deleteRefreshSessionLocked(ctx, token)
}

func (a *App) mintAuthLocked(ctx context.Context, user User) (AuthResponse, error) {
	accessToken, claims, err := a.tokens.MintAccessToken(user.ID, string(user.Role), "", a.accessTTL)
	if err != nil {
		return AuthResponse{}, err
	}
	refreshToken := id.New("rfr")
	a.refreshSessions[refreshToken] = refreshSession{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(a.refreshTTL),
	}
	if err := a.persistRefreshSessionLocked(ctx, refreshToken); err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Unix(claims.ExpiresAt, 0).UTC(),
	}, nil
}

func (a *App) refreshTokenFromRequest(r *http.Request, bodyToken string) string {
	if strings.TrimSpace(bodyToken) != "" {
		return strings.TrimSpace(bodyToken)
	}
	if r != nil {
		if cookie, err := r.Cookie(refreshTokenCookieName); err == nil {
			return strings.TrimSpace(cookie.Value)
		}
	}
	return ""
}

func (a *App) writeAuthCookies(w http.ResponseWriter, r *http.Request, resp AuthResponse) {
	secure := authCookieSecure(r)
	w.Header().Set("Cache-Control", "no-store")
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    resp.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  resp.ExpiresAt,
		MaxAge:   maxInt(int(time.Until(resp.ExpiresAt).Seconds()), 0),
	})
	if strings.TrimSpace(resp.RefreshToken) != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     refreshTokenCookieName,
			Value:    resp.RefreshToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   secure,
			Expires:  time.Now().UTC().Add(a.refreshTTL),
			MaxAge:   maxInt(int(a.refreshTTL.Seconds()), 0),
		})
	}
}

func (a *App) clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	secure := authCookieSecure(r)
	for _, cookieName := range []string{accessTokenCookieName, refreshTokenCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
			Expires:  time.Unix(0, 0).UTC(),
			MaxAge:   -1,
		})
	}
}

func authCookieSecure(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func (a *App) authenticatedUser(r *http.Request) (User, error) {
	claims, err := a.tokens.ClaimsFromRequest(r)
	if err != nil {
		return User{}, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	user, ok := a.users[claims.Subject]
	if !ok {
		return User{}, errors.New("user not found")
	}
	return user, nil
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := a.tokens.ClaimsFromRequest(r); err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) requireRoles(roles ...Role) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[string(role)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := a.tokens.ClaimsFromRequest(r)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, err.Error())
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				httpx.WriteError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *App) rateLimitByIP(scope string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return a.rateLimitMiddleware(scope, limit, window, func(r *http.Request) (string, error) {
		if ip := strings.TrimSpace(a.realIP(r)); ip != "" {
			return ip, nil
		}
		return "unknown", nil
	})
}

func (a *App) rateLimitByUser(scope string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return a.rateLimitMiddleware(scope, limit, window, func(r *http.Request) (string, error) {
		claims, err := a.tokens.ClaimsFromRequest(r)
		if err != nil {
			return "", err
		}
		return claims.Subject, nil
	})
}

func (a *App) rateLimitMiddleware(scope string, limit int, window time.Duration, subjectFn func(*http.Request) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := subjectFn(r)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, err.Error())
				return
			}
			decision, err := a.ops.Allow(r.Context(), fmt.Sprintf("rate:%s:%s", scope, subject), limit, window)
			if err != nil {
				httpx.WriteError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
				return
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
			if !decision.Allowed {
				httpx.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *App) listTemplates(ctx context.Context, category string) ([]ChallengeTemplate, error) {
	cacheKey := "templates:all"
	if category != "" {
		cacheKey = "templates:" + category
	}
	return cacheJSON(ctx, a.ops, cacheKey, 5*time.Minute, func() ([]ChallengeTemplate, error) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		templates := make([]ChallengeTemplate, 0, len(a.templates))
		for _, template := range a.templates {
			if category == "" || template.Category == category {
				templates = append(templates, template)
			}
		}
		sort.Slice(templates, func(i, j int) bool {
			if templates[i].Category == templates[j].Category {
				return templates[i].Title < templates[j].Title
			}
			return templates[i].Category < templates[j].Category
		})
		return templates, nil
	})
}

func (a *App) rankingsCached(ctx context.Context, kind, country, userID string) ([]RankingEntry, error) {
	cacheKey := fmt.Sprintf("rankings:%s:%s:%s", kind, country, userID)
	return cacheJSON(ctx, a.ops, cacheKey, 45*time.Second, func() ([]RankingEntry, error) {
		return a.rankings(ctx, kind, country, userID)
	})
}

func (a *App) listTemplatesLocked(category string) []ChallengeTemplate {
	a.mu.RLock()
	defer a.mu.RUnlock()
	templates := make([]ChallengeTemplate, 0, len(a.templates))
	for _, template := range a.templates {
		if category == "" || template.Category == category {
			templates = append(templates, template)
		}
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Category == templates[j].Category {
			return templates[i].Title < templates[j].Title
		}
		return templates[i].Category < templates[j].Category
	})
	return templates
}

func (a *App) startChallenge(ctx context.Context, userID string, req StartChallengeRequest) (ChallengeInstanceView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	templateDef, err := a.pickTemplateLocked(req)
	if err != nil {
		return ChallengeInstanceView{}, err
	}
	attemptNumber := a.nextAttemptNumberLocked(userID, templateDef.ID)
	seed := deterministicVariantSeed(userID, templateDef.ID, attemptNumber)
	variant := MutateTemplate(templateDef, seed)
	a.variants[variant.ID] = variant

	now := time.Now().UTC()
	instance := ChallengeInstance{
		ID:            id.New("chi"),
		UserID:        userID,
		TemplateID:    templateDef.ID,
		VariantID:     variant.ID,
		Category:      templateDef.Category,
		StartedAt:     now,
		ExpiresAt:     now.Add(90 * time.Minute),
		Status:        "assigned",
		AttemptNumber: attemptNumber,
		VisibleFiles:  buildVisibleFiles(variant),
	}
	a.instances[instance.ID] = instance
	user := a.users[userID]
	user.LastActiveAt = now
	a.users[userID] = user
	if err := a.persistUserStateLocked(ctx, userID); err != nil {
		return ChallengeInstanceView{}, err
	}
	if err := a.persistVariantLocked(ctx, variant.ID); err != nil {
		return ChallengeInstanceView{}, err
	}
	if err := a.persistInstanceLocked(ctx, instance.ID); err != nil {
		return ChallengeInstanceView{}, err
	}
	view := a.challengeViewLocked(instance, templateDef, variant)
	a.cacheChallengeView(ctx, view)
	return view, nil
}

func (a *App) challengeInstanceView(ctx context.Context, userID, instanceID string) (ChallengeInstanceView, error) {
	cacheKey := "challenge:view:" + instanceID
	if a.ops != nil {
		if payload, ok, err := a.ops.Get(ctx, cacheKey); err == nil && ok {
			var cached ChallengeInstanceView
			if err := json.Unmarshal(payload, &cached); err == nil && cached.Instance.UserID == userID {
				return cached, nil
			}
		}
	}

	a.mu.RLock()
	instance, ok := a.instances[instanceID]
	if !ok || instance.UserID != userID {
		a.mu.RUnlock()
		return ChallengeInstanceView{}, errors.New("challenge instance not found")
	}
	templateDef := a.templates[instance.TemplateID]
	variant := a.variants[instance.VariantID]
	view := a.challengeViewLocked(instance, templateDef, variant)
	a.mu.RUnlock()

	if a.ops != nil {
		if payload, err := json.Marshal(view); err == nil {
			_ = a.ops.Set(ctx, cacheKey, payload, 2*time.Hour)
		}
	}
	return view, nil
}

func (a *App) recordTelemetry(ctx context.Context, userID, instanceID string, req TelemetryEventRequest) (TelemetrySummary, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	instance, ok := a.instances[instanceID]
	if !ok || instance.UserID != userID {
		return TelemetrySummary{}, errors.New("challenge instance not found")
	}
	if instance.Status == "done" {
		return TelemetrySummary{}, errors.New("challenge instance already completed")
	}
	eventType := strings.ToLower(strings.TrimSpace(req.EventType))
	switch eventType {
	case "input", "paste", "focus_lost", "focus_gained", "snapshot":
	default:
		return TelemetrySummary{}, errors.New("invalid telemetry event type")
	}
	if req.OffsetSeconds < 0 {
		return TelemetrySummary{}, errors.New("offset_seconds must be non-negative")
	}
	event := TelemetryEvent{
		ID:                  id.New("tel"),
		ChallengeInstanceID: instanceID,
		UserID:              userID,
		EventType:           eventType,
		OffsetSeconds:       req.OffsetSeconds,
		Payload:             req.Payload,
		CreatedAt:           time.Now().UTC(),
	}
	a.telemetryEvents[instanceID] = append(a.telemetryEvents[instanceID], event)
	if err := a.persistTelemetryEventLocked(ctx, event); err != nil {
		return TelemetrySummary{}, err
	}
	return summarizeTelemetry(instance, a.telemetryEvents[instanceID], event.CreatedAt), nil
}

func (a *App) aiHint(ctx context.Context, userID, instanceID string, req AIHintRequest) (AIHintResponse, error) {
	a.mu.RLock()
	instance, ok := a.instances[instanceID]
	if !ok || instance.UserID != userID {
		a.mu.RUnlock()
		return AIHintResponse{}, errors.New("challenge instance not found")
	}
	user := a.users[userID]
	templateDef := a.templates[instance.TemplateID]
	variant := a.variants[instance.VariantID]
	usedHints := a.countAIInteractionsLocked(userID, instanceID, "hint")
	if usedHints >= maxHintsPerChallenge {
		a.mu.RUnlock()
		return AIHintResponse{}, errors.New("hint limit reached")
	}
	submission, evaluation, hasEvaluation := a.latestSubmissionAndEvaluationLocked(instanceID, "")
	summary := summarizeTelemetry(instance, a.telemetryEvents[instanceID], time.Now().UTC())
	var submissionPtr *Submission
	var evaluationPtr *EvaluationResult
	var antiCheat *AntiCheatAssessment
	if hasEvaluation {
		submissionCopy := submission
		evaluationCopy := evaluation
		antiCheatCopy := antiCheatFromEvaluationReport(evaluation.Report)
		submissionPtr = &submissionCopy
		evaluationPtr = &evaluationCopy
		antiCheat = &antiCheatCopy
	}
	provider := a.ai
	a.mu.RUnlock()

	if provider == nil {
		provider = NewDeterministicAIProvider()
	}
	response, err := provider.GenerateHint(ctx, AIHintContext{
		User:       user,
		Instance:   instance,
		Template:   templateDef,
		Variant:    variant,
		Submission: submissionPtr,
		Evaluation: evaluationPtr,
		Telemetry:  summary,
		UsedHints:  usedHints,
		Request:    req,
		AntiCheat:  antiCheat,
	})
	if err != nil {
		return AIHintResponse{}, err
	}
	response.UsedHints = usedHints + 1
	response.RemainingHints = maxHintsPerChallenge - response.UsedHints

	interaction := AIInteraction{
		ID:                  id.New("aii"),
		UserID:              userID,
		ChallengeInstanceID: instanceID,
		TemplateID:          templateDef.ID,
		InteractionType:     "hint",
		Input: map[string]any{
			"focus_area": req.FocusArea,
			"question":   req.Question,
			"used_hints": response.UsedHints,
		},
		Output:    structToMap(response),
		Provider:  response.Provider,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.recordAIInteraction(ctx, interaction); err != nil {
		return AIHintResponse{}, err
	}
	if err := a.recordAIUsage(ctx, AIUsageEvent{
		UserID: userID,
		Scope:  "developer",
		Action: "developer_hint",
		Units:  1,
		Context: map[string]any{
			"challenge_instance_id": instanceID,
			"template_id":           templateDef.ID,
		},
	}); err != nil {
		return AIHintResponse{}, err
	}
	return response, nil
}

func (a *App) aiExplain(ctx context.Context, userID, instanceID string, req AIExplanationRequest) (AIExplanationResponse, error) {
	a.mu.RLock()
	instance, ok := a.instances[instanceID]
	if !ok || instance.UserID != userID {
		a.mu.RUnlock()
		return AIExplanationResponse{}, errors.New("challenge instance not found")
	}
	user := a.users[userID]
	templateDef := a.templates[instance.TemplateID]
	variant := a.variants[instance.VariantID]
	submission, evaluation, ok := a.latestSubmissionAndEvaluationLocked(instanceID, req.SubmissionID)
	if !ok {
		a.mu.RUnlock()
		return AIExplanationResponse{}, errors.New("submission evaluation not found")
	}
	antiCheat := antiCheatFromEvaluationReport(evaluation.Report)
	provider := a.ai
	a.mu.RUnlock()

	if provider == nil {
		provider = NewDeterministicAIProvider()
	}
	response, err := provider.ExplainEvaluation(ctx, AIExplanationContext{
		User:       user,
		Instance:   instance,
		Template:   templateDef,
		Variant:    variant,
		Submission: submission,
		Evaluation: evaluation,
		AntiCheat:  antiCheat,
	})
	if err != nil {
		return AIExplanationResponse{}, err
	}

	interaction := AIInteraction{
		ID:                  id.New("aii"),
		UserID:              userID,
		ChallengeInstanceID: instanceID,
		TemplateID:          templateDef.ID,
		InteractionType:     "explain",
		Input: map[string]any{
			"submission_id": submission.ID,
		},
		Output:    structToMap(response),
		Provider:  response.Provider,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.recordAIInteraction(ctx, interaction); err != nil {
		return AIExplanationResponse{}, err
	}
	if err := a.recordAIUsage(ctx, AIUsageEvent{
		UserID: userID,
		Scope:  "developer",
		Action: "developer_explain",
		Units:  1,
		Context: map[string]any{
			"challenge_instance_id": instanceID,
			"template_id":           templateDef.ID,
			"submission_id":         submission.ID,
		},
	}); err != nil {
		return AIExplanationResponse{}, err
	}
	return response, nil
}

func (a *App) aiMutationPreview(ctx context.Context, userID, templateID string, req AIMutationPreviewRequest) (AIMutationPreviewResponse, error) {
	a.mu.RLock()
	user, ok := a.users[userID]
	if !ok {
		a.mu.RUnlock()
		return AIMutationPreviewResponse{}, errors.New("user not found")
	}
	templateDef, ok := a.templates[templateID]
	if !ok {
		a.mu.RUnlock()
		return AIMutationPreviewResponse{}, errors.New("template not found")
	}
	if err := a.enforceHRAIQuotaLocked(userID, 1); err != nil {
		a.mu.RUnlock()
		return AIMutationPreviewResponse{}, err
	}
	provider := a.ai
	a.mu.RUnlock()

	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UTC().UnixNano()
	}
	variant := MutateTemplate(templateDef, seed)
	if provider == nil {
		provider = NewDeterministicAIProvider()
	}
	response, err := provider.PreviewMutation(ctx, AIMutationContext{
		Requester: user,
		Template:  templateDef,
		Variant:   variant,
		Seed:      seed,
	})
	if err != nil {
		return AIMutationPreviewResponse{}, err
	}

	interaction := AIInteraction{
		ID:              id.New("aii"),
		UserID:          userID,
		TemplateID:      templateID,
		InteractionType: "mutation_preview",
		Input: map[string]any{
			"seed": seed,
		},
		Output:    structToMap(response),
		Provider:  response.Provider,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.recordAIInteraction(ctx, interaction); err != nil {
		return AIMutationPreviewResponse{}, err
	}
	if err := a.recordAIUsage(ctx, AIUsageEvent{
		UserID: userID,
		Scope:  "hr",
		Action: "hr_mutation_preview",
		Units:  1,
		Context: map[string]any{
			"template_id": templateID,
			"seed":        seed,
		},
	}); err != nil {
		return AIMutationPreviewResponse{}, err
	}
	return response, nil
}

func (a *App) evaluateSubmissionLocked(userID string, templateDef ChallengeTemplate, submission Submission, report RunnerReport, antiCheat AntiCheatAssessment) EvaluationResult {
	runResult := runsvc.RunResult{
		Passed:        report.TestsPassed,
		Failed:        report.TestsTotal - report.TestsPassed,
		HiddenPassed:  report.HiddenPassed,
		HiddenFailed:  report.HiddenFailed,
		QualityPassed: report.QualityPassed,
		QualityFailed: report.QualityFailed,
		TestResults:   runnerTestResultsFromChecks(report.Checks),
		Lint: runsvc.LintResult{
			ErrorCount:   report.LintErrors,
			WarningCount: report.LintWarnings,
		},
		Errors: append([]string(nil), report.Errors...),
	}
	runResult.ExecutionCostMS = report.ExecutionCostMS
	if runResult.ExecutionCostMS <= 0 {
		runResult.ExecutionCostMS = report.ExecutionTimeMS
	}
	runResult.ExecutionTimeMS = report.ExecutionTimeMS
	if runResult.ExecutionTimeMS <= 0 {
		runResult.ExecutionTimeMS = runResult.ExecutionCostMS
	}

	breakdown := evaluation.Score(evaluation.ScoreInput{
		Result:            runResult,
		ExecutionBaseline: templateDef.EvaluationConfig.MedianExecMS,
		History:           append([]float64(nil), a.scoreHistory[userID]...),
		QualityCheckIDs:   append([]string(nil), templateDef.EvaluationConfig.QualityCheckIDs...),
		LintWeight:        templateDef.EvaluationConfig.LintQualityWeight,
		TaskWeight:        templateDef.EvaluationConfig.TaskQualityWeight,
	})

	reportJSON := map[string]any{
		"tests_passed":         report.TestsPassed,
		"tests_total":          report.TestsTotal,
		"correctness":          breakdown.Correctness,
		"lint_quality":         breakdown.LintQuality,
		"task_quality":         breakdown.TaskQuality,
		"quality":              breakdown.Quality,
		"execution_cost_score": breakdown.ExecutionCost,
		"runtime_efficiency":   breakdown.ExecutionCost,
		"speed":                breakdown.ExecutionCost,
		"consistency":          breakdown.Consistency,
		"lint_errors":          report.LintErrors,
		"lint_warnings":        report.LintWarnings,
		"execution_cost_ms":    runResult.ExecutionCostMS,
		"execution_time_ms":    report.ExecutionTimeMS,
		"baseline_exec_ms":     templateDef.EvaluationConfig.MedianExecMS,
		"edit_count":           report.EditCount,
		"paste_events":         report.PasteEvents,
		"focus_loss_events":    report.FocusLossEvents,
		"snapshot_events":      report.SnapshotEvents,
		"first_input_secs":     report.FirstInputSeconds,
		"attempt_number":       report.AttemptNumber,
		"hidden_passed":        report.HiddenPassed,
		"hidden_failed":        report.HiddenFailed,
		"quality_passed":       report.QualityPassed,
		"quality_failed":       report.QualityFailed,
		"similarity_score":     round2(report.SimilarityScore),
		"errors":               append([]string(nil), report.Errors...),
		"anti_cheat":           antiCheat,
	}

	return EvaluationResult{
		ID:                 id.New("evl"),
		SubmissionID:       submission.ID,
		TestScore:          breakdown.Correctness,
		LintScore:          breakdown.LintQuality,
		PerfScore:          breakdown.ExecutionCost,
		QualityScore:       breakdown.Quality,
		ExecutionCostScore: breakdown.ExecutionCost,
		SpeedScore:         breakdown.ExecutionCost,
		ConsistencyScore:   breakdown.Consistency,
		FinalScore:         breakdown.Final,
		Report:             reportJSON,
		CreatedAt:          time.Now().UTC(),
	}
}

func runnerTestResultsFromChecks(checks []RunnerCheck) []runsvc.TestResult {
	results := make([]runsvc.TestResult, 0, len(checks))
	for _, check := range checks {
		name := check.Name
		file := ""
		if parts := strings.SplitN(check.Name, ": ", 2); len(parts) == 2 {
			file = parts[0]
			name = parts[1]
		}
		results = append(results, runsvc.TestResult{
			File:    file,
			Name:    name,
			CheckID: check.ID,
			Kind:    firstNonEmpty(check.Kind, "correctness"),
			Hidden:  check.Hidden,
			Passed:  check.Passed,
		})
	}
	return results
}

func (a *App) applySkillUpdateLocked(userID string, templateDef ChallengeTemplate, evalResult EvaluationResult, report RunnerReport) {
	now := time.Now().UTC()
	user := a.users[userID]
	previousActive := user.LastActiveAt
	user.LastActiveAt = now
	user.Profile.CompletedChallenges++
	user.Profile.UpdatedAt = now
	if previousActive.IsZero() {
		user.Profile.StreakDays = 1
	} else {
		prevDay := previousActive.UTC().Truncate(24 * time.Hour)
		curDay := now.UTC().Truncate(24 * time.Hour)
		diffDays := int(curDay.Sub(prevDay).Hours() / 24)
		switch diffDays {
		case 0:
		case 1:
			user.Profile.StreakDays++
		default:
			user.Profile.StreakDays = 1
		}
		if user.Profile.StreakDays == 0 {
			user.Profile.StreakDays = 1
		}
	}

	antiCheat := antiCheatFromEvaluationReport(evalResult.Report)
	confidence := evaluation.AssessConfidence(evaluation.ConfidenceInput{
		CurrentScore:     user.Profile.ConfidenceScore,
		CompletedTasks:   user.Profile.CompletedChallenges,
		ConsistencyScore: evalResult.ConsistencyScore,
		ChallengeScore:   evalResult.FinalScore,
		SolveTimeSeconds: report.SolveTimeSeconds,
		AttemptNumber:    report.AttemptNumber,
		PasteEvents:      report.PasteEvents,
		FocusLossEvents:  report.FocusLossEvents,
		HiddenFailures:   report.HiddenFailed,
		SimilarityScore:  report.SimilarityScore,
		SuspicionLevel:   antiCheat.Level,
	})
	user.Profile.ConfidenceScore = confidence.Score
	user.Profile.ConfidenceLevel = confidence.Level
	user.Profile.ConfidenceReasons = append([]string(nil), confidence.Reasons...)
	a.users[userID] = user

	if _, ok := a.userSkills[userID]; !ok {
		a.userSkills[userID] = map[string]UserSkill{}
	}

	for skillCode, weight := range templateDef.SkillWeights {
		skill := a.skills[skillCode]
		userSkill := a.userSkills[userID][skillCode]
		if userSkill.ID == "" {
			userSkill = UserSkill{
				ID:          id.New("usk"),
				UserID:      userID,
				SkillID:     skill.ID,
				SkillCode:   skill.Code,
				DecayFactor: 1,
			}
		}
		previousScore := userSkill.Score
		weightFactor := difficultyWeight(templateDef.Difficulty) * weight
		userSkill.Score = clamp(evaluation.UpdateSkillScore(userSkill.Score, evalResult.FinalScore, weightFactor), 0, 1000)
		userSkill.Confidence = user.Profile.ConfidenceScore
		userSkill.Level = levelForScore(userSkill.Score)
		userSkill.LastVerifiedAt = now
		userSkill.UpdatedAt = now
		a.userSkills[userID][skillCode] = userSkill
		a.scoreEvents[userID] = append(a.scoreEvents[userID], ScoreEvent{
			ID:         id.New("sev"),
			UserID:     userID,
			SkillID:    skill.ID,
			SourceID:   templateDef.ID,
			Delta:      round2(userSkill.Score - previousScore),
			ScoreAfter: userSkill.Score,
			CreatedAt:  now,
			SourceType: "challenge",
		})
	}

	consistencySkill := a.userSkills[userID]["consistency"]
	if consistencySkill.ID == "" {
		skill := a.skills["consistency"]
		consistencySkill = UserSkill{
			ID:        id.New("usk"),
			UserID:    userID,
			SkillID:   skill.ID,
			SkillCode: skill.Code,
		}
	}
	consistencySkill.Score = clamp(evaluation.UpdateSkillScore(consistencySkill.Score, evalResult.ConsistencyScore, 0.7), 0, 1000)
	consistencySkill.Confidence = user.Profile.ConfidenceScore
	consistencySkill.Level = levelForScore(consistencySkill.Score)
	consistencySkill.LastVerifiedAt = now
	consistencySkill.UpdatedAt = now
	consistencySkill.DecayFactor = 1
	a.userSkills[userID]["consistency"] = consistencySkill

	a.scoreHistory[userID] = append(a.scoreHistory[userID], evalResult.FinalScore)
	user = a.users[userID]
	user.Profile.CurrentSkillScore = round2(a.currentSkillScoreLocked(userID))
	a.users[userID] = user
	a.updateRoomLocked(userID)
}

func (a *App) currentSkillScoreLocked(userID string) float64 {
	skills := a.userSkills[userID]
	decayFactor := evaluation.DecayFactor(a.users[userID].LastActiveAt, time.Now().UTC())
	return clamp(
		(skills["react"].Score*0.45+
			skills["javascript"].Score*0.20+
			skills["performance"].Score*0.15+
			skills["architecture"].Score*0.10+
			skills["consistency"].Score*0.10)*decayFactor,
		0,
		1000,
	)
}

func (a *App) consistencyScoreLocked(userID string) float64 {
	history := a.scoreHistory[userID]
	if len(history) == 0 {
		return 60
	}
	start := 0
	if len(history) > 5 {
		start = len(history) - 5
	}
	sum := 0.0
	for _, item := range history[start:] {
		sum += item
	}
	return clamp(sum/float64(len(history[start:])), 0, 100)
}

func (a *App) listUserSkills(userID string) []UserSkill {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.listUserSkillsLocked(userID)
}

func (a *App) listUserSkillsLocked(userID string) []UserSkill {
	var items []UserSkill
	for _, skill := range a.userSkills[userID] {
		items = append(items, skill)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SkillCode < items[j].SkillCode })
	return items
}

func (a *App) listUserRoomItems(userID string) []UserRoomItem {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.listUserRoomItemsLocked(userID)
}

func (a *App) roomView(ctx context.Context, userID string) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	user, ok := a.users[userID]
	if !ok {
		return nil, errors.New("user not found")
	}
	if roleSupportsDeveloperRoom(user.Role) && a.syncDeveloperCosmeticsLocked(userID, time.Now().UTC()) {
		if err := a.persistUserMonetizationLocked(ctx, userID); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"items":         a.listUserRoomItemsLocked(userID),
		"customization": a.roomCustomizationLocked(userID),
	}, nil
}

func (a *App) listUserRoomItemsLocked(userID string) []UserRoomItem {
	var items []UserRoomItem
	for code, item := range a.userRoomItems[userID] {
		if _, ok := a.roomItems[code]; !ok {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RoomItemCode < items[j].RoomItemCode })
	return items
}

func (a *App) profileView(userID string) UserProfile {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.profileViewLocked(userID)
}

func (a *App) profileViewLocked(userID string) UserProfile {
	user, ok := a.users[userID]
	if !ok {
		return UserProfile{}
	}
	profile := user.Profile
	confidence := a.confidenceAssessmentLocked(userID)
	profile.ConfidenceScore = confidence.Score
	profile.ConfidenceLevel = confidence.Level
	profile.ConfidenceReasons = append([]string(nil), confidence.Reasons...)
	return profile
}

func (a *App) updateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (UserProfile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	user, ok := a.users[userID]
	if !ok {
		return UserProfile{}, errors.New("user not found")
	}
	if !roleSupportsDeveloperRoom(user.Role) {
		return UserProfile{}, errors.New("developer profile editing is unavailable for this role")
	}

	linkedInURL, err := normalizeLinkedInURL(req.LinkedInURL)
	if err != nil {
		return UserProfile{}, err
	}

	now := time.Now().UTC()
	profile := user.Profile
	profile.LinkedInURL = linkedInURL
	profile.UpdatedAt = now
	user.Profile = profile
	user.LastActiveAt = now
	a.users[userID] = user

	if err := a.persistUserStateLocked(ctx, userID); err != nil {
		return UserProfile{}, err
	}
	return a.profileViewLocked(userID), nil
}

func normalizeLinkedInURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(strings.ToLower(value), "http://") && !strings.HasPrefix(strings.ToLower(value), "https://") {
		value = "https://" + value
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return "", errors.New("linkedin_url must be a valid URL")
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host != "linkedin.com" {
		return "", errors.New("linkedin_url must point to linkedin.com")
	}
	if strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
		return "", errors.New("linkedin_url must include a profile path")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (a *App) confidenceAssessmentLocked(userID string) evaluation.ConfidenceAssessment {
	user, ok := a.users[userID]
	if !ok {
		return evaluation.AssessConfidence(evaluation.ConfidenceInput{})
	}

	profile := user.Profile
	input := evaluation.ConfidenceInput{
		CurrentScore:     profile.ConfidenceScore,
		CompletedTasks:   profile.CompletedChallenges,
		ConsistencyScore: a.consistencyScoreLocked(userID),
	}

	if latestEvaluation, report, antiCheat, ok := a.latestUserEvaluationLocked(userID); ok {
		input.ChallengeScore = latestEvaluation.FinalScore
		input.SolveTimeSeconds = report.SolveTimeSeconds
		input.AttemptNumber = report.AttemptNumber
		input.PasteEvents = report.PasteEvents
		input.FocusLossEvents = report.FocusLossEvents
		input.HiddenFailures = report.HiddenFailed
		input.SimilarityScore = report.SimilarityScore
		input.SuspicionLevel = antiCheat.Level
	}

	return evaluation.AssessConfidence(input)
}

func (a *App) requestFriend(ctx context.Context, userID, targetUserID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if userID == targetUserID {
		return errors.New("cannot friend yourself")
	}
	if _, ok := a.users[targetUserID]; !ok {
		return errors.New("target user not found")
	}
	if _, ok := a.friendships[userID]; !ok {
		a.friendships[userID] = map[string]Friendship{}
	}
	if _, ok := a.friendships[targetUserID]; !ok {
		a.friendships[targetUserID] = map[string]Friendship{}
	}
	now := time.Now().UTC()
	a.friendships[userID][targetUserID] = Friendship{UserID: userID, FriendUserID: targetUserID, Status: "pending", CreatedAt: now}
	a.friendships[targetUserID][userID] = Friendship{UserID: targetUserID, FriendUserID: userID, Status: "requested", CreatedAt: now}
	if err := a.persistFriendshipLocked(ctx, userID, targetUserID); err != nil {
		return err
	}
	a.invalidateFriendRankingCaches(ctx, userID, targetUserID)
	return nil
}

func (a *App) acceptFriend(ctx context.Context, userID, requesterID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if relation, ok := a.friendships[userID][requesterID]; !ok || relation.Status != "requested" {
		return errors.New("friend request not found")
	}
	now := time.Now().UTC()
	a.friendships[userID][requesterID] = Friendship{UserID: userID, FriendUserID: requesterID, Status: "accepted", CreatedAt: now}
	a.friendships[requesterID][userID] = Friendship{UserID: requesterID, FriendUserID: userID, Status: "accepted", CreatedAt: now}
	if err := a.persistFriendshipLocked(ctx, userID, requesterID); err != nil {
		return err
	}
	a.invalidateFriendRankingCaches(ctx, userID, requesterID)
	return nil
}

func (a *App) directMessages(userID, peerID string) ([]ChatMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.areFriendsLocked(userID, peerID) {
		return nil, errors.New("direct chat requires friendship")
	}
	chatID := a.ensureDirectChatLocked(userID, peerID)
	messages := a.chatMessages[chatID]
	out := make([]ChatMessage, len(messages))
	copy(out, messages)
	return out, nil
}

func (a *App) createDirectMessage(ctx context.Context, userID, peerID, body string) (ChatMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(body) == "" {
		return ChatMessage{}, errors.New("body is required")
	}
	if !a.areFriendsLocked(userID, peerID) {
		return ChatMessage{}, errors.New("direct chat requires friendship")
	}
	chatID := a.ensureDirectChatLocked(userID, peerID)
	msg := ChatMessage{
		ID:           id.New("msg"),
		ChatID:       chatID,
		SenderUserID: userID,
		Body:         strings.TrimSpace(body),
		CreatedAt:    time.Now().UTC(),
	}
	a.chatMessages[chatID] = append(a.chatMessages[chatID], msg)
	if err := a.persistChatLocked(ctx, chatID); err != nil {
		return ChatMessage{}, err
	}
	if err := a.persistMessageLocked(ctx, msg); err != nil {
		return ChatMessage{}, err
	}
	return msg, nil
}

func (a *App) createCompany(ctx context.Context, ownerUserID string, req CompanyRequest) (Company, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(req.Name) == "" {
		return Company{}, errors.New("name is required")
	}
	company := Company{
		ID:          id.New("cmp"),
		OwnerUserID: ownerUserID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		RoomState:   map[string]any{"theme": "react"},
		CreatedAt:   time.Now().UTC(),
	}
	a.companies[company.ID] = company
	if err := a.persistCompanyLocked(ctx, company.ID); err != nil {
		return Company{}, err
	}
	return company, nil
}

func (a *App) createJob(ctx context.Context, ownerUserID, companyID string, req JobRequest) (Job, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	company, ok := a.companies[companyID]
	if !ok || company.OwnerUserID != ownerUserID {
		return Job{}, errors.New("company not found")
	}
	if strings.TrimSpace(req.Title) == "" {
		return Job{}, errors.New("title is required")
	}
	job := Job{
		ID:             id.New("job"),
		CompanyID:      companyID,
		Title:          strings.TrimSpace(req.Title),
		Description:    strings.TrimSpace(req.Description),
		RequiredScore:  req.RequiredScore,
		RequiredSkills: req.RequiredSkills,
		CreatedAt:      time.Now().UTC(),
	}
	a.jobs[job.ID] = job
	if err := a.persistJobLocked(ctx, job.ID); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (a *App) searchCandidates(ctx context.Context, recruiterUserID string, minScore, minConfidence, topPercent float64, activeDays, limit, offset int) ([]CandidateView, PaginationInfo, MonetizationSummary, error) {
	if a.store != nil {
		return a.searchCandidatesSQL(ctx, recruiterUserID, CandidateSearchFilters{
			MinScore:      minScore,
			MinConfidence: minConfidence,
			TopPercent:    topPercent,
			ActiveDays:    activeDays,
			Limit:         limit,
			Offset:        offset,
		})
	}

	now := time.Now().UTC()
	if !a.rankingSnapshotFresh(now) {
		a.mu.Lock()
		a.ensureRankingSnapshotLocked(now)
		a.mu.Unlock()
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []CandidateView
	monetization := a.monetizationSummaryLocked(recruiterUserID)
	for _, userID := range a.rankingSnapshot.CandidateIDs {
		user, ok := a.users[userID]
		if !ok {
			continue
		}
		if user.Role != RoleUser {
			continue
		}
		if minScore > 0 && user.Profile.CurrentSkillScore < minScore {
			continue
		}
		if minConfidence > 0 && user.Profile.ConfidenceScore < minConfidence {
			continue
		}
		if topPercent > 0 && user.Profile.PercentileGlobal < (100-topPercent) {
			continue
		}
		if activeDays > 0 && now.Sub(user.LastActiveAt) > time.Duration(activeDays)*24*time.Hour {
			continue
		}
		out = append(out, a.candidatePreviewLocked(recruiterUserID, user, monetization))
	}
	pagination := buildPagination(limit, offset, len(out))
	return paginateCandidates(out, pagination), pagination, monetization, nil
}

func (a *App) searchCandidatesSQL(ctx context.Context, recruiterUserID string, filters CandidateSearchFilters) ([]CandidateView, PaginationInfo, MonetizationSummary, error) {
	entries, pagination, err := a.store.SearchCandidateEntries(ctx, filters, time.Now().UTC())
	if err != nil {
		return nil, PaginationInfo{}, MonetizationSummary{}, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	monetization := a.monetizationSummaryLocked(recruiterUserID)
	out := make([]CandidateView, 0, len(entries))
	for _, entry := range entries {
		user, ok := a.users[entry.UserID]
		if !ok {
			continue
		}
		user.Username = entry.Username
		user.Country = entry.Country
		user.LastActiveAt = entry.LastActiveAt
		user.Profile.CurrentSkillScore = entry.CurrentSkillScore
		user.Profile.PercentileGlobal = entry.Percentile
		user.Profile.ConfidenceScore = entry.ConfidenceScore
		user.Profile.CompletedChallenges = entry.CompletedChallenges
		out = append(out, a.candidatePreviewLocked(recruiterUserID, user, monetization))
	}
	return out, pagination, monetization, nil
}

func parsePaginationQuery(r *http.Request, defaultLimit int) (int, int) {
	limit := defaultLimit
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			limit = parsed
		}
	}
	offset := 0
	if value := strings.TrimSpace(r.URL.Query().Get("offset")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			offset = parsed
		}
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > hrPaginationMaxLimit {
		limit = hrPaginationMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func buildPagination(limit, offset, total int) PaginationInfo {
	if limit <= 0 {
		limit = hrCandidatePageLimit
	}
	if offset < 0 {
		offset = 0
	}
	if total < 0 {
		total = 0
	}
	return PaginationInfo{
		Limit:   limit,
		Offset:  offset,
		Total:   total,
		HasMore: offset+limit < total,
	}
}

func paginateCandidates(results []CandidateView, pagination PaginationInfo) []CandidateView {
	if pagination.Offset >= len(results) {
		return []CandidateView{}
	}
	end := pagination.Offset + pagination.Limit
	if end > len(results) {
		end = len(results)
	}
	return results[pagination.Offset:end]
}

func (a *App) observeChallengeExecutionError(err error) {
	switch {
	case errors.Is(err, errRunnerTimeout):
		a.metrics.IncrementEvent("runner_timeout")
	case errors.Is(err, errRunnerUnavailable):
		a.metrics.IncrementEvent("runner_unavailable")
	}
}

func (a *App) shortlistCandidate(ctx context.Context, ownerUserID string, req ShortlistRequest) (HRShortlist, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	company, ok := a.companies[req.CompanyID]
	if !ok || company.OwnerUserID != ownerUserID {
		return HRShortlist{}, errors.New("company not found")
	}
	if _, ok := a.users[req.UserID]; !ok {
		return HRShortlist{}, errors.New("candidate not found")
	}
	entry := HRShortlist{
		ID:        id.New("slt"),
		CompanyID: req.CompanyID,
		UserID:    req.UserID,
		Status:    firstNonEmpty(req.Status, "new"),
		Notes:     req.Notes,
		CreatedAt: time.Now().UTC(),
	}
	a.shortlists = append(a.shortlists, entry)
	if err := a.persistShortlistLocked(ctx, entry); err != nil {
		return HRShortlist{}, err
	}
	return entry, nil
}

func (a *App) rankings(ctx context.Context, kind, country, userID string) ([]RankingEntry, error) {
	if a.store != nil {
		return a.store.QueryRankingEntries(ctx, kind, country, userID, time.Now().UTC())
	}

	now := time.Now().UTC()
	if !a.rankingSnapshotFresh(now) {
		a.mu.Lock()
		a.ensureRankingSnapshotLocked(now)
		a.mu.Unlock()
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rankingsFromSnapshotLocked(kind, country, userID), nil
}

func (a *App) recomputeRankingsLocked() {
	a.ensureRankingSnapshotLocked(time.Now().UTC())
}

func (a *App) rankingSnapshotFresh(now time.Time) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rankingSnapshotFreshLocked(now)
}

func (a *App) rankingSnapshotFreshLocked(now time.Time) bool {
	return !a.rankingSnapshot.RefreshedAt.IsZero() && now.Sub(a.rankingSnapshot.RefreshedAt) < rankingSnapshotTTL
}

func (a *App) ensureRankingSnapshotLocked(now time.Time) {
	if a.rankingSnapshotFreshLocked(now) {
		return
	}

	var users []User
	for _, user := range a.users {
		if user.Role != RoleUser {
			continue
		}
		decayed := user
		decayed.Profile.CurrentSkillScore = round2(a.currentSkillScoreLocked(user.ID))
		a.users[user.ID] = decayed
		users = append(users, decayed)
	}
	sortUsers(users)

	globalEntries := buildRankingEntries(users)
	candidateIDs := make([]string, 0, len(users))
	for index, entry := range globalEntries {
		stored := a.users[entry.UserID]
		stored.Profile.PercentileGlobal = entry.Percentile
		a.users[entry.UserID] = stored
		candidateIDs = append(candidateIDs, users[index].ID)
	}

	byCountry := map[string][]RankingEntry{}
	countryBuckets := map[string][]User{}
	for _, user := range users {
		countryBuckets[user.Country] = append(countryBuckets[user.Country], user)
	}
	for countryCode, bucket := range countryBuckets {
		sortUsers(bucket)
		countryEntries := buildRankingEntries(bucket)
		byCountry[countryCode] = countryEntries
		for _, entry := range countryEntries {
			stored := a.users[entry.UserID]
			stored.Profile.PercentileCountry = entry.Percentile
			a.users[entry.UserID] = stored
		}
	}

	a.rankingSnapshot = rankingSnapshot{
		RefreshedAt:  now,
		Global:       globalEntries,
		ByCountry:    byCountry,
		CandidateIDs: candidateIDs,
	}
	a.updateAllRoomTrophiesLocked()
}

func (a *App) rankingsFromSnapshotLocked(kind, country, userID string) []RankingEntry {
	switch kind {
	case "country":
		return cloneRankingEntries(a.rankingSnapshot.ByCountry[country])
	case "friends":
		friendSet := map[string]struct{}{userID: {}}
		for peerID, relation := range a.friendships[userID] {
			if relation.Status == "accepted" {
				friendSet[peerID] = struct{}{}
			}
		}
		filtered := make([]RankingEntry, 0, len(friendSet))
		for _, entry := range a.rankingSnapshot.Global {
			if _, ok := friendSet[entry.UserID]; !ok {
				continue
			}
			filtered = append(filtered, entry)
		}
		return rerankEntries(filtered)
	default:
		return cloneRankingEntries(a.rankingSnapshot.Global)
	}
}

func buildRankingEntries(users []User) []RankingEntry {
	entries := make([]RankingEntry, 0, len(users))
	total := len(users)
	for index, user := range users {
		percentile := 100.0
		if total > 1 {
			percentile = round2((float64(total-index-1) / float64(total-1)) * 100)
		}
		entries = append(entries, RankingEntry{
			UserID:              user.ID,
			Username:            user.Username,
			Country:             user.Country,
			CurrentSkillScore:   user.Profile.CurrentSkillScore,
			ConfidenceScore:     user.Profile.ConfidenceScore,
			Percentile:          percentile,
			Rank:                index + 1,
			LastActiveAt:        user.LastActiveAt,
			CompletedChallenges: user.Profile.CompletedChallenges,
		})
	}
	return entries
}

func cloneRankingEntries(entries []RankingEntry) []RankingEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]RankingEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func rerankEntries(entries []RankingEntry) []RankingEntry {
	reranked := cloneRankingEntries(entries)
	total := len(reranked)
	for index := range reranked {
		reranked[index].Rank = index + 1
		percentile := 100.0
		if total > 1 {
			percentile = round2((float64(total-index-1) / float64(total-1)) * 100)
		}
		reranked[index].Percentile = percentile
	}
	return reranked
}

func (a *App) initUserStateLocked(userID string, now time.Time) {
	a.userSkills[userID] = map[string]UserSkill{}
	for _, skill := range a.skills {
		a.userSkills[userID][skill.Code] = UserSkill{
			ID:             id.New("usk"),
			UserID:         userID,
			SkillID:        skill.ID,
			SkillCode:      skill.Code,
			Confidence:     50,
			Level:          "bronze",
			LastVerifiedAt: now,
			DecayFactor:    1,
			UpdatedAt:      now,
		}
	}
	a.userRoomItems[userID] = map[string]UserRoomItem{}
	for _, roomItem := range a.roomItems {
		level := roomInitialLevel(roomItem.Code)
		a.userRoomItems[userID][roomItem.Code] = UserRoomItem{
			ID:             id.New("rit"),
			UserID:         userID,
			RoomItemID:     roomItem.ID,
			RoomItemCode:   roomItem.Code,
			CurrentLevel:   level,
			CurrentVariant: roomDefaultVariant(roomItem.Code, level),
			State:          roomDefaultState(roomItem.Code),
			UpdatedAt:      now,
		}
	}
}

func (a *App) seedSkillsAndRoom() {
	for _, skill := range []Skill{
		{ID: "skill_react", Name: "React", Category: "track", Code: "react"},
		{ID: "skill_javascript", Name: "JavaScript Core", Category: "core", Code: "javascript"},
		{ID: "skill_performance", Name: "Performance", Category: "advanced", Code: "performance"},
		{ID: "skill_architecture", Name: "Architecture", Category: "advanced", Code: "architecture"},
		{ID: "skill_consistency", Name: "Consistency", Category: "meta", Code: "consistency"},
	} {
		a.skills[skill.Code] = skill
	}
	for _, item := range []RoomItem{
		{ID: "room_monitor", Name: "Monitor", Slot: "monitor", RelatedSkillID: a.skills["react"].ID, Code: "monitor"},
		{ID: "room_desk", Name: "Desk", Slot: "desk", RelatedSkillID: a.skills["javascript"].ID, Code: "desk"},
		{ID: "room_shelf", Name: "Shelf", Slot: "shelf", RelatedSkillID: a.skills["react"].ID, Code: "shelf"},
		{ID: "room_chair", Name: "Chair", Slot: "chair", RelatedSkillID: a.skills["architecture"].ID, Code: "chair"},
		{ID: "room_plant", Name: "Plant", Slot: "plant", RelatedSkillID: a.skills["consistency"].ID, Code: "plant"},
		{ID: "room_trophy_case", Name: "Trophy Case", Slot: "trophy_case", RelatedSkillID: a.skills["react"].ID, Code: "trophy_case"},
	} {
		a.roomItems[item.Code] = item
	}
}

func (a *App) pickTemplateLocked(req StartChallengeRequest) (ChallengeTemplate, error) {
	if req.TemplateID != "" {
		templateDef, ok := a.templates[req.TemplateID]
		if !ok {
			return ChallengeTemplate{}, errors.New("template not found")
		}
		return templateDef, nil
	}
	var options []ChallengeTemplate
	for _, templateDef := range a.templates {
		if req.Category == "" || templateDef.Category == req.Category {
			options = append(options, templateDef)
		}
	}
	if len(options) == 0 {
		return ChallengeTemplate{}, errors.New("no templates for category")
	}
	return pickRandomTemplate(options, time.Now().UTC().UnixNano()), nil
}

func (a *App) nextAttemptNumberLocked(userID, templateID string) int {
	attempts := 0
	for _, instance := range a.instances {
		if instance.UserID == userID && instance.TemplateID == templateID {
			attempts++
		}
	}
	return attempts + 1
}

func deterministicVariantSeed(userID, templateID string, attemptNumber int) int64 {
	return int64(hashSeed(int64(attemptNumber), userID+":"+templateID))
}

func buildVisibleFiles(variant ChallengeVariant) map[string]string {
	visible := map[string]string{}
	for _, name := range variant.EditableFiles {
		if content, ok := variant.GeneratedFiles[name]; ok {
			visible[name] = content
		}
	}
	return visible
}

func cloneFiles(files map[string]string) map[string]string {
	if len(files) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(files))
	for name, content := range files {
		cloned[name] = content
	}
	return cloned
}

func (a *App) recentActivityLocked(userID string) []string {
	type activity struct {
		title string
		at    time.Time
	}
	var entries []activity
	for _, submission := range a.submissions {
		instance, ok := a.instances[submission.ChallengeInstanceID]
		if !ok || instance.UserID != userID {
			continue
		}
		templateDef := a.templates[instance.TemplateID]
		entries = append(entries, activity{
			title: templateDef.Title,
			at:    submission.SubmittedAt,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].at.After(entries[j].at)
	})
	limit := 3
	if len(entries) < limit {
		limit = len(entries)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, entries[i].title)
	}
	return out
}

func (a *App) skillSummaryLocked(userID string, strongest bool) []string {
	skills := a.listUserSkillsLocked(userID)
	filtered := make([]UserSkill, 0, len(skills))
	for _, skill := range skills {
		if skill.SkillCode == "consistency" {
			continue
		}
		filtered = append(filtered, skill)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if strongest {
			return filtered[i].Score > filtered[j].Score
		}
		return filtered[i].Score < filtered[j].Score
	})
	limit := 2
	if len(filtered) < limit {
		limit = len(filtered)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, filtered[i].SkillCode)
	}
	return out
}

func (a *App) linkedTasksForSkillLocked(userID, skillCode string) []string {
	type task struct {
		title string
		at    time.Time
	}
	var entries []task
	for _, submission := range a.submissions {
		instance, ok := a.instances[submission.ChallengeInstanceID]
		if !ok || instance.UserID != userID {
			continue
		}
		templateDef := a.templates[instance.TemplateID]
		if templateDef.SkillWeights[skillCode] <= 0 {
			continue
		}
		entries = append(entries, task{
			title: templateDef.Title,
			at:    submission.SubmittedAt,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].at.After(entries[j].at)
	})
	limit := 3
	if len(entries) < limit {
		limit = len(entries)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, entries[i].title)
	}
	return out
}

func (a *App) challengeViewLocked(instance ChallengeInstance, templateDef ChallengeTemplate, variant ChallengeVariant) ChallengeInstanceView {
	publicVariant := variant
	publicVariant.GeneratedFiles = buildVisibleFiles(variant)
	publicVariant.VisibleTests = cloneFiles(variant.VisibleTests)
	publicVariant.EditableFiles = append([]string(nil), variant.EditableFiles...)

	return ChallengeInstanceView{
		Instance:      instance,
		TemplateID:    templateDef.ID,
		Title:         RenderTitle(templateDef, variant.Params),
		Description:   RenderDescription(templateDef, variant.Params),
		Category:      templateDef.Category,
		Difficulty:    templateDef.Difficulty,
		VisibleTests:  cloneFiles(variant.VisibleTests),
		EditableFiles: append([]string(nil), variant.EditableFiles...),
		Variant:       publicVariant,
	}
}

func (a *App) areFriendsLocked(userID, peerID string) bool {
	if relation, ok := a.friendships[userID][peerID]; ok && relation.Status == "accepted" {
		return true
	}
	return false
}

func (a *App) ensureDirectChatLocked(userID, peerID string) string {
	key := pairKey(userID, peerID)
	if chatID, ok := a.directChats[key]; ok {
		return chatID
	}
	chat := Chat{
		ID:        id.New("cht"),
		Type:      "direct",
		CreatedAt: time.Now().UTC(),
	}
	a.chats[chat.ID] = chat
	a.directChats[key] = chat.ID
	return chat.ID
}

func (a *App) updateRoomLocked(userID string) {
	items := a.userRoomItems[userID]
	skills := a.userSkills[userID]
	user := a.users[userID]
	decayFactor := evaluation.DecayFactor(user.LastActiveAt, time.Now().UTC())

	setItem := func(code string, score float64, extra map[string]any) {
		item := items[code]
		item.CurrentLevel = levelForScore(score)
		item.CurrentVariant = roomVariant(code, item.CurrentLevel)
		if item.State == nil {
			item.State = map[string]any{}
		}
		item.State["level"] = item.CurrentLevel
		for key, value := range extra {
			item.State[key] = value
		}
		item.UpdatedAt = time.Now().UTC()
		items[code] = item
	}

	setItem("monitor", skills["react"].Score*decayFactor, map[string]any{
		"glow":         user.Profile.PercentileGlobal >= 80,
		"explanation":  fmt.Sprintf("React score %.0f powers the monitor.", skills["react"].Score*decayFactor),
		"linked_tasks": a.linkedTasksForSkillLocked(userID, "react"),
	})
	setItem("desk", skills["javascript"].Score*decayFactor, map[string]any{
		"glow":         skills["javascript"].Score*decayFactor >= 600,
		"explanation":  fmt.Sprintf("JavaScript score %.0f shapes the desk.", skills["javascript"].Score*decayFactor),
		"linked_tasks": a.linkedTasksForSkillLocked(userID, "javascript"),
	})
	setItem("chair", math.Max(skills["architecture"].Score, skills["performance"].Score)*decayFactor, map[string]any{
		"glow":         skills["architecture"].Score*decayFactor >= 600 || skills["performance"].Score*decayFactor >= 600,
		"explanation":  "Architecture and performance work upgrade the chair.",
		"linked_tasks": a.linkedTasksForSkillLocked(userID, "architecture"),
	})
	setItem("plant", skills["consistency"].Score*decayFactor, map[string]any{
		"streak_days":  user.Profile.StreakDays,
		"glow":         user.Profile.StreakDays >= 3,
		"explanation":  fmt.Sprintf("Consistency score %.0f and a %d day streak keep the plant alive.", skills["consistency"].Score*decayFactor, user.Profile.StreakDays),
		"linked_tasks": a.linkedTasksForSkillLocked(userID, "consistency"),
	})

	shelf := items["shelf"]
	shelf.CurrentLevel = levelForCompleted(user.Profile.CompletedChallenges)
	shelf.CurrentVariant = roomVariant("shelf", shelf.CurrentLevel)
	shelf.State = map[string]any{
		"completed_challenges": user.Profile.CompletedChallenges,
		"level":                shelf.CurrentLevel,
		"explanation":          fmt.Sprintf("%d completed tasks fill the shelf.", user.Profile.CompletedChallenges),
		"linked_tasks":         a.recentActivityLocked(userID),
	}
	shelf.UpdatedAt = time.Now().UTC()
	items["shelf"] = shelf

	a.userRoomItems[userID] = items
}

func (a *App) updateAllRoomTrophiesLocked() {
	for userID, user := range a.users {
		if user.Role != RoleUser {
			continue
		}
		items := a.userRoomItems[userID]
		trophy, ok := items["trophy_case"]
		if !ok {
			continue
		}
		achievements := a.buildTrophyAchievementsLocked(userID, user)
		trophy.CurrentLevel = roomInitialLevel("trophy_case")
		trophy.CurrentVariant = roomDefaultVariant("trophy_case", trophy.CurrentLevel)
		trophy.State = map[string]any{
			"presentation_mode": "achievement_case",
			"case_variant":      "default",
			"achievement_count": len(achievements),
			"achievements":      achievements,
			"explanation":       trophyCaseExplanation(achievements),
			"linked_tasks":      a.recentActivityLocked(userID),
		}
		trophy.UpdatedAt = time.Now().UTC()
		items["trophy_case"] = trophy
		a.userRoomItems[userID] = items
	}
}

func (a *App) buildTrophyAchievementsLocked(userID string, user User) []TrophyAchievement {
	achievements := make([]TrophyAchievement, 0, 6)
	appendAchievement := func(code, title, description string) {
		achievements = append(achievements, TrophyAchievement{
			Code:        code,
			Title:       title,
			Description: description,
		})
	}

	if user.Profile.PercentileGlobal >= 99 {
		appendAchievement("top_percentile_1", "Top 1% globally", fmt.Sprintf("Global percentile %.2f places this candidate in the top 1%%.", user.Profile.PercentileGlobal))
	} else if user.Profile.PercentileGlobal >= 90 {
		appendAchievement("top_percentile_10", "Top 10% globally", fmt.Sprintf("Global percentile %.2f places this candidate in the top 10%%.", user.Profile.PercentileGlobal))
	}

	if user.Profile.CompletedChallenges >= 25 {
		appendAchievement("challenge_volume_25", "25 verified challenges", fmt.Sprintf("%d completed challenges prove repeatable delivery.", user.Profile.CompletedChallenges))
	} else if user.Profile.CompletedChallenges >= 10 {
		appendAchievement("challenge_volume_10", "10 verified challenges", fmt.Sprintf("%d completed challenges make the track record more trustworthy.", user.Profile.CompletedChallenges))
	}

	if user.Profile.ConfidenceScore >= 85 {
		appendAchievement("high_confidence", "High confidence signal", fmt.Sprintf("Confidence score %.0f reflects stable and trustworthy outcomes.", user.Profile.ConfidenceScore))
	}

	if user.Profile.StreakDays >= 7 {
		appendAchievement("streak_7", "7-day streak", fmt.Sprintf("%d consecutive active days unlocked a consistency trophy.", user.Profile.StreakDays))
	} else if user.Profile.StreakDays >= 3 {
		appendAchievement("streak_3", "3-day streak", fmt.Sprintf("%d consecutive active days established momentum.", user.Profile.StreakDays))
	}

	if coverage := a.categoryCoverageLocked(userID); coverage >= 4 {
		appendAchievement("category_coverage_4", "Cross-category range", fmt.Sprintf("%d challenge categories have been solved successfully.", coverage))
	}

	if len(a.scoreHistory[userID]) >= 5 && a.consistencyScoreLocked(userID) >= 75 {
		appendAchievement("stable_results", "Stable recent results", fmt.Sprintf("Recent scoring has stayed stable across %d validated runs.", len(a.scoreHistory[userID])))
	}

	return achievements
}

func (a *App) categoryCoverageLocked(userID string) int {
	categories := map[string]struct{}{}
	for _, submission := range a.submissions {
		instance, ok := a.instances[submission.ChallengeInstanceID]
		if !ok || instance.UserID != userID {
			continue
		}
		templateDef, ok := a.templates[instance.TemplateID]
		if !ok || strings.TrimSpace(templateDef.Category) == "" {
			continue
		}
		categories[templateDef.Category] = struct{}{}
	}
	return len(categories)
}

func trophyCaseExplanation(achievements []TrophyAchievement) string {
	count := len(achievements)
	switch count {
	case 0:
		return "The trophy case stays neutral until verified achievements are unlocked."
	case 1:
		return "1 verified trophy is currently on display."
	default:
		return fmt.Sprintf("%d verified trophies are currently on display.", count)
	}
}

func difficultyWeight(difficulty int) float64 {
	switch difficulty {
	case 1:
		return 0.9
	case 2:
		return 1.1
	case 3:
		return 1.25
	default:
		return 1.0
	}
}

func levelForScore(score float64) string {
	switch {
	case score >= 800:
		return "platinum"
	case score >= 600:
		return "gold"
	case score >= 400:
		return "silver"
	default:
		return "bronze"
	}
}

func levelForCompleted(count int) string {
	switch {
	case count >= 20:
		return "platinum"
	case count >= 12:
		return "gold"
	case count >= 6:
		return "silver"
	default:
		return "bronze"
	}
}

func roomVariant(code, level string) string {
	return fmt.Sprintf("%s_%s", code, level)
}

func roomInitialLevel(code string) string {
	if code == "trophy_case" {
		return "static"
	}
	return "bronze"
}

func roomDefaultVariant(code, level string) string {
	if code == "trophy_case" {
		return "trophy_case_default"
	}
	return roomVariant(code, firstNonEmpty(level, roomInitialLevel(code)))
}

func roomDefaultState(code string) map[string]any {
	if code == "trophy_case" {
		return map[string]any{
			"presentation_mode": "achievement_case",
			"case_variant":      "default",
			"achievement_count": 0,
			"achievements":      []TrophyAchievement{},
			"explanation":       trophyCaseExplanation(nil),
			"linked_tasks":      []string{},
		}
	}
	return map[string]any{"glow": false, "track": "react"}
}

func pairKey(a, b string) string {
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}

func sortUsers(users []User) {
	sort.Slice(users, func(i, j int) bool {
		if users[i].Profile.CurrentSkillScore == users[j].Profile.CurrentSkillScore {
			if users[i].Profile.ConfidenceScore == users[j].Profile.ConfidenceScore {
				if users[i].LastActiveAt.Equal(users[j].LastActiveAt) {
					return users[i].Profile.CompletedChallenges > users[j].Profile.CompletedChallenges
				}
				return users[i].LastActiveAt.After(users[j].LastActiveAt)
			}
			return users[i].Profile.ConfidenceScore > users[j].Profile.ConfidenceScore
		}
		return users[i].Profile.CurrentSkillScore > users[j].Profile.CurrentSkillScore
	})
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (a *App) cacheChallengeView(ctx context.Context, view ChallengeInstanceView) {
	if a.ops == nil {
		return
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return
	}
	_ = a.ops.Set(ctx, "challenge:view:"+view.Instance.ID, payload, 2*time.Hour)
}

func (a *App) invalidateChallengeViewCache(ctx context.Context, instanceID string) {
	if a.ops == nil || instanceID == "" {
		return
	}
	_ = a.ops.Delete(ctx, "challenge:view:"+instanceID)
}

func (a *App) invalidateFriendRankingCaches(ctx context.Context, userIDs ...string) {
	if a.ops == nil {
		return
	}
	keys := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		keys = append(keys, fmt.Sprintf("rankings:friends::%s", userID))
	}
	_ = a.ops.Delete(ctx, keys...)
}

func (a *App) invalidateRankingCachesLocked(ctx context.Context, userID string) {
	a.rankingSnapshot.RefreshedAt = time.Time{}
	a.rankingSnapshot.Global = nil
	a.rankingSnapshot.ByCountry = nil
	a.rankingSnapshot.CandidateIDs = nil
	if a.ops == nil {
		return
	}
	keys := []string{"rankings:global::"}
	if user, ok := a.users[userID]; ok && user.Country != "" {
		keys = append(keys, fmt.Sprintf("rankings:country:%s:", user.Country))
	}
	keys = append(keys, fmt.Sprintf("rankings:friends::%s", userID))
	for peerID, relation := range a.friendships[userID] {
		if relation.Status == "accepted" {
			keys = append(keys, fmt.Sprintf("rankings:friends::%s", peerID))
		}
	}
	_ = a.ops.Delete(ctx, keys...)
}

func (a *App) recordAIInteraction(ctx context.Context, interaction AIInteraction) error {
	a.mu.Lock()
	a.aiInteractions = append(a.aiInteractions, interaction)
	a.mu.Unlock()
	return a.persistAIInteraction(ctx, interaction)
}

func (a *App) countAIInteractionsLocked(userID, instanceID, interactionType string) int {
	count := 0
	for _, interaction := range a.aiInteractions {
		if interaction.UserID == userID && interaction.ChallengeInstanceID == instanceID && interaction.InteractionType == interactionType {
			count++
		}
	}
	return count
}

func (a *App) latestSubmissionAndEvaluationLocked(instanceID, submissionID string) (Submission, EvaluationResult, bool) {
	var latestSubmission Submission
	var latestEvaluation EvaluationResult
	found := false
	for _, submission := range a.submissions {
		if submission.ChallengeInstanceID != instanceID {
			continue
		}
		if submissionID != "" && submission.ID != submissionID {
			continue
		}
		evaluation, ok := a.evaluations[submission.ID]
		if !ok {
			continue
		}
		if !found || submission.SubmittedAt.After(latestSubmission.SubmittedAt) {
			latestSubmission = submission
			latestEvaluation = evaluation
			found = true
		}
	}
	return latestSubmission, latestEvaluation, found
}

func antiCheatFromEvaluationReport(report map[string]any) AntiCheatAssessment {
	raw, ok := report["anti_cheat"]
	if !ok {
		return AntiCheatAssessment{}
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return AntiCheatAssessment{}
	}
	var assessment AntiCheatAssessment
	if err := json.Unmarshal(payload, &assessment); err != nil {
		return AntiCheatAssessment{}
	}
	return assessment
}

func runnerReportFromEvaluationReport(report map[string]any) RunnerReport {
	executionCost := reportInt(report, "execution_cost_ms")
	if executionCost == 0 {
		executionCost = reportInt(report, "execution_time_ms")
	}
	return RunnerReport{
		TestsPassed:       reportInt(report, "tests_passed"),
		TestsTotal:        reportInt(report, "tests_total"),
		LintErrors:        reportInt(report, "lint_errors"),
		LintWarnings:      reportInt(report, "lint_warnings"),
		ExecutionCostMS:   int64(executionCost),
		ExecutionTimeMS:   int64(reportInt(report, "execution_time_ms")),
		SolveTimeSeconds:  reportInt(report, "solve_time_seconds"),
		EditCount:         reportInt(report, "edit_count"),
		PasteEvents:       reportInt(report, "paste_events"),
		FocusLossEvents:   reportInt(report, "focus_loss_events"),
		SnapshotEvents:    reportInt(report, "snapshot_events"),
		FirstInputSeconds: reportInt(report, "first_input_secs"),
		AttemptNumber:     reportInt(report, "attempt_number"),
		HiddenPassed:      reportInt(report, "hidden_passed"),
		HiddenFailed:      reportInt(report, "hidden_failed"),
		QualityPassed:     reportInt(report, "quality_passed"),
		QualityFailed:     reportInt(report, "quality_failed"),
		SimilarityScore:   reportFloat(report, "similarity_score"),
		SuspicionScore:    reportInt(report, "suspicion_score"),
	}
}

func reportInt(report map[string]any, key string) int {
	value, ok := report[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(math.Round(typed))
	case json.Number:
		v, _ := typed.Int64()
		return int(v)
	default:
		return 0
	}
}

func reportFloat(report map[string]any, key string) float64 {
	value, ok := report[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		v, _ := typed.Float64()
		return v
	default:
		return 0
	}
}

func (a *App) latestUserEvaluationLocked(userID string) (EvaluationResult, RunnerReport, AntiCheatAssessment, bool) {
	var latestEvaluation EvaluationResult
	var latestSubmission Submission
	found := false
	for submissionID, evaluation := range a.evaluations {
		submission, ok := a.submissions[submissionID]
		if !ok {
			continue
		}
		instance, ok := a.instances[submission.ChallengeInstanceID]
		if !ok || instance.UserID != userID {
			continue
		}
		if !found || submission.SubmittedAt.After(latestSubmission.SubmittedAt) {
			latestSubmission = submission
			latestEvaluation = evaluation
			found = true
		}
	}
	if !found {
		return EvaluationResult{}, RunnerReport{}, AntiCheatAssessment{}, false
	}
	return latestEvaluation, runnerReportFromEvaluationReport(latestEvaluation.Report), antiCheatFromEvaluationReport(latestEvaluation.Report), true
}

func structToMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func (a *App) MarshalState() ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return json.Marshal(a.users)
}

func (a *App) rebuildDerivedStateFromPersistence() {
	for userID, user := range a.users {
		if _, ok := a.userSkills[userID]; !ok {
			a.userSkills[userID] = map[string]UserSkill{}
			for _, skill := range a.skills {
				a.userSkills[userID][skill.Code] = UserSkill{
					ID:             id.New("usk"),
					UserID:         userID,
					SkillID:        skill.ID,
					SkillCode:      skill.Code,
					Confidence:     50,
					Level:          "bronze",
					LastVerifiedAt: user.CreatedAt,
					DecayFactor:    1,
					UpdatedAt:      user.CreatedAt,
				}
			}
		}
		a.normalizeUserRoomItemsLocked(userID, user.CreatedAt)
		a.normalizeUserMonetizationLocked(userID, user.Role, user.CreatedAt)
	}

	type userScore struct {
		userID    string
		createdAt time.Time
		score     float64
	}
	var ordered []userScore
	for submissionID, evaluation := range a.evaluations {
		submission, ok := a.submissions[submissionID]
		if !ok {
			continue
		}
		instance, ok := a.instances[submission.ChallengeInstanceID]
		if !ok {
			continue
		}
		ordered = append(ordered, userScore{
			userID:    instance.UserID,
			createdAt: evaluation.CreatedAt,
			score:     evaluation.FinalScore,
		})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].createdAt.Before(ordered[j].createdAt)
	})
	a.scoreHistory = map[string][]float64{}
	for _, item := range ordered {
		a.scoreHistory[item.userID] = append(a.scoreHistory[item.userID], item.score)
	}
	a.recomputeRankingsLocked()
}

func (a *App) normalizeUserRoomItemsLocked(userID string, fallbackTime time.Time) {
	if _, ok := a.userRoomItems[userID]; !ok {
		a.userRoomItems[userID] = map[string]UserRoomItem{}
	}
	items := a.userRoomItems[userID]
	for code := range items {
		if _, ok := a.roomItems[code]; !ok {
			delete(items, code)
		}
	}
	for _, roomItem := range a.roomItems {
		item, ok := items[roomItem.Code]
		if !ok {
			level := roomInitialLevel(roomItem.Code)
			items[roomItem.Code] = UserRoomItem{
				ID:             id.New("rit"),
				UserID:         userID,
				RoomItemID:     roomItem.ID,
				RoomItemCode:   roomItem.Code,
				CurrentLevel:   level,
				CurrentVariant: roomDefaultVariant(roomItem.Code, level),
				State:          roomDefaultState(roomItem.Code),
				UpdatedAt:      fallbackTime,
			}
			continue
		}
		item.RoomItemID = roomItem.ID
		item.RoomItemCode = roomItem.Code
		if item.State == nil {
			item.State = roomDefaultState(roomItem.Code)
		}
		if roomItem.Code == "trophy_case" {
			item.CurrentLevel = roomInitialLevel(roomItem.Code)
			item.CurrentVariant = roomDefaultVariant(roomItem.Code, item.CurrentLevel)
			item.State["presentation_mode"] = "achievement_case"
			item.State["case_variant"] = "default"
			if _, ok := item.State["achievement_count"]; !ok {
				item.State["achievement_count"] = 0
			}
			if _, ok := item.State["achievements"]; !ok {
				item.State["achievements"] = []TrophyAchievement{}
			}
			if _, ok := item.State["explanation"]; !ok {
				item.State["explanation"] = trophyCaseExplanation(nil)
			}
			if _, ok := item.State["linked_tasks"]; !ok {
				item.State["linked_tasks"] = []string{}
			}
			delete(item.State, "level")
			items[roomItem.Code] = item
			continue
		}
		if item.CurrentLevel == "" {
			item.CurrentLevel = roomInitialLevel(roomItem.Code)
		}
		if item.CurrentVariant == "" {
			item.CurrentVariant = roomDefaultVariant(roomItem.Code, item.CurrentLevel)
		}
		items[roomItem.Code] = item
	}
	a.userRoomItems[userID] = items
}

func (a *App) persistUserStateLocked(ctx context.Context, userID string) error {
	if a.store == nil {
		return nil
	}
	return a.store.UpsertUserAggregate(ctx, a.users[userID], a.listUserSkillsLocked(userID), a.listUserRoomItemsLocked(userID))
}

func (a *App) persistRefreshSessionLocked(ctx context.Context, token string) error {
	if a.store == nil {
		return nil
	}
	session, ok := a.refreshSessions[token]
	if !ok {
		return nil
	}
	return a.store.UpsertRefreshSession(ctx, session)
}

func (a *App) deleteRefreshSessionLocked(ctx context.Context, token string) error {
	if a.store == nil {
		return nil
	}
	return a.store.DeleteRefreshSession(ctx, token)
}

func (a *App) persistVariantLocked(ctx context.Context, variantID string) error {
	if a.store == nil {
		return nil
	}
	variant, ok := a.variants[variantID]
	if !ok {
		return nil
	}
	return a.store.UpsertChallengeVariant(ctx, variant)
}

func (a *App) persistInstanceLocked(ctx context.Context, instanceID string) error {
	if a.store == nil {
		return nil
	}
	instance, ok := a.instances[instanceID]
	if !ok {
		return nil
	}
	return a.store.UpsertChallengeInstance(ctx, instance)
}

func (a *App) persistTelemetryEventLocked(ctx context.Context, event TelemetryEvent) error {
	if a.store == nil {
		return nil
	}
	return a.store.InsertTelemetryEvent(ctx, event)
}

func (a *App) persistAIInteraction(ctx context.Context, interaction AIInteraction) error {
	if a.store == nil {
		return nil
	}
	return a.store.InsertAIInteraction(ctx, interaction)
}

func (a *App) persistSubmissionLocked(ctx context.Context, submissionID string) error {
	if a.store == nil {
		return nil
	}
	submission, ok := a.submissions[submissionID]
	if !ok {
		return nil
	}
	evaluation, ok := a.evaluations[submissionID]
	if !ok {
		return nil
	}
	return a.store.UpsertSubmissionAndEvaluation(ctx, submission, evaluation)
}

func (a *App) persistScoreEventsLocked(ctx context.Context, userID string) error {
	if a.store == nil {
		return nil
	}
	return a.store.UpsertScoreEvents(ctx, a.scoreEvents[userID])
}

func (a *App) persistFriendshipLocked(ctx context.Context, userID, peerID string) error {
	if a.store == nil {
		return nil
	}
	if relation, ok := a.friendships[userID][peerID]; ok {
		if err := a.store.UpsertFriendship(ctx, relation); err != nil {
			return err
		}
	}
	if relation, ok := a.friendships[peerID][userID]; ok {
		if err := a.store.UpsertFriendship(ctx, relation); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) persistChatLocked(ctx context.Context, chatID string) error {
	if a.store == nil {
		return nil
	}
	chat, ok := a.chats[chatID]
	if !ok {
		return nil
	}
	var members []string
	for key, storedChatID := range a.directChats {
		if storedChatID != chatID {
			continue
		}
		parts := strings.Split(key, ":")
		members = append(members, parts...)
		break
	}
	return a.store.UpsertChat(ctx, chat, members)
}

func (a *App) persistMessageLocked(ctx context.Context, message ChatMessage) error {
	if a.store == nil {
		return nil
	}
	return a.store.InsertChatMessage(ctx, message)
}

func (a *App) persistCompanyLocked(ctx context.Context, companyID string) error {
	if a.store == nil {
		return nil
	}
	company, ok := a.companies[companyID]
	if !ok {
		return nil
	}
	return a.store.UpsertCompany(ctx, company)
}

func (a *App) persistJobLocked(ctx context.Context, jobID string) error {
	if a.store == nil {
		return nil
	}
	job, ok := a.jobs[jobID]
	if !ok {
		return nil
	}
	return a.store.UpsertJob(ctx, job)
}

func (a *App) persistShortlistLocked(ctx context.Context, entry HRShortlist) error {
	if a.store == nil {
		return nil
	}
	return a.store.InsertShortlist(ctx, entry)
}

func (a *App) persistRankingsLocked(ctx context.Context, userID string) error {
	if a.store == nil {
		return nil
	}
	a.ensureRankingSnapshotLocked(time.Now().UTC())
	global := a.rankingsFromSnapshotLocked("global", "", "")
	if err := a.store.UpsertRankingSnapshot(ctx, "global", "", "", global); err != nil {
		return err
	}
	seenCountries := map[string]struct{}{}
	for _, user := range a.users {
		if user.Role == RoleUser {
			if _, ok := seenCountries[user.Country]; !ok {
				seenCountries[user.Country] = struct{}{}
				countryEntries := a.rankingsFromSnapshotLocked("country", user.Country, "")
				if err := a.store.UpsertRankingSnapshot(ctx, "country", user.Country, "", countryEntries); err != nil {
					return err
				}
			}
		}
	}
	if userID != "" {
		friendEntries := a.rankingsFromSnapshotLocked("friends", "", userID)
		if err := a.store.UpsertRankingSnapshot(ctx, "friends", "", userID, friendEntries); err != nil {
			return err
		}
	}
	return nil
}
