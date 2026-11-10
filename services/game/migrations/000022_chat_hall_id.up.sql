-- Tử Tiêu Cung chia điện (decisions 2026-11-10): hall chat (channel tu_tieu_cung) is per điện for delivery and
-- backfill. The điện number is stored with each hall message (NULL for every other channel and for hall messages
-- written before the split). Nullable, no default → metadata-only ADD COLUMN; the CHECK is added NOT VALID (no scan
-- under the ACCESS EXCLUSIVE lock) and validated separately (SHARE UPDATE EXCLUSIVE). Index: 000023 (CONCURRENTLY).
ALTER TABLE chat_messages ADD COLUMN hall_id INTEGER;
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_hall_id_chk CHECK (hall_id IS NULL OR hall_id >= 1) NOT VALID;
ALTER TABLE chat_messages VALIDATE CONSTRAINT chat_messages_hall_id_chk;
