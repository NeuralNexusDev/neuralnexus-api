CREATE TABLE IF NOT EXISTS textures (
    hash text NOT NULL,
    CONSTRAINT textures_pkey PRIMARY KEY (hash),
    CONSTRAINT textures_hash_not_empty CHECK (hash <> '')
);

CREATE TABLE IF NOT EXISTS players (
    id uuid NOT NULL,
    name text NOT NULL,
    legacy boolean,
    demo boolean,
    profile_actions jsonb,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT players_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS player_names (
    player_id uuid NOT NULL REFERENCES players(id),
    name text NOT NULL,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT player_names_pkey PRIMARY KEY (player_id, name)
);

CREATE TABLE IF NOT EXISTS player_textures (
    player_id uuid REFERENCES players(id),
    skin text REFERENCES textures(hash),
    model text,
    cape text REFERENCES textures(hash),
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT player_textures_skin_not_empty CHECK (skin IS NULL OR skin <> ''),
    CONSTRAINT player_textures_cape_not_empty CHECK (cape IS NULL OR cape <> '')
);
CREATE UNIQUE INDEX IF NOT EXISTS player_textures_unique ON player_textures USING btree (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, ''));

CREATE TABLE IF NOT EXISTS geyser_players (
    id uuid NOT NULL,
    xuid bigint NOT NULL,
    gamertag text NOT NULL,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT geyser_players_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS geyser_player_names (
    player_id uuid NOT NULL REFERENCES geyser_players(id),
    name text NOT NULL,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT geyser_player_names_pkey PRIMARY KEY (player_id, name)
);

CREATE TABLE IF NOT EXISTS geyser_player_textures (
    player_id uuid REFERENCES geyser_players(id),
    -- A Geyser profile is built from a donor Java account that uploaded the skin.
    skin_id uuid REFERENCES players(id),
    skin text REFERENCES textures(hash),
    model text,
    cape text REFERENCES textures(hash),
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT geyser_player_skin_hash_not_empty CHECK (skin IS NULL OR skin <> ''),
    CONSTRAINT geyser_player_cape_hash_not_empty CHECK (cape IS NULL OR cape <> '')
);
CREATE UNIQUE INDEX IF NOT EXISTS geyser_player_textures_unique ON geyser_player_textures USING btree (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, ''));
