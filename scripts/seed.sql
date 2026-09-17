BEGIN;

-- Stable UUIDv7-shaped identifiers make development seeds reproducible while
-- keeping every Judgify domain foreign key on the production UUID contract.
CREATE OR REPLACE FUNCTION pg_temp.seed_uuid(namespace_id integer, item_id bigint)
RETURNS uuid
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
  SELECT (
    '019c1234-abcd-7' ||
    lpad(to_hex(namespace_id), 3, '0') ||
    '-8000-' ||
    lpad(to_hex(item_id), 12, '0')
  )::uuid
$$;

TRUNCATE TABLE "user_attribute_values" CASCADE;
TRUNCATE TABLE "credentials" CASCADE;
TRUNCATE TABLE "federated_identities" CASCADE;
-- audit_logs is a production-style append-only ledger even in development.
-- Re-seeding resets mutable fixtures but deliberately preserves its history.
TRUNCATE TABLE "authorization_states" CASCADE;
TRUNCATE TABLE "casbin_rules" CASCADE;
TRUNCATE TABLE "problem_tags" CASCADE;
TRUNCATE TABLE "test_cases" CASCADE;
TRUNCATE TABLE "problems" CASCADE;
TRUNCATE TABLE "tags" CASCADE;
TRUNCATE TABLE "users" CASCADE;
TRUNCATE TABLE "roles" CASCADE;
TRUNCATE TABLE "attribute_definitions" CASCADE;
TRUNCATE TABLE "elements" CASCADE;
TRUNCATE TABLE "levels" CASCADE;
TRUNCATE TABLE "ranks" CASCADE;
TRUNCATE TABLE "traits" CASCADE;
TRUNCATE TABLE "rarities" CASCADE;
TRUNCATE TABLE "difficulties" CASCADE;
TRUNCATE TABLE "tag_elements" CASCADE;

-- Insert Roles
INSERT INTO "roles" (id, key, name, description, is_system, created_at, updated_at) VALUES
(pg_temp.seed_uuid(1, 1), 'superadmin', 'Super administrator', 'Protected break-glass system administrator', true, NOW(), NOW()),
(pg_temp.seed_uuid(1, 2), 'teacher', 'Teacher', 'Problem author and reviewer', true, NOW(), NOW()),
(pg_temp.seed_uuid(1, 3), 'student', 'Student', 'Default learner role', true, NOW(), NOW());

-- Insert Attribute Definitions
INSERT INTO "attribute_definitions" (id, key, data_type, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(2, 1), 'first_name', 'string', 'First name', NOW(), NOW()),
(pg_temp.seed_uuid(2, 2), 'last_name', 'string', 'Last name', NOW(), NOW()),
(pg_temp.seed_uuid(2, 3), 'gender', 'int', 'Gender (0: Female, 1: Male, 2: Other)', NOW(), NOW()),
(pg_temp.seed_uuid(2, 4), 'birthday', 'string', 'Date of birth', NOW(), NOW());

-- Insert Default Admin User (password: admin123)
-- Hash generated via bcrypt
INSERT INTO "users" (id, username, created_at, updated_at) VALUES
(pg_temp.seed_uuid(3, 1), 'admin', NOW(), NOW());

INSERT INTO "credentials" (id, user_id, type, credential_data, created_at, updated_at) VALUES
(pg_temp.seed_uuid(4, 1), pg_temp.seed_uuid(3, 1), 'password', '{"hash": "$2y$12$/BYlRUwtO/rPeAhWaS5jLeYPhXNpRSqbQHB.WBTxXRC6Ltd4ysDOq"}', NOW(), NOW());

-- Insert Casbin RBAC policies.
-- p: role policy, g: user-role or role-role inheritance.
INSERT INTO "casbin_rules" (ptype, v0, v1, v2, v3, v4, v5) VALUES
('g', 'user:' || pg_temp.seed_uuid(3, 1)::text, 'role:superadmin', '', '', '', ''),
('g', 'role:superadmin', 'role:teacher', '', '', '', ''),
('g', 'role:teacher', 'role:student', '', '', '', ''),
('p', 'role:superadmin', '*', '*', '', '', ''),
('p', 'role:teacher', 'problem', 'create', '', '', ''),
('p', 'role:teacher', 'problem', 'read', '', '', ''),
('p', 'role:teacher', 'problem', 'update', '', '', ''),
('p', 'role:teacher', 'problem', 'delete', '', '', ''),
('p', 'role:teacher', 'test_case', 'create', '', '', ''),
('p', 'role:teacher', 'test_case', 'read', '', '', ''),
('p', 'role:teacher', 'test_case', 'update', '', '', ''),
('p', 'role:teacher', 'test_case', 'delete', '', '', ''),
('p', 'role:teacher', 'submission', 'read', '', '', ''),
('p', 'role:teacher', 'user', 'read', '', '', '');

-- Single-tenant desired policy fence. Revision 1 names the complete seed
-- snapshot above; every later mutation increments it transactionally.
INSERT INTO "authorization_states" (id, desired_revision, updated_by, created_at, updated_at) VALUES
('default', 1, pg_temp.seed_uuid(3, 1), NOW(), NOW());

-- Insert Elements
INSERT INTO "elements" (id, name, code, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(5, 1), 'Kim', 'metal', 'Thuộc tính Kim, sắc bén và cứng rắn.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 2), 'Mộc', 'wood', 'Thuộc tính Mộc, sinh trưởng và dẻo dai.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 3), 'Thủy', 'water', 'Thuộc tính Thủy, mềm mại và uyển chuyển.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 4), 'Hỏa', 'fire', 'Thuộc tính Hỏa, bùng nổ và thiêu rụi.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 5), 'Thổ', 'earth', 'Thuộc tính Thổ, vững chắc và bao dung.', NOW(), NOW());

