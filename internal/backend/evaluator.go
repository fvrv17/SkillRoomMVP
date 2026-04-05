package backend

import (
	"math"
	"strings"
	"time"
)

func enrichRunnerReport(instance ChallengeInstance, report RunnerReport, summary TelemetrySummary, finishedAt time.Time) RunnerReport {
	report.SolveTimeSeconds = int(math.Round(finishedAt.Sub(instance.StartedAt).Seconds()))
	report.EditCount = summary.InputEvents + summary.SnapshotEvents
	report.PasteEvents = summary.PasteEvents
	report.FocusLossEvents = summary.FocusLossEvents
	report.SnapshotEvents = summary.SnapshotEvents
	report.FirstInputSeconds = summary.TimeToFirstInputSeconds
	return report
}

func (a *App) finalizeRunnerReportLocked(userID string, instance ChallengeInstance, submission Submission, report RunnerReport) (RunnerReport, AntiCheatAssessment) {
	summary := summarizeTelemetry(instance, a.telemetryEvents[instance.ID], submission.SubmittedAt)
	report = enrichRunnerReport(instance, report, summary, submission.SubmittedAt)
	report.AttemptNumber = instance.AttemptNumber
	report.SimilarityScore = a.solutionSimilarityLocked(userID, instance, submission.RawCodeText)
	antiCheat := assessAntiCheat(summary, report)
	report.SuspicionScore = antiCheat.Score
	return report, antiCheat
}

func summarizeTelemetry(instance ChallengeInstance, events []TelemetryEvent, submittedAt time.Time) TelemetrySummary {
	firstInputSeconds := -1
	lastEventAt := instance.StartedAt
	summary := TelemetrySummary{}
	for _, event := range events {
		if event.CreatedAt.After(lastEventAt) {
			lastEventAt = event.CreatedAt
		}
		offset := event.OffsetSeconds
		if offset <= 0 && !event.CreatedAt.IsZero() && event.CreatedAt.After(instance.StartedAt) {
			offset = int(math.Round(event.CreatedAt.Sub(instance.StartedAt).Seconds()))
		}
		switch event.EventType {
		case "input":
			summary.InputEvents++
			if firstInputSeconds < 0 {
				firstInputSeconds = maxInt(offset, 0)
			}
		case "paste":
			summary.PasteEvents++
			if firstInputSeconds < 0 {
				firstInputSeconds = maxInt(offset, 0)
			}
		case "focus_lost":
			summary.FocusLossEvents++
		case "snapshot":
			summary.SnapshotEvents++
			if firstInputSeconds < 0 {
				firstInputSeconds = maxInt(offset, 0)
			}
		}
	}
	if firstInputSeconds < 0 {
		firstInputSeconds = maxInt(int(math.Round(submittedAt.Sub(instance.StartedAt).Seconds())), 0)
	}
	summary.TimeToFirstInputSeconds = firstInputSeconds
	summary.LastEventAt = lastEventAt
	return summary
}

func assessAntiCheat(summary TelemetrySummary, report RunnerReport) AntiCheatAssessment {
	reasons := []string{}
	score := 0

	if report.AttemptNumber > 1 {
		score += (report.AttemptNumber - 1) * 4
		reasons = append(reasons, "multiple attempts were needed for this template")
	}
	if summary.PasteEvents > 0 {
		score += 12
		reasons = append(reasons, "paste events recorded during challenge")
	}
	if summary.FocusLossEvents >= 2 {
		score += 8
		reasons = append(reasons, "frequent focus loss suggests tab switching")
	}
	if summary.TimeToFirstInputSeconds > 0 && summary.TimeToFirstInputSeconds <= 10 {
		score += 15
		reasons = append(reasons, "very short time to first input")
	} else if summary.TimeToFirstInputSeconds > 0 && summary.TimeToFirstInputSeconds <= 30 {
		score += 8
		reasons = append(reasons, "unusually fast time to first input")
	}
	if report.SimilarityScore >= 0.9 {
		score += 45
		reasons = append(reasons, "solution is nearly identical to another submission")
	} else if report.SimilarityScore >= 0.75 {
		score += 30
		reasons = append(reasons, "solution is highly similar to another submission")
	}
	if report.HiddenFailed > 0 {
		score += report.HiddenFailed * 4
		reasons = append(reasons, "hidden test failures remained")
	}
	if summary.InputEvents < 2 && summary.SnapshotEvents == 0 {
		score += 6
		reasons = append(reasons, "limited solution evolution was recorded")
	}

	level := "low"
	switch {
	case score >= 60:
		level = "high"
	case score >= 25:
		level = "medium"
	}
	return AntiCheatAssessment{
		Level:           level,
		Score:           score,
		Reasons:         reasons,
		Signals:         summary,
		SimilarityScore: round2(report.SimilarityScore),
	}
}

func (a *App) solutionSimilarityLocked(userID string, instance ChallengeInstance, code string) float64 {
	maxSimilarity := 0.0
	for _, submission := range a.submissions {
		if submission.ChallengeInstanceID == instance.ID {
			continue
		}
		otherInstance, ok := a.instances[submission.ChallengeInstanceID]
		if !ok || otherInstance.UserID == userID || otherInstance.TemplateID != instance.TemplateID {
			continue
		}
		similarity := sourceSimilarity(code, submission.RawCodeText)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}
	return round2(maxSimilarity)
}

func sourceSimilarity(left, right string) float64 {
	leftTokens := tokenSet(normalizeSource(left))
	rightTokens := tokenSet(normalizeSource(right))
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	union := len(leftTokens)
	for token := range rightTokens {
		if _, ok := leftTokens[token]; ok {
			intersection++
			continue
		}
		union++
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func normalizeSource(source string) string {
	var builder strings.Builder
	builder.Grow(len(source))
	for _, r := range strings.ToLower(source) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte(' ')
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func tokenSet(text string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, token := range strings.Fields(text) {
		if len(token) < 2 {
			continue
		}
		set[token] = struct{}{}
	}
	return set
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
