ALTER TABLE call_sessions
    ADD COLUMN callee_seen_at TIMESTAMP NULL AFTER ended_at;
