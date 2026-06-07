CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    firstname VARCHAR(100) NOT NULL,
    lastname VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    two_fa_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    blocked_until TIMESTAMP WITH TIME ZONE,
    failed_attempts INTEGER DEFAULT 0,
    last_failed_attempt TIMESTAMP WITH TIME ZONE,
    avatar TEXT
);

CREATE TABLE refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(512) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE two_fa_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    attempts INTEGER DEFAULT 0,
    is_used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE login_attempts (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    success BOOLEAN NOT NULL,
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(45),
    user_agent TEXT
);

-- Индексы для оптимизации
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX idx_two_fa_codes_user_id ON two_fa_codes(user_id);
CREATE INDEX idx_two_fa_codes_expires_at ON two_fa_codes(expires_at);
CREATE INDEX idx_login_attempts_email ON login_attempts(email);
CREATE INDEX idx_login_attempts_attempted_at ON login_attempts(attempted_at);

-- Индекс для поиска по email и времени
CREATE INDEX idx_login_attempts_email_time ON login_attempts(email, attempted_at);

CREATE TABLE IF NOT EXISTS rooms (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    owner_id   TEXT NOT NULL,
    name       TEXT NOT NULL,
    members    JSONB NOT NULL, -- список ID участников
    settings   JSONB NOT NULL, -- объект настроек
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    avatar     TEXT
);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    room_id         TEXT NOT NULL,
    timestamp       BIGINT NOT NULL,
    payload         JSONB NOT NULL,
    signature       TEXT,
    version         INTEGER NOT NULL DEFAULT 1,
    edited_at       TIMESTAMP WITH TIME ZONE,
    deleted_at      TIMESTAMP WITH TIME ZONE,
    reply_to_id     TEXT,
    forwarded_from_id TEXT,
    topic_id        TEXT,
    scheduled_at    TIMESTAMP WITH TIME ZONE,
    search_vector   tsvector,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at      TIMESTAMP WITH TIME ZONE
);

