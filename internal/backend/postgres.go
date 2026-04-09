package backend

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schemaSQL string

type SQLStore struct {
	db *sql.DB
}

type CandidateSearchFilters struct {
	MinScore      float64
	MinConfidence float64
	TopPercent    float64
	ActiveDays    int
	Limit         int
	Offset        int
}

func OpenSQLStore(ctx context.Context, dsn string) (*SQLStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	store := &SQLStore{db: db}
	if err := store.ApplySchema(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLStore) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.PingContext(ctx)
}

func (s *SQLStore) ApplySchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	return err
}

func (s *SQLStore) SyncCatalog(ctx context.Context, app *App) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, skillCode := range sortedSkillCodes(app.skills) {
		skill := app.skills[skillCode]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO skills (id, name, category, code)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, code = EXCLUDED.code
		`, skill.ID, skill.Name, skill.Category, skill.Code); err != nil {
			return err
		}
	}

	for _, itemCode := range sortedRoomItemCodes(app.roomItems) {
		item := app.roomItems[itemCode]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO room_items (id, name, slot, related_skill_id, code)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, slot = EXCLUDED.slot, related_skill_id = EXCLUDED.related_skill_id, code = EXCLUDED.code
		`, item.ID, item.Name, item.Slot, item.RelatedSkillID, item.Code); err != nil {
			return err
		}
	}

	for _, planCode := range sortedPlanCodes(app.plans) {
		plan := app.plans[planCode]
		featuresJSON, err := marshalJSON(plan.Features)
		if err != nil {
			return err
		}
		entitlementsJSON, err := marshalJSON(plan.Entitlements)
		if err != nil {
			return err
		}
		metadataJSON, err := marshalJSON(plan.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO plans (id, code, name, audience, tier, currency, monthly_price_cents, active, features_json, entitlements_json, metadata_json)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (id) DO UPDATE SET
				code = EXCLUDED.code,
				name = EXCLUDED.name,
				audience = EXCLUDED.audience,
				tier = EXCLUDED.tier,
				currency = EXCLUDED.currency,
				monthly_price_cents = EXCLUDED.monthly_price_cents,
				active = EXCLUDED.active,
				features_json = EXCLUDED.features_json,
				entitlements_json = EXCLUDED.entitlements_json,
				metadata_json = EXCLUDED.metadata_json
		`, plan.ID, plan.Code, plan.Name, plan.Audience, plan.Tier, plan.Currency, plan.MonthlyPriceCents, plan.Active, featuresJSON, entitlementsJSON, metadataJSON); err != nil {
			return err
		}
	}

	for _, cosmeticCode := range sortedCosmeticCodes(app.cosmeticCatalog) {
		item := app.cosmeticCatalog[cosmeticCode]
		metadataJSON, err := marshalJSON(item.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO cosmetic_catalog (id, code, name, category, slot_code, description, audience, rarity, premium, asset_ref, active, metadata_json)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (id) DO UPDATE SET
				code = EXCLUDED.code,
				name = EXCLUDED.name,
				category = EXCLUDED.category,
				slot_code = EXCLUDED.slot_code,
				description = EXCLUDED.description,
				audience = EXCLUDED.audience,
				rarity = EXCLUDED.rarity,
				premium = EXCLUDED.premium,
				asset_ref = EXCLUDED.asset_ref,
				active = EXCLUDED.active,
				metadata_json = EXCLUDED.metadata_json
		`, item.ID, item.Code, item.Name, item.Category, item.SlotCode, item.Description, item.Audience, item.Rarity, item.Premium, item.AssetRef, item.Active, metadataJSON); err != nil {
			return err
		}
	}
	activeRoomCodes := sortedRoomItemCodes(app.roomItems)
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM user_room_items
		WHERE NOT (room_item_code = ANY($1))
	`, activeRoomCodes); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM room_items
		WHERE NOT (code = ANY($1))
	`, activeRoomCodes); err != nil {
		return err
	}

	for _, templateID := range sortedTemplateIDs(app.templates) {
		templateDef := app.templates[templateID]
		evaluationConfig, err := marshalJSON(templateDef.EvaluationConfig)
		if err != nil {
			return err
		}
		variationStrings, err := marshalJSON(templateDef.VariationStrings)
		if err != nil {
			return err
		}
		variationNumbers, err := marshalJSON(templateDef.VariationNumbers)
		if err != nil {
			return err
		}
		skillWeights, err := marshalJSON(templateDef.SkillWeights)
		if err != nil {
			return err
		}
		editableFiles, err := marshalJSON(templateDef.EditableFiles)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO challenge_templates (
				id, slug, title, difficulty, description_md, asset_directory, editable_files_json, starter_code_template, visible_tests_template,
				evaluation_config_json, is_active, category, track, variation_strings_json, variation_numbers_json, skill_weights_json
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
			ON CONFLICT (id) DO UPDATE SET
				slug = EXCLUDED.slug,
				title = EXCLUDED.title,
				difficulty = EXCLUDED.difficulty,
				description_md = EXCLUDED.description_md,
				asset_directory = EXCLUDED.asset_directory,
				editable_files_json = EXCLUDED.editable_files_json,
				starter_code_template = EXCLUDED.starter_code_template,
				visible_tests_template = EXCLUDED.visible_tests_template,
				evaluation_config_json = EXCLUDED.evaluation_config_json,
				is_active = EXCLUDED.is_active,
				category = EXCLUDED.category,
				track = EXCLUDED.track,
				variation_strings_json = EXCLUDED.variation_strings_json,
				variation_numbers_json = EXCLUDED.variation_numbers_json,
				skill_weights_json = EXCLUDED.skill_weights_json
		`, templateDef.ID, templateDef.Slug, templateDef.Title, templateDef.Difficulty, templateDef.Description,
			templateDef.AssetDirectory, editableFiles, templateDef.StarterCodeTemplate, templateDef.VisibleTestsTemplate, evaluationConfig, templateDef.IsActive,
			templateDef.Category, templateDef.Track, variationStrings, variationNumbers, skillWeights); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLStore) LoadInto(ctx context.Context, app *App) error {
	if err := s.loadUsers(ctx, app); err != nil {
		return err
	}
	if err := s.loadRefreshSessions(ctx, app); err != nil {
		return err
	}
	if err := s.loadUserSkills(ctx, app); err != nil {
		return err
	}
	if err := s.loadUserRoomItems(ctx, app); err != nil {
		return err
	}
	if err := s.loadChallengeVariants(ctx, app); err != nil {
		return err
	}
	if err := s.loadChallengeInstances(ctx, app); err != nil {
		return err
	}
	if err := s.loadTelemetryEvents(ctx, app); err != nil {
		return err
	}
	if err := s.loadSubmissions(ctx, app); err != nil {
		return err
	}
	if err := s.loadEvaluations(ctx, app); err != nil {
		return err
	}
	if err := s.loadScoreEvents(ctx, app); err != nil {
		return err
	}
	if err := s.loadFriendships(ctx, app); err != nil {
		return err
	}
	if err := s.loadChats(ctx, app); err != nil {
		return err
	}
	if err := s.loadCompanies(ctx, app); err != nil {
		return err
	}
	if err := s.loadJobs(ctx, app); err != nil {
		return err
	}
	if err := s.loadShortlists(ctx, app); err != nil {
		return err
	}
	if err := s.loadAIInteractions(ctx, app); err != nil {
		return err
	}
	if err := s.loadSubscriptions(ctx, app); err != nil {
		return err
	}
	if err := s.loadCandidateUnlocks(ctx, app); err != nil {
		return err
	}
	if err := s.loadCandidateInvites(ctx, app); err != nil {
		return err
	}
	if err := s.loadAIUsageEvents(ctx, app); err != nil {
		return err
	}
	if err := s.loadUserCosmetics(ctx, app); err != nil {
		return err
	}
	if err := s.loadEquippedCosmetics(ctx, app); err != nil {
		return err
	}
	app.rebuildDerivedStateFromPersistence()
	return nil
}

func (s *SQLStore) UpsertRefreshSession(ctx context.Context, session refreshSession) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO refresh_sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at
	`, session.Token, session.UserID, session.ExpiresAt)
	return err
}

