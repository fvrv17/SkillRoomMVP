package runner

import "context"

type RunRequest struct {
	Language      string            `json:"language"`
	Files         map[string]string `json:"files"`
	EditableFiles []string          `json:"editable_files,omitempty"`
	TimeoutMS     int               `json:"timeout_ms,omitempty"`
	MemoryMB      int               `json:"memory_mb,omitempty"`
	CPUFraction   string            `json:"cpu_fraction,omitempty"`
}

type TestResult struct {
	File       string  `json:"file"`
	Name       string  `json:"name"`
	CheckID    string  `json:"check_id,omitempty"`
	Kind       string  `json:"kind,omitempty"`
	Passed     bool    `json:"passed"`
	Hidden     bool    `json:"hidden"`
	DurationMS float64 `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
}

type LintMessage struct {
	File     string `json:"file"`
	RuleID   string `json:"rule_id,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type LintResult struct {
	ErrorCount   int           `json:"error_count"`
	WarningCount int           `json:"warning_count"`
	Messages     []LintMessage `json:"messages,omitempty"`
}

type RunResult struct {
	TestResults     []TestResult `json:"test_results"`
	Passed          int          `json:"passed"`
	Failed          int          `json:"failed"`
	VisiblePassed   int          `json:"visible_passed"`
	VisibleFailed   int          `json:"visible_failed"`
	HiddenPassed    int          `json:"hidden_passed"`
	HiddenFailed    int          `json:"hidden_failed"`
	QualityPassed   int          `json:"quality_passed"`
	QualityFailed   int          `json:"quality_failed"`
	ExecutionCostMS int64        `json:"execution_cost_ms"`
	ExecutionTimeMS int64        `json:"execution_time_ms"`
	Errors          []string     `json:"errors,omitempty"`
	Lint            LintResult   `json:"lint"`
}

type RunResponse struct {
	Status string    `json:"status"`
	Result RunResult `json:"result"`
}

type Engine interface {
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}