-- Индексы (sqlc их не генерирует, но они нужны в БД)
CREATE INDEX IF NOT EXISTS idx_messages_room_timestamp ON messages(room_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages(expires_at) WHERE expires_at IS NOT NULL;

CREATE INDEX idx_messages_search
ON messages
USING GIN(search_vector);

CREATE FUNCTION messages_search_trigger()
RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        to_tsvector(
            'simple',
            COALESCE(NEW.payload->>'content', '')
        );

    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER messages_search_update
BEFORE INSERT OR UPDATE
ON messages
FOR EACH ROW
EXECUTE FUNCTION messages_search_trigger();

CREATE INDEX idx_messages_reply_to
ON messages(reply_to_id);
CREATE INDEX idx_messages_forwarded
ON messages(forwarded_from_id);

CREATE TABLE message_retry_queue (
    id UUID PRIMARY KEY,
    message_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    attempt INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retry_next_retry
ON message_retry_queue(next_retry_at);

CREATE TABLE message_dlq (
    id UUID PRIMARY KEY,
    message_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    reason TEXT NOT NULL,
    failed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE message_delivery_status (
    message_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status TEXT NOT NULL,
    delivered_at TIMESTAMP NULL,
    read_at TIMESTAMP NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY(message_id, user_id)
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP,
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX idx_outbox_unpublished
ON outbox_events(published_at)
WHERE published_at IS NULL;

CREATE TABLE message_deduplication (
    idempotency_key TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    muted_until TIMESTAMP WITH TIME ZONE,
    banned_until TIMESTAMP WITH TIME ZONE,
    UNIQUE(chat_id, user_id)
);

CREATE INDEX idx_chat_memberships_chat_id
ON chat_memberships(chat_id);

CREATE INDEX idx_chat_memberships_user_id
ON chat_memberships(user_id);

CREATE TABLE invite_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    token TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    usage_limit INTEGER,
    usage_count INTEGER NOT NULL DEFAULT 0,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE join_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    user_id UUID NOT NULL,
    message TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMP
);

CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY,
    username TEXT UNIQUE,
    bio TEXT,
    avatar_media_id TEXT,
    payload JSONB NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}',
    version BIGINT NOT NULL DEFAULT 1,
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    allow_mentions BOOLEAN NOT NULL DEFAULT TRUE,
    allow_group_invites BOOLEAN NOT NULL DEFAULT TRUE,
    show_last_seen BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- UPDATE user_profiles
-- SET payload = $1, version = version + 1
-- WHERE user_id = $2
-- AND version = $3;

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    original_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_key TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    encryption_nonce BYTEA,
    compressed BOOLEAN NOT NULL DEFAULT FALSE,
    encrypted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE message_edit_history (
    id BIGSERIAL PRIMARY KEY,
    message_id TEXT NOT NULL,
    editor_id TEXT NOT NULL,
    old_payload JSONB NOT NULL,
    diff JSONB,
    version BIGINT NOT NULL DEFAULT 1,
    edited_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_message_edit_history_message
ON message_edit_history(message_id, edited_at DESC);

CREATE TABLE topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    title TEXT NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE pinned_messages (
    chat_id UUID NOT NULL,
    message_id UUID NOT NULL,
    pinned_by UUID NOT NULL,
    pinned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY(chat_id, message_id)
);

CREATE TABLE reactions (
    message_id UUID NOT NULL,
    user_id UUID NOT NULL,
    reaction TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY(message_id, user_id, reaction)
);

CREATE TABLE mentions (
    message_id UUID NOT NULL,
    mentioned_user_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE bans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    user_id UUID NOT NULL,
    reason TEXT,
    banned_by UUID NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL,
    message_id UUID,
    reason TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE events (
    id BIGSERIAL PRIMARY KEY,
    aggregate_id TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_version INTEGER NOT NULL,
    sequence BIGSERIAL,
    payload JSONB NOT NULL,
    created_by TEXT,
    correlation_id TEXT,
    causation_id TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_events_aggregate
ON events(aggregate_id);
CREATE INDEX idx_events_aggregate_sequence
ON events(
    aggregate_type,
    aggregate_id,
    sequence
);
CREATE INDEX idx_events_created_at
ON events(created_at);

CREATE TABLE polls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL,
    question TEXT NOT NULL,
    multiple_choice BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE poll_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL,
    title TEXT NOT NULL
);

CREATE TABLE poll_votes (
    poll_option_id UUID NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY(poll_option_id, user_id)
);

CREATE TABLE scheduled_messages (
    message_id TEXT PRIMARY KEY,
    scheduled_at TIMESTAMP WITH TIME ZONE NOT NULL,
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE roles (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

INSERT INTO roles (id, name)
VALUES
('owner', 'owner'),
('admin', 'admin'),
('moderator', 'moderator'),
('member', 'member'),
('guest', 'guest');

CREATE TABLE permissions (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

INSERT INTO permissions (id, name)
VALUES
('delete_message', 'delete_message'),
('ban_user', 'ban_user'),
('pin_message', 'pin_message'),
('edit_message', 'edit_message'),
('manage_invites', 'manage_invites');

CREATE TABLE role_permissions (
    role_id TEXT NOT NULL,
    permission_id TEXT NOT NULL,
    PRIMARY KEY(role_id, permission_id)
);

INSERT INTO role_permissions (role_id, permission_id)
VALUES
('owner', 'delete_message'),
('owner', 'ban_user'),
('owner', 'pin_message'),
('admin', 'delete_message'),
('admin', 'pin_message'),
('moderator', 'delete_message');

CREATE TABLE media_files (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    media_type TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    original_name TEXT NOT NULL,
    original_size BIGINT NOT NULL,
    compressed_size BIGINT,
    storage_key TEXT NOT NULL,
    thumbnail_key TEXT,
    sha256 TEXT NOT NULL,
    encryption_nonce BYTEA,
    width INTEGER,
    height INTEGER,
    duration_seconds INTEGER,
    processing_status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE message_attachments (
    message_id TEXT NOT NULL,
    media_file_id TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY(message_id, media_file_id)
);

CREATE TABLE device_tokens (
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    push_token TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY(user_id, device_id)
);

CREATE TABLE device_settings (
    device_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notification_sound TEXT,
    language TEXT,
    timezone TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_device_settings_user
ON device_settings(user_id);

CREATE TABLE notification_preferences (
    user_id TEXT PRIMARY KEY,
    push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    mention_notifications BOOLEAN NOT NULL DEFAULT TRUE,
    message_notifications BOOLEAN NOT NULL DEFAULT TRUE,
    sound_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    payload JSONB,
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    payload JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_actor
ON audit_logs(actor_id);
CREATE INDEX idx_audit_logs_entity
ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_created
ON audit_logs(created_at DESC);

CREATE TABLE saved_messages (
    user_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, message_id)
);

CREATE INDEX idx_saved_messages_user
ON saved_messages(user_id, saved_at DESC);

CREATE TABLE moderation_actions (
    id BIGSERIAL PRIMARY KEY,
    room_id TEXT NOT NULL,
    target_user_id TEXT NOT NULL,
    actor_user_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    reason TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_moderation_target
ON moderation_actions(room_id, target_user_id);

CREATE TABLE aggregate_snapshots (
    id BIGSERIAL PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    last_sequence BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_snapshots_unique
ON aggregate_snapshots(aggregate_type, aggregate_id);

CREATE TABLE message_search_projection (
    message_id TEXT PRIMARY KEY,
    room_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    mentions TSVECTOR,
    search_vector TSVECTOR,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_message_search_projection_room
ON message_search_projection(room_id);
CREATE INDEX idx_message_search_projection_search
ON message_search_projection
USING GIN(search_vector);
CREATE INDEX idx_message_search_projection_mentions
ON message_search_projection
USING GIN(mentions);

CREATE TABLE user_storage_quotas (
    user_id TEXT PRIMARY KEY,
    used_bytes BIGINT NOT NULL DEFAULT 0,
    max_bytes BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE projection_checkpoints (
    consumer_name TEXT PRIMARY KEY,
    last_sequence BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_search_content_trgm
ON message_search_projection
USING GIN(content gin_trgm_ops);

CREATE TABLE moderation_projection (
    user_id TEXT PRIMARY KEY,
    warnings_count INT NOT NULL DEFAULT 0,
    bans_count INT NOT NULL DEFAULT 0,
    mutes_count INT NOT NULL DEFAULT 0,
    last_action_at TIMESTAMPTZ
);

CREATE TABLE media_retention_policies (
    id BIGSERIAL PRIMARY KEY,
    media_type TEXT NOT NULL,
    retention_days INT NOT NULL,
    auto_delete BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE processed_events (
    consumer_name TEXT NOT NULL,
    event_id BIGINT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(consumer_name, event_id)
);

CREATE TABLE event_dlq (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL,
    consumer_name TEXT NOT NULL,
    error TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE attachment_search_projection (
    media_id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attachment_search_room
ON attachment_search_projection(room_id);
CREATE INDEX idx_attachment_search_name
ON attachment_search_projection
USING GIN(file_name gin_trgm_ops);

CREATE TABLE mention_projection (
    message_id TEXT NOT NULL,
    mentioned_user_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(message_id, mentioned_user_id)
);

CREATE TABLE unread_projection (
    user_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    unread_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, room_id)
);

CREATE TABLE reaction_counters (
    message_id TEXT NOT NULL,
    reaction TEXT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY(message_id, reaction)
);

CREATE TABLE upload_chunks (
    session_id TEXT NOT NULL,
    chunk_index BIGINT NOT NULL,
    chunk_size BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    data BYTEA,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(session_id, chunk_index)
);

CREATE TABLE search_index_dlq (
    id BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    operation TEXT NOT NULL,
    error TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE saved_message_projection (
    user_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    folder_id TEXT,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, message_id)
);

CREATE TABLE saved_folders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sync_state (
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    last_event_sequence BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, device_id)
);

CREATE TABLE abuse_scores (
    user_id TEXT PRIMARY KEY,
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE shadow_bans (
    user_id TEXT PRIMARY KEY,
    reason TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE mutes (
    user_id    TEXT NOT NULL,
    room_id    TEXT,
    muted_until TIMESTAMPTZ,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id)
);

CREATE TABLE sticker_packs (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE stickers (
    id TEXT PRIMARY KEY,
    pack_id TEXT NOT NULL REFERENCES sticker_packs(id),
    emoji TEXT,
    media_file_id TEXT NOT NULL
);

CREATE TABLE custom_emoji (
    id TEXT PRIMARY KEY,
    shortcode TEXT NOT NULL UNIQUE,
    media_file_id TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE upload_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    total_size BIGINT NOT NULL,
    total_chunks BIGINT NOT NULL,
    uploaded_chunks BIGINT NOT NULL DEFAULT 0,
    expected_hash TEXT NOT NULL,
    upload_status TEXT NOT NULL DEFAULT 'pending',
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_upload_sessions_user
ON upload_sessions(user_id);
CREATE INDEX idx_upload_sessions_status
ON upload_sessions(upload_status);
CREATE INDEX idx_upload_sessions_expires
ON upload_sessions(expires_at);

CREATE TABLE media_jobs (
    id BIGSERIAL PRIMARY KEY,
    media_file_id TEXT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_jobs_status ON media_jobs(status, scheduled_at);
