-- +goose Up
CREATE TABLE identity.users (
    id uuid PRIMARY KEY,
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE identity.memberships (
    organization_id uuid NOT NULL REFERENCES identity.organizations(id),
    user_id uuid NOT NULL REFERENCES identity.users(id),
    role text NOT NULL CHECK (role IN ('OWNER')),
    PRIMARY KEY (organization_id, user_id)
);
CREATE TABLE identity.sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES identity.users(id),
    organization_id uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, user_id) REFERENCES identity.memberships(organization_id, user_id)
);
CREATE INDEX sessions_expiry ON identity.sessions(expires_at);

-- +goose Down
DROP TABLE identity.sessions;
DROP TABLE identity.memberships;
DROP TABLE identity.users;
