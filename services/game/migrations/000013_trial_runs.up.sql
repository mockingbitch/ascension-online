-- Phase 3.1/3.2: mini-game trial runs (decisions D6; docs/design/tam-ma-luat-choi.md; content/catalog/trials.yaml).
-- One row per server-issued seeded run. The client submits its input log once; the server replays it
-- deterministically and writes the verdict here. A "pass" row gates the major breakthrough of trials.yaml gate.tier
-- until pass_valid_until, and is consumed by the SUCCESSFUL breakthrough (kept through failed attempts).
-- Lock order (data-model §2.3): character_progressions → daily_contribution → trial_runs → vip_accounts → inventory.
CREATE TABLE trial_runs (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id     UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    kind             TEXT        NOT NULL,                        -- trials.yaml kind id (tam_ma)
    seed             BIGINT      NOT NULL CHECK (seed BETWEEN 1 AND 4294967295),
    params           JSONB       NOT NULL,                        -- rules snapshot at issue time (replay uses this)
    issue_key        TEXT        NOT NULL,                        -- Idempotency-Key of POST /trials/{kind}/start
    issued_at        TIMESTAMPTZ NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    submit_key       TEXT,                                        -- Idempotency-Key of the (single) submission
    submitted_at     TIMESTAMPTZ,                                 -- NULL when closed without a submission (abandoned)
    claim            JSONB,                                       -- audit: inputs + claimed end state as received
    result           JSONB,                                       -- audit: the server replay's outcome
    verdict          TEXT        CHECK (verdict IN ('pass', 'fail', 'rejected', 'expired')),
    reason           TEXT,
    verdict_at       TIMESTAMPTZ,
    cooldown_until   TIMESTAMPTZ,                                 -- non-pass verdicts: no new run before this
    pass_valid_until TIMESTAMPTZ,                                 -- pass only
    consumed_at      TIMESTAMPTZ,                                 -- pass used by a successful breakthrough
    consumed_ref     TEXT,                                        -- 'bt:<breakthrough idempotency key>'
    UNIQUE (character_id, issue_key),
    CHECK (expires_at > issued_at),
    CHECK ((verdict IS NULL) = (verdict_at IS NULL)),
    CHECK ((verdict IS NULL) = (reason IS NULL)),
    CHECK (submitted_at IS NULL OR verdict IS NOT NULL),
    CHECK ((submit_key IS NULL) = (submitted_at IS NULL)),
    CHECK ((verdict = 'pass') = (pass_valid_until IS NOT NULL)),
    CHECK (verdict IS NULL OR verdict = 'pass' OR cooldown_until IS NOT NULL),
    CHECK (consumed_at IS NULL OR verdict = 'pass'),
    CHECK ((consumed_at IS NULL) = (consumed_ref IS NULL))
);

-- At most one open (unjudged) run per character and kind.
CREATE UNIQUE INDEX trial_runs_open_uq ON trial_runs (character_id, kind) WHERE verdict IS NULL;
-- Valid-pass lookup (breakthrough preview/attempt, GET /trials).
CREATE INDEX trial_runs_pass_idx ON trial_runs (character_id, kind, pass_valid_until DESC)
    WHERE verdict = 'pass' AND consumed_at IS NULL;
-- Lose-cooldown lookup.
CREATE INDEX trial_runs_cooldown_idx ON trial_runs (character_id, kind, cooldown_until DESC)
    WHERE cooldown_until IS NOT NULL;
