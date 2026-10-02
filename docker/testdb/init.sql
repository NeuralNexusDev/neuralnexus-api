CREATE TABLE IF NOT EXISTS textures (
    hash TEXT NOT NULL PRIMARY KEY,
    CONSTRAINT textures_hash_not_empty CHECK (hash <> '')
);

CREATE TABLE IF NOT EXISTS players (
    id UUID PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    legacy BOOLEAN,
    demo BOOLEAN,
    profile_actions JSONB,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS player_textures (
    player_id UUID REFERENCES players(id),
    skin TEXT REFERENCES textures(hash),
    model TEXT,
    cape TEXT REFERENCES textures(hash),
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    CONSTRAINT player_textures_skin_not_empty CHECK (skin IS NULL OR skin <> ''),
    CONSTRAINT player_textures_cape_not_empty CHECK (cape IS NULL OR cape <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS player_textures_unique
    ON player_textures (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, ''));

CREATE TABLE IF NOT EXISTS player_names (
    player_id UUID REFERENCES players(id),
    name TEXT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    PRIMARY KEY (player_id, name)
);

CREATE TABLE IF NOT EXISTS geyser_players (
    xuid BIGINT PRIMARY KEY NOT NULL,
    gamertag TEXT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS geyser_player_textures (
    xuid BIGINT NOT NULL,
    hash TEXT NOT NULL,
    is_steve BOOLEAN NOT NULL,
    signature TEXT,
    texture_id TEXT NOT NULL,
    value TEXT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    CONSTRAINT geyser_player_textures_hash_not_empty CHECK (hash <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS geyser_player_textures_unique
    ON geyser_player_textures (xuid, hash);

CREATE TABLE IF NOT EXISTS bee_name (
    name TEXT PRIMARY KEY NOT NULL CHECK (name !~ '^\s*$')
);

CREATE TABLE IF NOT EXISTS bee_name_suggestion (
    name TEXT PRIMARY KEY NOT NULL CHECK (name !~ '^\s*$')
);

CREATE TABLE IF NOT EXISTS roles (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    CONSTRAINT roles_name_not_empty CHECK (name <> '')
);

CREATE TABLE IF NOT EXISTS permissions (
    id BIGINT PRIMARY KEY,
    node TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    value_type TEXT,
    merge TEXT,
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
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id),
    value JSONB,
    PRIMARY KEY (role_id, permission_id)
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

CREATE TABLE IF NOT EXISTS accounts (
    user_id BIGINT PRIMARY KEY NOT NULL,
    username TEXT UNIQUE,
    email TEXT UNIQUE,
    hashed_secret BYTEA,
    salt BYTEA,
    role_ids BIGINT[] NOT NULL DEFAULT '{}',
    updated_at timestamp with time zone default current_timestamp
);

CREATE DATABASE pet_pictures;

\connect pet_pictures

CREATE TABLE IF NOT EXISTS pictures (
    id text not null primary key,
    file_ext text not null,
    prime_subj integer not null,
    othr_subj integer[],
    aliases text[],
    created_at timestamp with time zone default current_timestamp,
    CONSTRAINT id_check UNIQUE ( id )
);

CREATE TABLE IF NOT EXISTS pets (
    id serial not null primary key,
    name text not null,
    profile_picture text default null,
    created_at timestamp with time zone default current_timestamp,
    CONSTRAINT name_check UNIQUE ( name ),
    CONSTRAINT pets_name_not_empty CHECK ( name <> '' )
);
