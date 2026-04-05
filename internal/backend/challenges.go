package backend

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand"
	"path"
	"sort"
	"strings"
)

//go:embed challenge_assets/*
var challengeAssets embed.FS

func DefaultChallengeTemplates() []ChallengeTemplate {
	return []ChallengeTemplate{
		withQualityChecks(template(
			"react_debug_resize_cleanup",
			"Fix Resize Listener Cleanup",
			"debug",
			2,
			18,
			map[string]float64{"react": 0.6, "javascript": 0.4},
			"react_debug_resize_cleanup",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"ResponsiveStatus", "ViewportBadge", "WindowState"},
				"desktop_label":  {"Desktop Ready", "Wide Layout", "Expanded View"},
				"mobile_label":   {"Compact View", "Mobile Ready", "Tight Layout"},
			},
			map[string][]int{
				"breakpoint": {640, 720, 768},
			},
			"Fix a leaking resize effect without changing the component API.",
		), "listener-cleanup"),
		withQualityChecks(template(
			"react_feature_search",
			"Build Debounced Search",
			"feature",
			2,
			20,
			map[string]float64{"react": 0.65, "javascript": 0.35},
			"react_feature_search",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"CandidateSearch", "TalentLookup", "SearchPanel"},
				"empty_state":    {"No results found", "Nothing matched", "No candidates yet"},
				"input_label":    {"Search candidates", "Find people", "Filter results"},
				"placeholder":    {"Type a name", "Search by name", "Start typing"},
				"results_label":  {"Candidate results", "Search results", "Filtered people"},
			},
			map[string][]int{
				"debounce_ms": {120, 150, 250},
			},
			"Implement a debounced search flow across variant-specific data shapes and matching rules.",
		), "debounce-reset"),
		withQualityChecks(template(
			"react_refactor_invite_form",
			"Refactor Invite Form",
			"refactor",
			3,
			24,
			map[string]float64{"react": 0.45, "architecture": 0.55},
			"react_refactor_invite_form",
			[]string{"src/App.jsx"},
			map[string][]string{
				"allowed_domain": {"@skillroom.dev", "@roomlabs.dev", "@frontend.dev"},
				"backup_domain":  {"@talent.dev", "@company.dev", "@review.dev"},
			},
			nil,
			"Extract validation and payload building helpers without breaking submit behavior.",
		), "helper-contract"),
		withQualityChecks(template(
			"react_logic_selection_state",
			"Implement Selection Hook",
			"logic",
			2,
			19,
			map[string]float64{"react": 0.45, "javascript": 0.55},
			"react_logic_selection_state",
			[]string{"src/useSelectionState.js"},
			map[string][]string{
				"hook_name": {"useSelectionState", "useCandidateSelection", "useItemSelection"},
			},
			map[string][]int{
				"max_selected": {2, 3, 4},
			},
			"Implement selection rules that survive disabled items and item list changes.",
		), "selection-pruning"),
		withQualityChecks(template(
			"react_performance_virtual_list",
			"Window a Large List",
			"performance",
			3,
			22,
			map[string]float64{"react": 0.35, "performance": 0.65},
			"react_performance_virtual_list",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"VirtualCandidateList", "WindowedResults", "SlimList"},
				"item_prefix":    {"Candidate", "Profile", "Result"},
				"list_label":     {"Windowed list", "Virtualized results", "Visible rows"},
				"items_fixture": {
					"Array.from({ length: 200 }, (_, index) => ({\n  id: `candidate-${index}`,\n  label: \"{{item_prefix}} \" + index\n}))",
					"Array.from({ length: 200 }, (_, index) => ({\n  key: `candidate-${index}`,\n  meta: { title: \"{{item_prefix}} \" + index }\n}))",
				},
				"item_key_expr":   {"item.id", "item.key"},
				"item_label_expr": {"item.label", "item.meta.title"},
			},
			map[string][]int{
				"viewport_height": {120, 160, 200},
				"row_height":      {20, 24, 30},
				"overscan":        {1, 2, 3},
			},
			"Render only the visible portion of a long list while keeping order correct.",
		), "bounded-window"),
		withQualityChecks(template(
			"react_form_validation_profile",
			"Build Profile Validation",
			"feature",
			2,
			20,
			map[string]float64{"react": 0.55, "javascript": 0.45},
			"react_form_validation_profile",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"ProfileForm", "CandidateProfileForm", "ApplicantProfileForm"},
			},
			nil,
			"Validate and normalize a profile form without breaking the submit API.",
		), "helper-validation"),
		withQualityChecks(template(
			"react_async_members_board",
			"Load Members With Status States",
			"feature",
			3,
			24,
			map[string]float64{"react": 0.6, "javascript": 0.4},
			"react_async_members_board",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"MembersBoard", "TeamMembers", "RosterPanel"},
				"empty_state":    {"No members found", "Nothing to show", "No matching teammates"},
			},
			nil,
			"Handle loading, success, and error states for async team members while avoiding stale updates.",
		), "stale-response"),
		withQualityChecks(template(
			"react_state_sync_tabs",
			"Sync Controlled Tabs",
			"logic",
			2,
			18,
			map[string]float64{"react": 0.55, "javascript": 0.45},
			"react_state_sync_tabs",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"ControlledTabs", "StageTabs", "TrackTabs"},
			},
			nil,
			"Keep tab state aligned with controlled props and local interaction.",
		), "prop-sync"),
		withQualityChecks(template(
			"react_memoized_metrics",
			"Memoize Expensive Metrics",
			"performance",
			3,
			21,
			map[string]float64{"react": 0.45, "performance": 0.55},
			"react_memoized_metrics",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"MetricsPanel", "ScoreSummary", "ReviewTotals"},
			},
			nil,
			"Prevent unnecessary recomputation in a metrics panel while keeping output stable.",
		), "memoized-summary"),
		withQualityChecks(template(
			"react_event_click_outside",
			"Handle Click Outside",
			"debug",
			2,
			16,
			map[string]float64{"react": 0.6, "javascript": 0.4},
			"react_event_click_outside",
			[]string{"src/App.jsx"},
			map[string][]string{
				"component_name": {"DismissiblePanel", "FocusPanel", "OutsideClickCard"},
			},
			nil,
			"Close a panel on outside interaction without leaking global listeners.",
		), "listener-cleanup"),
		withTrack(withQualityChecks(template(
			"javascript_data_transform_summary",
			"Summarize Candidate Data",
			"logic",
			2,
			17,
			map[string]float64{"javascript": 0.7, "architecture": 0.3},
			"javascript_data_transform_summary",
			[]string{"src/transform.js"},
			nil,
			nil,
			"Transform raw candidate data into a deterministic summary without mutating the input.",
		), "immutable-input"), "javascript"),
		withTrack(withQualityChecks(template(
			"javascript_async_retry_queue",
			"Retry Async Operations",
			"logic",
			3,
			22,
			map[string]float64{"javascript": 0.75, "consistency": 0.25},
			"javascript_async_retry_queue",
			[]string{"src/retry.js"},
			nil,
			nil,
			"Retry a failing async operation with bounded attempts and predictable callbacks.",
		), "attempt-order"), "javascript"),
		withTrack(withQualityChecks(template(
			"javascript_lru_cache",
			"Implement an LRU Cache",
			"performance",
			3,
			20,
			map[string]float64{"javascript": 0.65, "performance": 0.35},
			"javascript_lru_cache",
			[]string{"src/cache.js"},
			nil,
			nil,
			"Implement a small LRU cache with deterministic ordering and eviction.",
		), "entries-copy"), "javascript"),
	}
}

