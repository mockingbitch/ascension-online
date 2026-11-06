-- Phase 1 world-story foundation (docs/design/data-model-the-gioi-chung.md §13):
-- realm ladder v2 (milli points, auto minor stages), single credit path with daily caps, catch-up, late-join grant,
-- tribulation audit, world clock + per-day totals (server median).
--
-- ⚠️ PRE-LAUNCH ONLY: character progression and Tiên Duyên data are RESET. Old stage IDs (pham_nhan, phi_thang,
-- per-second rates) and the old 1 point = 1.000đ VIP scale cannot be migrated meaningfully. Users, Google identities
-- and refresh tokens are kept. Never run this on a launched server.

-- 1. Reset.
TRUNCATE characters CASCADE;   -- progressions, settlements, attempts, events, currencies (+ledger), inventory, tasks
TRUNCATE vip_point_ledger, vip_accounts, vip_daily_gift_claims;
DELETE FROM outbox_events;

-- 2. Progression: milli points, idle-rate carry, late-join reserve.
ALTER TABLE character_progressions RENAME COLUMN cultivation TO cultivation_milli;
ALTER TABLE character_progressions DROP COLUMN remainder_milli;
ALTER TABLE character_progressions
    ADD COLUMN rate_carry          INTEGER NOT NULL DEFAULT 0 CHECK (rate_carry BETWEEN 0 AND 3599),
    ADD COLUMN grant_reserve_milli BIGINT  NOT NULL DEFAULT 0 CHECK (grant_reserve_milli >= 0);

-- 3. Settlement log: time accounting + what was credited.
ALTER TABLE cultivation_settlements RENAME COLUMN rate_milli TO rate_milli_per_hour;
ALTER TABLE cultivation_settlements RENAME COLUMN earned TO credited_milli;
ALTER TABLE cultivation_settlements RENAME COLUMN cultivation_after TO cultivation_after_milli;
ALTER TABLE cultivation_settlements
    ADD COLUMN raw_milli     BIGINT  NOT NULL CHECK (raw_milli >= 0),
    ADD COLUMN multiplier_bp INTEGER NOT NULL CHECK (multiplier_bp BETWEEN 10000 AND 50000),
    ADD COLUMN game_day      DATE    NOT NULL;
COMMENT ON COLUMN cultivation_settlements.stage_id IS
    'Stage AFTER the settlement (auto-advance included); pairs with cultivation_after_milli.';

-- 4. Tribulation audit.
ALTER TABLE breakthrough_attempts
    ADD COLUMN tribulation              BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN guaranteed               BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN cultivation_before_milli BIGINT  NOT NULL DEFAULT 0 CHECK (cultivation_before_milli >= 0),
    ADD COLUMN loss_milli               BIGINT  NOT NULL DEFAULT 0 CHECK (loss_milli >= 0),
    ADD COLUMN odds                     JSONB   NOT NULL DEFAULT '[]'::jsonb,   -- audit: [{source, bp}]
    ADD CONSTRAINT breakthrough_attempts_loss_chk CHECK (tribulation OR loss_milli = 0),
    ADD CONSTRAINT breakthrough_attempts_guaranteed_chk CHECK (NOT guaranteed OR (tribulation AND success));

-- 5. Contribution: per-day aggregate (hot), per-credit log, item uses, late-join grant.
CREATE TABLE daily_contribution (
    character_id         UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day             DATE        NOT NULL,
    idle_milli           BIGINT      NOT NULL DEFAULT 0 CHECK (idle_milli >= 0),
    activity_milli       BIGINT      NOT NULL DEFAULT 0 CHECK (activity_milli >= 0),
    community_milli      BIGINT      NOT NULL DEFAULT 0 CHECK (community_milli >= 0),
    points_milli         BIGINT GENERATED ALWAYS AS (idle_milli + activity_milli + community_milli) STORED,
    speed_item_milli     BIGINT      NOT NULL DEFAULT 0 CHECK (speed_item_milli >= 0),     -- subset of activity_milli
    catch_up_bonus_milli BIGINT      NOT NULL DEFAULT 0 CHECK (catch_up_bonus_milli >= 0), -- ×2 portion (excluded from world)
    faction_id           TEXT,                                                              -- Phase 2
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, game_day),
    CHECK (speed_item_milli <= activity_milli),
    CHECK (catch_up_bonus_milli <= idle_milli + activity_milli + community_milli)
);
CREATE INDEX daily_contribution_day_idx ON daily_contribution (game_day) WHERE points_milli > 0;
CREATE INDEX daily_contribution_faction_day_idx ON daily_contribution (faction_id, game_day) WHERE faction_id IS NOT NULL;

