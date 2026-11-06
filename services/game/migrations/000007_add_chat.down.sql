ALTER TABLE reports DROP COLUMN IF EXISTS evidence;
ALTER TABLE reports DROP COLUMN IF EXISTS chat_message_id;
DROP TABLE IF EXISTS chat_mutes;
DROP TABLE IF EXISTS chat_messages;