func (s *SQLStore) DeleteRefreshSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM refresh_sessions WHERE token = $1`, token)
	return err
}

func (s *SQLStore) UpsertUserAggregate(ctx context.Context, user User, skills []UserSkill, roomItems []UserRoomItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, username, password_hash, role, country, created_at, last_active_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			username = EXCLUDED.username,
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role,
			country = EXCLUDED.country,
			last_active_at = EXCLUDED.last_active_at
	`, user.ID, user.Email, user.Username, user.PasswordHash, user.Role, user.Country, user.CreatedAt, user.LastActiveAt); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_profiles (
			user_id, selected_track, bio, avatar_url, linkedin_url, current_skill_score, percentile_global, percentile_country,
			streak_days, confidence_score, completed_challenges, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (user_id) DO UPDATE SET
			selected_track = EXCLUDED.selected_track,
			bio = EXCLUDED.bio,
			avatar_url = EXCLUDED.avatar_url,
			linkedin_url = EXCLUDED.linkedin_url,
			current_skill_score = EXCLUDED.current_skill_score,
			percentile_global = EXCLUDED.percentile_global,
			percentile_country = EXCLUDED.percentile_country,
			streak_days = EXCLUDED.streak_days,
			confidence_score = EXCLUDED.confidence_score,
			completed_challenges = EXCLUDED.completed_challenges,
			updated_at = EXCLUDED.updated_at
	`, user.Profile.UserID, user.Profile.SelectedTrack, user.Profile.Bio, user.Profile.AvatarURL, user.Profile.LinkedInURL, user.Profile.CurrentSkillScore,
		user.Profile.PercentileGlobal, user.Profile.PercentileCountry, user.Profile.StreakDays, user.Profile.ConfidenceScore,
		user.Profile.CompletedChallenges, user.Profile.UpdatedAt); err != nil {
		return err
	}

	for _, skill := range skills {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_skills (id, user_id, skill_id, skill_code, score, confidence, level, last_verified_at, decay_factor, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (user_id, skill_code) DO UPDATE SET
				id = EXCLUDED.id,
				skill_id = EXCLUDED.skill_id,
				score = EXCLUDED.score,
				confidence = EXCLUDED.confidence,
				level = EXCLUDED.level,
				last_verified_at = EXCLUDED.last_verified_at,
				decay_factor = EXCLUDED.decay_factor,
				updated_at = EXCLUDED.updated_at
		`, skill.ID, skill.UserID, skill.SkillID, skill.SkillCode, skill.Score, skill.Confidence, skill.Level,
			skill.LastVerifiedAt, skill.DecayFactor, skill.UpdatedAt); err != nil {
			return err
		}
	}

	for _, roomItem := range roomItems {
		stateJSON, err := marshalJSON(roomItem.State)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_room_items (id, user_id, room_item_id, room_item_code, current_level, current_variant, state_json, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (user_id, room_item_code) DO UPDATE SET
				id = EXCLUDED.id,
				room_item_id = EXCLUDED.room_item_id,
				current_level = EXCLUDED.current_level,
				current_variant = EXCLUDED.current_variant,
				state_json = EXCLUDED.state_json,
				updated_at = EXCLUDED.updated_at
		`, roomItem.ID, roomItem.UserID, roomItem.RoomItemID, roomItem.RoomItemCode, roomItem.CurrentLevel,
			roomItem.CurrentVariant, stateJSON, roomItem.UpdatedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLStore) UpsertChallengeVariant(ctx context.Context, variant ChallengeVariant) error {
	paramsJSON, err := marshalJSON(variant.Params)
	if err != nil {
		return err
	}
	filesJSON, err := marshalJSON(variant.GeneratedFiles)
	if err != nil {
		return err
	}
	visibleTestsJSON, err := marshalJSON(variant.VisibleTests)
	if err != nil {
		return err
	}
	editableFilesJSON, err := marshalJSON(variant.EditableFiles)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO challenge_variants (
			id, template_id, variant_hash, seed, params_json, generated_files_json, visible_tests_json, editable_files_json, starter_code_path, test_bundle_path
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			template_id = EXCLUDED.template_id,
			variant_hash = EXCLUDED.variant_hash,
			seed = EXCLUDED.seed,
			params_json = EXCLUDED.params_json,
			generated_files_json = EXCLUDED.generated_files_json,
			visible_tests_json = EXCLUDED.visible_tests_json,
			editable_files_json = EXCLUDED.editable_files_json,
			starter_code_path = EXCLUDED.starter_code_path,
			test_bundle_path = EXCLUDED.test_bundle_path
	`, variant.ID, variant.TemplateID, variant.VariantHash, variant.Seed, paramsJSON, filesJSON, visibleTestsJSON, editableFilesJSON, variant.StarterCodePath, variant.TestBundlePath)
	return err
}

