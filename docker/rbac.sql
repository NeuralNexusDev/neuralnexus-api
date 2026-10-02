CREATE TABLE IF NOT EXISTS roles (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    CONSTRAINT roles_name_not_empty CHECK (name <> '')
);

CREATE TABLE IF NOT EXISTS permissions (
    id BIGINT PRIMARY KEY,
    scope_name TEXT NOT NULL,
    scope_value TEXT NOT NULL,
    CONSTRAINT permissions_scope_unique UNIQUE (scope_name, scope_value),
    CONSTRAINT permissions_scope_name_not_empty CHECK (scope_name <> ''),
    CONSTRAINT permissions_scope_name_no_colon CHECK (scope_name NOT LIKE '%:%')
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

INSERT INTO roles (id, name, description) VALUES
    (1, 'system', 'System'),
    (2, 'owner', 'Owner'),
    (3, 'bee_admin', 'Bee Name Generator Admin')
ON CONFLICT DO NOTHING;

INSERT INTO permissions (id, scope_name, scope_value) VALUES
    (1, 'beenamegenerator', '*'),
    (2, 'petpictures', '*'),
    (3, 'ratelimit', '1000'),
    (4, 'datastore', '*'),
    (5, 'numberstore', '*'),
    (6, 'users', '*'),
    (7, 'roles', '*')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON
    (r.name IN ('system', 'owner') AND (p.scope_name, p.scope_value) IN (('beenamegenerator', '*'), ('petpictures', '*'), ('ratelimit', '1000'), ('datastore', '*'), ('numberstore', '*'), ('users', '*'), ('roles', '*')))
    OR (r.name = 'bee_admin' AND (p.scope_name, p.scope_value) = ('beenamegenerator', '*'))
ON CONFLICT DO NOTHING;
