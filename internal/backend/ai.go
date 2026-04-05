package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxHintsPerChallenge = 2

type AIProvider interface {
	GenerateHint(ctx context.Context, input AIHintContext) (AIHintResponse, error)
	ExplainEvaluation(ctx context.Context, input AIExplanationContext) (AIExplanationResponse, error)
	PreviewMutation(ctx context.Context, input AIMutationContext) (AIMutationPreviewResponse, error)
}

type AIHintContext struct {
	User       User
	Instance   ChallengeInstance
	Template   ChallengeTemplate
	Variant    ChallengeVariant
	Submission *Submission
	Evaluation *EvaluationResult
	Telemetry  TelemetrySummary
	UsedHints  int
	Request    AIHintRequest
	AntiCheat  *AntiCheatAssessment
}

type AIExplanationContext struct {
	User       User
	Instance   ChallengeInstance
	Template   ChallengeTemplate
	Variant    ChallengeVariant
	Submission Submission
	Evaluation EvaluationResult
	AntiCheat  AntiCheatAssessment
}

type AIMutationContext struct {
	Requester User
	Template  ChallengeTemplate
	Variant   ChallengeVariant
	Seed      int64
}

type CompositeAIProvider struct {
	primary  AIProvider
	fallback AIProvider
}

func NewCompositeAIProvider(primary, fallback AIProvider) *CompositeAIProvider {
	return &CompositeAIProvider{
		primary:  primary,
		fallback: fallback,
	}
}

func (c *CompositeAIProvider) GenerateHint(ctx context.Context, input AIHintContext) (AIHintResponse, error) {
	if c.primary != nil {
		if response, err := c.primary.GenerateHint(ctx, input); err == nil {
			return response, nil
		}
	}
	if c.fallback == nil {
		return AIHintResponse{}, errors.New("ai provider unavailable")
	}
	return c.fallback.GenerateHint(ctx, input)
}

func (c *CompositeAIProvider) ExplainEvaluation(ctx context.Context, input AIExplanationContext) (AIExplanationResponse, error) {
	if c.primary != nil {
		if response, err := c.primary.ExplainEvaluation(ctx, input); err == nil {
			return response, nil
		}
	}
	if c.fallback == nil {
		return AIExplanationResponse{}, errors.New("ai provider unavailable")
	}
	return c.fallback.ExplainEvaluation(ctx, input)
}

func (c *CompositeAIProvider) PreviewMutation(ctx context.Context, input AIMutationContext) (AIMutationPreviewResponse, error) {
	if c.primary != nil {
		if response, err := c.primary.PreviewMutation(ctx, input); err == nil {
			return response, nil
		}
	}
	if c.fallback == nil {
		return AIMutationPreviewResponse{}, errors.New("ai provider unavailable")
	}
	return c.fallback.PreviewMutation(ctx, input)
}

type DeterministicAIProvider struct{}

func NewDeterministicAIProvider() *DeterministicAIProvider {
	return &DeterministicAIProvider{}
}

func (d *DeterministicAIProvider) GenerateHint(_ context.Context, input AIHintContext) (AIHintResponse, error) {
	focusArea := firstNonEmpty(input.Request.FocusArea, input.Template.Category)
	hint := deterministicHint(input)
	return AIHintResponse{
		Provider:  "deterministic",
		Hint:      hint,
		FocusArea: focusArea,
	}, nil
}

