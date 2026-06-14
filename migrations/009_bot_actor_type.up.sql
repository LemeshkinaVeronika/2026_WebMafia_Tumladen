ALTER TABLE room_participants
    ALTER COLUMN actor_id TYPE TEXT USING actor_id::TEXT;

ALTER TABLE match_players
    ALTER COLUMN actor_id TYPE TEXT USING actor_id::TEXT;

ALTER TABLE match_players
    ADD COLUMN IF NOT EXISTS bot_difficulty TEXT;

ALTER TABLE matches
    ALTER COLUMN terminated_by_actor_id TYPE TEXT USING terminated_by_actor_id::TEXT;

ALTER TABLE room_participants
    DROP CONSTRAINT IF EXISTS room_participants_actor_type_check;

ALTER TABLE room_participants
    ADD CONSTRAINT room_participants_actor_type_check
    CHECK (actor_type IN ('guest', 'user', 'bot'));

ALTER TABLE match_players
    DROP CONSTRAINT IF EXISTS match_players_actor_type_check;

ALTER TABLE match_players
    ADD CONSTRAINT match_players_actor_type_check
    CHECK (actor_type IN ('guest', 'user', 'bot'));

ALTER TABLE match_players
    DROP CONSTRAINT IF EXISTS match_players_bot_difficulty_check;

ALTER TABLE match_players
    ADD CONSTRAINT match_players_bot_difficulty_check
    CHECK (
        (actor_type = 'bot' AND bot_difficulty IN ('easy', 'medium', 'hard'))
        OR (actor_type != 'bot' AND (bot_difficulty IS NULL OR bot_difficulty = ''))
    );
