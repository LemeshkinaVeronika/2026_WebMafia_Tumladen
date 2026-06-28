CREATE TABLE IF NOT EXISTS achievements (
    code TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    game_type TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_achievements (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_code TEXT NOT NULL REFERENCES achievements(code) ON DELETE CASCADE,
    match_id UUID REFERENCES matches(id) ON DELETE SET NULL,
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, achievement_code)
);

CREATE INDEX IF NOT EXISTS idx_user_achievements_user_id
    ON user_achievements(user_id);

CREATE INDEX IF NOT EXISTS idx_user_achievements_match_id
    ON user_achievements(match_id);

INSERT INTO achievements (code, title, description, game_type, sort_order)
VALUES
    ('first_game_any', 'Спасибо, что вы с нами', 'За первую завершённую игру в любом режиме', NULL, 10),
    ('carcassonne_first_game', 'Мелкий феодал', 'За первую игру в Fortresses & Roads', 'carcassonne', 20),
    ('carcassonne_first_win', 'Primus inter pares', 'За первую победу в Fortresses & Roads', 'carcassonne', 30)
ON CONFLICT (code) DO UPDATE
SET title = EXCLUDED.title,
    description = EXCLUDED.description,
    game_type = EXCLUDED.game_type,
    sort_order = EXCLUDED.sort_order;

WITH eligible_unlocks AS (
    SELECT
        u.id AS user_id,
        unlocks.achievement_code,
        m.id AS match_id,
        COALESCE(m.terminated_at, m.updated_at, m.created_at) AS unlocked_at
    FROM matches m
    JOIN match_players mp ON mp.match_id = m.id
    JOIN users u ON u.id::TEXT = mp.actor_id
    CROSS JOIN LATERAL (
        VALUES
            ('first_game_any', TRUE),
            ('carcassonne_first_game', m.game_type = 'carcassonne'),
            ('carcassonne_first_win', m.game_type = 'carcassonne' AND COALESCE(m.result->'winners', '[]'::JSONB) ? mp.actor_id)
    ) AS unlocks(achievement_code, is_eligible)
    WHERE mp.actor_type = 'user'
      AND m.status = 'finished'
      AND m.termination_reason = 'normal_completion'
      AND unlocks.is_eligible
),
first_unlocks AS (
    SELECT DISTINCT ON (user_id, achievement_code)
        user_id,
        achievement_code,
        match_id,
        unlocked_at
    FROM eligible_unlocks
    ORDER BY user_id, achievement_code, unlocked_at ASC, match_id ASC
)
INSERT INTO user_achievements (user_id, achievement_code, match_id, unlocked_at)
SELECT user_id, achievement_code, match_id, unlocked_at
FROM first_unlocks
ON CONFLICT (user_id, achievement_code) DO NOTHING;