func (d *DeterministicAIProvider) ExplainEvaluation(_ context.Context, input AIExplanationContext) (AIExplanationResponse, error) {
	strengths := []string{}
	improvements := []string{}

	if input.Evaluation.TestScore >= 75 {
		strengths = append(strengths, "Correctness held up on most of the task checks.")
	} else {
		improvements = append(improvements, deterministicCorrectnessAdvice(input.Template))
	}
	if input.Evaluation.QualityScore >= 70 {
		strengths = append(strengths, "Code quality stayed readable enough for reviewer follow-through.")
	} else {
		improvements = append(improvements, "Break the solution into smaller helpers or hooks so the intent is easier to audit.")
	}
	if input.Evaluation.PerfScore >= 70 {
		strengths = append(strengths, "The implementation avoided the main rerender or runtime traps for this category.")
	} else {
		improvements = append(improvements, deterministicPerformanceAdvice(input.Template))
	}
	executionCostScore := input.Evaluation.ExecutionCostScore
	if executionCostScore <= 0 {
		executionCostScore = input.Evaluation.SpeedScore
	}
	if executionCostScore < 55 {
		improvements = append(improvements, "Reduce repeated derived work or unbounded rendering so execution cost stays closer to the template baseline.")
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "The submission still established a workable baseline for the task.")
	}
	if len(improvements) == 0 {
		improvements = append(improvements, deterministicCategoryNextStep(input.Template.Category))
	}

	suspicionNotes := []string{"No strong suspicious pattern needed reviewer escalation."}
	if len(input.AntiCheat.Reasons) > 0 {
		suspicionNotes = append([]string{"Heuristic anti-cheat signals should be reviewed as context, not treated as proof."}, input.AntiCheat.Reasons...)
	}

	summary := fmt.Sprintf(
		"Final score %.2f with correctness %.2f, quality %.2f, runtime efficiency %.2f, and consistency %.2f.",
		input.Evaluation.FinalScore,
		input.Evaluation.TestScore,
		input.Evaluation.QualityScore,
		executionCostScore,
		input.Evaluation.ConsistencyScore,
	)

	return AIExplanationResponse{
		Provider:        "deterministic",
		Summary:         summary,
		Strengths:       strengths,
		Improvements:    improvements,
		SuspicionNotes:  suspicionNotes,
		RecommendedNext: deterministicCategoryNextStep(input.Template.Category),
	}, nil
}

func (d *DeterministicAIProvider) PreviewMutation(_ context.Context, input AIMutationContext) (AIMutationPreviewResponse, error) {
	variableRenames := map[string]string{}
	for key, value := range input.Variant.Params {
		if rendered, ok := value.(string); ok && rendered != "" {
			if strings.Contains(key, "name") || strings.Contains(key, "component") || strings.Contains(key, "feature") || strings.Contains(key, "state") {
				variableRenames[key] = rendered
			}
		}
	}

	notes := []string{
		fmt.Sprintf("Keep the logical rubric fixed while varying the %s surface language.", input.Template.Category),
		fmt.Sprintf("Difficulty %d means the mutated prompt should still preserve reviewer expectations.", input.Template.Difficulty),
	}
	if input.Template.Category == "performance" {
		notes = append(notes, "Preserve the same performance target so rankings stay comparable across users.")
	}

	return AIMutationPreviewResponse{
		Provider:        "deterministic",
		Seed:            input.Seed,
		Title:           RenderTitle(input.Template, input.Variant.Params),
		Description:     RenderDescription(input.Template, input.Variant.Params),
		VariableRenames: variableRenames,
		ReviewerNotes:   notes,
	}, nil
}

type OpenAIResponsesProvider struct {
	apiKey       string
	model        string
	baseURL      string
	organization string
	project      string
	client       *http.Client
}

