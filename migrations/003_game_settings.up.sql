ALTER TABLE rooms
    ADD COLUMN settings JSONB NOT NULL DEFAULT '{}'::jsonb;