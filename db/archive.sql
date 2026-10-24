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
    player_id uuid NOT NULL,
    name text NOT NULL,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT player_names_pkey PRIMARY KEY (player_id, name),
    CONSTRAINT player_names_player_id_fkey FOREIGN KEY (player_id) REFERENCES players(id)
);

CREATE TABLE IF NOT EXISTS player_textures (
    player_id uuid,
    skin text,
    model text,
    cape text,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT player_textures_player_id_fkey FOREIGN KEY (player_id) REFERENCES players(id),
    CONSTRAINT player_textures_skin_fkey FOREIGN KEY (skin) REFERENCES textures(hash),
    CONSTRAINT player_textures_cape_fkey FOREIGN KEY (cape) REFERENCES textures(hash),
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
    player_id uuid NOT NULL,
    name text NOT NULL,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT geyser_player_names_pkey PRIMARY KEY (player_id, name),
    CONSTRAINT geyser_player_names_player_id_fkey FOREIGN KEY (player_id) REFERENCES geyser_players(id)
);

CREATE TABLE IF NOT EXISTS geyser_player_textures (
    player_id uuid,
    skin_id uuid,
    skin text,
    model text,
    cape text,
    first_seen bigint NOT NULL,
    last_seen bigint NOT NULL,
    CONSTRAINT geyser_player_textures_player_id_fkey FOREIGN KEY (player_id) REFERENCES geyser_players(id),
    -- A Geyser profile is built from a donor Java account that uploaded the skin.
    CONSTRAINT geyser_player_textures_skin_id_fkey FOREIGN KEY (skin_id) REFERENCES players(id),
    CONSTRAINT geyser_player_textures_skin_fkey FOREIGN KEY (skin) REFERENCES textures(hash),
    CONSTRAINT geyser_player_textures_cape_fkey FOREIGN KEY (cape) REFERENCES textures(hash),
    CONSTRAINT geyser_player_skin_hash_not_empty CHECK (skin IS NULL OR skin <> ''),
    CONSTRAINT geyser_player_cape_hash_not_empty CHECK (cape IS NULL OR cape <> '')
);
CREATE UNIQUE INDEX IF NOT EXISTS geyser_player_textures_unique ON geyser_player_textures USING btree (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, ''));
