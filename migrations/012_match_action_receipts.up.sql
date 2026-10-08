CREATE TABLE IF NOT EXISTS match_action_receipts (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    actor_id TEXT NOT NULL,
    action_id TEXT NOT NULL CHECK (CHAR_LENGTH(action_id) BETWEEN 1 AND 128),
    request_hash CHAR(64) NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'rejected')),
    state_version BIGINT NOT NULL CHECK (state_version >= 0),
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, actor_id, action_id),
    CHECK (
        (status = 'accepted' AND error_code IS NULL AND error_message IS NULL)
        OR
        (status = 'rejected' AND error_code IS NOT NULL AND error_message IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_match_action_receipts_match_id
    ON match_action_receipts (match_id);
