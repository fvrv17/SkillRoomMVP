package backend

import (
	"strings"
	"testing"
)

func TestDefaultChallengeTemplatesProvidesBroadCoverage(t *testing.T) {
	templates := DefaultChallengeTemplates()
	if len(templates) < 12 {
		t.Fatalf("expected at least 12 templates, got %d", len(templates))
	}

	seen := map[string]struct{}{}
	for _, templateDef := range templates {
		seen[templateDef.ID] = struct{}{}
		if len(templateDef.EditableFiles) == 0 {
			t.Fatalf("template %s is missing editable files", templateDef.ID)
		}
		if len(templateDef.EvaluationConfig.QualityCheckIDs) == 0 {
			t.Fatalf("template %s is missing quality checks", templateDef.ID)
		}
	}
	if len(seen) != len(templates) {
		t.Fatalf("expected unique template ids")
	}
}

func TestMutateTemplateIsDeterministic(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_feature_search" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected template definition")
	}

	left := MutateTemplate(templateDef, 1234)
	right := MutateTemplate(templateDef, 1234)

	if MergeFiles(left.GeneratedFiles) != MergeFiles(right.GeneratedFiles) {
		t.Fatalf("expected generated files to be deterministic")
	}
	if MergeFiles(left.VisibleTests) != MergeFiles(right.VisibleTests) {
		t.Fatalf("expected visible tests to be deterministic")
	}
}

func TestMutateTemplateKeepsHiddenFilesOutOfVisibleView(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_performance_virtual_list" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected performance template definition")
	}

	variant := MutateTemplate(templateDef, 88)
	if len(variant.GeneratedFiles) == 0 {
		t.Fatalf("expected generated files")
	}
	if len(variant.VisibleTests) == 0 {
		t.Fatalf("expected visible tests")
	}
	if _, ok := variant.VisibleTests["tests/hidden.spec.jsx"]; ok {
		t.Fatalf("hidden tests must not be exposed as visible tests")
	}
	if _, ok := buildVisibleFiles(variant)["tests/hidden.spec.jsx"]; ok {
		t.Fatalf("hidden tests must not be exposed as editable files")
	}
}

func TestMutateTemplateChangesDataShapeAcrossSeeds(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_feature_search" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected search template definition")
	}

	shapes := map[string]struct{}{}
	for seed := int64(1); seed <= 64; seed++ {
		source := MutateTemplate(templateDef, seed).GeneratedFiles["src/App.jsx"]
		switch {
		case strings.Contains(source, "candidate.profile.displayName"):
			shapes["nested_profile"] = struct{}{}
		case strings.Contains(source, "candidate.name"):
			shapes["flat_name"] = struct{}{}
		}
	}
	if len(shapes) < 2 {
		t.Fatalf("expected at least two generated data shapes, got %v", shapes)
	}
}

func TestMutateTemplateChangesSearchFlowAcrossSeeds(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_feature_search" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected search template definition")
	}

	executionModels := map[string]struct{}{}
	dataContracts := map[string]struct{}{}
	for seed := int64(1); seed <= 64; seed++ {
		variant := MutateTemplate(templateDef, seed)
		sourceName, _ := variant.Params["candidate_source_name"].(string)
		fixture, _ := variant.Params["candidate_fixture"].(string)
		matchCondition, _ := variant.Params["match_condition"].(string)

		switch sourceName {
		case "candidates", "pages":
			executionModels["sync"] = struct{}{}
		case "loadCandidates", "loadPages":
			executionModels["async"] = struct{}{}
		}
		switch {
		case strings.Contains(fixture, "profile:"):
			dataContracts["nested_objects"] = struct{}{}
		case strings.Contains(fixture, "[\n  ["):
			dataContracts["paginated_arrays"] = struct{}{}
		default:
			dataContracts["flat_list"] = struct{}{}
		}
		if strings.Contains(matchCondition, "||") {
			dataContracts["multi_field_match"] = struct{}{}
		} else if strings.Contains(matchCondition, ".includes(normalizedQuery)") {
			dataContracts["single_field_match"] = struct{}{}
		}
	}
	if len(executionModels) < 2 {
		t.Fatalf("expected sync and async search execution models, got %v", executionModels)
	}
	if len(dataContracts) < 4 {
		t.Fatalf("expected multiple search data contracts, got %v", dataContracts)
	}
}

