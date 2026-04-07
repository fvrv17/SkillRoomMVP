package backend

import "time"

type Role string

const (
	RoleUser  Role = "user"
	RoleHR    Role = "hr"
	RoleAdmin Role = "admin"
)

type User struct {
	ID           string      `json:"id"`
	Email        string      `json:"email"`
	Username     string      `json:"username"`
	PasswordHash string      `json:"-"`
	Role         Role        `json:"role"`
	Country      string      `json:"country"`
	CreatedAt    time.Time   `json:"created_at"`
	LastActiveAt time.Time   `json:"last_active_at"`
	Profile      UserProfile `json:"profile"`
}

type UserProfile struct {
	UserID              string    `json:"user_id"`
	SelectedTrack       string    `json:"selected_track"`
	Bio                 string    `json:"bio,omitempty"`
	AvatarURL           string    `json:"avatar_url,omitempty"`
	CurrentSkillScore   float64   `json:"current_skill_score"`
	PercentileGlobal    float64   `json:"percentile_global"`
	PercentileCountry   float64   `json:"percentile_country"`
	StreakDays          int       `json:"streak_days"`
	ConfidenceScore     float64   `json:"confidence_score"`
	ConfidenceLevel     string    `json:"confidence_level,omitempty"`
	ConfidenceReasons   []string  `json:"confidence_reasons,omitempty"`
	CompletedChallenges int       `json:"completed_challenges"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Skill struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Code     string `json:"code"`
}

type UserSkill struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	SkillID        string    `json:"skill_id"`
	SkillCode      string    `json:"skill_code"`
	Score          float64   `json:"score"`
	Confidence     float64   `json:"confidence"`
	Level          string    `json:"level"`
	LastVerifiedAt time.Time `json:"last_verified_at"`
	DecayFactor    float64   `json:"decay_factor"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type RoomItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slot           string `json:"slot"`
	RelatedSkillID string `json:"related_skill_id"`
	Code           string `json:"code"`
}

type UserRoomItem struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	RoomItemID     string         `json:"room_item_id"`
	RoomItemCode   string         `json:"room_item_code"`
	CurrentLevel   string         `json:"current_level"`
	CurrentVariant string         `json:"current_variant"`
	State          map[string]any `json:"state_json"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type TrophyAchievement struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ChallengeTemplate struct {
	ID                   string              `json:"id"`
	Slug                 string              `json:"slug"`
	Title                string              `json:"title"`
	Difficulty           int                 `json:"difficulty"`
	Description          string              `json:"description_md"`
	Category             string              `json:"category"`
	AssetDirectory       string              `json:"asset_directory,omitempty"`
	EditableFiles        []string            `json:"editable_files,omitempty"`
	StarterCodeTemplate  string              `json:"starter_code_template"`
	VisibleTestsTemplate string              `json:"visible_tests_template"`
	EvaluationConfig     EvaluationConfig    `json:"evaluation_config_json"`
	IsActive             bool                `json:"is_active"`
	Track                string              `json:"track"`
	VariationStrings     map[string][]string `json:"variation_strings"`
	VariationNumbers     map[string][]int    `json:"variation_numbers"`
	SkillWeights         map[string]float64  `json:"skill_weights"`
}

type EvaluationConfig struct {
	MedianSolveSeconds int      `json:"median_solve_seconds"`
	MedianExecMS       int      `json:"median_exec_ms"`
	MaxAttempts        int      `json:"max_attempts"`
	TimeoutMS          int      `json:"timeout_ms"`
	MemoryMB           int      `json:"memory_mb"`
	TargetRenderCount  int      `json:"target_render_count"`
	LintQualityWeight  float64  `json:"lint_quality_weight,omitempty"`
	TaskQualityWeight  float64  `json:"task_quality_weight,omitempty"`
	QualityCheckIDs    []string `json:"quality_check_ids,omitempty"`
}

type ChallengeVariant struct {
	ID              string            `json:"id"`
	TemplateID      string            `json:"template_id"`
	VariantHash     string            `json:"variant_hash"`
	Seed            int64             `json:"seed"`
	Params          map[string]any    `json:"params_json"`
	GeneratedFiles  map[string]string `json:"generated_files"`
	VisibleTests    map[string]string `json:"visible_tests,omitempty"`
	EditableFiles   []string          `json:"editable_files,omitempty"`
	StarterCodePath string            `json:"starter_code_path"`
	TestBundlePath  string            `json:"test_bundle_path"`
}

type ChallengeInstance struct {
	ID            string            `json:"id"`
	UserID        string            `json:"user_id"`
	TemplateID    string            `json:"template_id"`
	VariantID     string            `json:"variant_id"`
	Category      string            `json:"category"`
	StartedAt     time.Time         `json:"started_at"`
	ExpiresAt     time.Time         `json:"expires_at"`
	Status        string            `json:"status"`
	AttemptNumber int               `json:"attempt_number"`
	VisibleFiles  map[string]string `json:"visible_files"`
}

type Submission struct {
	ID                  string            `json:"id"`
	ChallengeInstanceID string            `json:"challenge_instance_id"`
	SubmittedAt         time.Time         `json:"submitted_at"`
	SourceArchivePath   string            `json:"source_archive_path,omitempty"`
	RawCodeText         string            `json:"raw_code_text,omitempty"`
	SourceFiles         map[string]string `json:"source_files,omitempty"`
	Language            string            `json:"language"`
	ExecutionStatus     string            `json:"execution_status"`
}

type RunnerReport struct {
	TestsPassed       int           `json:"tests_passed"`
	TestsTotal        int           `json:"tests_total"`
	HiddenPassed      int           `json:"hidden_passed"`
	HiddenFailed      int           `json:"hidden_failed"`
	QualityPassed     int           `json:"quality_passed"`
	QualityFailed     int           `json:"quality_failed"`
	LintErrors        int           `json:"lint_errors"`
	LintWarnings      int           `json:"lint_warnings"`
	ExecutionCostMS   int64         `json:"execution_cost_ms"`
	ExecutionTimeMS   int64         `json:"execution_time_ms"`
	Errors            []string      `json:"errors,omitempty"`
	SolveTimeSeconds  int           `json:"solve_time_seconds"`
	EditCount         int           `json:"edit_count"`
	PasteEvents       int           `json:"paste_events"`
	FocusLossEvents   int           `json:"focus_loss_events"`
	SnapshotEvents    int           `json:"snapshot_events"`
	FirstInputSeconds int           `json:"first_input_seconds"`
	AttemptNumber     int           `json:"attempt_number"`
	SimilarityScore   float64       `json:"similarity_score"`
	SuspicionScore    int           `json:"suspicion_score"`
	Checks            []RunnerCheck `json:"checks,omitempty"`
}

type RunnerCheck struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Kind   string `json:"kind,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
	Passed bool   `json:"passed"`
}

