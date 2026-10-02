-- Run docker/rbac.sql first: it creates the tables and seeds the built-in roles this converts to.
BEGIN;

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS role_ids BIGINT[] NOT NULL DEFAULT '{}';

UPDATE accounts a SET role_ids = COALESCE((SELECT array_agg(r.id) FROM roles r WHERE r.name = ANY(a.roles)), '{}');

ALTER TABLE accounts DROP COLUMN roles;

DELETE FROM sessions;

COMMIT;
