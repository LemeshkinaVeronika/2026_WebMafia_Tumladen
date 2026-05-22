UPDATE guest_sessions
SET expires_at = created_at + INTERVAL '24 hours'
WHERE expires_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_guest_sessions_expires_at
    ON guest_sessions(expires_at)
    WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_room_participants_actor_type_actor_id
    ON room_participants(actor_type, actor_id);