func (s *SQLStore) UpsertChallengeInstance(ctx context.Context, instance ChallengeInstance) error {
	visibleFilesJSON, err := marshalJSON(instance.VisibleFiles)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO challenge_instances (
			id, user_id, template_id, variant_id, category, started_at, expires_at, status, attempt_number, visible_files_json
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			template_id = EXCLUDED.template_id,
			variant_id = EXCLUDED.variant_id,
			category = EXCLUDED.category,
			started_at = EXCLUDED.started_at,
			expires_at = EXCLUDED.expires_at,
			status = EXCLUDED.status,
			attempt_number = EXCLUDED.attempt_number,
			visible_files_json = EXCLUDED.visible_files_json
	`, instance.ID, instance.UserID, instance.TemplateID, instance.VariantID, instance.Category, instance.StartedAt,
		instance.ExpiresAt, instance.Status, instance.AttemptNumber, visibleFilesJSON)
	return err
}

func (s *SQLStore) InsertTelemetryEvent(ctx context.Context, event TelemetryEvent) error {
	payloadJSON, err := marshalJSON(event.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO telemetry_events (id, challenge_instance_id, user_id, event_type, offset_seconds, payload_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			challenge_instance_id = EXCLUDED.challenge_instance_id,
			user_id = EXCLUDED.user_id,
			event_type = EXCLUDED.event_type,
			offset_seconds = EXCLUDED.offset_seconds,
			payload_json = EXCLUDED.payload_json,
			created_at = EXCLUDED.created_at
	`, event.ID, event.ChallengeInstanceID, event.UserID, event.EventType, event.OffsetSeconds, payloadJSON, event.CreatedAt)
	return err
}

