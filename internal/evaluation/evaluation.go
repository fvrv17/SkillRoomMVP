package evaluation

import (
	"math"
	"strings"
	"time"

	"github.com/fvrv17/mvp/internal/runner"
)

type ScoreInput struct {
	Result            runner.RunResult
	ExecutionBaseline int
	History           []float64
	QualityCheckIDs   []string
	LintWeight        float64
	TaskWeight        float64
}

type Breakdown struct {
	Correctness   float64
	LintQuality   float64
	TaskQuality   float64
	Quality       float64
	ExecutionCost float64
	Consistency   float64
	Final         float64
}

type ConfidenceInput struct {
	CurrentScore     float64
	CompletedTasks   int
	ConsistencyScore float64
	ChallengeScore   float64
	SolveTimeSeconds int
	AttemptNumber    int
	PasteEvents      int
	FocusLossEvents  int
	HiddenFailures   int
	SimilarityScore  float64
	SuspicionLevel   string
}

type ConfidenceAssessment struct {
	Score   float64
	Level   string
	Reasons []string
}

func Score(input ScoreInput) Breakdown {
	correctness := Correctness(input.Result)
	lintQuality := LintQuality(input.Result.Lint)
	taskQuality := TaskSpecificQuality(input.Result.TestResults, input.QualityCheckIDs)
	quality := Quality(lintQuality, taskQuality, input.LintWeight, input.TaskWeight)
	executionCost := ExecutionCost(firstNonZero(input.Result.ExecutionCostMS, input.Result.ExecutionTimeMS), input.ExecutionBaseline)
	consistency := Consistency(input.History)

	return Breakdown{
		Correctness:   round2(correctness),
		LintQuality:   round2(lintQuality),
		TaskQuality:   round2(taskQuality),
		Quality:       round2(quality),
		ExecutionCost: round2(executionCost),
		Consistency:   round2(consistency),
		Final:         round2(correctness*0.4 + quality*0.2 + executionCost*0.2 + consistency*0.2),
	}
}

func Correctness(result runner.RunResult) float64 {
	total := result.Passed + result.Failed
	if total == 0 {
		return 0
	}
	return clamp((float64(result.Passed) / float64(total)) * 100)
}

func Quality(lintQuality, taskQuality, lintWeight, taskWeight float64) float64 {
	if lintWeight <= 0 {
		lintWeight = 0.5
	}
	if taskWeight <= 0 {
		taskWeight = 0.5
	}
	totalWeight := lintWeight + taskWeight
	if totalWeight <= 0 {
		return 0
	}
	return clamp((lintQuality*lintWeight + taskQuality*taskWeight) / totalWeight)
}

func LintQuality(lint runner.LintResult) float64 {
	score := 1.0 - float64(lint.ErrorCount)*0.45 - float64(lint.WarningCount)*0.15
	return clamp(score * 100)
}

func TaskSpecificQuality(results []runner.TestResult, expectedCheckIDs []string) float64 {
	if len(expectedCheckIDs) > 0 {
		expected := map[string]bool{}
		for _, id := range expectedCheckIDs {
			expected[id] = false
		}
		for _, result := range results {
			if result.Kind != "quality" || result.CheckID == "" {
				continue
			}
			if _, ok := expected[result.CheckID]; ok && result.Passed {
				expected[result.CheckID] = true
			}
		}
		passed := 0
		for _, ok := range expected {
			if ok {
				passed++
			}
		}
		return clamp((float64(passed) / float64(len(expected))) * 100)
	}

	passed := 0
	total := 0
	for _, result := range results {
		if result.Kind != "quality" {
			continue
		}
		total++
		if result.Passed {
			passed++
		}
	}
	if total == 0 {
		return 60
	}
	return clamp((float64(passed) / float64(total)) * 100)
}

func ExecutionCost(executionCostMS int64, baselineMS int) float64 {
	if executionCostMS <= 0 || baselineMS <= 0 {
		return 60
	}
	ratio := float64(executionCostMS) / float64(baselineMS)
	switch {
	case ratio <= 1:
		return 100
	case ratio >= 2:
		return 20
	default:
		return clamp(100 - ((ratio - 1) * 80))
	}
}

func LintScore(lint runner.LintResult) float64 {
	return LintQuality(lint)
}

