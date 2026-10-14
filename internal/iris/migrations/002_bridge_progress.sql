ALTER TABLE bridge_bindings ADD COLUMN turns_used INTEGER NOT NULL DEFAULT 0 CHECK (turns_used >= 0);
ALTER TABLE bridge_bindings ADD COLUMN started_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE bridge_bindings ADD COLUMN pending_request_id TEXT;
ALTER TABLE bridge_bindings ADD COLUMN pending_method TEXT;