func NewOpenAIResponsesProvider(apiKey, model, baseURL, organization, project string) *OpenAIResponsesProvider {
	if strings.TrimSpace(model) == "" {
		model = "gpt-4.1-mini"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIResponsesProvider{
		apiKey:       strings.TrimSpace(apiKey),
		model:        model,
		baseURL:      strings.TrimRight(baseURL, "/"),
		organization: strings.TrimSpace(organization),
		project:      strings.TrimSpace(project),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (o *OpenAIResponsesProvider) GenerateHint(ctx context.Context, input AIHintContext) (AIHintResponse, error) {
	if o.apiKey == "" {
		return AIHintResponse{}, errors.New("openai api key is not configured")
	}
	requestBody := map[string]any{
		"title":       input.Template.Title,
		"category":    input.Template.Category,
		"description": RenderDescription(input.Template, input.Variant.Params),
		"focus_area":  input.Request.FocusArea,
		"question":    input.Request.Question,
		"used_hints":  input.UsedHints,
		"submission":  trimForPrompt(submissionSource(input.Submission), 900),
		"evaluation":  input.Evaluation,
		"telemetry":   input.Telemetry,
		"anti_cheat":  input.AntiCheat,
		"constraint":  "Do not provide a full solution. Keep the hint short.",
		"response_contract": map[string]any{
			"hint":       "string",
			"focus_area": "string",
		},
	}
	var response AIHintResponse
	if err := o.callJSON(ctx, "You are an interview hint assistant for React assessments. Respond with compact JSON only.", requestBody, &response); err != nil {
		return AIHintResponse{}, err
	}
	response.Provider = "openai"
	response.Hint = strings.TrimSpace(response.Hint)
	response.FocusArea = firstNonEmpty(response.FocusArea, input.Request.FocusArea, input.Template.Category)
	if response.Hint == "" {
		return AIHintResponse{}, errors.New("openai hint response was empty")
	}
	return response, nil
}

func (o *OpenAIResponsesProvider) ExplainEvaluation(ctx context.Context, input AIExplanationContext) (AIExplanationResponse, error) {
	if o.apiKey == "" {
		return AIExplanationResponse{}, errors.New("openai api key is not configured")
	}
	requestBody := map[string]any{
		"title":       input.Template.Title,
		"category":    input.Template.Category,
		"description": RenderDescription(input.Template, input.Variant.Params),
		"submission":  trimForPrompt(input.Submission.RawCodeText, 1400),
		"evaluation":  input.Evaluation,
		"anti_cheat":  input.AntiCheat,
		"response_contract": map[string]any{
			"summary":          "string",
			"strengths":        []string{},
			"improvements":     []string{},
			"suspicion_notes":  []string{},
			"recommended_next": "string",
		},
	}
	var response AIExplanationResponse
	if err := o.callJSON(ctx, "You explain assessment results to candidates and recruiters. Do not overclaim certainty. Return JSON only.", requestBody, &response); err != nil {
		return AIExplanationResponse{}, err
	}
	response.Provider = "openai"
	if strings.TrimSpace(response.Summary) == "" {
		return AIExplanationResponse{}, errors.New("openai explanation response was empty")
	}
	return response, nil
}

func (o *OpenAIResponsesProvider) PreviewMutation(ctx context.Context, input AIMutationContext) (AIMutationPreviewResponse, error) {
	if o.apiKey == "" {
		return AIMutationPreviewResponse{}, errors.New("openai api key is not configured")
	}
	requestBody := map[string]any{
		"title":          input.Template.Title,
		"category":       input.Template.Category,
		"difficulty":     input.Template.Difficulty,
		"description":    input.Template.Description,
		"seed":           input.Seed,
		"variant_params": input.Variant.Params,
		"starter_code":   trimForPrompt(input.Template.StarterCodeTemplate, 900),
		"response_contract": map[string]any{
			"title":            "string",
			"description_md":   "string",
			"variable_renames": map[string]string{},
			"reviewer_notes":   []string{},
		},
	}
	var response AIMutationPreviewResponse
	if err := o.callJSON(ctx, "You create surface-level task mutation previews for interview authors. Preserve logic and rubric. Return JSON only.", requestBody, &response); err != nil {
		return AIMutationPreviewResponse{}, err
	}
	response.Provider = "openai"
	response.Seed = input.Seed
	if strings.TrimSpace(response.Title) == "" || strings.TrimSpace(response.Description) == "" {
		return AIMutationPreviewResponse{}, errors.New("openai mutation preview response was incomplete")
	}
	return response, nil
}

func (o *OpenAIResponsesProvider) callJSON(ctx context.Context, systemPrompt string, payload any, out any) error {
	userJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	requestPayload := map[string]any{
		"model": o.model,
		"input": []map[string]any{
			{
				"role": "developer",
				"content": []map[string]string{
					{"type": "input_text", "text": systemPrompt},
				},
			},
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": "Return valid JSON only.\n" + string(userJSON)},
				},
			},
		},
		"temperature":       0.2,
		"max_output_tokens": 600,
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if o.organization != "" {
		req.Header.Set("OpenAI-Organization", o.organization)
	}
	if o.project != "" {
		req.Header.Set("OpenAI-Project", o.project)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("openai responses api returned %s", resp.Status)
	}

	var apiResp struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return err
	}

	var parts []string
	for _, output := range apiResp.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" || content.Text != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	if len(parts) == 0 {
		return errors.New("openai response did not contain output_text")
	}

	normalized := normalizeModelJSON(strings.Join(parts, "\n"))
	return json.Unmarshal([]byte(normalized), out)
}

