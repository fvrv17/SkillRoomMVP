package backend

import "strings"

const (
	defaultLegacyTrackCode      = "react"
	defaultProfessionCode       = "developer"
	defaultDeveloperTrackCode   = "frontend"
	defaultDeveloperRuntimeCode = "javascript"
	defaultDeveloperRoomProfile = "developer_default"
)

type ProfessionDefinition struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	DefaultTrackCode string `json:"default_track_code"`
	RoomProfileCode  string `json:"room_profile_code"`
	Active           bool   `json:"active"`
}

type TrackDefinition struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	ProfessionCode     string `json:"profession_code"`
	DefaultRuntimeCode string `json:"default_runtime_code"`
	Active             bool   `json:"active"`
}

type RuntimeDefinition struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	ProfessionCode   string   `json:"profession_code"`
	SupportedTracks  []string `json:"supported_tracks"`
	ExecutionEnabled bool     `json:"execution_enabled"`
	Active           bool     `json:"active"`
}

type RoomProfileDefinition struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	ProfessionCode string `json:"profession_code"`
	Active         bool   `json:"active"`
}

var professionRegistry = map[string]ProfessionDefinition{
	defaultProfessionCode: {
		Code:             defaultProfessionCode,
		Name:             "Developer",
		DefaultTrackCode: defaultDeveloperTrackCode,
		RoomProfileCode:  defaultDeveloperRoomProfile,
		Active:           true,
	},
	"designer": {
		Code:             "designer",
		Name:             "Designer",
		DefaultTrackCode: "product_design",
		RoomProfileCode:  "designer_default",
		Active:           true,
	},
}

var trackRegistry = map[string]TrackDefinition{
	defaultDeveloperTrackCode: {
		Code:               defaultDeveloperTrackCode,
		Name:               "Frontend",
		ProfessionCode:     defaultProfessionCode,
		DefaultRuntimeCode: defaultDeveloperRuntimeCode,
		Active:             true,
	},
	"backend": {
		Code:               "backend",
		Name:               "Backend",
		ProfessionCode:     defaultProfessionCode,
		DefaultRuntimeCode: "go",
		Active:             true,
	},
}

var runtimeRegistry = map[string]RuntimeDefinition{
	"javascript": {Code: "javascript", Name: "JavaScript", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"frontend", "backend"}, ExecutionEnabled: true, Active: true},
	"go":         {Code: "go", Name: "Go", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"java":       {Code: "java", Name: "Java", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"python":     {Code: "python", Name: "Python", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"rust":       {Code: "rust", Name: "Rust", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"cpp":        {Code: "cpp", Name: "C++", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"csharp":     {Code: "csharp", Name: "C#", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"swift":      {Code: "swift", Name: "Swift", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"php":        {Code: "php", Name: "PHP", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
	"kotlin":     {Code: "kotlin", Name: "Kotlin", ProfessionCode: defaultProfessionCode, SupportedTracks: []string{"backend"}, ExecutionEnabled: false, Active: true},
}

var roomProfileRegistry = map[string]RoomProfileDefinition{
	defaultDeveloperRoomProfile: {
		Code:           defaultDeveloperRoomProfile,
		Name:           "Developer Default",
		ProfessionCode: defaultProfessionCode,
		Active:         true,
	},
	"designer_default": {
		Code:           "designer_default",
		Name:           "Designer Default",
		ProfessionCode: "designer",
		Active:         true,
	},
}

func defaultProfileMetadataForRole(role Role) (professionCode, trackCode, runtimeCode, roomProfileCode string) {
	if !roleSupportsDeveloperRoom(role) {
		return "", "", "", ""
	}
	return defaultProfessionCode, defaultDeveloperTrackCode, defaultDeveloperRuntimeCode, defaultDeveloperRoomProfile
}

func normalizeUserProfileMetadata(profile UserProfile, role Role) UserProfile {
	professionCode, trackCode, runtimeCode, roomProfileCode := defaultProfileMetadataForRole(role)
	if strings.TrimSpace(profile.SelectedTrack) == "" {
		profile.SelectedTrack = defaultLegacyTrackCode
	}
	if professionCode == "" {
		return profile
	}
	if strings.TrimSpace(profile.ProfessionCode) == "" {
		profile.ProfessionCode = professionCode
	}
	if strings.TrimSpace(profile.TrackCode) == "" {
		profile.TrackCode = trackCode
	}
	if strings.TrimSpace(profile.RuntimeCode) == "" {
		profile.RuntimeCode = runtimeCode
	}
	if strings.TrimSpace(profile.RoomProfileCode) == "" {
		profile.RoomProfileCode = roomProfileCode
	}

	professionDef, ok := professionRegistry[profile.ProfessionCode]
	if !ok || !professionDef.Active {
		profile.ProfessionCode = professionCode
		professionDef = professionRegistry[professionCode]
	}
	trackDef, ok := trackRegistry[profile.TrackCode]
	if !ok || !trackDef.Active || trackDef.ProfessionCode != profile.ProfessionCode {
		profile.TrackCode = professionDef.DefaultTrackCode
		trackDef = trackRegistry[profile.TrackCode]
	}
	runtimeDef, ok := runtimeRegistry[profile.RuntimeCode]
	if !ok || !runtimeDef.Active || runtimeDef.ProfessionCode != profile.ProfessionCode || !containsString(runtimeDef.SupportedTracks, profile.TrackCode) {
		profile.RuntimeCode = trackDef.DefaultRuntimeCode
	}
	roomProfileDef, ok := roomProfileRegistry[profile.RoomProfileCode]
	if !ok || !roomProfileDef.Active || roomProfileDef.ProfessionCode != profile.ProfessionCode {
		profile.RoomProfileCode = professionDef.RoomProfileCode
	}
	return profile
}

func normalizeChallengeTemplateMetadata(templateDef ChallengeTemplate) ChallengeTemplate {
	if strings.TrimSpace(templateDef.ProfessionCode) == "" {
		templateDef.ProfessionCode = defaultProfessionCode
	}
	if strings.TrimSpace(templateDef.TrackCode) == "" {
		templateDef.TrackCode = defaultDeveloperTrackCode
	}
	if strings.TrimSpace(templateDef.RuntimeCode) == "" {
		templateDef.RuntimeCode = defaultDeveloperRuntimeCode
	}
	professionDef, ok := professionRegistry[templateDef.ProfessionCode]
	if !ok || !professionDef.Active {
		templateDef.ProfessionCode = defaultProfessionCode
		professionDef = professionRegistry[defaultProfessionCode]
	}
	trackDef, ok := trackRegistry[templateDef.TrackCode]
	if !ok || !trackDef.Active || trackDef.ProfessionCode != professionDef.Code {
		templateDef.TrackCode = professionDef.DefaultTrackCode
		trackDef = trackRegistry[templateDef.TrackCode]
	}
	runtimeDef, ok := runtimeRegistry[templateDef.RuntimeCode]
	if !ok || !runtimeDef.Active || runtimeDef.ProfessionCode != professionDef.Code || !containsString(runtimeDef.SupportedTracks, templateDef.TrackCode) {
		templateDef.RuntimeCode = trackDef.DefaultRuntimeCode
	}
	return templateDef
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