func (s *SQLStore) InsertAIInteraction(ctx context.Context, interaction AIInteraction) error {
	inputJSON, err := marshalJSON(interaction.Input)
	if err != nil {
		return err
	}
	outputJSON, err := marshalJSON(interaction.Output)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_interactions (
			id, user_id, challenge_instance_id, template_id, interaction_type, input_json, output_json, provider, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			challenge_instance_id = EXCLUDED.challenge_instance_id,
			template_id = EXCLUDED.template_id,
			interaction_type = EXCLUDED.interaction_type,
			input_json = EXCLUDED.input_json,
			output_json = EXCLUDED.output_json,
			provider = EXCLUDED.provider,
			created_at = EXCLUDED.created_at
	`, interaction.ID, interaction.UserID, interaction.ChallengeInstanceID, interaction.TemplateID, interaction.InteractionType,
		inputJSON, outputJSON, interaction.Provider, interaction.CreatedAt)
	return err
}

func (s *SQLStore) UpsertSubmissionAndEvaluation(ctx context.Context, submission Submission, evaluation EvaluationResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	sourceFilesJSON, err := marshalJSON(submission.SourceFiles)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO submissions (
			id, challenge_instance_id, submitted_at, source_archive_path, raw_code_text, source_files_json, language, execution_status
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			challenge_instance_id = EXCLUDED.challenge_instance_id,
			submitted_at = EXCLUDED.submitted_at,
			source_archive_path = EXCLUDED.source_archive_path,
			raw_code_text = EXCLUDED.raw_code_text,
			source_files_json = EXCLUDED.source_files_json,
			language = EXCLUDED.language,
			execution_status = EXCLUDED.execution_status
	`, submission.ID, submission.ChallengeInstanceID, submission.SubmittedAt, submission.SourceArchivePath,
		submission.RawCodeText, sourceFilesJSON, submission.Language, submission.ExecutionStatus); err != nil {
		return err
	}

	reportJSON, err := marshalJSON(evaluation.Report)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO evaluation_results (
			id, submission_id, test_score, lint_score, perf_score, quality_score, speed_score, consistency_score, final_score, report_json, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (submission_id) DO UPDATE SET
			id = EXCLUDED.id,
			test_score = EXCLUDED.test_score,
			lint_score = EXCLUDED.lint_score,
			perf_score = EXCLUDED.perf_score,
			quality_score = EXCLUDED.quality_score,
			speed_score = EXCLUDED.speed_score,
			consistency_score = EXCLUDED.consistency_score,
			final_score = EXCLUDED.final_score,
			report_json = EXCLUDED.report_json,
			created_at = EXCLUDED.created_at
	`, evaluation.ID, evaluation.SubmissionID, evaluation.TestScore, evaluation.LintScore, evaluation.PerfScore,
		evaluation.QualityScore, firstNonZeroFloat(evaluation.ExecutionCostScore, evaluation.SpeedScore), evaluation.ConsistencyScore, evaluation.FinalScore, reportJSON, evaluation.CreatedAt); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) UpsertScoreEvents(ctx context.Context, events []ScoreEvent) error {
	for _, event := range events {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO score_events (id, user_id, skill_id, source_id, delta, score_after, created_at, source_type)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO UPDATE SET
				user_id = EXCLUDED.user_id,
				skill_id = EXCLUDED.skill_id,
				source_id = EXCLUDED.source_id,
				delta = EXCLUDED.delta,
				score_after = EXCLUDED.score_after,
				created_at = EXCLUDED.created_at,
				source_type = EXCLUDED.source_type
		`, event.ID, event.UserID, event.SkillID, event.SourceID, event.Delta, event.ScoreAfter, event.CreatedAt, event.SourceType); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) UpsertFriendship(ctx context.Context, relation Friendship) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO friendships (user_id, friend_user_id, status, created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id, friend_user_id) DO UPDATE SET
			status = EXCLUDED.status,
			created_at = EXCLUDED.created_at
	`, relation.UserID, relation.FriendUserID, relation.Status, relation.CreatedAt)
	return err
}

func (s *SQLStore) UpsertChat(ctx context.Context, chat Chat, members []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO chats (id, type, created_at)
		VALUES ($1,$2,$3)
		ON CONFLICT (id) DO UPDATE SET type = EXCLUDED.type, created_at = EXCLUDED.created_at
	`, chat.ID, chat.Type, chat.CreatedAt); err != nil {
		return err
	}
	for _, memberID := range members {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chat_members (chat_id, user_id)
			VALUES ($1,$2)
			ON CONFLICT (chat_id, user_id) DO NOTHING
		`, chat.ID, memberID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStore) InsertChatMessage(ctx context.Context, message ChatMessage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chat_messages (id, chat_id, sender_user_id, body, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET body = EXCLUDED.body, created_at = EXCLUDED.created_at
	`, message.ID, message.ChatID, message.SenderUserID, message.Body, message.CreatedAt)
	return err
}

func (s *SQLStore) UpsertCompany(ctx context.Context, company Company) error {
	roomStateJSON, err := marshalJSON(company.RoomState)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO companies (id, owner_user_id, name, description, room_state_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			owner_user_id = EXCLUDED.owner_user_id,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			room_state_json = EXCLUDED.room_state_json,
			created_at = EXCLUDED.created_at
	`, company.ID, company.OwnerUserID, company.Name, company.Description, roomStateJSON, company.CreatedAt)
	return err
}

func (s *SQLStore) UpsertJob(ctx context.Context, job Job) error {
	requiredSkillsJSON, err := marshalJSON(job.RequiredSkills)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO jobs (id, company_id, title, description, required_score, required_skills_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			company_id = EXCLUDED.company_id,
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			required_score = EXCLUDED.required_score,
			required_skills_json = EXCLUDED.required_skills_json,
			created_at = EXCLUDED.created_at
	`, job.ID, job.CompanyID, job.Title, job.Description, job.RequiredScore, requiredSkillsJSON, job.CreatedAt)
	return err
}

func (s *SQLStore) InsertShortlist(ctx context.Context, entry HRShortlist) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO hr_shortlists (id, company_id, user_id, status, notes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			created_at = EXCLUDED.created_at
	`, entry.ID, entry.CompanyID, entry.UserID, entry.Status, entry.Notes, entry.CreatedAt)
	return err
}

func (s *SQLStore) UpsertRankingSnapshot(ctx context.Context, rankingType, country, scopeUserID string, entries []RankingEntry) error {
	dataJSON, err := marshalJSON(entries)
	if err != nil {
		return err
	}
	date := time.Now().UTC().Truncate(24 * time.Hour)
	id := fmt.Sprintf("ranking:%s:%s:%s:%s", rankingType, country, scopeUserID, date.Format("2006-01-02"))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO rankings_snapshots (id, ranking_type, country, scope_user_id, snapshot_date, data_json)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (ranking_type, country, scope_user_id, snapshot_date) DO UPDATE SET
			id = EXCLUDED.id,
			data_json = EXCLUDED.data_json
	`, id, rankingType, country, scopeUserID, date, dataJSON)
	return err
}

