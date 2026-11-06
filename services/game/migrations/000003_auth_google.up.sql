-- Google-only sign-in (OIDC). No passwords are stored.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD COLUMN display_name  TEXT;
ALTER TABLE users ADD COLUMN avatar_url    TEXT;
ALTER TABLE users ADD COLUMN last_login_at TIMESTAMPTZ;

-- External identities. The stable key is (provider, subject) — never the email, which can change.
CREATE TABLE user_identities (
    provider       TEXT        NOT NULL CHECK (provider IN ('google')),
    subject        TEXT        NOT NULL,
    user_id        UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email          TEXT        NOT NULL,
    email_verified BOOLEAN     NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX user_identities_user_idx ON user_identities (user_id);

-- Refresh-token rotation with reuse detection: every token belongs to a family (one login);
-- presenting an already-rotated token revokes the whole family.
ALTER TABLE refresh_tokens ADD COLUMN family_id    UUID        NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE refresh_tokens ADD COLUMN replaced_by  UUID        REFERENCES refresh_tokens (id);
ALTER TABLE refresh_tokens ADD COLUMN user_agent   TEXT;
ALTER TABLE refresh_tokens ADD COLUMN last_used_at TIMESTAMPTZ;
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
