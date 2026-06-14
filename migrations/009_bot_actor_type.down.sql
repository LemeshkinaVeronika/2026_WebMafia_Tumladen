ALTER TABLE match_players
    DROP CONSTRAINT IF EXISTS match_players_actor_type_check;

ALTER TABLE match_players
    ADD CONSTRAINT match_players_actor_type_check
    CHECK (actor_type IN ('guest', 'user'));

ALTER TABLE match_players
    DROP CONSTRAINT IF EXISTS match_players_bot_difficulty_check;

ALTER TABLE match_players
    DROP COLUMN IF EXISTS bot_difficulty;

ALTER TABLE room_participants
    DROP CONSTRAINT IF EXISTS room_participants_actor_type_check;

ALTER TABLE room_participants
    ADD CONSTRAINT room_participants_actor_type_check
    CHECK (actor_type IN ('guest', 'user'));

ALTER TABLE matches
    ALTER COLUMN terminated_by_actor_id TYPE UUID USING terminated_by_actor_id::UUID;

ALTER TABLE match_players
    ALTER COLUMN actor_id TYPE UUID USING actor_id::UUID;

ALTER TABLE room_participants
    ALTER COLUMN actor_id TYPE UUID USING actor_id::UUID;
