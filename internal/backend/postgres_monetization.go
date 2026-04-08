package backend

import (
	"context"
	"encoding/json"
	"sort"
)

func (s *SQLStore) UpsertSubscription(ctx context.Context, subscription Subscription) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (id, user_id, plan_id, plan_code, status, source, auto_renew, current_period_start, current_period_end, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (user_id) DO UPDATE SET
			id = EXCLUDED.id,
			plan_id = EXCLUDED.plan_id,
			plan_code = EXCLUDED.plan_code,
			status = EXCLUDED.status,
			source = EXCLUDED.source,
			auto_renew = EXCLUDED.auto_renew,
			current_period_start = EXCLUDED.current_period_start,
			current_period_end = EXCLUDED.current_period_end,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at
	`, subscription.ID, subscription.UserID, subscription.PlanID, subscription.PlanCode, subscription.Status, subscription.Source,
		subscription.AutoRenew, subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd, subscription.CreatedAt, subscription.UpdatedAt)
	return err
}

func (s *SQLStore) UpsertCandidateUnlock(ctx context.Context, unlock CandidateUnlock) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO candidate_unlocks (id, recruiter_user_id, candidate_user_id, source, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (recruiter_user_id, candidate_user_id) DO UPDATE SET
			id = EXCLUDED.id,
			source = EXCLUDED.source,
			status = EXCLUDED.status,
			created_at = EXCLUDED.created_at
	`, unlock.ID, unlock.RecruiterUserID, unlock.CandidateUserID, unlock.Source, unlock.Status, unlock.CreatedAt)
	return err
}

func (s *SQLStore) UpsertCandidateInvite(ctx context.Context, invite CandidateInvite) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO candidate_invites (id, recruiter_user_id, candidate_user_id, source, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (recruiter_user_id, candidate_user_id) DO UPDATE SET
			id = EXCLUDED.id,
			source = EXCLUDED.source,
			status = EXCLUDED.status,
			created_at = EXCLUDED.created_at
	`, invite.ID, invite.RecruiterUserID, invite.CandidateUserID, invite.Source, invite.Status, invite.CreatedAt)
	return err
}

func (s *SQLStore) InsertAIUsageEvent(ctx context.Context, event AIUsageEvent) error {
	contextJSON, err := marshalJSON(event.Context)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_usage_events (id, user_id, scope, action, units, context_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			scope = EXCLUDED.scope,
			action = EXCLUDED.action,
			units = EXCLUDED.units,
			context_json = EXCLUDED.context_json,
			created_at = EXCLUDED.created_at
	`, event.ID, event.UserID, event.Scope, event.Action, event.Units, contextJSON, event.CreatedAt)
	return err
}

func (s *SQLStore) UpsertUserCosmetic(ctx context.Context, item UserCosmetic) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_cosmetics (id, user_id, cosmetic_id, cosmetic_code, source, owned_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id, cosmetic_code) DO UPDATE SET
			id = EXCLUDED.id,
			cosmetic_id = EXCLUDED.cosmetic_id,
			source = EXCLUDED.source,
			owned_at = EXCLUDED.owned_at
	`, item.ID, item.UserID, item.CosmeticID, item.CosmeticCode, item.Source, item.OwnedAt)
	return err
}

func (s *SQLStore) UpsertEquippedCosmetic(ctx context.Context, item EquippedCosmetic) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO equipped_cosmetics (user_id, slot_code, cosmetic_code, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id, slot_code) DO UPDATE SET
			cosmetic_code = EXCLUDED.cosmetic_code,
			updated_at = EXCLUDED.updated_at
	`, item.UserID, item.SlotCode, item.CosmeticCode, item.UpdatedAt)
	return err
}

