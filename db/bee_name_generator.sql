CREATE TABLE IF NOT EXISTS bee_name (
    name text PRIMARY KEY NOT NULL,
    CONSTRAINT bee_name_name_not_blank CHECK (name !~ '^\s*$')
);

CREATE TABLE IF NOT EXISTS bee_name_suggestion (
    name text PRIMARY KEY NOT NULL,
    CONSTRAINT bee_name_suggestion_name_not_blank CHECK (name !~ '^\s*$')
);
