CREATE TABLE IF NOT EXISTS event_log (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    platform text NOT NULL,
    type text NOT NULL,
    status integer NOT NULL,
    payload jsonb,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now(),
    CONSTRAINT events_unique PRIMARY KEY (id)
);
