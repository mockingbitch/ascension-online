-- Per-điện hall chat history: newest visible messages of one điện (ListChatHallMessages). Own migration, no
-- transaction, built concurrently (chat_messages keeps 90 days and may be large).
CREATE INDEX CONCURRENTLY IF NOT EXISTS chat_messages_hall_visible_idx ON chat_messages (channel, hall_id, id DESC)
    WHERE hidden_at IS NULL AND hall_id IS NOT NULL;
