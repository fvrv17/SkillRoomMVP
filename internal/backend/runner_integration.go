package backend

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fvrv17/mvp/internal/platform/id"
	runsvc "github.com/fvrv17/mvp/internal/runner"
)

var (
	errRunnerUnavailable = errors.New("runner unavailable")
	errRunnerTimeout     = errors.New("runner timed out")
)

func (a *App) runChallenge(ctx context.Context, userID, instanceID string, req SubmitChallengeRequest) (map[string]any, error) {
	if strings.TrimSpace(req.Language) == "" {
		return nil, errors.New("language is required")
	}

	a.mu.RLock()
	instance, ok := a.instances[instanceID]
	if !ok || instance.UserID != userID {
		a.mu.RUnlock()
		return nil, errors.New("challenge instance not found")
	}
	templateDef := a.templates[instance.TemplateID]
	variant := a.variants[instance.VariantID]
	if instance.AttemptNumber > templateDef.EvaluationConfig.MaxAttempts {
		a.mu.RUnlock()
		return nil, errors.New("attempt limit reached")
	}
	events := append([]TelemetryEvent(nil), a.telemetryEvents[instanceID]...)
	a.mu.RUnlock()

	workspaceFiles, submittedFiles, sourceText, err := prepareWorkspaceFiles(variant, req)
	if err != nil {
		return nil, err
	}

	report, err := a.executeChallenge(ctx, templateDef, variant, req.Language, workspaceFiles)
	if err != nil {
		return nil, err
	}

	finishedAt := time.Now().UTC()
	summary := summarizeTelemetry(instance, events, finishedAt)
	report = enrichRunnerReport(instance, report, summary, finishedAt)
	report.AttemptNumber = instance.AttemptNumber

	return map[string]any{
		"status":          "preview",
		"instance_id":     instanceID,
		"template_id":     templateDef.ID,
		"execution":       report,
		"telemetry":       summary,
		"submitted_files": submittedFiles,
		"source_text":     sourceText,
	}, nil
}

func (a *App) submitChallenge(ctx context.Context, userID, instanceID string, req SubmitChallengeRequest) (map[string]any, error) {
	if strings.TrimSpace(req.Language) == "" {
		return nil, errors.New("language is required")
	}

	a.mu.RLock()
	instance, ok := a.instances[instanceID]
	if !ok || instance.UserID != userID {
		a.mu.RUnlock()
		return nil, errors.New("challenge instance not found")
	}
	templateDef := a.templates[instance.TemplateID]
	variant := a.variants[instance.VariantID]
	if instance.AttemptNumber > templateDef.EvaluationConfig.MaxAttempts {
		a.mu.RUnlock()
		return nil, errors.New("attempt limit reached")
	}
	a.mu.RUnlock()

	workspaceFiles, submittedFiles, sourceText, err := prepareWorkspaceFiles(variant, req)
	if err != nil {
		return nil, err
	}

	baseReport, err := a.executeChallenge(ctx, templateDef, variant, req.Language, workspaceFiles)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	instance, ok = a.instances[instanceID]
	if !ok || instance.UserID != userID {
		return nil, errors.New("challenge instance not found")
	}
	templateDef = a.templates[instance.TemplateID]
	variant = a.variants[instance.VariantID]
	if instance.AttemptNumber > templateDef.EvaluationConfig.MaxAttempts {
		return nil, errors.New("attempt limit reached")
	}

	submission := Submission{
		ID:                  id.New("sub"),
		ChallengeInstanceID: instance.ID,
		SubmittedAt:         time.Now().UTC(),
		RawCodeText:         sourceText,
		SourceFiles:         submittedFiles,
		Language:            req.Language,
		ExecutionStatus:     "done",
	}
	a.submissions[submission.ID] = submission

	report, antiCheat := a.finalizeRunnerReportLocked(userID, instance, submission, baseReport)
	evaluation := a.evaluateSubmissionLocked(userID, templateDef, submission, report, antiCheat)
	a.evaluations[submission.ID] = evaluation

	instance.Status = "done"
	a.instances[instanceID] = instance

	a.applySkillUpdateLocked(userID, templateDef, evaluation, report)
	a.recomputeRankingsLocked()
	if err := a.persistInstanceLocked(ctx, instanceID); err != nil {
		return nil, err
	}
	if err := a.persistSubmissionLocked(ctx, submission.ID); err != nil {
		return nil, err
	}
	if err := a.persistUserStateLocked(ctx, userID); err != nil {
		return nil, err
	}
	if err := a.persistScoreEventsLocked(ctx, userID); err != nil {
		return nil, err
	}
	if err := a.persistRankingsLocked(ctx, userID); err != nil {
		return nil, err
	}
	a.invalidateRankingCachesLocked(ctx, userID)
	a.invalidateChallengeViewCache(ctx, instanceID)

	user := a.users[userID]
	return map[string]any{
		"submission":    submission,
		"evaluation":    evaluation,
		"execution":     report,
		"anti_cheat":    antiCheat,
		"profile":       user.Profile,
		"skills":        a.listUserSkillsLocked(userID),
		"room":          a.listUserRoomItemsLocked(userID),
		"telemetry":     antiCheat.Signals,
		"template_id":   templateDef.ID,
		"instance_id":   instanceID,
		"challenge_id":  templateDef.ID,
		"visible_tests": variant.VisibleTests,
	}, nil
}

