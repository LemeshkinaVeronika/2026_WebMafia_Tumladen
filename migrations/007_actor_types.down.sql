ALTER TABLE match_players
    DROP CONSTRAINT IF EXISTS match_players_actor_type_check;

ALTER TABLE match_players
    DROP COLUMN IF EXISTS actor_type;

ALTER TABLE room_participants
    DROP CONSTRAINT IF EXISTS room_participants_actor_type_check;

ALTER TABLE room_participants
    DROP COLUMN IF EXISTS actor_type;

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS rooms_owner_actor_type_check;

ALTER TABLE rooms
    DROP COLUMN IF EXISTS owner_actor_type;
