CREATE TABLE IF NOT EXISTS guest_sessions (
    actor_id UUID PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
    );

CREATE TABLE IF NOT EXISTS rooms (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    invite_code TEXT UNIQUE,
    owner_actor_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('waiting', 'playing', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE TABLE IF NOT EXISTS room_participants (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL,
    display_name TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, actor_id)
);

CREATE TABLE IF NOT EXISTS matches (
    id UUID PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    game_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'finished', 'abandoned')),
    game_state JSONB NOT NULL,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS match_players (
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL,
    display_name TEXT NOT NULL,
    seat INT NOT NULL,
    PRIMARY KEY (match_id, actor_id),
    UNIQUE (match_id, seat)
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL,
    display_name TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_room_participants_room_id
    ON room_participants(room_id);

CREATE INDEX IF NOT EXISTS idx_matches_room_id ON matches(room_id);

CREATE INDEX IF NOT EXISTS idx_chat_messages_room_id_created_at
    ON chat_messages(room_id, created_at);