type TelemetryEvent struct {
	ID                  string         `json:"id"`
	ChallengeInstanceID string         `json:"challenge_instance_id"`
	UserID              string         `json:"user_id"`
	EventType           string         `json:"event_type"`
	OffsetSeconds       int            `json:"offset_seconds"`
	Payload             map[string]any `json:"payload_json"`
	CreatedAt           time.Time      `json:"created_at"`
}

type TelemetrySummary struct {
	TimeToFirstInputSeconds int       `json:"time_to_first_input_seconds"`
	InputEvents             int       `json:"input_events"`
	PasteEvents             int       `json:"paste_events"`
	FocusLossEvents         int       `json:"focus_loss_events"`
	SnapshotEvents          int       `json:"snapshot_events"`
	LastEventAt             time.Time `json:"last_event_at"`
}

type AntiCheatAssessment struct {
	Level           string           `json:"level"`
	Score           int              `json:"score"`
	Reasons         []string         `json:"reasons"`
	Signals         TelemetrySummary `json:"signals"`
	SimilarityScore float64          `json:"similarity_score"`
}

type AIHintRequest struct {
	FocusArea string `json:"focus_area,omitempty"`
	Question  string `json:"question,omitempty"`
}

type AIHintResponse struct {
	Provider       string `json:"provider"`
	Hint           string `json:"hint"`
	FocusArea      string `json:"focus_area,omitempty"`
	UsedHints      int    `json:"used_hints"`
	RemainingHints int    `json:"remaining_hints"`
}

type AIExplanationRequest struct {
	SubmissionID string `json:"submission_id,omitempty"`
}

type AIExplanationResponse struct {
	Provider        string   `json:"provider"`
	Summary         string   `json:"summary"`
	Strengths       []string `json:"strengths"`
	Improvements    []string `json:"improvements"`
	SuspicionNotes  []string `json:"suspicion_notes"`
	RecommendedNext string   `json:"recommended_next"`
}