CREATE TABLE cultivation_credits (
    id                       BIGSERIAL PRIMARY KEY,
    character_id             UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day                 DATE        NOT NULL,
    source                   TEXT        NOT NULL CHECK (source IN ('settle','lecture','daily_task','ritual','fortune',
                                                     'trial','item','faction','protector','dual_cultivation','event',
                                                     'late_join_grant')),
    bucket                   TEXT        CHECK (bucket IN ('idle','activity','community')),  -- NULL only for grant
    stage_id                 TEXT        NOT NULL,   -- stage BEFORE the credit (see COMMENT)
    raw_milli                BIGINT      NOT NULL CHECK (raw_milli >= 0),
    multiplier_bp            INTEGER     NOT NULL CHECK (multiplier_bp BETWEEN 10000 AND 50000),
    credited_milli           BIGINT      NOT NULL CHECK (credited_milli >= 0),
    catch_up_bonus_milli     BIGINT      NOT NULL DEFAULT 0 CHECK (catch_up_bonus_milli >= 0),
    clipped_ceiling_milli    BIGINT      NOT NULL DEFAULT 0 CHECK (clipped_ceiling_milli >= 0),
    clipped_bottleneck_milli BIGINT      NOT NULL DEFAULT 0 CHECK (clipped_bottleneck_milli >= 0),
    ref_type                 TEXT,
    ref_id                   TEXT,
    idempotency_key          TEXT        NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key),
    CHECK ((source = 'late_join_grant') = (bucket IS NULL)),
    CHECK (catch_up_bonus_milli <= credited_milli)
);
COMMENT ON COLUMN cultivation_credits.stage_id IS
    'Stage BEFORE the credit was applied (the stage whose bottleneck room clipped it).';
CREATE INDEX cultivation_credits_char_day_idx ON cultivation_credits (character_id, game_day);

CREATE TABLE item_daily_uses (
    character_id UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE    NOT NULL,
    item_id      TEXT    NOT NULL,
    uses         INTEGER NOT NULL CHECK (uses >= 0),
    PRIMARY KEY (character_id, game_day, item_id)
);

-- At most one late-join grant per Google account, ever (survives character deletion).
CREATE TABLE late_join_grants (
    user_id               UUID        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    character_id          UUID        REFERENCES characters (id) ON DELETE SET NULL,
    server_day            INTEGER     NOT NULL CHECK (server_day >= 0),
    median_position_milli BIGINT      NOT NULL CHECK (median_position_milli >= 0),
    median_game_day       DATE        NOT NULL,
    grant_milli           BIGINT      NOT NULL CHECK (grant_milli > 0),
    granted_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 6. World clock (singleton) + per-day totals (worker recompute; frozen once closed).
CREATE TABLE world_state (
    id                    SMALLINT    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    started_at            TIMESTAMPTZ,            -- NULL = pre-launch: no catch-up, grant or milestones
    era_id                TEXT,
    last_milestone_id     TEXT,
    progress_milli        BIGINT      NOT NULL DEFAULT 0 CHECK (progress_milli >= 0), -- Σ closed per-capita
    closed_through_day    DATE,
    median_position_milli BIGINT      NOT NULL DEFAULT 0 CHECK (median_position_milli >= 0),
    median_game_day       DATE,
    paused_at             TIMESTAMPTZ,
    version               BIGINT      NOT NULL DEFAULT 0,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO world_state (id) VALUES (1);

CREATE TABLE world_daily_totals (
    game_day              DATE        PRIMARY KEY,
    credited_milli        BIGINT      NOT NULL CHECK (credited_milli >= 0),
    catch_up_bonus_milli  BIGINT      NOT NULL CHECK (catch_up_bonus_milli >= 0),
    countable_milli       BIGINT      NOT NULL CHECK (countable_milli >= 0),
    dau                   INTEGER     NOT NULL CHECK (dau >= 0),
    per_capita_milli      BIGINT      NOT NULL CHECK (per_capita_milli >= 0),
    median_position_milli BIGINT      CHECK (median_position_milli >= 0),      -- set at close
    computed_at           TIMESTAMPTZ NOT NULL,
    closed_at             TIMESTAMPTZ
);
