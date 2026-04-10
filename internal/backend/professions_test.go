package backend

import "testing"

func TestNormalizeUserProfileMetadataUsesDeveloperDefaults(t *testing.T) {
	profile := normalizeUserProfileMetadata(UserProfile{}, RoleUser)
	if profile.SelectedTrack != defaultLegacyTrackCode {
		t.Fatalf("expected selected_track %q, got %q", defaultLegacyTrackCode, profile.SelectedTrack)
	}
	if profile.ProfessionCode != defaultProfessionCode {
		t.Fatalf("expected profession_code %q, got %q", defaultProfessionCode, profile.ProfessionCode)
	}
	if profile.TrackCode != defaultDeveloperTrackCode {
		t.Fatalf("expected track_code %q, got %q", defaultDeveloperTrackCode, profile.TrackCode)
	}
	if profile.RuntimeCode != defaultDeveloperRuntimeCode {
		t.Fatalf("expected runtime_code %q, got %q", defaultDeveloperRuntimeCode, profile.RuntimeCode)
	}
	if profile.RoomProfileCode != defaultDeveloperRoomProfile {
		t.Fatalf("expected room_profile_code %q, got %q", defaultDeveloperRoomProfile, profile.RoomProfileCode)
	}
}

func TestNormalizeUserProfileMetadataLeavesHRFoundationBlank(t *testing.T) {
	profile := normalizeUserProfileMetadata(UserProfile{}, RoleHR)
	if profile.ProfessionCode != "" || profile.TrackCode != "" || profile.RuntimeCode != "" || profile.RoomProfileCode != "" {
		t.Fatalf("expected non-developer foundation fields to stay blank, got %+v", profile)
	}
}

func TestNormalizeChallengeTemplateMetadataUsesSafeDefaults(t *testing.T) {
	templateDef := normalizeChallengeTemplateMetadata(ChallengeTemplate{})
	if templateDef.ProfessionCode != defaultProfessionCode {
		t.Fatalf("expected profession_code %q, got %q", defaultProfessionCode, templateDef.ProfessionCode)
	}
	if templateDef.TrackCode != defaultDeveloperTrackCode {
		t.Fatalf("expected track_code %q, got %q", defaultDeveloperTrackCode, templateDef.TrackCode)
	}
	if templateDef.RuntimeCode != defaultDeveloperRuntimeCode {
		t.Fatalf("expected runtime_code %q, got %q", defaultDeveloperRuntimeCode, templateDef.RuntimeCode)
	}
}