func (s *SQLStore) QueryRankingEntries(ctx context.Context, kind, country, userID string, now time.Time) ([]RankingEntry, error) {
	scopeWhere := "u.role = 'user'"
	args := []any{now}
	switch kind {
	case "country":
		args = append(args, country)
		scopeWhere += fmt.Sprintf(" AND u.country = $%d", len(args))
	case "friends":
		args = append(args, userID)
		scopeWhere += fmt.Sprintf(" AND (u.id = $%d OR EXISTS (SELECT 1 FROM friendships f WHERE f.user_id = $%d AND f.friend_user_id = u.id AND f.status = 'accepted'))", len(args), len(args))
	}

	query := fmt.Sprintf(`
		WITH scored AS (
			SELECT
				u.id AS user_id,
				u.username,
				u.country,
				u.last_active_at,
				COALESCE(p.confidence_score, 50) AS confidence_score,
				COALESCE(p.completed_challenges, 0) AS completed_challenges,
				ROUND(
					LEAST(
						1000.0,
						GREATEST(
							0.0,
							(
								COALESCE(MAX(CASE WHEN us.skill_code = 'react' THEN us.score END), 0) * 0.45 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'javascript' THEN us.score END), 0) * 0.20 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'performance' THEN us.score END), 0) * 0.15 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'architecture' THEN us.score END), 0) * 0.10 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'consistency' THEN us.score END), 0) * 0.10
							) * CASE
								WHEN u.last_active_at IS NULL OR $1::timestamptz <= u.last_active_at OR $1::timestamptz - u.last_active_at <= INTERVAL '21 days' THEN 1.0
								ELSE POWER(0.99::double precision, GREATEST(EXTRACT(EPOCH FROM ($1::timestamptz - u.last_active_at)) / 86400.0 - 21.0, 0.0))
							END
						)
					)::numeric,
					2
				) AS current_skill_score
			FROM users u
			LEFT JOIN user_profiles p ON p.user_id = u.id
			LEFT JOIN user_skills us ON us.user_id = u.id
			WHERE %s
			GROUP BY u.id, u.username, u.country, u.last_active_at, p.confidence_score, p.completed_challenges
		),
		ranked AS (
			SELECT
				user_id,
				username,
				country,
				current_skill_score,
				confidence_score,
				last_active_at,
				completed_challenges,
				ROW_NUMBER() OVER (ORDER BY current_skill_score DESC, confidence_score DESC, last_active_at DESC, username ASC) AS rank,
				COUNT(*) OVER () AS total_count
			FROM scored
		)
		SELECT
			user_id,
			username,
			country,
			current_skill_score,
			confidence_score,
			CASE
				WHEN total_count <= 1 THEN 100.0
				ELSE ROUND((((total_count - rank)::numeric) / ((total_count - 1)::numeric)) * 100.0, 2)
			END AS percentile,
			rank,
			last_active_at,
			completed_challenges
		FROM ranked
		ORDER BY rank
	`, scopeWhere)

	return s.queryRankingEntries(ctx, query, args...)
}

func (s *SQLStore) SearchCandidateEntries(ctx context.Context, filters CandidateSearchFilters, now time.Time) ([]RankingEntry, PaginationInfo, error) {
	args := []any{now}
	conditions := make([]string, 0, 4)
	if filters.MinScore > 0 {
		args = append(args, filters.MinScore)
		conditions = append(conditions, fmt.Sprintf("current_skill_score >= $%d", len(args)))
	}
	if filters.MinConfidence > 0 {
		args = append(args, filters.MinConfidence)
		conditions = append(conditions, fmt.Sprintf("confidence_score >= $%d", len(args)))
	}
	if filters.TopPercent > 0 {
		args = append(args, 100-filters.TopPercent)
		conditions = append(conditions, fmt.Sprintf("percentile >= $%d", len(args)))
	}
	if filters.ActiveDays > 0 {
		args = append(args, now.Add(-time.Duration(filters.ActiveDays)*24*time.Hour))
		conditions = append(conditions, fmt.Sprintf("last_active_at >= $%d", len(args)))
	}

	filterSQL := ""
	if len(conditions) > 0 {
		filterSQL = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = hrCandidatePageLimit
	}
	if limit > hrPaginationMaxLimit {
		limit = hrPaginationMaxLimit
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	baseCTE := fmt.Sprintf(`
		WITH scored AS (
			SELECT
				u.id AS user_id,
				u.username,
				u.country,
				u.last_active_at,
				COALESCE(p.confidence_score, 50) AS confidence_score,
				COALESCE(p.completed_challenges, 0) AS completed_challenges,
				ROUND(
					LEAST(
						1000.0,
						GREATEST(
							0.0,
							(
								COALESCE(MAX(CASE WHEN us.skill_code = 'react' THEN us.score END), 0) * 0.45 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'javascript' THEN us.score END), 0) * 0.20 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'performance' THEN us.score END), 0) * 0.15 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'architecture' THEN us.score END), 0) * 0.10 +
								COALESCE(MAX(CASE WHEN us.skill_code = 'consistency' THEN us.score END), 0) * 0.10
							) * CASE
								WHEN u.last_active_at IS NULL OR $1::timestamptz <= u.last_active_at OR $1::timestamptz - u.last_active_at <= INTERVAL '21 days' THEN 1.0
								ELSE POWER(0.99::double precision, GREATEST(EXTRACT(EPOCH FROM ($1::timestamptz - u.last_active_at)) / 86400.0 - 21.0, 0.0))
							END
						)
					)::numeric,
					2
				) AS current_skill_score
			FROM users u
			LEFT JOIN user_profiles p ON p.user_id = u.id
			LEFT JOIN user_skills us ON us.user_id = u.id
			WHERE u.role = 'user'
			GROUP BY u.id, u.username, u.country, u.last_active_at, p.confidence_score, p.completed_challenges
		),
		ranked AS (
			SELECT
				user_id,
				username,
				country,
				current_skill_score,
				confidence_score,
				last_active_at,
				completed_challenges,
				ROW_NUMBER() OVER (ORDER BY current_skill_score DESC, confidence_score DESC, last_active_at DESC, username ASC) AS rank,
				COUNT(*) OVER () AS total_count
			FROM scored
		),
		filtered AS (
			SELECT
				user_id,
				username,
				country,
				current_skill_score,
				confidence_score,
				CASE
					WHEN total_count <= 1 THEN 100.0
					ELSE ROUND((((total_count - rank)::numeric) / ((total_count - 1)::numeric)) * 100.0, 2)
				END AS percentile,
				rank,
				last_active_at,
				completed_challenges
			FROM ranked
			%s
		)
	`, filterSQL)

	countQuery := baseCTE + `
		SELECT COUNT(*)
		FROM filtered
	`

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, PaginationInfo{}, err
	}

	pageArgs := append(append([]any(nil), args...), limit, offset)
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	pageQuery := baseCTE + fmt.Sprintf(`
		SELECT
			user_id,
			username,
			country,
			current_skill_score,
			confidence_score,
			percentile,
			rank,
			last_active_at,
			completed_challenges
		FROM filtered
		ORDER BY rank
		LIMIT $%d OFFSET $%d
	`, limitArg, offsetArg)

	entries, err := s.queryRankingEntries(ctx, pageQuery, pageArgs...)
	if err != nil {
		return nil, PaginationInfo{}, err
	}
	return entries, buildPagination(limit, offset, total), nil
}

