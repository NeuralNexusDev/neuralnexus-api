BEGIN;

INSERT INTO roles (id, name, description) VALUES
    (1, 'system', 'System'),
    (2, 'owner', 'Owner'),
    (3, 'bee_admin', 'Bee Name Generator Admin')
ON CONFLICT (name) DO NOTHING;

INSERT INTO permissions (id, scope_name, scope_value) VALUES
    (1, 'beenamegenerator', '*'),
    (2, 'petpictures', '*'),
    (3, 'ratelimit', '1000'),
    (4, 'datastore', '*'),
    (5, 'numberstore', '*'),
    (6, 'users', '*'),
    (7, 'roles', '*')
ON CONFLICT (scope_name, scope_value) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON
    (r.name IN ('system', 'owner') AND (p.scope_name, p.scope_value) IN (('beenamegenerator', '*'), ('petpictures', '*'), ('ratelimit', '1000'), ('datastore', '*'), ('numberstore', '*'), ('users', '*'), ('roles', '*')))
    OR (r.name = 'bee_admin' AND (p.scope_name, p.scope_value) = ('beenamegenerator', '*'))
ON CONFLICT DO NOTHING;

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS role_ids BIGINT[] NOT NULL DEFAULT '{}';

UPDATE accounts a SET role_ids = COALESCE((SELECT array_agg(r.id) FROM roles r WHERE r.name = ANY(a.roles)), '{}');

ALTER TABLE accounts DROP COLUMN roles;

DELETE FROM sessions;

COMMIT;
