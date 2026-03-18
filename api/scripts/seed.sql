TRUNCATE TABLE "user_attribute_values" CASCADE;
TRUNCATE TABLE "credentials" CASCADE;
TRUNCATE TABLE "federated_identities" CASCADE;
TRUNCATE TABLE "permissions" CASCADE;
TRUNCATE TABLE "problem_tags" CASCADE;
TRUNCATE TABLE "test_cases" CASCADE;
TRUNCATE TABLE "problems" CASCADE;
TRUNCATE TABLE "tags" CASCADE;
TRUNCATE TABLE "users" CASCADE;
TRUNCATE TABLE "roles" CASCADE;
TRUNCATE TABLE "resources" CASCADE;
TRUNCATE TABLE "attribute_definitions" CASCADE;

-- Insert Roles
INSERT INTO "roles" (id, name, level, lft, rgt, parent_id, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'admin',   0, 1, 6, -1, NOW(), NOW()),
(2, 'teacher', 1, 2, 5,  1, NOW(), NOW()),
(3, 'student', 2, 3, 4,  2, NOW(), NOW());

-- Insert Resources
INSERT INTO "resources" (id, key, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'user', 'User management', NOW(), NOW()),
(2, 'role', 'Role management', NOW(), NOW()),
(3, 'permission', 'Permission management', NOW(), NOW()),
(4, 'resource', 'Resource management', NOW(), NOW()),
(5, 'attribute_definition', 'Attribute definition management', NOW(), NOW()),
(6, 'problem', 'Problem management', NOW(), NOW()),
(7, 'test_case', 'Test case management', NOW(), NOW()),
(8, 'tag', 'Tag management', NOW(), NOW()),
(9, 'generation', 'Link generation management', NOW(), NOW());

-- Insert Permissions (Admin has full CRUD: 15)
INSERT INTO "permissions" (role_id, resource_id, description, scopes, created_at, updated_at) VALUES
(1, 1, 'Admin: Full user management', 15, NOW(), NOW()),
(1, 2, 'Admin: Full role management', 15, NOW(), NOW()),
(1, 3, 'Admin: Full permission management', 15, NOW(), NOW()),
(1, 4, 'Admin: Full resource management', 15, NOW(), NOW()),
(1, 5, 'Admin: Full attribute definition management', 15, NOW(), NOW()),
(1, 6, 'Admin: Full problem management', 15, NOW(), NOW()),
(1, 7, 'Admin: Full test case management', 15, NOW(), NOW()),
(1, 8, 'Admin: Full tag management', 15, NOW(), NOW()),
-- Teacher permissions
(2, 6, 'Teacher: Manage problems', 15, NOW(), NOW()),
(2, 7, 'Teacher: Manage test cases', 15, NOW(), NOW()),
(2, 1, 'Teacher: Read users', 2, NOW(), NOW());

-- Insert Attribute Definitions
INSERT INTO "attribute_definitions" (id, key, data_type, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'first_name', 'string', 'First name', NOW(), NOW()),
(2, 'last_name', 'string', 'Last name', NOW(), NOW()),
(3, 'gender', 'int', 'Gender (0: Female, 1: Male, 2: Other)', NOW(), NOW()),
(4, 'birthday', 'string', 'Date of birth', NOW(), NOW());

-- Insert Default Admin User (password: admin123)
-- Hash generated via bcrypt
INSERT INTO "users" (id, username, role_id, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'admin', 1, NOW(), NOW());

INSERT INTO "credentials" (user_id, type, credential_data, created_at, updated_at) VALUES
(1, 'password', '{"hash": "$2y$12$/BYlRUwtO/rPeAhWaS5jLeYPhXNpRSqbQHB.WBTxXRC6Ltd4ysDOq"}', NOW(), NOW());

-- Reset Sequences (For PostgreSQL identity columns)
SELECT setval(pg_get_serial_sequence('roles', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "roles";
SELECT setval(pg_get_serial_sequence('users', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "users";
SELECT setval(pg_get_serial_sequence('resources', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "resources";
SELECT setval(pg_get_serial_sequence('attribute_definitions', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "attribute_definitions";
