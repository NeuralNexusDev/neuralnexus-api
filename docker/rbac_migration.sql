-- Run docker/rbac.sql first: it creates the tables and seeds the built-in roles this converts to.
BEGIN;

DO $$
DECLARE missing text;
BEGIN
    SELECT string_agg(DISTINCT n, ', ') INTO missing
    FROM accounts a, unnest(a.roles) AS n
    WHERE n IN ('system', 'owner', 'bee_admin') AND NOT EXISTS (SELECT 1 FROM roles r WHERE r.name = n);
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'built-in roles missing from roles (run docker/rbac.sql first): %', missing;
    END IF;
END $$;

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS role_ids BIGINT[] NOT NULL DEFAULT '{}';

UPDATE accounts a SET role_ids = COALESCE((SELECT array_agg(r.id) FROM roles r WHERE r.name = ANY(a.roles)), '{}');

ALTER TABLE accounts DROP COLUMN roles;

DELETE FROM sessions;

COMMIT;
