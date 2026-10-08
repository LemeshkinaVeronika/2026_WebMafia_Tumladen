CREATE TABLE IF NOT EXISTS match_activities (
    id UUID PRIMARY KEY,
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    state_version BIGINT NOT NULL CHECK (state_version >= 0),
    ordinal SMALLINT NOT NULL CHECK (ordinal >= 0),
    turn_number INTEGER NOT NULL CHECK (turn_number >= 0),
    actor_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (CHAR_LENGTH(type) BETWEEN 1 AND 64),
    payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (match_id, state_version, ordinal),
    FOREIGN KEY (match_id, actor_id)
        REFERENCES match_players(match_id, actor_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_match_activities_recent
    ON match_activities (match_id, state_version DESC, ordinal DESC);
