package evaluation

import (
	"testing"
	"time"

	"github.com/fvrv17/mvp/internal/runner"
)

func TestScoreUsesRunnerOutput(t *testing.T) {
	breakdown := Score(ScoreInput{
		Result: runner.RunResult{
			Passed:          4,
			Failed:          1,
			ExecutionCostMS: 35,
			Lint: runner.LintResult{
				ErrorCount:   0,
				WarningCount: 1,
			},
			TestResults: []runner.TestResult{
				{Kind: "quality", CheckID: "memoized-selector", Passed: true},
			},
		},
		ExecutionBaseline: 30,
		History:           []float64{72, 78, 80},
		QualityCheckIDs:   []string{"memoized-selector"},
	})

	if breakdown.Correctness <= 0 {
		t.Fatalf("expected correctness score")
	}
	if breakdown.Final <= 0 {
		t.Fatalf("expected final score")
	}
}

func TestTaskSpecificQualityRequiresConfiguredChecks(t *testing.T) {
	score := TaskSpecificQuality([]runner.TestResult{
		{Kind: "quality", CheckID: "memoized-selector", Passed: true},
	}, []string{"memoized-selector", "stable-callback"})

	if score >= 100 {
		t.Fatalf("expected missing configured quality checks to reduce score, got %f", score)
	}
}

func TestExecutionCostPenalizesExpensiveSolutions(t *testing.T) {
	score := ExecutionCost(120, 50)
	if score >= 50 {
		t.Fatalf("expected higher execution cost to lower the score, got %f", score)
	}
}

func TestDecayFactorStartsAfterTwentyOneDays(t *testing.T) {
	now := time.Date(2025, 3, 24, 0, 0, 0, 0, time.UTC)
	lastActive := now.AddDate(0, 0, -25)
	factor := DecayFactor(lastActive, now)
	if factor >= 1 {
		t.Fatalf("expected decay factor below 1, got %f", factor)
	}
}

func TestUpdateConfidencePenalizesSuspicion(t *testing.T) {
	score := UpdateConfidence(ConfidenceInput{
		CurrentScore:     70,
		CompletedTasks:   4,
		ConsistencyScore: 82,
		ChallengeScore:   91,
		AttemptNumber:    2,
		PasteEvents:      1,
		HiddenFailures:   2,
		SuspicionLevel:   "medium",
	})
	if score >= 70 {
		t.Fatalf("expected confidence penalty, got %f", score)
	}
}

func TestAssessConfidenceReturnsReasonsAndLevel(t *testing.T) {
	assessment := AssessConfidence(ConfidenceInput{
		CurrentScore:     74,
		CompletedTasks:   12,
		ConsistencyScore: 85,
		ChallengeScore:   88,
		AttemptNumber:    1,
		SolveTimeSeconds: 210,
		SuspicionLevel:   "low",
	})

	if assessment.Level != "high" {
		t.Fatalf("expected high confidence, got %s", assessment.Level)
	}
	if assessment.Score <= 74 {
		t.Fatalf("expected confidence score increase, got %f", assessment.Score)
	}
	if len(assessment.Reasons) == 0 {
		t.Fatalf("expected confidence reasons")
	}
}
