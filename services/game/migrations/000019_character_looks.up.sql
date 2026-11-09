-- Character looks (decisions 2026-11-09 "Ngoại hình…", GĐ-A): body, hair accessory, robe colour and meditation pose
-- from content/catalog/appearances.yaml. DISPLAY ONLY — nothing here is read by cultivation, breakthrough, ranking,
-- merit or Tiên Duyên. Catalogue slugs, no FK (the catalogue lives in YAML); the service validates path + ownership.
-- NULL = not chosen yet → the catalogue default of the character's current path (no backfill needed).
ALTER TABLE character_appearance
    ADD COLUMN body_id         TEXT CHECK (body_id ~ '^[a-z][a-z0-9_]*$'),
    ADD COLUMN hair_id         TEXT CHECK (hair_id ~ '^[a-z][a-z0-9_]*$'),
    ADD COLUMN robe_palette_id TEXT CHECK (robe_palette_id ~ '^[a-z][a-z0-9_]*$'),
    ADD COLUMN pose_id         TEXT CHECK (pose_id ~ '^[a-z][a-z0-9_]*$'),
    -- server time of the last in-game body change (24 h cooldown, appearances.yaml); NULL = next change is free
    -- (the onboarding pick does not start the cooldown).
    ADD COLUMN body_changed_at TIMESTAMPTZ;

-- Equip log: the look part of the loadout (look_set = the request carried a look; replay compares it too).
ALTER TABLE cosmetic_equips
    ADD COLUMN look_set        BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN body_id         TEXT,
    ADD COLUMN hair_id         TEXT,
    ADD COLUMN robe_palette_id TEXT,
    ADD COLUMN pose_id         TEXT;

-- Resolved look of the sender at send time (like name_fx): others' portrait cards opened from chat show it.
-- Snapshot only, never filtered or aggregated on.
ALTER TABLE chat_messages ADD COLUMN look JSONB;
