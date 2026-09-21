-- Schema for the Minecraft module's player/texture tables, mirroring
-- production DDL. Applied automatically by Postgres on first boot of the
-- test-env container (see docker-compose.test.yml).

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
    -- Empty-string hashes are never valid: an absent skin/cape must be
    -- represented as NULL, not "". Enforced here so no write path (this
    -- app, a future one, a manual backfill) can smuggle one in.
    CONSTRAINT player_textures_skin_not_empty CHECK (skin IS NULL OR skin <> ''),
    CONSTRAINT player_textures_cape_not_empty CHECK (cape IS NULL OR cape <> '')
);

-- Not promotable to a table CONSTRAINT: it's built on COALESCE(...)
-- expressions (so a NULL model/cape collides with an empty one for dedup
-- purposes), and Postgres only allows ADD CONSTRAINT ... USING INDEX for
-- plain-column indexes. store.go's ON CONFLICT targets these same
-- expressions directly instead of a constraint name.
CREATE UNIQUE INDEX IF NOT EXISTS player_textures_unique
    ON player_textures (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, ''));

CREATE TABLE IF NOT EXISTS player_names (
    player_id UUID REFERENCES players(id),
    name TEXT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    PRIMARY KEY (player_id, name)
);

-- Bedrock players' gamertag<->XUID mapping. Keyed by xuid (stable across
-- gamertag changes); the synthetic UUID is derived at read time, never stored.
CREATE TABLE IF NOT EXISTS geyser_players (
    xuid BIGINT PRIMARY KEY NOT NULL,
    gamertag TEXT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL
);

-- Bedrock players' converted skins, half-mirroring player_textures. No FK to
-- geyser_players(xuid): a xuid can reach this table without ever going
-- through the gamertag->xuid lookup first.
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