func withQualityChecks(templateDef ChallengeTemplate, checkIDs ...string) ChallengeTemplate {
	templateDef.EvaluationConfig.QualityCheckIDs = append([]string(nil), checkIDs...)
	return templateDef
}

func withTrack(templateDef ChallengeTemplate, track string) ChallengeTemplate {
	if strings.TrimSpace(track) != "" {
		templateDef.Track = strings.TrimSpace(track)
	}
	return templateDef
}

func template(id, title, category string, difficulty, medianSolve int, skillWeights map[string]float64, assetDirectory string, editableFiles []string, stringsMap map[string][]string, numbersMap map[string][]int, description string) ChallengeTemplate {
	return ChallengeTemplate{
		ID:             id,
		Slug:           id,
		Title:          title,
		Difficulty:     difficulty,
		Description:    description,
		Category:       category,
		AssetDirectory: assetDirectory,
		EditableFiles:  append([]string(nil), editableFiles...),
		EvaluationConfig: EvaluationConfig{
			MedianSolveSeconds: medianSolve * 60,
			MedianExecMS:       40,
			MaxAttempts:        3,
			TimeoutMS:          60000,
			MemoryMB:           256,
			LintQualityWeight:  0.5,
			TaskQualityWeight:  0.5,
		},
		IsActive:         true,
		Track:            "react",
		VariationStrings: stringsMap,
		VariationNumbers: numbersMap,
		SkillWeights:     skillWeights,
	}
}

