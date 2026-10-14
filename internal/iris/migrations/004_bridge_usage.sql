ALTER TABLE bridge_bindings ADD COLUMN usage_floor INTEGER NOT NULL DEFAULT 0 CHECK (usage_floor >= 0);
UPDATE bridge_bindings SET usage_floor=COALESCE(usage_tokens,0);
