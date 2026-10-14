CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version > 0),
    checksum TEXT NOT NULL,
    applied_at INTEGER NOT NULL
);
CREATE TABLE projects (
    id TEXT PRIMARY KEY NOT NULL,
    canonical_path TEXT UNIQUE,
    alias TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    registered_at INTEGER,
    CHECK ((canonical_path IS NULL) = (registered_at IS NULL))
);
CREATE TABLE pair_sequences (
    project_low TEXT NOT NULL REFERENCES projects(id),
    project_high TEXT NOT NULL REFERENCES projects(id),
    last_ordinal INTEGER NOT NULL CHECK (last_ordinal >= 0),
    PRIMARY KEY (project_low, project_high),
    CHECK (project_low < project_high)
);
CREATE TABLE conversations (
    id TEXT PRIMARY KEY NOT NULL,
    project_low TEXT NOT NULL,
    project_high TEXT NOT NULL,
    pair_ordinal INTEGER NOT NULL CHECK (pair_ordinal > 0),
    topic TEXT NOT NULL CHECK (length(CAST(topic AS BLOB)) BETWEEN 1 AND 256),
    state TEXT NOT NULL CHECK (state IN ('pending','active','closed','expired')),
    created_at INTEGER NOT NULL,
    last_activity_at INTEGER NOT NULL,
    terminal_at INTEGER,
    next_sequence INTEGER NOT NULL DEFAULT 1 CHECK (next_sequence > 0),
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    message_count INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
    body_bytes INTEGER NOT NULL DEFAULT 0 CHECK (body_bytes >= 0),
    FOREIGN KEY (project_low, project_high)
        REFERENCES pair_sequences(project_low, project_high),
    UNIQUE (project_low, project_high, pair_ordinal),
    CHECK ((state IN ('closed','expired')) = (terminal_at IS NOT NULL))
);
CREATE TABLE call_requests (
    initiator_project_id TEXT NOT NULL REFERENCES projects(id),
    request_key TEXT NOT NULL,
    payload_digest TEXT NOT NULL,
    conversation_id TEXT REFERENCES conversations(id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (initiator_project_id, request_key)
);
CREATE TABLE participants (
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id),
    alias_snapshot TEXT NOT NULL,
    joined_at INTEGER,
    PRIMARY KEY (conversation_id, project_id)
);
CREATE TABLE invitation_code_tombstones (
    code_digest TEXT PRIMARY KEY NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE invitations (
    conversation_id TEXT PRIMARY KEY NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    code TEXT NOT NULL UNIQUE,
    code_digest TEXT NOT NULL UNIQUE REFERENCES invitation_code_tombstones(code_digest),
    expected_project_id TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    accepted_at INTEGER,
    FOREIGN KEY (conversation_id, expected_project_id)
        REFERENCES participants(conversation_id, project_id)
);
CREATE TABLE messages (
    id TEXT PRIMARY KEY NOT NULL,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    sender_project_id TEXT,
    sender_alias TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('message','status','system')),
    body TEXT NOT NULL,
    reply_to TEXT,
    correlation_id TEXT,
    idempotency_key TEXT,
    payload_digest TEXT NOT NULL,
    UNIQUE (conversation_id, sequence),
    UNIQUE (conversation_id, id),
    UNIQUE (conversation_id, sender_project_id, idempotency_key),
    FOREIGN KEY (conversation_id, sender_project_id)
        REFERENCES participants(conversation_id, project_id),
    FOREIGN KEY (conversation_id, reply_to)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    CHECK ((kind = 'system') = (sender_project_id IS NULL))
);
CREATE TABLE message_recipients (
    conversation_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    PRIMARY KEY (message_id, project_id),
    FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, project_id)
        REFERENCES participants(conversation_id, project_id) ON DELETE CASCADE
);
CREATE TABLE consumer_cursors (
    conversation_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    consumer_id TEXT NOT NULL,
    acked_sequence INTEGER NOT NULL DEFAULT 0 CHECK (acked_sequence >= 0),
    delivered_sequence INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, project_id, consumer_id),
    FOREIGN KEY (conversation_id, project_id)
        REFERENCES participants(conversation_id, project_id) ON DELETE CASCADE,
    CHECK (delivered_sequence >= acked_sequence)
);
CREATE TABLE bridge_bindings (
    id TEXT PRIMARY KEY NOT NULL,
    conversation_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    consumer_id TEXT NOT NULL,
    thread_id TEXT UNIQUE,
    turn_id TEXT,
    input_message_id TEXT,
    result_message_id TEXT,
    state TEXT NOT NULL CHECK (state IN
        ('waiting','invoking','approval_required','result_recorded','acked',
         'stopped','interrupted','failed','capacity','uncertain')),
    limits_json TEXT NOT NULL CHECK (json_valid(limits_json)),
    usage_tokens INTEGER CHECK (usage_tokens >= 0),
    owner_token TEXT,
    lease_until INTEGER,
    updated_at INTEGER NOT NULL,
    error_code TEXT,
    stop_requested_at INTEGER,
    UNIQUE (conversation_id, project_id),
    FOREIGN KEY (conversation_id, project_id, consumer_id)
        REFERENCES consumer_cursors(conversation_id, project_id, consumer_id)
        ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, input_message_id)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, result_message_id)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    CHECK ((owner_token IS NULL) = (lease_until IS NULL))
);
CREATE TABLE prune_previews (
    token_digest TEXT PRIMARY KEY NOT NULL,
    selection_digest TEXT NOT NULL,
    policy_digest TEXT NOT NULL,
    eligibility_cutoff INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE prune_candidates (
    token_digest TEXT NOT NULL REFERENCES prune_previews(token_digest) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    terminal_at INTEGER NOT NULL,
    PRIMARY KEY (token_digest, conversation_id)
);
CREATE INDEX participant_project ON participants(project_id, conversation_id);
CREATE INDEX conversation_state_activity ON conversations(state, last_activity_at, id);
CREATE INDEX conversation_terminal ON conversations(terminal_at, id);
CREATE INDEX conversation_created ON conversations(created_at, id);
CREATE INDEX invitation_expiry ON invitations(expires_at, conversation_id);
CREATE INDEX recipient_inbox ON message_recipients(project_id, conversation_id, message_id);
CREATE INDEX bridge_recovery ON bridge_bindings(state, lease_until);
CREATE INDEX preview_expiry ON prune_previews(expires_at);