-- Insert Difficulties
INSERT INTO "difficulties" (id, name, level, exp_reward, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(6, 1), 'Dễ', 1, 10, 'Bài tập cơ bản, phù hợp cho người mới bắt đầu.', NOW(), NOW()),
(pg_temp.seed_uuid(6, 2), 'Trung Bình', 2, 25, 'Bài tập yêu cầu tư duy thuật toán vừa phải.', NOW(), NOW()),
(pg_temp.seed_uuid(6, 3), 'Khó', 3, 50, 'Bài tập nâng cao, đòi hỏi kiến thức sâu rộng.', NOW(), NOW());

-- Insert Levels
INSERT INTO "levels" (id, name, min_exp, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(7, 1), 'Phàm Nhân', 0, 'Người bình thường, chưa bước vào con đường tu luyện.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 2), 'Tán Tu', 1000, 'Tu sĩ tự do, không môn phái.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 3), 'Ngoại Môn Đệ Tử', 5000, 'Đệ tử cấp thấp của tông môn, làm tạp dịch.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 4), 'Nội Môn Đệ Tử', 20000, 'Đệ tử chính thức, được truyền thụ công pháp cơ bản.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 5), 'Hạch Tâm Đệ Tử', 100000, 'Thành phần nòng cốt, nhận được nhiều tài nguyên.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 6), 'Chân Truyền Đệ Tử', 500000, 'Đệ tử do trưởng lão hoặc chưởng môn trực tiếp chỉ dạy.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 7), 'Chấp Sự', 2000000, 'Quản lý các công việc thường ngày trong tông môn.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 8), 'Trưởng Lão', 10000000, 'Sở hữu quyền lực lớn, chỉ dạy đệ tử.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 9), 'Thái Thượng Trưởng Lão', 50000000, 'Những tồn tại cường đại, ẩn tu sau màn.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 10), 'Chưởng Môn', 100000000, 'Người đứng đầu một tông môn, quyền lực tối cao.', NOW(), NOW()),
(pg_temp.seed_uuid(7, 11), 'Vô Thượng Tiên Tôn', 500000000, 'Truyền thuyết tồn tại, vô địch thiên hạ.', NOW(), NOW());

-- Insert Ranks
INSERT INTO "ranks" (id, name, min_rating, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(8, 1), 'Luyện Khí Kỳ', 0, 'Cảnh giới sơ tuyển, hấp thu thiên địa linh khí vào cơ thể.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 2), 'Trúc Cơ Kỳ', 1200, 'Xây dựng đạo thai, tuổi thọ tăng lên đáng kể.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 3), 'Kết Đan Kỳ', 1400, 'Ngưng kết kim đan, có thể ngự kiếm phi hành.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 4), 'Nguyên Anh Kỳ', 1600, 'Phá đan thành anh, linh hồn bất diệt.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 5), 'Hóa Thần Kỳ', 1800, 'Cảm ngộ thiên địa quy tắc, nguyên thần ngoại xuất.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 6), 'Luyện Hư Kỳ', 2000, 'Hòa mình vào hư không, không gian dung hợp.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 7), 'Hợp Thể Kỳ', 2200, 'Nguyên anh và thể xác hoàn toàn hợp nhất.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 8), 'Đại Thừa Kỳ', 2400, 'Tiếp cận tiên đạo, sắp phi thăng tiên giới.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 9), 'Độ Kiếp Kỳ', 2600, 'Trải qua thiên kiếp để lột xác thành tiên.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 10), 'Bán Tiên', 2800, 'Chuyển hóa một nửa tiên thể, chuẩn bị phi thăng tiên giới.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 11), 'Địa Tiên', 3000, 'Phi thăng thành công, bước đầu làm quen với tiên khí.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 12), 'Thiên Tiên', 3200, 'Tích lũy tiên nguyên, pháp lực vô biên.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 13), 'Chân Tiên', 3400, 'Khấu triều đại đạo, nắm giữ quy tắc tiên giới.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 14), 'Huyền Tiên', 3600, 'Siêu thoát tam giới, không nằm trong ngũ hành.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 15), 'Đại La Kim Tiên', 3800, 'Vạn kiếp bất diệt, tồn tại vĩnh hằng.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 16), 'Tiên Tôn', 4000, 'Hùng bá một phương tiên giới, vạn tiên triều bái.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 17), 'Tiên Đế', 4200, 'Chúa tể tiên giới, cai quản càn khôn.', NOW(), NOW()),
(pg_temp.seed_uuid(8, 18), 'Đạo Tổ', 5000, 'Người dung hợp đại đạo, tồn tại tối thượng.', NOW(), NOW());

-- Insert Tags
INSERT INTO "tags" (id, name, created_at, updated_at) VALUES
(pg_temp.seed_uuid(9, 1), 'Array', NOW(), NOW()),
(pg_temp.seed_uuid(9, 2), 'String', NOW(), NOW()),
(pg_temp.seed_uuid(9, 3), 'Hash Map', NOW(), NOW()),
(pg_temp.seed_uuid(9, 4), 'Dynamic Programming', NOW(), NOW()),
(pg_temp.seed_uuid(9, 5), 'Math', NOW(), NOW()),
(pg_temp.seed_uuid(9, 6), 'Sorting', NOW(), NOW()),
(pg_temp.seed_uuid(9, 7), 'Greedy', NOW(), NOW()),
(pg_temp.seed_uuid(9, 8), 'Depth-First Search', NOW(), NOW()),
(pg_temp.seed_uuid(9, 9), 'Breadth-First Search', NOW(), NOW()),
(pg_temp.seed_uuid(9, 10), 'Tree', NOW(), NOW()),
(pg_temp.seed_uuid(9, 11), 'Binary Search', NOW(), NOW()),
(pg_temp.seed_uuid(9, 12), 'Two Pointers', NOW(), NOW()),
(pg_temp.seed_uuid(9, 13), 'Graph', NOW(), NOW()),
(pg_temp.seed_uuid(9, 14), 'Sliding Window', NOW(), NOW()),
(pg_temp.seed_uuid(9, 15), 'Linked List', NOW(), NOW()),
(pg_temp.seed_uuid(9, 16), 'Divide and Conquer', NOW(), NOW()),
(pg_temp.seed_uuid(9, 17), 'Dijkstra', NOW(), NOW()),
(pg_temp.seed_uuid(9, 18), 'Bit Manipulation', NOW(), NOW());

