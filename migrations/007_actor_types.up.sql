ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS owner_actor_type TEXT NOT NULL DEFAULT 'guest';

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS rooms_owner_actor_type_check;

ALTER TABLE rooms
    ADD CONSTRAINT rooms_owner_actor_type_check
    CHECK (owner_actor_type IN ('guest', 'user'));

ALTER TABLE room_participants
    ADD COLUMN IF NOT EXISTS actor_type TEXT NOT NULL DEFAULT 'guest';

ALTER TABLE room_participants
    DROP CONSTRAINT IF EXISTS room_participants_actor_type_check;

ALTER TABLE room_participants
    ADD CONSTRAINT room_participants_actor_type_check
    CHECK (actor_type IN ('guest', 'user'));

ALTER TABLE match_players
    ADD COLUMN IF NOT EXISTS actor_type TEXT NOT NULL DEFAULT 'guest';

ALTER TABLE match_players
    DROP CONSTRAINT IF EXISTS match_players_actor_type_check;

ALTER TABLE match_players
    ADD CONSTRAINT match_players_actor_type_check
    CHECK (actor_type IN ('guest', 'user'));
