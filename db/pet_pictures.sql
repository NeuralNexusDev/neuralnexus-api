CREATE SEQUENCE IF NOT EXISTS pets_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1;

CREATE TABLE IF NOT EXISTS pets (
    id integer DEFAULT nextval('pets_id_seq') PRIMARY KEY NOT NULL,
    name text NOT NULL,
    profile_picture text,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT name_check UNIQUE (name),
    CONSTRAINT pets_name_not_empty CHECK (name <> '')
);

CREATE TABLE IF NOT EXISTS pictures (
    id text PRIMARY KEY NOT NULL,
    file_ext text NOT NULL,
    subjects integer[] NOT NULL,
    aliases text[],
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT subjects_check CHECK (subjects <> '{}'::integer[])
);