func (s *SQLStore) queryRankingEntries(ctx context.Context, query string, args ...any) ([]RankingEntry, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []RankingEntry
	for rows.Next() {
		var entry RankingEntry
		if err := rows.Scan(
			&entry.UserID,
			&entry.Username,
			&entry.Country,
			&entry.CurrentSkillScore,
			&entry.ConfidenceScore,
			&entry.Percentile,
			&entry.Rank,
			&entry.LastActiveAt,
			&entry.CompletedChallenges,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *SQLStore) loadUsers(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.username, u.password_hash, u.role, u.country, u.created_at, u.last_active_at,
		       p.user_id, p.selected_track, p.bio, p.avatar_url, p.linkedin_url, p.current_skill_score, p.percentile_global,
		       p.percentile_country, p.streak_days, p.confidence_score, p.completed_challenges, p.updated_at
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		var role string
		var profileUserID, selectedTrack, bio, avatarURL, linkedInURL sql.NullString
		var currentSkillScore, percentileGlobal, percentileCountry, confidenceScore sql.NullFloat64
		var streakDays, completedChallenges sql.NullInt64
		var profileUpdatedAt sql.NullTime
		if err := rows.Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &role, &user.Country,
			&user.CreatedAt, &user.LastActiveAt, &profileUserID, &selectedTrack, &bio, &avatarURL, &linkedInURL, &currentSkillScore,
			&percentileGlobal, &percentileCountry, &streakDays, &confidenceScore, &completedChallenges, &profileUpdatedAt); err != nil {
			return err
		}
		user.Role = Role(role)
		user.Profile = UserProfile{
			UserID:              user.ID,
			SelectedTrack:       "react",
			CurrentSkillScore:   0,
			PercentileGlobal:    0,
			PercentileCountry:   0,
			StreakDays:          0,
			ConfidenceScore:     50,
			CompletedChallenges: 0,
			UpdatedAt:           user.CreatedAt,
		}
		if profileUserID.Valid {
			user.Profile.SelectedTrack = selectedTrack.String
			user.Profile.Bio = bio.String
			user.Profile.AvatarURL = avatarURL.String
			user.Profile.LinkedInURL = linkedInURL.String
			user.Profile.CurrentSkillScore = currentSkillScore.Float64
			user.Profile.PercentileGlobal = percentileGlobal.Float64
			user.Profile.PercentileCountry = percentileCountry.Float64
			user.Profile.StreakDays = int(streakDays.Int64)
			user.Profile.ConfidenceScore = confidenceScore.Float64
			user.Profile.CompletedChallenges = int(completedChallenges.Int64)
			if profileUpdatedAt.Valid {
				user.Profile.UpdatedAt = profileUpdatedAt.Time
			}
		}
		app.users[user.ID] = user
		app.emailIndex[user.Email] = user.ID
	}
	return rows.Err()
}

func (s *SQLStore) loadRefreshSessions(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `SELECT token, user_id, expires_at FROM refresh_sessions`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var session refreshSession
		if err := rows.Scan(&session.Token, &session.UserID, &session.ExpiresAt); err != nil {
			return err
		}
		app.refreshSessions[session.Token] = session
	}
	return rows.Err()
}

