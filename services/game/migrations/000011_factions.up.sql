-- Phase 2.2 factions (docs/design/data-model-the-gioi-chung.md §6.2) + what executing the world's faction effects
-- needs: world_effects.noticed_at (notice announced once; a late notice reschedules due_at/deadline_at so an outage
-- never shortens the notice or the choice window) and world_state.era_started_at (faction merit counts within the
-- current era, decision H). Split defaults are derived from faction_history rows of the effect (no extra column).

ALTER TABLE world_state ADD COLUMN era_started_at TIMESTAMPTZ;
-- Pre-launch data only: approximate the start of the current era by the last milestone (or the launch).
UPDATE world_state
SET era_started_at = COALESCE((SELECT max(reached_at) FROM world_milestones_reached), started_at)
WHERE era_id IS NOT NULL;

ALTER TABLE world_effects ADD COLUMN noticed_at TIMESTAMPTZ;  -- split/migrate: notice announced (faction.*_notice)

-- F1: at most one faction per character (PK). faction_id is a catalogue slug (no FK).
CREATE TABLE character_faction (
    character_id            UUID        PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    faction_id              TEXT        NOT NULL,
    joined_at               TIMESTAMPTZ NOT NULL,             -- joined the CURRENT faction
    last_switch_at          TIMESTAMPTZ,                      -- last PLAYER-initiated switch (cooldown anchor, F4)
    lock_reason             TEXT        CHECK (lock_reason IN ('fengshen', 'admin', 'event')),
    locked_at               TIMESTAMPTZ,
    locked_until            TIMESTAMPTZ,                      -- NULL with a lock = permanent
    pending_split_effect_id BIGINT      REFERENCES world_effects (id),  -- must choose a branch of this split
    version                 BIGINT      NOT NULL DEFAULT 0,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((lock_reason IS NULL) = (locked_at IS NULL)),
    CHECK (locked_until IS NULL OR lock_reason IS NOT NULL)
);
CREATE INDEX character_faction_faction_idx ON character_faction (faction_id, character_id);
CREATE INDEX character_faction_pending_idx ON character_faction (pending_split_effect_id, character_id)
    WHERE pending_split_effect_id IS NOT NULL;

-- F6: one row per change, same tx. Player changes carry the request's Idempotency-Key; world moves use
-- 'effect:<world_effects.id>' (a colon never appears in player keys).
CREATE TABLE faction_history (
    id              BIGSERIAL   PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    from_faction_id TEXT,
    to_faction_id   TEXT        NOT NULL,
    from_path       TEXT,       -- set when the change converted the path (F3)
    to_path         TEXT,
    reason          TEXT        NOT NULL CHECK (reason IN ('join', 'switch', 'split_choice', 'split_default',
                                                           'auto_migrate', 'reincarnation', 'admin')),
    world_effect_id BIGINT      REFERENCES world_effects (id),
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key),
    CHECK ((from_path IS NULL) = (to_path IS NULL))
);
CREATE INDEX faction_history_char_idx ON faction_history (character_id, created_at DESC);
CREATE INDEX faction_history_effect_idx ON faction_history (world_effect_id, to_faction_id) WHERE world_effect_id IS NOT NULL;
