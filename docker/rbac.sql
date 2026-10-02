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
    CONSTRAINT permissions_scope_name_not_empty CHECK (scope_name <> '')
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);
