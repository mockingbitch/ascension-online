-- Phase 2.1 world milestones (docs/design/data-model-the-gioi-chung.md §5.4). world_state / world_daily_totals exist
-- since 000005; this adds what the reach rule records, the effects it schedules, ranged events and admin overrides.

-- W1/W2: each milestone at most once, in catalogue order (seq = catalogue index).
CREATE TABLE world_milestones_reached (
    milestone_id    TEXT        PRIMARY KEY,
    seq             INTEGER     NOT NULL UNIQUE CHECK (seq >= 0),
    reached_at      TIMESTAMPTZ NOT NULL,
    reason          TEXT        NOT NULL CHECK (reason IN ('threshold', 'max_time', 'admin')),
    server_day      INTEGER     NOT NULL CHECK (server_day >= 0),
    progress_milli  BIGINT      NOT NULL CHECK (progress_milli >= 0),
    threshold_milli BIGINT      NOT NULL CHECK (threshold_milli >= 0),  -- effective, incl. override
    target_day      INTEGER     NOT NULL CHECK (target_day >= 0),       -- effective, incl. override
    admin_user_id   UUID        REFERENCES users (id)                   -- the override's author when reason = admin
);

-- Every effect of a reached milestone (audit + schedule). set_era / start_event are applied in the reaching tx;
-- split_faction / migrate_faction stay pending (applied_at NULL) until Phase 2.2 (factions) executes them.
CREATE TABLE world_effects (
    id           BIGSERIAL   PRIMARY KEY,
    milestone_id TEXT        NOT NULL REFERENCES world_milestones_reached (milestone_id),
    effect_index SMALLINT    NOT NULL CHECK (effect_index >= 0),
    kind         TEXT        NOT NULL CHECK (kind IN ('set_era', 'start_event', 'split_faction', 'migrate_faction')),
    due_at       TIMESTAMPTZ NOT NULL,          -- reached_at + notice_days
    deadline_at  TIMESTAMPTZ,                   -- split_faction: due_at + choose_days
    applied_at   TIMESTAMPTZ,
    finalized_at TIMESTAMPTZ,                   -- split_faction: defaults assigned after the deadline
    snapshot     JSONB       NOT NULL,          -- effect params at schedule time (audit only)
    UNIQUE (milestone_id, effect_index)
);
CREATE INDEX world_effects_pending_idx ON world_effects (due_at) WHERE applied_at IS NULL OR finalized_at IS NULL;

-- Ranged events (start_event … duration_days). Ended once by the worker (ended_at), which then runs on_end.
CREATE TABLE world_events (
    event_id     TEXT        PRIMARY KEY,
    milestone_id TEXT        NOT NULL REFERENCES world_milestones_reached (milestone_id),
    started_at   TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL CHECK (ends_at > started_at),
    ended_at     TIMESTAMPTZ
);
CREATE INDEX world_events_open_idx ON world_events (ends_at) WHERE ended_at IS NULL;

-- Admin overrides (W9: every change also writes admin_audit_logs 'world.milestone_override' in the same tx).
CREATE TABLE world_milestone_overrides (
    milestone_id                TEXT        PRIMARY KEY,
    threshold_points_per_active BIGINT      CHECK (threshold_points_per_active >= 0),
    target_day                  INTEGER     CHECK (target_day >= 0),
    force_reach                 BOOLEAN     NOT NULL DEFAULT false,
    paused                      BOOLEAN     NOT NULL DEFAULT false,
    updated_by                  UUID        NOT NULL REFERENCES users (id),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