func MutateTemplate(templateDef ChallengeTemplate, seed int64) ChallengeVariant {
	params := map[string]any{}
	for _, key := range sortedStringKeys(templateDef.VariationStrings) {
		options := templateDef.VariationStrings[key]
		if len(options) > 0 {
			params[key] = options[pickIndex(seed, templateDef.ID, key, len(options))]
		}
	}
	for _, key := range sortedNumberKeys(templateDef.VariationNumbers) {
		options := templateDef.VariationNumbers[key]
		if len(options) > 0 {
			params[key] = options[pickIndex(seed, templateDef.ID, key, len(options))]
		}
	}
	if domain, ok := params["allowed_domain"].(string); ok {
		params["allowed_domain_without_at"] = strings.TrimPrefix(domain, "@")
	}
	params = applyStructuredVariantProfiles(templateDef, seed, params)

	files := map[string]string{}
	visibleTests := map[string]string{}
	root := path.Join("challenge_assets", templateDef.AssetDirectory)
	entries, err := readAssetTemplates(root)
	if err == nil {
		for name, content := range entries {
			rendered := renderPlaceholders(content, params)
			if name == "README.md" {
				continue
			}
			files[name] = rendered
			if strings.HasPrefix(name, "tests/visible.") {
				visibleTests[name] = rendered
			}
		}
	}

	readme, _ := readChallengeAsset(path.Join(root, "README.md.tmpl"))
	description := strings.TrimSpace(renderPlaceholders(readme, params))
	if description != "" {
		params["_description"] = description
	}

	return ChallengeVariant{
		ID:              fmt.Sprintf("%s:%d", templateDef.ID, seed),
		TemplateID:      templateDef.ID,
		VariantHash:     fmt.Sprintf("%x", hashSeed(seed, templateDef.ID)),
		Seed:            seed,
		Params:          params,
		GeneratedFiles:  files,
		VisibleTests:    visibleTests,
		EditableFiles:   append([]string(nil), templateDef.EditableFiles...),
		StarterCodePath: fmt.Sprintf("embedded://%s/%d/starter", templateDef.ID, seed),
		TestBundlePath:  fmt.Sprintf("embedded://%s/%d/tests", templateDef.ID, seed),
	}
}

func PickTemplatesByCategory(templates []ChallengeTemplate, category string) []ChallengeTemplate {
	if category == "" {
		return append([]ChallengeTemplate(nil), templates...)
	}
	var out []ChallengeTemplate
	for _, tpl := range templates {
		if tpl.Category == category {
			out = append(out, tpl)
		}
	}
	return out
}

func pickRandomTemplate(templates []ChallengeTemplate, seed int64) ChallengeTemplate {
	rng := rand.New(rand.NewSource(seed))
	return templates[rng.Intn(len(templates))]
}

