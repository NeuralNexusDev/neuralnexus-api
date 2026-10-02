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
    scope_name TEXT NOT NULL,
    scope_value TEXT NOT NULL,
    CONSTRAINT permissions_scope_unique UNIQUE (scope_name, scope_value),
    CONSTRAINT permissions_scope_name_not_empty CHECK (scope_name <> ''),
    CONSTRAINT permissions_scope_name_no_colon CHECK (scope_name NOT LIKE '%:%')
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

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