func (a *App) executeChallenge(ctx context.Context, templateDef ChallengeTemplate, variant ChallengeVariant, language string, files map[string]string) (RunnerReport, error) {
	engine := a.runner
	if engine == nil {
		return RunnerReport{}, fmt.Errorf("%w: engine is not configured", errRunnerUnavailable)
	}
	result, err := engine.Run(ctx, runsvc.RunRequest{
		Language:      language,
		Files:         files,
		EditableFiles: append([]string(nil), variant.EditableFiles...),
		TimeoutMS:     templateDef.EvaluationConfig.TimeoutMS,
		MemoryMB:      templateDef.EvaluationConfig.MemoryMB,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timed out") {
			return RunnerReport{}, fmt.Errorf("%w: %v", errRunnerTimeout, err)
		}
		return RunnerReport{}, fmt.Errorf("%w: %v", errRunnerUnavailable, err)
	}
	return runnerReportFromResult(result), nil
}

func runnerReportFromResult(result runsvc.RunResult) RunnerReport {
	checks := make([]RunnerCheck, 0, len(result.TestResults))
	for _, testResult := range result.TestResults {
		checks = append(checks, RunnerCheck{
			ID:     testResult.CheckID,
			Name:   fmt.Sprintf("%s: %s", testResult.File, testResult.Name),
			Kind:   firstNonEmpty(testResult.Kind, "correctness"),
			Hidden: testResult.Hidden,
			Passed: testResult.Passed,
		})
	}

	executionCostMS := result.ExecutionCostMS
	if executionCostMS <= 0 {
		executionCostMS = result.ExecutionTimeMS
	}

	return RunnerReport{
		TestsPassed:     result.Passed,
		TestsTotal:      result.Passed + result.Failed,
		HiddenPassed:    result.HiddenPassed,
		HiddenFailed:    result.HiddenFailed,
		QualityPassed:   result.QualityPassed,
		QualityFailed:   result.QualityFailed,
		LintErrors:      result.Lint.ErrorCount,
		LintWarnings:    result.Lint.WarningCount,
		ExecutionCostMS: executionCostMS,
		ExecutionTimeMS: result.ExecutionTimeMS,
		Errors:          append([]string(nil), result.Errors...),
		Checks:          checks,
	}
}

func prepareWorkspaceFiles(variant ChallengeVariant, req SubmitChallengeRequest) (map[string]string, map[string]string, string, error) {
	if len(variant.GeneratedFiles) == 0 {
		return nil, nil, "", errors.New("challenge variant has no generated files")
	}

	editable := map[string]struct{}{}
	for _, file := range variant.EditableFiles {
		editable[file] = struct{}{}
	}

	submittedFiles := map[string]string{}
	if strings.TrimSpace(req.RawCodeText) != "" {
		if len(variant.EditableFiles) != 1 {
			return nil, nil, "", errors.New("raw_code_text requires exactly one editable file")
		}
		submittedFiles[variant.EditableFiles[0]] = req.RawCodeText
	}
	for name, content := range req.SourceFiles {
		if _, ok := editable[name]; !ok {
			return nil, nil, "", fmt.Errorf("file %s is not editable", name)
		}
		submittedFiles[name] = content
	}
	if len(submittedFiles) == 0 {
		return nil, nil, "", errors.New("source code is required")
	}

	workspaceFiles := map[string]string{}
	for name, content := range variant.GeneratedFiles {
		workspaceFiles[name] = content
	}
	for name, content := range submittedFiles {
		workspaceFiles[name] = content
	}

	return workspaceFiles, submittedFiles, MergeFiles(submittedFiles), nil
}

func sortedEditableFiles(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