func (s *SQLStore) loadUserSkills(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, skill_id, skill_code, score, confidence, level, last_verified_at, decay_factor, updated_at
		FROM user_skills
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var skill UserSkill
		if err := rows.Scan(&skill.ID, &skill.UserID, &skill.SkillID, &skill.SkillCode, &skill.Score, &skill.Confidence,
			&skill.Level, &skill.LastVerifiedAt, &skill.DecayFactor, &skill.UpdatedAt); err != nil {
			return err
		}
		if _, ok := app.userSkills[skill.UserID]; !ok {
			app.userSkills[skill.UserID] = map[string]UserSkill{}
		}
		app.userSkills[skill.UserID][skill.SkillCode] = skill
	}
	return rows.Err()
}

func (s *SQLStore) loadUserRoomItems(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, room_item_id, room_item_code, current_level, current_variant, state_json, updated_at
		FROM user_room_items
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item UserRoomItem
		var stateRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.RoomItemID, &item.RoomItemCode, &item.CurrentLevel, &item.CurrentVariant, &stateRaw, &item.UpdatedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(stateRaw, &item.State); err != nil {
			return err
		}
		if _, ok := app.roomItems[item.RoomItemCode]; !ok {
			continue
		}
		if _, ok := app.userRoomItems[item.UserID]; !ok {
			app.userRoomItems[item.UserID] = map[string]UserRoomItem{}
		}
		app.userRoomItems[item.UserID][item.RoomItemCode] = item
	}
	return rows.Err()
}

func (s *SQLStore) loadChallengeVariants(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, template_id, variant_hash, seed, params_json, generated_files_json, visible_tests_json, editable_files_json, starter_code_path, test_bundle_path
		FROM challenge_variants
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var variant ChallengeVariant
		var paramsRaw, filesRaw, visibleTestsRaw, editableFilesRaw []byte
		if err := rows.Scan(&variant.ID, &variant.TemplateID, &variant.VariantHash, &variant.Seed, &paramsRaw, &filesRaw, &visibleTestsRaw, &editableFilesRaw, &variant.StarterCodePath, &variant.TestBundlePath); err != nil {
			return err
		}
		if err := json.Unmarshal(paramsRaw, &variant.Params); err != nil {
			return err
		}
		if err := json.Unmarshal(filesRaw, &variant.GeneratedFiles); err != nil {
			return err
		}
		if err := json.Unmarshal(visibleTestsRaw, &variant.VisibleTests); err != nil {
			return err
		}
		if err := json.Unmarshal(editableFilesRaw, &variant.EditableFiles); err != nil {
			return err
		}
		app.variants[variant.ID] = variant
	}
	return rows.Err()
}

func (s *SQLStore) loadChallengeInstances(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, template_id, variant_id, category, started_at, expires_at, status, attempt_number, visible_files_json
		FROM challenge_instances
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var instance ChallengeInstance
		var visibleRaw []byte
		if err := rows.Scan(&instance.ID, &instance.UserID, &instance.TemplateID, &instance.VariantID, &instance.Category,
			&instance.StartedAt, &instance.ExpiresAt, &instance.Status, &instance.AttemptNumber, &visibleRaw); err != nil {
			return err
		}
		if err := json.Unmarshal(visibleRaw, &instance.VisibleFiles); err != nil {
			return err
		}
		app.instances[instance.ID] = instance
	}
	return rows.Err()
}

func (s *SQLStore) loadTelemetryEvents(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, challenge_instance_id, user_id, event_type, offset_seconds, payload_json, created_at
		FROM telemetry_events
		ORDER BY created_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var event TelemetryEvent
		var payloadRaw []byte
		if err := rows.Scan(&event.ID, &event.ChallengeInstanceID, &event.UserID, &event.EventType, &event.OffsetSeconds, &payloadRaw, &event.CreatedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(payloadRaw, &event.Payload); err != nil {
			return err
		}
		app.telemetryEvents[event.ChallengeInstanceID] = append(app.telemetryEvents[event.ChallengeInstanceID], event)
	}
	return rows.Err()
}

func (s *SQLStore) loadSubmissions(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, challenge_instance_id, submitted_at, source_archive_path, raw_code_text, source_files_json, language, execution_status
		FROM submissions
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var submission Submission
		var filesRaw []byte
		if err := rows.Scan(&submission.ID, &submission.ChallengeInstanceID, &submission.SubmittedAt, &submission.SourceArchivePath,
			&submission.RawCodeText, &filesRaw, &submission.Language, &submission.ExecutionStatus); err != nil {
			return err
		}
		if err := json.Unmarshal(filesRaw, &submission.SourceFiles); err != nil {
			return err
		}
		app.submissions[submission.ID] = submission
	}
	return rows.Err()
}

func (s *SQLStore) loadEvaluations(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, submission_id, test_score, lint_score, perf_score, quality_score, speed_score, consistency_score, final_score, report_json, created_at
		FROM evaluation_results
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var evaluation EvaluationResult
		var reportRaw []byte
		if err := rows.Scan(&evaluation.ID, &evaluation.SubmissionID, &evaluation.TestScore, &evaluation.LintScore,
			&evaluation.PerfScore, &evaluation.QualityScore, &evaluation.SpeedScore, &evaluation.ConsistencyScore,
			&evaluation.FinalScore, &reportRaw, &evaluation.CreatedAt); err != nil {
			return err
		}
		evaluation.ExecutionCostScore = firstNonZeroFloat(evaluation.SpeedScore, evaluation.ExecutionCostScore)
		if err := json.Unmarshal(reportRaw, &evaluation.Report); err != nil {
			return err
		}
		app.evaluations[evaluation.SubmissionID] = evaluation
	}
	return rows.Err()
}