-- Insert Tag-Element Mapping (tag_id, element_id)
-- Kim (metal/logic): Math, Binary Search, Two Pointers, Sliding Window, Divide and Conquer
-- Mộc (wood/growth): Dynamic Programming, Tree, Greedy
-- Thủy (water/flow): String, Array, Sorting, Linked List
-- Hỏa (fire/explore): Depth-First Search, Breadth-First Search, Graph, Dijkstra
-- Thổ (earth/foundation): Hash Map, Bit Manipulation
INSERT INTO "tag_elements" (tag_id, element_id) VALUES
(pg_temp.seed_uuid(9, 5), pg_temp.seed_uuid(5, 1)),   -- Math → Kim
(pg_temp.seed_uuid(9, 11), pg_temp.seed_uuid(5, 1)),  -- Binary Search → Kim
(pg_temp.seed_uuid(9, 12), pg_temp.seed_uuid(5, 1)),  -- Two Pointers → Kim
(pg_temp.seed_uuid(9, 14), pg_temp.seed_uuid(5, 1)),  -- Sliding Window → Kim
(pg_temp.seed_uuid(9, 16), pg_temp.seed_uuid(5, 1)),  -- Divide and Conquer → Kim
(pg_temp.seed_uuid(9, 4), pg_temp.seed_uuid(5, 2)),   -- Dynamic Programming → Mộc
(pg_temp.seed_uuid(9, 10), pg_temp.seed_uuid(5, 2)),  -- Tree → Mộc
(pg_temp.seed_uuid(9, 7), pg_temp.seed_uuid(5, 2)),   -- Greedy → Mộc
(pg_temp.seed_uuid(9, 2), pg_temp.seed_uuid(5, 3)),   -- String → Thủy
(pg_temp.seed_uuid(9, 1), pg_temp.seed_uuid(5, 3)),   -- Array → Thủy
(pg_temp.seed_uuid(9, 6), pg_temp.seed_uuid(5, 3)),   -- Sorting → Thủy
(pg_temp.seed_uuid(9, 15), pg_temp.seed_uuid(5, 3)),  -- Linked List → Thủy
(pg_temp.seed_uuid(9, 8), pg_temp.seed_uuid(5, 4)),   -- Depth-First Search → Hỏa
(pg_temp.seed_uuid(9, 9), pg_temp.seed_uuid(5, 4)),   -- Breadth-First Search → Hỏa
(pg_temp.seed_uuid(9, 13), pg_temp.seed_uuid(5, 4)),  -- Graph → Hỏa
(pg_temp.seed_uuid(9, 17), pg_temp.seed_uuid(5, 4)),  -- Dijkstra → Hỏa
(pg_temp.seed_uuid(9, 3), pg_temp.seed_uuid(5, 5)),   -- Hash Map → Thổ
(pg_temp.seed_uuid(9, 18), pg_temp.seed_uuid(5, 5));  -- Bit Manipulation → Thổ

-- Insert Rarities (Phẩm chất)
INSERT INTO "rarities" (id, name, code, weight, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(10, 1), 'Phàm Phẩm', 'common', 75, 'Phẩm chất bèo bọt, nhan nhản mọi nơi.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 2), 'Nhân Phẩm', 'uncommon', 50, 'Dành cho nhân kiệt, có tư chất tu luyện.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 3), 'Địa Phẩm', 'rare', 35, 'Phẩm chất trung cấp, có linh khí đại địa.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 4), 'Thiên Phẩm', 'epic', 20, 'Phẩm chất cao cấp, ngưng tụ tinh hoa nhật nguyệt.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 5), 'Tiên Phẩm', 'legendary', 12, 'Phẩm chất tiên giới, thoát thai hoán cốt.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 6), 'Thần Phẩm', 'mythic', 5, 'Phẩm chất truyền thuyết, mang uy áp thần minh.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 7), 'Thánh Phẩm', 'transcendent', 2, 'Phẩm chất của thánh nhân, vạn thế vô nhất.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 8), 'Hỗn Độn Phẩm', 'origin', 1, 'Khởi nguyên vạn vật, cắn nuốt từ cõi vô hư.', NOW(), NOW());