func deterministicHint(input AIHintContext) string {
	if input.Evaluation != nil {
		switch {
		case input.Evaluation.TestScore < 70:
			return deterministicCorrectnessAdvice(input.Template)
		case input.Evaluation.PerfScore < 70:
			return deterministicPerformanceAdvice(input.Template)
		case input.Evaluation.QualityScore < 70:
			return "Refactor the state and rendering branches into smaller helpers before you change more logic."
		}
	}

	if input.UsedHints > 0 {
		switch input.Template.Category {
		case "debug":
			return "Trace the unstable dependency or lifecycle edge first, then make the smallest fix that preserves the feature."
		case "feature":
			return "List the exact state transitions and empty-state behavior before adding more JSX."
		case "refactor":
			return "Split orchestration from rendering, then keep side effects behind a smaller hook boundary."
		case "logic":
			return "Write down the state transitions explicitly and code those branches before polishing the UI."
		case "performance":
			return deterministicPerformanceAdvice(input.Template)
		}
	}

	switch input.Template.ID {
	case "react_feature_search":
		return "Model query state, empty-state rendering, and filter behavior first. Add debounce only after the base filter path is correct."
	case "react_performance_virtual_list":
		return "Calculate the visible window from scrollTop, rowHeight, height, and overscan before you render any rows."
	case "react_debug_resize_cleanup":
		return "Keep the listener stable, register it once, and return cleanup from the effect."
	case "react_logic_selection_state":
		return "Define the toggle rules first: disabled ids, maxSelected, and what happens when items disappear."
	default:
		return deterministicCategoryNextStep(input.Template.Category)
	}
}

func deterministicCorrectnessAdvice(template ChallengeTemplate) string {
	switch template.Category {
	case "debug":
		return "Reproduce the broken path with the smallest state transition possible, then fix that branch without widening the effect surface."
	case "feature":
		return "Check the user-facing contract again: input handling, empty states, and the exact filter or navigation rule."
	case "logic":
		return "Make the state transitions explicit and cover the edge case named in the prompt before refining the implementation."
	default:
		return "Re-read the prompt and align the code with the visible task checks before optimizing anything else."
	}
}

func deterministicPerformanceAdvice(template ChallengeTemplate) string {
	switch template.ID {
	case "react_performance_virtual_list":
		return "Render only the visible window plus overscan instead of mapping the full collection every time."
	default:
		return "Check for repeated derived work, unstable props, and render paths that can be memoized or deferred."
	}
}

func deterministicCategoryNextStep(category string) string {
	switch category {
	case "debug":
		return "Next step: isolate the failing lifecycle path and verify the fix still preserves the intended behavior."
	case "feature":
		return "Next step: verify empty states, loading behavior, and the main user interaction path end to end."
	case "refactor":
		return "Next step: reduce coupling between data flow and rendering so future changes stay localized."
	case "logic":
		return "Next step: document the state transitions and add one edge-case replay before extending the UI."
	case "performance":
		return "Next step: measure the hot path and remove the largest rerender or repeated-computation source first."
	default:
		return "Next step: tighten the core task behavior before making stylistic changes."
	}
}

func normalizeModelJSON(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func trimForPrompt(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func submissionSource(submission *Submission) string {
	if submission == nil {
		return ""
	}
	if submission.RawCodeText != "" {
		return submission.RawCodeText
	}
	return MergeFiles(submission.SourceFiles)
}
