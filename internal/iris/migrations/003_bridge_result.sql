ALTER TABLE bridge_bindings ADD COLUMN pending_result_json TEXT CHECK (pending_result_json IS NULL OR json_valid(pending_result_json));