func (s *SQLStore) loadScoreEvents(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, skill_id, source_id, delta, score_after, created_at, source_type
		FROM score_events
		ORDER BY created_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var event ScoreEvent
		if err := rows.Scan(&event.ID, &event.UserID, &event.SkillID, &event.SourceID, &event.Delta, &event.ScoreAfter, &event.CreatedAt, &event.SourceType); err != nil {
			return err
		}
		app.scoreEvents[event.UserID] = append(app.scoreEvents[event.UserID], event)
	}
	return rows.Err()
}

func firstNonZeroFloat(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (s *SQLStore) loadFriendships(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, friend_user_id, status, created_at FROM friendships`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var relation Friendship
		if err := rows.Scan(&relation.UserID, &relation.FriendUserID, &relation.Status, &relation.CreatedAt); err != nil {
			return err
		}
		if _, ok := app.friendships[relation.UserID]; !ok {
			app.friendships[relation.UserID] = map[string]Friendship{}
		}
		app.friendships[relation.UserID][relation.FriendUserID] = relation
	}
	return rows.Err()
}

func (s *SQLStore) loadChats(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, type, created_at FROM chats`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var chat Chat
		if err := rows.Scan(&chat.ID, &chat.Type, &chat.CreatedAt); err != nil {
			return err
		}
		app.chats[chat.ID] = chat
	}
	if err := rows.Err(); err != nil {
		return err
	}

	memberRows, err := s.db.QueryContext(ctx, `SELECT chat_id, user_id FROM chat_members`)
	if err != nil {
		return err
	}
	defer memberRows.Close()
	chatMembers := map[string][]string{}
	for memberRows.Next() {
		var chatID, userID string
		if err := memberRows.Scan(&chatID, &userID); err != nil {
			return err
		}
		chatMembers[chatID] = append(chatMembers[chatID], userID)
	}
	if err := memberRows.Err(); err != nil {
		return err
	}

	messageRows, err := s.db.QueryContext(ctx, `SELECT id, chat_id, sender_user_id, body, created_at FROM chat_messages ORDER BY created_at ASC`)
	if err != nil {
		return err
	}
	defer messageRows.Close()
	for messageRows.Next() {
		var message ChatMessage
		if err := messageRows.Scan(&message.ID, &message.ChatID, &message.SenderUserID, &message.Body, &message.CreatedAt); err != nil {
			return err
		}
		app.chatMessages[message.ChatID] = append(app.chatMessages[message.ChatID], message)
	}
	if err := messageRows.Err(); err != nil {
		return err
	}

	for chatID, members := range chatMembers {
		if app.chats[chatID].Type == "direct" {
			slices.Sort(members)
			if len(members) == 2 {
				app.directChats[pairKey(members[0], members[1])] = chatID
			}
		}
	}
	return nil
}

func (s *SQLStore) loadCompanies(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, owner_user_id, name, description, room_state_json, created_at FROM companies`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var company Company
		var roomStateRaw []byte
		if err := rows.Scan(&company.ID, &company.OwnerUserID, &company.Name, &company.Description, &roomStateRaw, &company.CreatedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(roomStateRaw, &company.RoomState); err != nil {
			return err
		}
		app.companies[company.ID] = company
	}
	return rows.Err()
}

func (s *SQLStore) loadJobs(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, company_id, title, description, required_score, required_skills_json, created_at FROM jobs`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var job Job
		var skillsRaw []byte
		if err := rows.Scan(&job.ID, &job.CompanyID, &job.Title, &job.Description, &job.RequiredScore, &skillsRaw, &job.CreatedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(skillsRaw, &job.RequiredSkills); err != nil {
			return err
		}
		app.jobs[job.ID] = job
	}
	return rows.Err()
}

func (s *SQLStore) loadShortlists(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, company_id, user_id, status, notes, created_at FROM hr_shortlists ORDER BY created_at ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var entry HRShortlist
		if err := rows.Scan(&entry.ID, &entry.CompanyID, &entry.UserID, &entry.Status, &entry.Notes, &entry.CreatedAt); err != nil {
			return err
		}
		app.shortlists = append(app.shortlists, entry)
	}
	return rows.Err()
}

func (s *SQLStore) loadAIInteractions(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, challenge_instance_id, template_id, interaction_type, input_json, output_json, provider, created_at
		FROM ai_interactions
		ORDER BY created_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var interaction AIInteraction
		var inputRaw, outputRaw []byte
		if err := rows.Scan(&interaction.ID, &interaction.UserID, &interaction.ChallengeInstanceID, &interaction.TemplateID,
			&interaction.InteractionType, &inputRaw, &outputRaw, &interaction.Provider, &interaction.CreatedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(inputRaw, &interaction.Input); err != nil {
			return err
		}
		if err := json.Unmarshal(outputRaw, &interaction.Output); err != nil {
			return err
		}
		app.aiInteractions = append(app.aiInteractions, interaction)
	}
	return rows.Err()
}

func marshalJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func sortedSkillCodes(skills map[string]Skill) []string {
	keys := make([]string, 0, len(skills))
	for key := range skills {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRoomItemCodes(items map[string]RoomItem) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedTemplateIDs(items map[string]ChallengeTemplate) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
