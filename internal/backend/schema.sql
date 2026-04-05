CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    selected_track TEXT NOT NULL,
    bio TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    current_skill_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    percentile_global DOUBLE PRECISION NOT NULL DEFAULT 0,
    percentile_country DOUBLE PRECISION NOT NULL DEFAULT 0,
    streak_days INTEGER NOT NULL DEFAULT 0,
    confidence_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    completed_challenges INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_sessions (
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS user_skills (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id TEXT NOT NULL REFERENCES skills(id),
    skill_code TEXT NOT NULL,
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    level TEXT NOT NULL,
    last_verified_at TIMESTAMPTZ NOT NULL,
    decay_factor DOUBLE PRECISION NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (user_id, skill_code)
);

CREATE INDEX IF NOT EXISTS idx_user_skills_user_id ON user_skills(user_id);

CREATE TABLE IF NOT EXISTS room_items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slot TEXT NOT NULL,
    related_skill_id TEXT NOT NULL REFERENCES skills(id),
    code TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS user_room_items (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    room_item_id TEXT NOT NULL REFERENCES room_items(id),
    room_item_code TEXT NOT NULL,
    current_level TEXT NOT NULL,
    current_variant TEXT NOT NULL,
    state_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (user_id, room_item_code)
);

CREATE INDEX IF NOT EXISTS idx_user_room_items_user_id ON user_room_items(user_id);

CREATE TABLE IF NOT EXISTS challenge_templates (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    difficulty INTEGER NOT NULL,
    description_md TEXT NOT NULL,
    asset_directory TEXT NOT NULL DEFAULT '',
    editable_files_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    starter_code_template TEXT NOT NULL,
    visible_tests_template TEXT NOT NULL,
    evaluation_config_json JSONB NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    category TEXT NOT NULL,
    track TEXT NOT NULL,
    variation_strings_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    variation_numbers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    skill_weights_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_challenge_templates_category ON challenge_templates(category);

CREATE TABLE IF NOT EXISTS challenge_variants (
    id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL REFERENCES challenge_templates(id),
    variant_hash TEXT NOT NULL,
    seed BIGINT NOT NULL,
    params_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    generated_files_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    visible_tests_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    editable_files_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    starter_code_path TEXT NOT NULL,
    test_bundle_path TEXT NOT NULL
);

ALTER TABLE challenge_templates ADD COLUMN IF NOT EXISTS asset_directory TEXT NOT NULL DEFAULT '';
ALTER TABLE challenge_templates ADD COLUMN IF NOT EXISTS editable_files_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE challenge_variants ADD COLUMN IF NOT EXISTS visible_tests_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE challenge_variants ADD COLUMN IF NOT EXISTS editable_files_json JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_challenge_variants_template_id ON challenge_variants(template_id);

CREATE TABLE IF NOT EXISTS challenge_instances (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    template_id TEXT NOT NULL REFERENCES challenge_templates(id),
    variant_id TEXT NOT NULL REFERENCES challenge_variants(id),
    category TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    attempt_number INTEGER NOT NULL,
    visible_files_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_challenge_instances_user_id ON challenge_instances(user_id);
CREATE INDEX IF NOT EXISTS idx_challenge_instances_status ON challenge_instances(status);

CREATE TABLE IF NOT EXISTS telemetry_events (
    id TEXT PRIMARY KEY,
    challenge_instance_id TEXT NOT NULL REFERENCES challenge_instances(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    offset_seconds INTEGER NOT NULL DEFAULT 0,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_telemetry_events_instance_id ON telemetry_events(challenge_instance_id, created_at);

CREATE TABLE IF NOT EXISTS ai_interactions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    challenge_instance_id TEXT NOT NULL DEFAULT '',
    template_id TEXT NOT NULL DEFAULT '',
    interaction_type TEXT NOT NULL,
    input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    provider TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ai_interactions_instance_id ON ai_interactions(challenge_instance_id, created_at);
CREATE INDEX IF NOT EXISTS idx_ai_interactions_user_id ON ai_interactions(user_id, created_at);

CREATE TABLE IF NOT EXISTS submissions (
    id TEXT PRIMARY KEY,
    challenge_instance_id TEXT NOT NULL REFERENCES challenge_instances(id) ON DELETE CASCADE,
    submitted_at TIMESTAMPTZ NOT NULL,
    source_archive_path TEXT NOT NULL DEFAULT '',
    raw_code_text TEXT NOT NULL DEFAULT '',
    source_files_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    language TEXT NOT NULL,
    execution_status TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_submissions_instance_id ON submissions(challenge_instance_id);

CREATE TABLE IF NOT EXISTS evaluation_results (
    id TEXT PRIMARY KEY,
    submission_id TEXT NOT NULL UNIQUE REFERENCES submissions(id) ON DELETE CASCADE,
    test_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    lint_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    perf_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    speed_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    consistency_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    final_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    report_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS score_events (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id TEXT NOT NULL REFERENCES skills(id),
    source_id TEXT NOT NULL,
    delta DOUBLE PRECISION NOT NULL,
    score_after DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    source_type TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_score_events_user_id ON score_events(user_id);

CREATE TABLE IF NOT EXISTS rankings_snapshots (
    id TEXT PRIMARY KEY,
    ranking_type TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT '',
    scope_user_id TEXT NOT NULL DEFAULT '',
    snapshot_date DATE NOT NULL,
    data_json JSONB NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rankings_snapshots_scope
    ON rankings_snapshots(ranking_type, country, scope_user_id, snapshot_date);

CREATE TABLE IF NOT EXISTS friendships (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, friend_user_id)
);

CREATE TABLE IF NOT EXISTS chats (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_members (
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_id ON chat_messages(chat_id, created_at);

CREATE TABLE IF NOT EXISTS companies (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    room_state_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    company_id TEXT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    required_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    required_skills_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_company_id ON jobs(company_id);

CREATE TABLE IF NOT EXISTS hr_shortlists (
    id TEXT PRIMARY KEY,
    company_id TEXT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_hr_shortlists_company_id ON hr_shortlists(company_id);
