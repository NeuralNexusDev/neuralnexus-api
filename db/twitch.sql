CREATE TABLE IF NOT EXISTS eventsub_subscriptions (
    id uuid PRIMARY KEY NOT NULL,
    user_id text NOT NULL,
    status text,
    type text,
    version integer,
    created_at timestamptz,
    revoked_at timestamptz,
    cost integer
);