func TestMutateTemplateChangesSelectionRulesAcrossSeeds(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_logic_selection_state" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected selection template definition")
	}

	identifierShapes := map[string]struct{}{}
	overflowRules := map[string]struct{}{}
	dependencyRules := map[string]struct{}{}
	for seed := int64(1); seed <= 64; seed++ {
		variant := MutateTemplate(templateDef, seed)
		source := variant.GeneratedFiles["src/useSelectionState.js"]
		assertion, _ := variant.Params["overflow_strategy_assertion"].(string)
		lockedIDs, _ := variant.Params["locked_ids"].(string)
		groupAccessor, _ := variant.Params["group_accessor"].(string)
		conflictSelectable, _ := variant.Params["conflict_selectable"].(string)
		switch {
		case strings.Contains(source, "item.key"):
			identifierShapes["nested_key"] = struct{}{}
		case strings.Contains(source, "item.id"):
			identifierShapes["flat_id"] = struct{}{}
		}
		switch {
		case strings.Contains(assertion, "toEqual([\"grace\"])"):
			overflowRules["replace_oldest"] = struct{}{}
		case strings.Contains(assertion, "toEqual([\"ada\"])"):
			overflowRules["keep_earliest"] = struct{}{}
		}
		switch {
		case lockedIDs != "[]" && groupAccessor != "() => null" && conflictSelectable == "false":
			dependencyRules["locked_group_constraints"] = struct{}{}
		case lockedIDs == "[]":
			dependencyRules["independent_selection"] = struct{}{}
		}
	}
	if len(identifierShapes) < 2 {
		t.Fatalf("expected at least two identifier shapes, got %v", identifierShapes)
	}
	if len(overflowRules) < 2 {
		t.Fatalf("expected at least two overflow rules, got %v", overflowRules)
	}
	if len(dependencyRules) < 2 {
		t.Fatalf("expected both independent and dependent selection rules, got %v", dependencyRules)
	}
}

func TestMutateTemplateChangesValidationContractAcrossSeeds(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_refactor_invite_form" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected invite template definition")
	}

	contracts := map[string]struct{}{}
	payloads := map[string]struct{}{}
	for seed := int64(1); seed <= 64; seed++ {
		source := MutateTemplate(templateDef, seed).GeneratedFiles["src/App.jsx"]
		switch {
		case strings.Contains(source, "allowedDomains.some"):
			contracts["multi_domain"] = struct{}{}
		case strings.Contains(source, "allowedDomain"):
			contracts["single_domain"] = struct{}{}
		}
		switch {
		case strings.Contains(source, "permissions: [role]"):
			payloads["nested_permissions"] = struct{}{}
		case strings.Contains(source, "role };"):
			payloads["flat_role"] = struct{}{}
		}
	}
	if len(contracts) < 2 {
		t.Fatalf("expected at least two validation contracts, got %v", contracts)
	}
	if len(payloads) < 2 {
		t.Fatalf("expected at least two payload shapes, got %v", payloads)
	}
}

func TestMutateTemplateChangesVirtualListStructureAcrossSeeds(t *testing.T) {
	var templateDef ChallengeTemplate
	for _, item := range DefaultChallengeTemplates() {
		if item.ID == "react_performance_virtual_list" {
			templateDef = item
			break
		}
	}
	if templateDef.ID == "" {
		t.Fatal("expected virtual list template definition")
	}

	structures := map[string]struct{}{}
	for seed := int64(1); seed <= 64; seed++ {
		variant := MutateTemplate(templateDef, seed)
		itemsFixture, _ := variant.Params["items_fixture"].(string)
		labelExpr, _ := variant.Params["item_label_expr"].(string)
		switch {
		case strings.Contains(labelExpr, "sectionLabel"):
			structures["grouped_sections"] = struct{}{}
		case strings.Contains(labelExpr, "item.meta.title"):
			structures["nested_meta"] = struct{}{}
		case strings.Contains(itemsFixture, "label:"):
			structures["flat_rows"] = struct{}{}
		}
	}
	if len(structures) < 3 {
		t.Fatalf("expected flat, nested, and grouped list structures, got %v", structures)
	}
}
