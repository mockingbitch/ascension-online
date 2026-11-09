ALTER TABLE chat_messages DROP COLUMN look;
ALTER TABLE cosmetic_equips
    DROP COLUMN pose_id,
    DROP COLUMN robe_palette_id,
    DROP COLUMN hair_id,
    DROP COLUMN body_id,
    DROP COLUMN look_set;
ALTER TABLE character_appearance
    DROP COLUMN body_changed_at,
    DROP COLUMN pose_id,
    DROP COLUMN robe_palette_id,
    DROP COLUMN hair_id,
    DROP COLUMN body_id;