type AIMutationPreviewRequest struct {
	Seed int64 `json:"seed,omitempty"`
}

type AIMutationPreviewResponse struct {
	Provider        string            `json:"provider"`
	Seed            int64             `json:"seed"`
	Title           string            `json:"title"`
	Description     string            `json:"description_md"`
	VariableRenames map[string]string `json:"variable_renames"`
	ReviewerNotes   []string          `json:"reviewer_notes"`
}

type AIInteraction struct {
	ID                  string         `json:"id"`
	UserID              string         `json:"user_id"`
	ChallengeInstanceID string         `json:"challenge_instance_id,omitempty"`
	TemplateID          string         `json:"template_id,omitempty"`
	InteractionType     string         `json:"interaction_type"`
	Input               map[string]any `json:"input_json"`
	Output              map[string]any `json:"output_json"`
	Provider            string         `json:"provider"`
	CreatedAt           time.Time      `json:"created_at"`
}

type EvaluationResult struct {
	ID                 string         `json:"id"`
	SubmissionID       string         `json:"submission_id"`
	TestScore          float64        `json:"test_score"`
	LintScore          float64        `json:"lint_score"`
	PerfScore          float64        `json:"perf_score"`
	QualityScore       float64        `json:"quality_score"`
	ExecutionCostScore float64        `json:"execution_cost_score"`
	SpeedScore         float64        `json:"speed_score,omitempty"`
	ConsistencyScore   float64        `json:"consistency_score"`
	FinalScore         float64        `json:"final_score"`
	Report             map[string]any `json:"report_json"`
	CreatedAt          time.Time      `json:"created_at"`
}

type ScoreEvent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	SkillID    string    `json:"skill_id"`
	SourceID   string    `json:"source_id"`
	Delta      float64   `json:"delta"`
	ScoreAfter float64   `json:"score_after"`
	CreatedAt  time.Time `json:"created_at"`
	SourceType string    `json:"source_type"`
}

type Friendship struct {
	UserID       string    `json:"user_id"`
	FriendUserID string    `json:"friend_user_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Chat struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID           string    `json:"id"`
	ChatID       string    `json:"chat_id"`
	SenderUserID string    `json:"sender_user_id"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

type Company struct {
	ID          string         `json:"id"`
	OwnerUserID string         `json:"owner_user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	RoomState   map[string]any `json:"room_state_json"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Job struct {
	ID             string             `json:"id"`
	CompanyID      string             `json:"company_id"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	RequiredScore  float64            `json:"required_score"`
	RequiredSkills map[string]float64 `json:"required_skills_json"`
	CreatedAt      time.Time          `json:"created_at"`
}

type HRShortlist struct {
	ID        string    `json:"id"`
	CompanyID string    `json:"company_id"`
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

type CandidateSummary struct {
	Score           float64   `json:"score"`
	Percentile      float64   `json:"percentile"`
	ConfidenceScore float64   `json:"confidence_score"`
	ConfidenceLevel string    `json:"confidence_level,omitempty"`
	LastActiveAt    time.Time `json:"last_active_at"`
	TasksCompleted  int       `json:"tasks_completed"`
}

type CandidateView struct {
	Summary           CandidateSummary `json:"summary"`
	UserID            string           `json:"user_id"`
	Username          string           `json:"username"`
	Country           string           `json:"country"`
	CurrentSkillScore float64          `json:"current_skill_score"`
	PercentileGlobal  float64          `json:"percentile_global"`
	ConfidenceScore   float64          `json:"confidence_score"`
	ConfidenceLevel   string           `json:"confidence_level,omitempty"`
	ConfidenceReasons []string         `json:"confidence_reasons,omitempty"`
	LastActiveAt      time.Time        `json:"last_active_at"`
	TasksSolved       int              `json:"tasks_solved"`
	RecentActivity    []string         `json:"recent_activity,omitempty"`
	Strengths         []string         `json:"strengths,omitempty"`
	Weaknesses        []string         `json:"weaknesses,omitempty"`
}

type RankingEntry struct {
	UserID              string    `json:"user_id"`
	Username            string    `json:"username"`
	Country             string    `json:"country"`
	CurrentSkillScore   float64   `json:"current_skill_score"`
	ConfidenceScore     float64   `json:"confidence_score"`
	Percentile          float64   `json:"percentile"`
	Rank                int       `json:"rank"`
	LastActiveAt        time.Time `json:"last_active_at"`
	CompletedChallenges int       `json:"completed_challenges"`
}