func (s *SQLStore) loadSubscriptions(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, plan_id, plan_code, status, source, auto_renew, current_period_start, current_period_end, created_at, updated_at
		FROM subscriptions
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var subscription Subscription
		if err := rows.Scan(&subscription.ID, &subscription.UserID, &subscription.PlanID, &subscription.PlanCode, &subscription.Status,
			&subscription.Source, &subscription.AutoRenew, &subscription.CurrentPeriodStart, &subscription.CurrentPeriodEnd,
			&subscription.CreatedAt, &subscription.UpdatedAt); err != nil {
			return err
		}
		app.subscriptions[subscription.UserID] = subscription
	}
	return rows.Err()
}

func (s *SQLStore) loadCandidateUnlocks(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, recruiter_user_id, candidate_user_id, source, status, created_at
		FROM candidate_unlocks
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var unlock CandidateUnlock
		if err := rows.Scan(&unlock.ID, &unlock.RecruiterUserID, &unlock.CandidateUserID, &unlock.Source, &unlock.Status, &unlock.CreatedAt); err != nil {
			return err
		}
		if _, ok := app.candidateUnlocks[unlock.RecruiterUserID]; !ok {
			app.candidateUnlocks[unlock.RecruiterUserID] = map[string]CandidateUnlock{}
		}
		app.candidateUnlocks[unlock.RecruiterUserID][unlock.CandidateUserID] = unlock
	}
	return rows.Err()
}

func (s *SQLStore) loadCandidateInvites(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, recruiter_user_id, candidate_user_id, source, status, created_at
		FROM candidate_invites
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var invite CandidateInvite
		if err := rows.Scan(&invite.ID, &invite.RecruiterUserID, &invite.CandidateUserID, &invite.Source, &invite.Status, &invite.CreatedAt); err != nil {
			return err
		}
		if _, ok := app.candidateInvites[invite.RecruiterUserID]; !ok {
			app.candidateInvites[invite.RecruiterUserID] = map[string]CandidateInvite{}
		}
		app.candidateInvites[invite.RecruiterUserID][invite.CandidateUserID] = invite
	}
	return rows.Err()
}

func (s *SQLStore) loadAIUsageEvents(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, scope, action, units, context_json, created_at
		FROM ai_usage_events
		ORDER BY created_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var event AIUsageEvent
		var contextRaw []byte
		if err := rows.Scan(&event.ID, &event.UserID, &event.Scope, &event.Action, &event.Units, &contextRaw, &event.CreatedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(contextRaw, &event.Context); err != nil {
			return err
		}
		app.aiUsageEvents[event.UserID] = append(app.aiUsageEvents[event.UserID], event)
	}
	return rows.Err()
}

func (s *SQLStore) loadUserCosmetics(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, cosmetic_id, cosmetic_code, source, owned_at
		FROM user_cosmetics
		ORDER BY owned_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item UserCosmetic
		if err := rows.Scan(&item.ID, &item.UserID, &item.CosmeticID, &item.CosmeticCode, &item.Source, &item.OwnedAt); err != nil {
			return err
		}
		if _, ok := app.userCosmetics[item.UserID]; !ok {
			app.userCosmetics[item.UserID] = map[string]UserCosmetic{}
		}
		app.userCosmetics[item.UserID][item.CosmeticCode] = item
	}
	return rows.Err()
}

func (s *SQLStore) loadEquippedCosmetics(ctx context.Context, app *App) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, slot_code, cosmetic_code, updated_at
		FROM equipped_cosmetics
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item EquippedCosmetic
		if err := rows.Scan(&item.UserID, &item.SlotCode, &item.CosmeticCode, &item.UpdatedAt); err != nil {
			return err
		}
		if _, ok := app.equippedCosmetics[item.UserID]; !ok {
			app.equippedCosmetics[item.UserID] = map[string]EquippedCosmetic{}
		}
		app.equippedCosmetics[item.UserID][item.SlotCode] = item
	}
	return rows.Err()
}

func sortedPlanCodes(items map[string]Plan) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCosmeticCodes(items map[string]CosmeticCatalogItem) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
