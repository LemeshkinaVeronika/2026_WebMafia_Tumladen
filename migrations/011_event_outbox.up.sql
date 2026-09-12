CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    topic TEXT NOT NULL,
    event_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_until TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON outbox_events (available_at, created_at)
    WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS processed_events (
    consumer_group TEXT NOT NULL,
    event_id UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (consumer_group, event_id)
);

CREATE TABLE IF NOT EXISTS user_game_stats (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_type TEXT NOT NULL,
    matches INT NOT NULL DEFAULT 0,
    wins INT NOT NULL DEFAULT 0,
    losses INT NOT NULL DEFAULT 0,
    draws INT NOT NULL DEFAULT 0,
    total_score BIGINT NOT NULL DEFAULT 0,
    best_score INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, game_type)
);

WITH finished_user_matches AS (
    SELECT
        mp.actor_id::UUID AS user_id,
        m.game_type,
        COALESCE((score.value->>'score')::INT, 0) AS score,
        COALESCE(m.result->'winners', '[]'::JSONB) ? mp.actor_id AS is_winner,
        JSONB_ARRAY_LENGTH(COALESCE(m.result->'winners', '[]'::JSONB)) AS winner_count
    FROM matches m
    JOIN match_players mp ON mp.match_id = m.id AND mp.actor_type = 'user'
    LEFT JOIN LATERAL JSONB_ARRAY_ELEMENTS(COALESCE(m.result->'finalScores', '[]'::JSONB)) score(value)
        ON score.value->>'actorId' = mp.actor_id
    WHERE m.status = 'finished'
      AND m.termination_reason = 'normal_completion'
      AND m.result IS NOT NULL
)
INSERT INTO user_game_stats (
    user_id, game_type, matches, wins, losses, draws, total_score, best_score, updated_at
)
SELECT
    user_id,
    game_type,
    COUNT(*)::INT,
    COUNT(*) FILTER (WHERE is_winner AND winner_count = 1)::INT,
    COUNT(*) FILTER (WHERE NOT is_winner)::INT,
    COUNT(*) FILTER (WHERE is_winner AND winner_count > 1)::INT,
    SUM(score),
    MAX(score),
    NOW()
FROM finished_user_matches
GROUP BY user_id, game_type
ON CONFLICT (user_id, game_type) DO NOTHING;
