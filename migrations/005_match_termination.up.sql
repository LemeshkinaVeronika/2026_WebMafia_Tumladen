ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS termination_reason TEXT,
    ADD COLUMN IF NOT EXISTS terminated_by_actor_id UUID,
    ADD COLUMN IF NOT EXISTS terminated_at TIMESTAMPTZ;

ALTER TABLE match_players
    ADD COLUMN IF NOT EXISTS disconnected_at TIMESTAMPTZ;

UPDATE matches
SET termination_reason = CASE
        WHEN status = 'finished' THEN 'normal_completion'
        WHEN status = 'abandoned' THEN COALESCE(result->>'reason', 'reconnect_timeout')
        ELSE termination_reason
    END,
    terminated_at = COALESCE(terminated_at, updated_at)
WHERE status IN ('finished', 'abandoned')
  AND (termination_reason IS NULL OR terminated_at IS NULL);
