DROP INDEX IF EXISTS idx_chat_messages_room_id_created_at;
DROP INDEX IF EXISTS idx_matches_room_id;
DROP INDEX IF EXISTS idx_room_participants_room_id;

DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS match_players;
DROP TABLE IF EXISTS matches;
DROP TABLE IF EXISTS room_participants;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS guest_sessions;