INSERT INTO "traits" (id, type, name, rarity_id, description, metadata, created_at, updated_at) VALUES
-- Root Bones - Phàm Phẩm
(pg_temp.seed_uuid(11, 1), 'root_bone', 'Tạp Linh Căn', pg_temp.seed_uuid(10, 1), 'Linh căn hỗn tạp, tốc độ tu luyện cực chậm.', '{"exp_multiplier": 0.5, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 2), 'root_bone', 'Ngũ Hành Phế Căn', pg_temp.seed_uuid(10, 1), 'Năm luồng khí xung khắc, gần như không thể tu luyện.', '{"exp_multiplier": 0.3, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 3), 'root_bone', 'Tứ Linh Căn', pg_temp.seed_uuid(10, 1), 'Linh căn thiếu hụt, tư chất thấp kém.', '{"exp_multiplier": 0.8, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 4), 'root_bone', 'Sắt Gỉ Căn', pg_temp.seed_uuid(10, 1), 'Căn cốt rỉ sét, tư chất cực tệ.', '{"exp_multiplier": 0.9, "exp_bonus": 5, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 5), 'root_bone', 'Phế Trúc Căn', pg_temp.seed_uuid(10, 1), 'Cốt cách như tre mục, dễ gãy.', '{"exp_multiplier": 0.9, "exp_bonus": 5, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),

-- Root Bone - Nhân Phẩm
(pg_temp.seed_uuid(11, 6), 'root_bone', 'Tam Linh Căn', pg_temp.seed_uuid(10, 2), 'Linh căn trung bình, có thể miễn cưỡng bước vào con đường tu tiên.', '{"exp_multiplier": 1.0, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 7), 'root_bone', 'Chân Linh Căn', pg_temp.seed_uuid(10, 2), 'Linh căn cơ bản, cơ sở của đại đa số tu sĩ.', '{"exp_multiplier": 1.1, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 8), 'root_bone', 'Song Linh Căn', pg_temp.seed_uuid(10, 2), 'Linh căn khá tốt, được các môn phái nhỏ săn đón.', '{"exp_multiplier": 1.3, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 9), 'root_bone', 'Mộc Hỏa Tương Sinh Căn', pg_temp.seed_uuid(10, 2), 'Mộc sinh Hỏa, linh căn tương hỗ tu luyện.', '{"exp_multiplier": 1.4, "exp_bonus": 10, "target_elements": ["wood", "fire"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 10), 'root_bone', 'Ngạnh Cốt Căn', pg_temp.seed_uuid(10, 2), 'Cơ thể cứng cáp, gõ bàn phím không biết mỏi.', '{"exp_multiplier": 1.2, "exp_bonus": 10}'::jsonb, NOW(), NOW()),

-- Root Bone - Địa Phẩm
(pg_temp.seed_uuid(11, 11), 'root_bone', 'Thiên Linh Căn (Hỏa)', pg_temp.seed_uuid(10, 3), 'Chỉ có hệ Hỏa tinh thuần, tốc độ tu luyện cực độ.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 12), 'root_bone', 'Thiên Linh Căn (Thủy)', pg_temp.seed_uuid(10, 3), 'Hệ Thủy tinh thuần, ngộ tính hệt như dòng nước.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 13), 'root_bone', 'Thiên Linh Căn (Kim)', pg_temp.seed_uuid(10, 3), 'Hệ Kim tinh thuần, sát phạt quyết đoán.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 14), 'root_bone', 'Thiên Linh Căn (Mộc)', pg_temp.seed_uuid(10, 3), 'Hệ Mộc tinh thuần, sinh cơ bừng bừng vô tận.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 15), 'root_bone', 'Thiên Linh Căn (Thổ)', pg_temp.seed_uuid(10, 3), 'Hệ Thổ tinh thuần, căn cơ vững vàng như bàn thạch.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),

-- Root Bone - Thiên Phẩm
(pg_temp.seed_uuid(11, 16), 'root_bone', 'Dị Linh Căn (Băng)', pg_temp.seed_uuid(10, 4), 'Biến dị từ Thủy, cực hàn cực băng.', '{"exp_multiplier": 2.0, "exp_bonus": 30, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 17), 'root_bone', 'Dị Linh Căn (Lôi)', pg_temp.seed_uuid(10, 4), 'Biến dị cực hiếm, mang sức mạnh của thiên kiếp.', '{"exp_multiplier": 2.2, "exp_bonus": 30, "target_elements": ["metal", "fire"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 18), 'root_bone', 'Dị Linh Căn (Phong)', pg_temp.seed_uuid(10, 4), 'Biến dị từ Mộc, nhanh nhẹn như gió.', '{"exp_multiplier": 2.0, "exp_bonus": 30, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 19), 'root_bone', 'Xích Cốt Linh Căn', pg_temp.seed_uuid(10, 4), 'Xương tủy hóa đỏ, sức mạnh thuật toán vượt bậc.', '{"exp_multiplier": 2.1, "exp_bonus": 25, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),

-- Root Bone - Tiên Phẩm
(pg_temp.seed_uuid(11, 20), 'root_bone', 'Chân Long Bất Mạch', pg_temp.seed_uuid(10, 5), 'Dòng máu rồng vĩnh cửu chảy trong mã nguồn.', '{"exp_multiplier": 2.5, "exp_bonus": 80, "target_elements": ["water", "wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 21), 'root_bone', 'Kiếm Cốt Tiên Thai', pg_temp.seed_uuid(10, 5), 'Sinh ra từ bào thai tiên chứa vô tận kiếm ý.', '{"exp_multiplier": 2.6, "exp_bonus": 50, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),

-- Root Bone - Thần Phẩm
(pg_temp.seed_uuid(11, 22), 'root_bone', 'Hư Không Thần Căn', pg_temp.seed_uuid(10, 6), 'Căn cốt mờ ảo, không nằm trong ngũ hành.', '{"exp_multiplier": 3.0, "exp_bonus": 100}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 23), 'root_bone', 'Đế Tôn Long Mạch', pg_temp.seed_uuid(10, 6), 'Mạch máu của bậc chân đế trị vì muôn loài Coder.', '{"exp_multiplier": 3.2, "exp_bonus": 150}'::jsonb, NOW(), NOW()),

-- Root Bone - Thánh Phẩm
(pg_temp.seed_uuid(11, 24), 'root_bone', 'Hồng Mông Thánh Căn', pg_temp.seed_uuid(10, 7), 'Khí hồng mông từ thuở hỗn mang sơ khai.', '{"exp_multiplier": 4.0, "exp_bonus": 300}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 25), 'root_bone', 'Sáng Thế Thần Nhãn', pg_temp.seed_uuid(10, 7), 'Đôi mắt thiên bẩm nhìn thấu mọi chân lý cội nguồn của Engine.', '{"exp_multiplier": 3.8, "exp_bonus": 400}'::jsonb, NOW(), NOW()),

-- Root Bone - Hỗn Độn Phẩm
(pg_temp.seed_uuid(11, 26), 'root_bone', 'Hỗn Độn Linh Căn', pg_temp.seed_uuid(10, 8), 'Khởi nguyên vạn vật, cắn nuốt thiên địa Data.', '{"exp_multiplier": 3.0, "exp_bonus": 500}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 27), 'root_bone', 'Thái Âm Thần Căn', pg_temp.seed_uuid(10, 8), 'Cực âm thiên địa, thao túng vĩnh hằng.', '{"exp_multiplier": 2.8, "exp_bonus": 450}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 28), 'root_bone', 'Thái Dương Thần Căn', pg_temp.seed_uuid(10, 8), 'Nóng bức như mặt trời, dọn dẹp sạch sẽ Garbage Collector.', '{"exp_multiplier": 2.8, "exp_bonus": 450}'::jsonb, NOW(), NOW()),

-- Talent - Phàm Phẩm
(pg_temp.seed_uuid(11, 29), 'talent', 'Trí Nhớ Tốt', pg_temp.seed_uuid(10, 1), 'Học công pháp nhanh hơn người thường một chút.', '{"exp_multiplier": 1.05, "exp_bonus": 1}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 30), 'talent', 'Khéo Tay', pg_temp.seed_uuid(10, 1), 'Thao tác tay linh hoạt.', '{"exp_multiplier": 1.02, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 31), 'talent', 'Mọt Sách', pg_temp.seed_uuid(10, 1), 'Thích đọc sách, kiên nhẫn hơn người.', '{"exp_multiplier": 1.05, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 32), 'talent', 'Sức Khỏe Dẻo Dai', pg_temp.seed_uuid(10, 1), 'Ngồi thiền được lâu.', '{"exp_multiplier": 1.0, "exp_bonus": 7}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 33), 'talent', 'Khí Tràng Bốc Mùi', pg_temp.seed_uuid(10, 1), 'Một tuần không tắm, đẩy lùi mọi đối thủ.', '{"exp_multiplier": 1.0, "exp_bonus": -20, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 34), 'talent', 'Kẻ Thức Đêm Trắng', pg_temp.seed_uuid(10, 1), 'Hấp thụ sương đêm, gõ code từ 12h đêm đến sáng.', '{"exp_multiplier": 1.05, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 35), 'talent', 'Chúa Tể Cà Phê', pg_temp.seed_uuid(10, 1), 'Chuyển hóa Caffeine thành những dòng code vô tri.', '{"exp_multiplier": 1.08, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 36), 'talent', 'Mắt Cận Cửu Độ', pg_temp.seed_uuid(10, 1), 'Cận thị siêu nặng, dán mắt vào màn hình mới thấy Bug.', '{"exp_multiplier": 1.1, "exp_bonus": -5}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 37), 'talent', 'Thánh Lười Biếng', pg_temp.seed_uuid(10, 1), 'Chỉ muốn nằm ngủ, tu vi tăng cực chậm.', '{"exp_multiplier": 0.5, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 38), 'talent', 'Gù Lưng Đại Pháp', pg_temp.seed_uuid(10, 1), 'Cột sống biến dạng vì ngồi code quá lâu.', '{"exp_multiplier": 1.0, "exp_bonus": 5, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 39), 'talent', 'Chuyên Gia Tự Ái', pg_temp.seed_uuid(10, 1), 'Luôn đổ lỗi cho compiler chứ không bao giờ nhận sai.', '{"exp_multiplier": 0.9, "exp_bonus": 15}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 40), 'talent', 'Tay Bấm Nhanh Chân Cụp', pg_temp.seed_uuid(10, 1), 'Bấm cực nhanh nhưng sai chính tả liên tục.', '{"exp_multiplier": 1.1, "exp_bonus": -10}'::jsonb, NOW(), NOW()),

-- Talent - Nhân Phẩm
(pg_temp.seed_uuid(11, 41), 'talent', 'Siêu Nhân StackOverflow', pg_temp.seed_uuid(10, 2), 'Chỉ cần copy trên mạng là code tự nhiên chạy.', '{"exp_multiplier": 1.2, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 42), 'talent', 'Thợ Săn Template', pg_temp.seed_uuid(10, 2), 'Gom nhặt mọi đoạn code mẫu của tiền bối.', '{"exp_multiplier": 1.15, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 43), 'talent', 'Hỏa Khí Xông Viện', pg_temp.seed_uuid(10, 2), 'Tính khí nóng nảy, hay đập chuột khi code lỗi.', '{"exp_multiplier": 1.1, "exp_bonus": 20, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 44), 'talent', 'Ma Trận Nhập Nhoạng', pg_temp.seed_uuid(10, 2), 'Luôn quên mất Index của Mảng, hay bị Out Of Bound.', '{"exp_multiplier": 1.1, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 45), 'talent', 'Bạch Tuộc Tám Vòi', pg_temp.seed_uuid(10, 2), 'Khả năng gõ 10 ngón siêu chuẩn, không cần nhìn.', '{"exp_multiplier": 1.2, "exp_bonus": 15}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 46), 'talent', 'Thánh Gán Lỗi', pg_temp.seed_uuid(10, 2), 'Chuyên gán TLE thành MLE bằng logic vô thực.', '{"exp_multiplier": 1.1, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 47), 'talent', 'Kỳ Môn Cầu May', pg_temp.seed_uuid(10, 2), 'Cầu nguyện cho test case yếu để vượt ải dễ dàng.', '{"exp_multiplier": 1.0, "exp_bonus": 30}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 48), 'talent', 'Bậc Thầy Phím Cứng', pg_temp.seed_uuid(10, 2), 'Cơ bắp tay ngưng tụ, gõ thủng cả Switch bàn phím.', '{"exp_multiplier": 1.05, "exp_bonus": 18, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 49), 'talent', 'Mộc Diệp Chân Khí', pg_temp.seed_uuid(10, 2), 'Chân khí dịu nhẹ giúp duy trì sức bền trong phòng thi.', '{"exp_multiplier": 1.12, "exp_bonus": 10, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 50), 'talent', 'Bát Trảo Linh Đồng', pg_temp.seed_uuid(10, 2), 'Đôi mắt có thể đọc lướt qua hàng nghìn dòng code.', '{"exp_multiplier": 1.18, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 51), 'talent', 'Đại Năng Thiếu Dấu Phẩy', pg_temp.seed_uuid(10, 2), 'Tuyệt kỹ tìm dấu chấm phẩy mất tích siêu phàm.', '{"exp_multiplier": 1.15, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 52), 'talent', 'Bậc Thầy In Ra Bug', pg_temp.seed_uuid(10, 2), 'Chuyên dùng print("A") để debug thần sầu.', '{"exp_multiplier": 1.1, "exp_bonus": 12}'::jsonb, NOW(), NOW()),

-- Talent - Địa Phẩm
(pg_temp.seed_uuid(11, 53), 'talent', 'Hỏa Đức Chi Thân', pg_temp.seed_uuid(10, 3), 'Thân thiện với hỏa nguyên tố.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 54), 'talent', 'Kim Canh Chi Khu', pg_temp.seed_uuid(10, 3), 'Cơ thể như kim sắt, dẻo dai vô cùng.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 55), 'talent', 'Mộc Nguyên Bản Năng', pg_temp.seed_uuid(10, 3), 'Sinh lực như cây cổ thụ.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 56), 'talent', 'Thủy Đạo Tiểu Thành', pg_temp.seed_uuid(10, 3), 'Khống chế thủy nguyên tố dễ dàng.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 57), 'talent', 'Thổ Linh Thuẫn', pg_temp.seed_uuid(10, 3), 'Phòng thủ vững chắc như vách núi.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 58), 'talent', 'Trí Nhớ Siêu Phàm', pg_temp.seed_uuid(10, 3), 'Đọc qua một lần là nhớ mãi bí kíp.', '{"exp_multiplier": 1.15, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 59), 'talent', 'Thái Tinh Nhãn', pg_temp.seed_uuid(10, 3), 'Con ngươi rọi sáng như vì sao, thấu thị mọi logic lỗi.', '{"exp_multiplier": 1.4, "exp_bonus": 20}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 60), 'talent', 'Đại Sư Đệ Quy', pg_temp.seed_uuid(10, 3), 'Kiểm soát đệ quy không bao giờ bị Stack Overflow.', '{"exp_multiplier": 1.45, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 61), 'talent', 'Thanh Liên Băng Quyết', pg_temp.seed_uuid(10, 3), 'Tâm lạnh như băng, tản nhiệt hoàn hảo cho máy tính.', '{"exp_multiplier": 1.35, "exp_bonus": 30, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 62), 'talent', 'Linh Thạch Tụ Đỉnh', pg_temp.seed_uuid(10, 3), 'Thu hút tài nguyên, thường được anh em share code.', '{"exp_multiplier": 1.3, "exp_bonus": 40, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 63), 'talent', 'Kiến Trúc Sư Tự Học', pg_temp.seed_uuid(10, 3), 'Kỹ năng hệ thống tự nhiên, bẩm sinh giỏi OOP.', '{"exp_multiplier": 1.4, "exp_bonus": 25}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 64), 'talent', 'Hư Vô Ảnh Thế', pg_temp.seed_uuid(10, 3), 'Ẩn mình trước mặt sếp, yên tĩnh làm việc độc lập.', '{"exp_multiplier": 1.3, "exp_bonus": 50}'::jsonb, NOW(), NOW()),

-- Talent - Thiên Phẩm
(pg_temp.seed_uuid(11, 65), 'talent', 'Kiếm Tâm Thông Minh', pg_temp.seed_uuid(10, 4), 'Sinh ra để dùng kiếm, ngộ tính tuyệt luân.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 66), 'talent', 'Băng Ngọc Chi Thân', pg_temp.seed_uuid(10, 4), 'Thân thể tinh khiết, tu luyện thủy hệ tuyệt đỉnh.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 67), 'talent', 'Thanh Trúc Linh Mạch', pg_temp.seed_uuid(10, 4), 'Sinh mệnh dồi dào, code thâu đêm không mệt.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 68), 'talent', 'Đại Dương Vô Tận', pg_temp.seed_uuid(10, 4), 'Khí hải mênh mông, lưu trữ hàng ngàn thuật toán con.', '{"exp_multiplier": 1.55, "exp_bonus": 50, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 69), 'talent', 'Functional Bất Diệt', pg_temp.seed_uuid(10, 4), 'Nhập đạo bằng Functional Programming, code 1 dòng đắc đạo.', '{"exp_multiplier": 1.7, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 70), 'talent', 'Phân Tán Lưu Trữ', pg_temp.seed_uuid(10, 4), 'Bản thể trải vạn nơi, chạy Distributed System cực hạn.', '{"exp_multiplier": 1.6, "exp_bonus": 40}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 71), 'talent', 'Huyền Băng Chi Nội', pg_temp.seed_uuid(10, 4), 'Làm chủ trái tim băng giá, kiên định đục thủng hệ thống.', '{"exp_multiplier": 1.55, "exp_bonus": 60, "target_elements": ["water", "metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 72), 'talent', 'Thần Quang Ngự Mạch', pg_temp.seed_uuid(10, 4), 'Ánh sáng bảo vệ đường truyền Mạng, ping luôn bằng 0.', '{"exp_multiplier": 1.6, "exp_bonus": 20}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 73), 'talent', 'Dịch Dung Thuật Đại Thành', pg_temp.seed_uuid(10, 4), 'Đổi tên biến siêu đẳng, thầy giáo nhìn không ra.', '{"exp_multiplier": 1.5, "exp_bonus": 70}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 74), 'talent', 'Thiên Hỏa Phần Thành', pg_temp.seed_uuid(10, 4), 'Ngọn lửa cuồng nộ đốt cháy mọi Datacenter cản lối.', '{"exp_multiplier": 1.65, "exp_bonus": 20, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 75), 'talent', 'Rồng Gỗ Sống Lại', pg_temp.seed_uuid(10, 4), 'Phục sinh một bài code tưởng chừng đã chết mục nát.', '{"exp_multiplier": 1.55, "exp_bonus": 50, "target_elements": ["wood", "earth"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 76), 'talent', 'Kim Giáp Chiến Thần', pg_temp.seed_uuid(10, 4), 'Uy phong lẫm liệt, là điểm tựa vững chãi cho Teamwork.', '{"exp_multiplier": 1.6, "exp_bonus": 45, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),

-- Talent - Tiên Phẩm
(pg_temp.seed_uuid(11, 77), 'talent', 'Bát Hoang Kiếm Quyết', pg_temp.seed_uuid(10, 5), 'Kiếm khí dập dờn tám hướng, chém bay mọi bug logic.', '{"exp_multiplier": 1.6, "exp_bonus": 50, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 78), 'talent', 'Bích Thủy Huyền Công', pg_temp.seed_uuid(10, 5), 'Dòng nước thanh tịnh uốn nắn mọi data flow.', '{"exp_multiplier": 1.6, "exp_bonus": 50, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 79), 'talent', 'Vạn Mộc Hồi Xuân', pg_temp.seed_uuid(10, 5), 'Chạy thuật toán nhánh (Tree) sinh sôi vạn vật thiện lành.', '{"exp_multiplier": 1.8, "exp_bonus": 0, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 80), 'talent', 'Thái Cực Bát Quái', pg_temp.seed_uuid(10, 5), 'Cân bằng âm dương, phân đôi mảng (Divide and Conquer).', '{"exp_multiplier": 1.6, "exp_bonus": 50}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 81), 'talent', 'Thái Thượng Vong Tình', pg_temp.seed_uuid(10, 5), 'Rũ bỏ thất tình lục dục, trở thành công cụ Debug hoàn hảo.', '{"exp_multiplier": 1.9, "exp_bonus": 80}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 82), 'talent', 'Chân Thần Thuật Toán', pg_temp.seed_uuid(10, 5), 'Nắm bắt các lý thuyết toán học tinh thâm nhất cõi trần.', '{"exp_multiplier": 2.0, "exp_bonus": 50, "target_elements": ["metal", "earth"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 83), 'talent', 'Lưu Tinh Trụy Lạc', pg_temp.seed_uuid(10, 5), 'Vạn vì sao rơi xuống rực sáng lên bầu trời IDE.', '{"exp_multiplier": 1.8, "exp_bonus": 120, "target_elements": ["fire", "water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 84), 'talent', 'TuLa Vạn Kiếp Cốt', pg_temp.seed_uuid(10, 5), 'Khung xương cứng rắn lách qua vạn đạo kiếp nan TLE/MLE.', '{"exp_multiplier": 2.1, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 85), 'talent', 'Đại Bàng Chín Chảo', pg_temp.seed_uuid(10, 5), 'Bay lượn trên mây với khả năng xử lý bất đồng bộ kinh hãi.', '{"exp_multiplier": 1.9, "exp_bonus": 40}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 86), 'talent', 'Thiên Đạo Luân', pg_temp.seed_uuid(10, 5), 'Thấu tỏ mọi khía cạnh của bánh xe luân hồi mã lệnh.', '{"exp_multiplier": 1.8, "exp_bonus": 110}'::jsonb, NOW(), NOW()),

-- Talent - Thần Phẩm
(pg_temp.seed_uuid(11, 87), 'talent', 'Thần Đồng Tiên Giới', pg_temp.seed_uuid(10, 6), 'Trí tuệ vượt xa phàm nhân, nhìn qua là AC ngay.', '{"exp_multiplier": 2.2, "exp_bonus": 30}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 88), 'talent', 'Chiến Diệt Cuồng Tôn', pg_temp.seed_uuid(10, 6), 'Càng nhiều testcase sai, ngộ tính càng tăng khủng khiếp.', '{"exp_multiplier": 1.9, "exp_bonus": 100}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 89), 'talent', 'Kim Cương Bất Hoại', pg_temp.seed_uuid(10, 6), 'Tâm lý thép, không hoảng sợ trước Time Limit Exceeded.', '{"exp_multiplier": 1.8, "exp_bonus": 120, "target_elements": ["metal", "earth"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 90), 'talent', 'Ma Thần Cái Mệnh', pg_temp.seed_uuid(10, 6), 'Đoạt lấy sinh mệnh của compiler, cưỡng chế nó phải AC.', '{"exp_multiplier": 2.4, "exp_bonus": -50}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 91), 'talent', 'Đảo Lộn Âm Dương', pg_temp.seed_uuid(10, 6), 'Xoay chuyển True/False, định lại hệ quy chiếu logic ảo diệu.', '{"exp_multiplier": 2.2, "exp_bonus": 100}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 92), 'talent', 'Trúa Tể Big Data', pg_temp.seed_uuid(10, 6), 'Nuốt gọn Terabyte file mảng trong nháy mắt không cần tối ưu.', '{"exp_multiplier": 2.5, "exp_bonus": 0, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 93), 'talent', 'Đấng Sáng Lập AI', pg_temp.seed_uuid(10, 6), 'Linh cảm tuyệt đối, tự sinh ra một cỗ máy AI để code hộ.', '{"exp_multiplier": 2.3, "exp_bonus": 120}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 94), 'talent', 'Thiên Long Chấn Bát Nho', pg_temp.seed_uuid(10, 6), 'Rồng thiêng gầm thét, áp đảo tinh thần toàn bộ Contestant.', '{"exp_multiplier": 2.2, "exp_bonus": 80, "target_elements": ["wood", "fire", "water"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 95), 'talent', 'Cửu Thiên Huyền Thể', pg_temp.seed_uuid(10, 6), 'Thể chất vô hình vô tướng, khó bị bắt lỗi.', '{"exp_multiplier": 2.3, "exp_bonus": 20}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 96), 'talent', 'Tuyệt Đối Băng Phong', pg_temp.seed_uuid(10, 6), 'Đóng băng sự phán xét của Hệ thống Judger.', '{"exp_multiplier": 2.3, "exp_bonus": 40, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),

-- Talent - Thánh Phẩm
(pg_temp.seed_uuid(11, 97), 'talent', 'Cửu Ngũ Chí Tôn Thể', pg_temp.seed_uuid(10, 7), 'Mệnh hoàng đế, mọi lời gõ ra đều là thánh chỉ.', '{"exp_multiplier": 3.0, "exp_bonus": 300}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 98), 'talent', 'Vô Thượng Bồ Tát Tâm', pg_temp.seed_uuid(10, 7), 'Vạn pháp bất xâm, độ hóa mọi loại O(N!).', '{"exp_multiplier": 2.8, "exp_bonus": 500}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 99), 'talent', 'Chí Tôn Thần Khúc', pg_temp.seed_uuid(10, 7), 'Khúc hát cõi thần linh, vỗ về linh hồn máy tính.', '{"exp_multiplier": 2.7, "exp_bonus": 400}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 100), 'talent', 'Đại La Kim Thân Bất Diệt', pg_temp.seed_uuid(10, 7), 'Lớp vỏ vàng vĩnh hằng bảo kê Server không bao giờ ddos.', '{"exp_multiplier": 3.1, "exp_bonus": 150, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 101), 'talent', 'Phật Tổ Mở Mắt', pg_temp.seed_uuid(10, 7), 'Nhãn lực đại từ đại bi, ban phát phép mầu cho vạn mã nguồn.', '{"exp_multiplier": 2.8, "exp_bonus": 800}'::jsonb, NOW(), NOW()),

-- Talent - Hỗn Độn Phẩm
(pg_temp.seed_uuid(11, 102), 'talent', 'Càn Khôn Tạo Hóa', pg_temp.seed_uuid(10, 8), 'Tái tạo lại càn khôn vũ trụ, vạn vòng lặp như không.', '{"exp_multiplier": 4.0, "exp_bonus": 300}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 103), 'talent', 'Hư Vô Thôn Phệ', pg_temp.seed_uuid(10, 8), 'Hút sạch mọi Exp của càn khôn, speedrun đẳng cấp chân thần.', '{"exp_multiplier": 5.0, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 104), 'talent', 'Khởi Nguyên Chi Chủ', pg_temp.seed_uuid(10, 8), 'Nắm giữ source code vũ trụ, bản thân chính là Creator.', '{"exp_multiplier": 3.5, "exp_bonus": 1000}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 105), 'talent', 'Tuyệt Đối Chân Lý Thể', pg_temp.seed_uuid(10, 8), 'Hóa thân thành chính bản đồ thuật toán vĩnh hằng bất xâm.', '{"exp_multiplier": 4.5, "exp_bonus": 1500}'::jsonb, NOW(), NOW()),
(pg_temp.seed_uuid(11, 106), 'talent', 'Hư Vô Tịch Diệt Đạo', pg_temp.seed_uuid(10, 8), 'Chân lý trở về với cát bụi, xóa sạch mọi ranh giới kỹ thuật.', '{"exp_multiplier": 5.0, "exp_bonus": -500}'::jsonb, NOW(), NOW());

-- Casbin is the only adapter-owned table with an integer identity key.
SELECT setval(pg_get_serial_sequence('casbin_rules', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "casbin_rules";


-- Insert seed problem: Sum of A + B
-- Stable seed references use pg_temp.seed_uuid(namespace, item).
INSERT INTO problems (id, title, description, difficulty_id, time_limit_ms, memory_limit_kb, author_id, is_published, created_at, updated_at)
VALUES (
  pg_temp.seed_uuid(12, 1),
  'Tính tổng A + B',
  '## Mô tả

Cho hai số nguyên **a** và **b**. Hãy tính tổng của chúng.

## Đầu vào

- Một dòng duy nhất chứa hai số nguyên **a** và **b** ($-10^9 \le a, b \le 10^9$), cách nhau bởi dấu cách.

## Đầu ra

- In ra một số nguyên duy nhất là tổng của **a** và **b**.

## Ràng buộc

- $-10^9 \le a, b \le 10^9$
- Thời gian: 1000ms
- Bộ nhớ: 256MB

## Ví dụ 1:

```text
Input: 1 2
Output: 3
Giải thích: 1 + 2 = 3.
```

## Ví dụ 2:

```text
Input: -5 10
Output: 5
```

## Ví dụ 3:

```text
Input: 0 0
Output: 0
```',
  pg_temp.seed_uuid(6, 1), -- difficulty_id (Easy)
  1000,     -- time_limit_ms
  262144,   -- memory_limit_kb (256MB)
  pg_temp.seed_uuid(3, 1), -- author_id
  true,     -- is_published
  NOW(),
  NOW()
) ON CONFLICT (id) DO NOTHING;

-- Test cases for A+B problem
INSERT INTO test_cases (id, problem_id, input, expected_output, is_hidden, order_index, created_at, updated_at)
VALUES
  (pg_temp.seed_uuid(13, 1), pg_temp.seed_uuid(12, 1), '1 2', '3', false, 1, NOW(), NOW()),
  (pg_temp.seed_uuid(13, 2), pg_temp.seed_uuid(12, 1), '-5 10', '5', false, 2, NOW(), NOW()),
  (pg_temp.seed_uuid(13, 3), pg_temp.seed_uuid(12, 1), '0 0', '0', false, 3, NOW(), NOW()),
  (pg_temp.seed_uuid(13, 4), pg_temp.seed_uuid(12, 1), '1000000000 1000000000', '2000000000', true, 4, NOW(), NOW()),
  (pg_temp.seed_uuid(13, 5), pg_temp.seed_uuid(12, 1), '-1000000000 -1000000000', '-2000000000', true, 5, NOW(), NOW()),
  (pg_temp.seed_uuid(13, 6), pg_temp.seed_uuid(12, 1), '-1000000000 1000000000', '0', true, 6, NOW(), NOW())
ON CONFLICT DO NOTHING;

-- Problem-Tag associations (problem_id, tag_id)
INSERT INTO problem_tags (problem_id, tag_id) VALUES
  (pg_temp.seed_uuid(12, 1), pg_temp.seed_uuid(9, 5)), -- Math
  (pg_temp.seed_uuid(12, 1), pg_temp.seed_uuid(9, 1))  -- Array
ON CONFLICT DO NOTHING;

COMMIT;