func renderPlaceholders(raw string, params map[string]any) string {
	out := raw
	for _, key := range sortedParamKeys(params) {
		value := params[key]
		out = strings.ReplaceAll(out, "{{"+key+"}}", fmt.Sprint(value))
	}
	return out
}

func pickIndex(seed int64, templateID, key string, size int) int {
	return int(hashSeed(seed, templateID+":"+key) % uint64(size))
}

func hashSeed(seed int64, value string) uint64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(value))
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(seed))
	_, _ = hasher.Write(buf[:])
	return hasher.Sum64()
}

func sortedStringKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedNumberKeys(values map[string][]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedParamKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func applyStructuredVariantProfiles(templateDef ChallengeTemplate, seed int64, params map[string]any) map[string]any {
	switch templateDef.ID {
	case "react_feature_search":
		profiles := []map[string]any{
			{
				"candidate_source_name":  "candidates",
				"candidate_source_value": "dataset",
				"initial_results_count":  "4",
				"restored_results_count": "4",
				"candidate_fixture":      "[\n  { id: \"1\", name: \"Ada Lovelace\" },\n  { id: \"2\", name: \"Grace Hopper\" },\n  { id: \"3\", name: \"Margaret Hamilton\" },\n  { id: \"4\", name: \"Radia Perlman\" }\n]",
				"candidate_key":          "candidate.id",
				"candidate_label_expr":   "candidate.name",
				"match_condition":        "candidate.name.toLowerCase().includes(normalizedQuery)",
				"match_query":            "grace",
				"match_expected":         "Grace Hopper",
				"trim_query":             "  ada  ",
				"trim_expected":          "Ada Lovelace",
			},
			{
				"candidate_source_name":  "loadCandidates",
				"candidate_source_value": "() => Promise.resolve(dataset)",
				"initial_results_count":  "0",
				"restored_results_count": "4",
				"candidate_fixture":      "[\n  { uid: \"1\", profile: { displayName: \"Ada Lovelace\", title: \"Systems\" } },\n  { uid: \"2\", profile: { displayName: \"Grace Hopper\", title: \"Platform\" } },\n  { uid: \"3\", profile: { displayName: \"Margaret Hamilton\", title: \"Launch\" } },\n  { uid: \"4\", profile: { displayName: \"Radia Perlman\", title: \"Network\" } }\n]",
				"candidate_key":          "candidate.uid",
				"candidate_label_expr":   "`${candidate.profile.displayName} · ${candidate.profile.title}`",
				"match_condition":        "candidate.profile.displayName.toLowerCase().includes(normalizedQuery) || candidate.profile.title.toLowerCase().includes(normalizedQuery)",
				"match_query":            "platform",
				"match_expected":         "Grace Hopper · Platform",
				"trim_query":             "  marg  ",
				"trim_expected":          "Margaret Hamilton · Launch",
			},
			{
				"candidate_source_name":  "pages",
				"candidate_source_value": "dataset",
				"initial_results_count":  "4",
				"restored_results_count": "4",
				"candidate_fixture":      "[\n  [\n    { id: \"1\", name: \"Ada Lovelace\", role: \"Algorithms\" },\n    { id: \"2\", name: \"Grace Hopper\", role: \"Platform\" }\n  ],\n  [\n    { id: \"3\", name: \"Margaret Hamilton\", role: \"Launch\" },\n    { id: \"4\", name: \"Radia Perlman\", role: \"Network\" }\n  ]\n]",
				"candidate_key":          "candidate.id",
				"candidate_label_expr":   "`${candidate.name} · ${candidate.role}`",
				"match_condition":        "candidate.name.toLowerCase().includes(normalizedQuery) || candidate.role.toLowerCase().includes(normalizedQuery)",
				"match_query":            "network",
				"match_expected":         "Radia Perlman · Network",
				"trim_query":             "  algo  ",
				"trim_expected":          "Ada Lovelace · Algorithms",
			},
			{
				"candidate_source_name":  "loadPages",
				"candidate_source_value": "() => Promise.resolve(dataset)",
				"initial_results_count":  "0",
				"restored_results_count": "4",
				"candidate_fixture":      "[\n  [\n    { uid: \"1\", profile: { displayName: \"Ada Lovelace\", tag: \"Algorithms\" } },\n    { uid: \"2\", profile: { displayName: \"Grace Hopper\", tag: \"Platform\" } }\n  ],\n  [\n    { uid: \"3\", profile: { displayName: \"Margaret Hamilton\", tag: \"Launch\" } },\n    { uid: \"4\", profile: { displayName: \"Radia Perlman\", tag: \"Network\" } }\n  ]\n]",
				"candidate_key":          "candidate.uid",
				"candidate_label_expr":   "`${candidate.profile.displayName} · ${candidate.profile.tag}`",
				"match_condition":        "candidate.profile.displayName.toLowerCase().includes(normalizedQuery) || candidate.profile.tag.toLowerCase().includes(normalizedQuery)",
				"match_query":            "launch",
				"match_expected":         "Margaret Hamilton · Launch",
				"trim_query":             "  grace  ",
				"trim_expected":          "Grace Hopper · Platform",
			},
		}
		applyVariantProfile(params, profiles[pickIndex(seed, templateDef.ID, "search_profile", len(profiles))])
	case "react_refactor_invite_form":
		profiles := []map[string]any{
			{
				"allowed_input_name":      "allowedDomain",
				"allowed_input_default":   fmt.Sprintf("%q", params["allowed_domain"]),
				"allowed_input_prop":      fmt.Sprintf("allowedDomain=%q", params["allowed_domain"]),
				"validator_input_value":   fmt.Sprintf("%q", params["allowed_domain"]),
				"validator_condition":     "if (!normalizedEmail.endsWith(allowedDomain)) {\n      return { valid: false, message: \"Use a \" + allowedDomain + \" address\" };\n    }",
				"domain_error_message":    fmt.Sprintf("Use a %s address", params["allowed_domain"]),
				"payload_return":          "return { name: normalizedName, email: normalizedEmail, role };",
				"helper_expected_payload": fmt.Sprintf("{\n      name: \"Alex Smith\",\n      email: \"alex@%s\",\n      role: \"member\"\n    }", strings.TrimPrefix(params["allowed_domain"].(string), "@")),
				"submit_expected_payload": fmt.Sprintf("{\n      name: \"Alex Smith\",\n      email: \"alex@%s\",\n      role: \"reviewer\"\n    }", strings.TrimPrefix(params["allowed_domain"].(string), "@")),
			},
			{
				"allowed_input_name":      "allowedDomains",
				"allowed_input_default":   fmt.Sprintf("[%q, %q]", params["allowed_domain"], params["backup_domain"]),
				"allowed_input_prop":      fmt.Sprintf("allowedDomains={[%q, %q]}", params["allowed_domain"], params["backup_domain"]),
				"validator_input_value":   fmt.Sprintf("[%q, %q]", params["allowed_domain"], params["backup_domain"]),
				"validator_condition":     "const allowed = allowedDomains.some((domain) => normalizedEmail.endsWith(domain));\n    if (!allowed) {\n      return { valid: false, message: \"Use an approved company address\" };\n    }",
				"domain_error_message":    "Use an approved company address",
				"payload_return":          "return { profile: { name: normalizedName }, contact: { email: normalizedEmail }, permissions: [role] };",
				"helper_expected_payload": fmt.Sprintf("{\n      profile: { name: \"Alex Smith\" },\n      contact: { email: \"alex@%s\" },\n      permissions: [\"member\"]\n    }", strings.TrimPrefix(params["allowed_domain"].(string), "@")),
				"submit_expected_payload": fmt.Sprintf("{\n      profile: { name: \"Alex Smith\" },\n      contact: { email: \"alex@%s\" },\n      permissions: [\"reviewer\"]\n    }", strings.TrimPrefix(params["allowed_domain"].(string), "@")),
			},
		}
		applyVariantProfile(params, profiles[pickIndex(seed, templateDef.ID, "invite_profile", len(profiles))])
	case "react_logic_selection_state":
		profiles := []map[string]any{
			{
				"items_fixture":                "[\n  { id: \"ada\", label: \"Ada\" },\n  { id: \"grace\", label: \"Grace\" },\n  { id: \"margaret\", label: \"Margaret\" }\n]",
				"single_item_fixture":          "[{ id: \"grace\", label: \"Grace\" }]",
				"item_id_expr":                 "item.id",
				"disabled_id":                  "grace",
				"locked_ids":                   "[]",
				"group_accessor":               "() => null",
				"conflict_id":                  "margaret",
				"conflict_selectable":          "true",
				"hidden_selected_after_toggle": "[\"ada\", \"margaret\"]",
				"stale_expected":               "[]",
				"overflow_expected":            "[\"ada\"]",
				"overflow_rule_description":    "keep the earliest selected id when the selection is full",
				"overflow_strategy_assertion":  "expect(result.current.selectedIds).toEqual([\"ada\"])",
			},
			{
				"items_fixture":                "[\n  { key: \"ada\", meta: { label: \"Ada\" } },\n  { key: \"grace\", meta: { label: \"Grace\" } },\n  { key: \"margaret\", meta: { label: \"Margaret\" } }\n]",
				"single_item_fixture":          "[{ key: \"grace\", meta: { label: \"Grace\" } }]",
				"item_id_expr":                 "item.key",
				"disabled_id":                  "margaret",
				"locked_ids":                   "[]",
				"group_accessor":               "() => null",
				"conflict_id":                  "grace",
				"conflict_selectable":          "true",
				"hidden_selected_after_toggle": "[\"ada\", \"grace\"]",
				"stale_expected":               "[]",
				"overflow_expected":            "[\"grace\"]",
				"overflow_rule_description":    "replace the oldest selection when the selection is full",
				"overflow_strategy_assertion":  "expect(result.current.selectedIds).toEqual([\"grace\"])",
			},
			{
				"items_fixture":                "[\n  { id: \"ada\", label: \"Ada\", group: \"core\" },\n  { id: \"grace\", label: \"Grace\", group: \"support\" },\n  { id: \"linus\", label: \"Linus\", group: \"core\" }\n]",
				"single_item_fixture":          "[{ id: \"ada\", label: \"Ada\", group: \"core\" }]",
				"item_id_expr":                 "item.id",
				"disabled_id":                  "grace",
				"locked_ids":                   "[\"ada\"]",
				"group_accessor":               "(item) => item.group ?? null",
				"conflict_id":                  "linus",
				"conflict_selectable":          "false",
				"hidden_selected_after_toggle": "[\"ada\"]",
				"stale_expected":               "[\"ada\"]",
				"overflow_expected":            "[\"ada\"]",
				"overflow_rule_description":    "preserve locked selections and reject ids from the same derived group",
				"overflow_strategy_assertion":  "expect(result.current.selectedIds).toEqual([\"ada\"])",
			},
		}
		applyVariantProfile(params, profiles[pickIndex(seed, templateDef.ID, "selection_profile", len(profiles))])
	case "react_performance_virtual_list":
		viewportHeight := params["viewport_height"].(int)
		rowHeight := params["row_height"].(int)
		overscan := params["overscan"].(int)
		scrollTarget := fmt.Sprintf("%d", rowHeight*20)
		maxVisibleCount := fmt.Sprintf("%d", (viewportHeight+rowHeight-1)/rowHeight+overscan*2+1)
		profiles := []map[string]any{
			{
				"items_fixture":          fmt.Sprintf("Array.from({ length: 200 }, (_, index) => ({\n  id: `candidate-${index}`,\n  label: %q + index\n}))", params["item_prefix"].(string)+" "),
				"item_key_expr":          "item.id",
				"item_label_expr":        "item.label",
				"rows_total_count":       "200",
				"first_visible_label":    fmt.Sprintf("%s 0", params["item_prefix"].(string)),
				"scroll_target":          scrollTarget,
				"scrolled_visible_label": fmt.Sprintf("%s 20", params["item_prefix"].(string)),
				"max_visible_count":      maxVisibleCount,
			},
			{
				"items_fixture":          fmt.Sprintf("Array.from({ length: 200 }, (_, index) => ({\n  key: `candidate-${index}`,\n  meta: { title: %q + index }\n}))", params["item_prefix"].(string)+" "),
				"item_key_expr":          "item.key",
				"item_label_expr":        "item.meta.title",
				"rows_total_count":       "200",
				"first_visible_label":    fmt.Sprintf("%s 0", params["item_prefix"].(string)),
				"scroll_target":          scrollTarget,
				"scrolled_visible_label": fmt.Sprintf("%s 20", params["item_prefix"].(string)),
				"max_visible_count":      maxVisibleCount,
			},
			{
				"items_fixture":          fmt.Sprintf("Array.from({ length: 10 }, (_, sectionIndex) => ({\n  id: `section-${sectionIndex}`,\n  label: `Track ${sectionIndex}`,\n  items: Array.from({ length: 20 }, (_, itemIndex) => ({\n    id: `candidate-${sectionIndex}-${itemIndex}`,\n    label: %q + (sectionIndex * 20 + itemIndex)\n  }))\n}))", params["item_prefix"].(string)+" "),
				"item_key_expr":          "item.id",
				"item_label_expr":        "`${item.sectionLabel} · ${item.label}`",
				"rows_total_count":       "200",
				"first_visible_label":    fmt.Sprintf("Track 0 · %s 0", params["item_prefix"].(string)),
				"scroll_target":          scrollTarget,
				"scrolled_visible_label": fmt.Sprintf("Track 1 · %s 20", params["item_prefix"].(string)),
				"max_visible_count":      maxVisibleCount,
			},
		}
		applyVariantProfile(params, profiles[pickIndex(seed, templateDef.ID, "virtual_list_profile", len(profiles))])
	}
	return params
}

func applyVariantProfile(params map[string]any, profile map[string]any) {
	for key, value := range profile {
		params[key] = value
	}
}

func RenderTitle(templateDef ChallengeTemplate, params map[string]any) string {
	return renderPlaceholders(templateDef.Title, params)
}

func RenderDescription(templateDef ChallengeTemplate, params map[string]any) string {
	if description, ok := params["_description"].(string); ok && strings.TrimSpace(description) != "" {
		return strings.TrimSpace(description)
	}
	return strings.TrimSpace(renderPlaceholders(templateDef.Description, params))
}

func MergeFiles(files map[string]string) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buffer bytes.Buffer
	for _, name := range names {
		buffer.WriteString(name)
		buffer.WriteByte('\n')
		buffer.WriteString(files[name])
		buffer.WriteByte('\n')
	}
	return buffer.String()
}

func readAssetTemplates(root string) (map[string]string, error) {
	entries := map[string]string{}
	if err := walkEmbeddedDir(root, func(name string) error {
		if !strings.HasSuffix(name, ".tmpl") {
			return nil
		}
		content, err := readChallengeAsset(name)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(name, root+"/")
		entries[strings.TrimSuffix(relative, ".tmpl")] = content
		return nil
	}); err != nil {
		return nil, err
	}
	return entries, nil
}

func walkEmbeddedDir(root string, fn func(name string) error) error {
	items, err := challengeAssets.ReadDir(root)
	if err != nil {
		return err
	}
	for _, item := range items {
		name := path.Join(root, item.Name())
		if item.IsDir() {
			if err := walkEmbeddedDir(name, fn); err != nil {
				return err
			}
			continue
		}
		if err := fn(name); err != nil {
			return err
		}
	}
	return nil
}

func readChallengeAsset(name string) (string, error) {
	content, err := challengeAssets.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
