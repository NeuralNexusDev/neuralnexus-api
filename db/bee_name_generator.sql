CREATE TABLE IF NOT EXISTS bee_name (
    name text NOT NULL,
    CONSTRAINT pkey_bee_name PRIMARY KEY (name),
    CONSTRAINT bee_name_name_not_blank CHECK (name !~ '^\s*$')
);

CREATE TABLE IF NOT EXISTS bee_name_suggestion (
    name text NOT NULL,
    CONSTRAINT pkey_bee_name_suggestion PRIMARY KEY (name),
    CONSTRAINT bee_name_suggestion_name_not_blank CHECK (name !~ '^\s*$')
);
