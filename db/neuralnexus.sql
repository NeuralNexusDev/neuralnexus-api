CREATE TABLE IF NOT EXISTS accounts (
    user_id bigint PRIMARY KEY NOT NULL,
    username text UNIQUE,
    email text UNIQUE,
    hashed_secret bytea,
    salt bytea,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    role_ids bigint[] DEFAULT '{}' NOT NULL
);

CREATE TABLE IF NOT EXISTS account_settings (
    user_id bigint PRIMARY KEY NOT NULL REFERENCES accounts(user_id),
    password_auth boolean DEFAULT true NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS datastores (
    store_id bigint PRIMARY KEY NOT NULL,
    owner_id bigint NOT NULL REFERENCES accounts(user_id),
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS datastore_numbers (
    store_id bigint PRIMARY KEY NOT NULL REFERENCES datastores(store_id),
    user_id bigint NOT NULL REFERENCES accounts(user_id),
    value double precision NOT NULL,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS linked_accounts (
    user_id bigint NOT NULL REFERENCES accounts(user_id),
    platform text NOT NULL,
    platform_username text NOT NULL,
    platform_id text NOT NULL,
    data jsonb NOT NULL,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    verified boolean DEFAULT false NOT NULL,
    login_enabled boolean DEFAULT false NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS linked_accounts_unique ON linked_accounts USING btree (user_id, platform);
CREATE UNIQUE INDEX IF NOT EXISTS linked_accounts_platform_unique ON linked_accounts USING btree (platform, platform_id);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    user_id bigint NOT NULL REFERENCES accounts(user_id),
    platform text NOT NULL,
    access_token text NOT NULL,
    token_type text,
    refresh_token text,
    expiry bigint,
    expires_in bigint,
    scope text[],
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS oauth_tokens_unique ON oauth_tokens USING btree (user_id, platform);

CREATE TABLE IF NOT EXISTS sessions (
    session_id bigint PRIMARY KEY NOT NULL,
    user_id bigint NOT NULL REFERENCES accounts(user_id),
    permissions text[] NOT NULL,
    iat bigint NOT NULL,
    lua bigint NOT NULL,
    exp bigint NOT NULL
);

CREATE TABLE IF NOT EXISTS roles (
    id bigint PRIMARY KEY NOT NULL,
    name text NOT NULL UNIQUE,
    description text DEFAULT '' NOT NULL,
    CONSTRAINT roles_name_not_empty CHECK (name <> '')
);

CREATE TABLE IF NOT EXISTS permissions (
    id bigint PRIMARY KEY NOT NULL,
    node text NOT NULL UNIQUE,
    description text DEFAULT '' NOT NULL,
    value_type text,
    merge text,
    CONSTRAINT permissions_node_format CHECK (node ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$'),
    CONSTRAINT permissions_merge_matches_type CHECK (COALESCE(
        (value_type IS NULL AND merge IS NULL)
        OR (value_type = 'int' AND merge IN ('max', 'min'))
        OR (value_type = 'string' AND merge = 'first')
        OR (value_type = 'string_list' AND merge = 'union'),
        FALSE
    ))
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id bigint NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id bigint NOT NULL REFERENCES permissions(id),
    value jsonb,
    CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission_id)
);

INSERT INTO roles (id, name, description) VALUES
    (1, 'system', 'System'),
    (2, 'owner', 'Owner'),
    (3, 'bee_admin', 'Bee Name Generator Admin')
ON CONFLICT DO NOTHING;

INSERT INTO permissions (id, node, description, value_type, merge) VALUES
    (1, 'beenamegenerator.admin', 'Bee name generator', NULL, NULL),
    (2, 'petpictures.admin', 'Pet pictures', NULL, NULL),
    (3, 'ratelimit', 'Rate limit', 'int', 'max'),
    (4, 'datastore.admin', 'Data store', NULL, NULL),
    (5, 'numberstore.admin', 'Number store', NULL, NULL),
    (6, 'users.admin', 'Users', NULL, NULL),
    (7, 'roles.admin', 'Roles and permissions', NULL, NULL),
    (8, 'petpictures.pets', 'Pet pictures', 'string_list', 'union')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id, value)
SELECT r.id, p.id, CASE WHEN p.node = 'ratelimit' THEN '1000'::jsonb END FROM roles r JOIN permissions p ON
    (r.name IN ('system', 'owner') AND p.node IN ('beenamegenerator.admin', 'petpictures.admin', 'ratelimit', 'datastore.admin', 'numberstore.admin', 'users.admin', 'roles.admin'))
    OR (r.name = 'bee_admin' AND p.node = 'beenamegenerator.admin')
ON CONFLICT DO NOTHING;