func Speed(executionCostMS int64, baselineMS int) float64 {
	return ExecutionCost(executionCostMS, baselineMS)
}

func Consistency(history []float64) float64 {
	if len(history) == 0 {
		return 60
	}
	start := 0
	if len(history) > 5 {
		start = len(history) - 5
	}
	sum := 0.0
	for _, score := range history[start:] {
		sum += score
	}
	return clamp(sum / float64(len(history[start:])))
}

func UpdateSkillScore(oldScore, challengeScore, weight float64) float64 {
	return round2(math.Max(0, oldScore*0.85+challengeScore*weight))
}

func UpdateConfidence(input ConfidenceInput) float64 {
	return AssessConfidence(input).Score
}

func AssessConfidence(input ConfidenceInput) ConfidenceAssessment {
	delta := 0.0
	reasons := []string{}

	if input.CompletedTasks < 3 {
		delta += 1
		reasons = append(reasons, "Confidence is still early because fewer than 3 tasks are complete.")
	} else {
		if input.CompletedTasks >= 12 {
			delta += 4
			reasons = append(reasons, "12+ completed tasks make the score more reliable.")
		} else if input.CompletedTasks >= 6 {
			delta += 3
			reasons = append(reasons, "Several completed tasks make the score more stable.")
		} else {
			delta += 2
			reasons = append(reasons, "Multiple completed tasks make the score more stable.")
		}
	}
	if input.ConsistencyScore >= 75 {
		delta += 2
		reasons = append(reasons, "Recent challenge results have stayed consistent.")
	}
	if input.AttemptNumber > 1 {
		delta -= float64(input.AttemptNumber-1) * 1.5
		reasons = append(reasons, "Multiple attempts reduced trust in the latest result.")
	}
	if input.SolveTimeSeconds > 0 && input.SolveTimeSeconds < 45 && input.ChallengeScore >= 85 {
		delta -= 3
		reasons = append(reasons, "A very fast high-scoring submission needs more evidence.")
	}
	if input.PasteEvents > 0 {
		delta -= float64(input.PasteEvents) * 4
		reasons = append(reasons, "Paste events lowered confidence in solution authorship.")
	}
	if input.FocusLossEvents >= 3 {
		delta -= 3
		reasons = append(reasons, "Frequent focus changes reduced confidence.")
	}
	if input.HiddenFailures > 0 {
		delta -= float64(input.HiddenFailures) * 2
		reasons = append(reasons, "Hidden test failures reduced trust in the visible result.")
	}
	if input.SimilarityScore >= 0.9 {
		delta -= 18
		reasons = append(reasons, "The submission is nearly identical to another solution.")
	} else if input.SimilarityScore >= 0.75 {
		delta -= 10
		reasons = append(reasons, "The submission is highly similar to another solution.")
	}
	switch input.SuspicionLevel {
	case "high":
		delta -= 15
		reasons = append(reasons, "High anti-cheat suspicion strongly reduced confidence.")
	case "medium":
		delta -= 8
		reasons = append(reasons, "Medium anti-cheat suspicion lowered confidence.")
	case "low":
		if input.PasteEvents == 0 && input.FocusLossEvents < 2 && input.SimilarityScore < 0.75 {
			reasons = append(reasons, "No major anomaly signals were recorded.")
		}
	}

	score := clamp(input.CurrentScore + delta)
	return ConfidenceAssessment{
		Score:   score,
		Level:   confidenceLevel(score),
		Reasons: dedupeReasons(reasons),
	}
}

func DecayFactor(lastActiveAt, now time.Time) float64 {
	if lastActiveAt.IsZero() || !now.After(lastActiveAt) {
		return 1
	}
	daysInactive := int(now.Sub(lastActiveAt).Hours() / 24)
	if daysInactive <= 21 {
		return 1
	}
	return math.Pow(0.99, float64(daysInactive-21))
}

func clamp(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func confidenceLevel(score float64) string {
	switch {
	case score >= 80:
		return "high"
	case score >= 55:
		return "medium"
	default:
		return "low"
	}
}

func dedupeReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return []string{"No completed challenge evidence is available yet."}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	if len(out) == 0 {
		return []string{"No completed challenge evidence is available yet."}
	}
	return out
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
