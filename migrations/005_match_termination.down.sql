ALTER TABLE match_players
    DROP COLUMN IF EXISTS disconnected_at;

ALTER TABLE matches
    DROP COLUMN IF EXISTS terminated_at,
    DROP COLUMN IF EXISTS terminated_by_actor_id,
    DROP COLUMN IF EXISTS termination_reason;
