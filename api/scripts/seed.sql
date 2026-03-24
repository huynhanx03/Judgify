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
TRUNCATE TABLE "elements" CASCADE;
TRUNCATE TABLE "levels" CASCADE;
TRUNCATE TABLE "ranks" CASCADE;
TRUNCATE TABLE "traits" CASCADE;
TRUNCATE TABLE "rarities" CASCADE;
TRUNCATE TABLE "difficulties" CASCADE;
TRUNCATE TABLE "tag_elements" CASCADE;

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
(9, 'generation', 'Link generation management', NOW(), NOW()),
(10, 'difficulty', 'Difficulty management', NOW(), NOW()),
(11, 'rarity', 'Rarity management', NOW(), NOW());

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
(1, 10, 'Admin: Full difficulty management', 15, NOW(), NOW()),
(1, 11, 'Admin: Full rarity management', 15, NOW(), NOW()),
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

-- Insert Elements
INSERT INTO "elements" (id, name, code, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Kim', 'metal', 'Thuộc tính Kim, sắc bén và cứng rắn.', NOW(), NOW()),
(2, 'Mộc', 'wood', 'Thuộc tính Mộc, sinh trưởng và dẻo dai.', NOW(), NOW()),
(3, 'Thủy', 'water', 'Thuộc tính Thủy, mềm mại và uyển chuyển.', NOW(), NOW()),
(4, 'Hỏa', 'fire', 'Thuộc tính Hỏa, bùng nổ và thiêu rụi.', NOW(), NOW()),
(5, 'Thổ', 'earth', 'Thuộc tính Thổ, vững chắc và bao dung.', NOW(), NOW());

-- Insert Difficulties
INSERT INTO "difficulties" (id, name, level, exp_reward, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Dễ', 1, 10, 'Bài tập cơ bản, phù hợp cho người mới bắt đầu.', NOW(), NOW()),
(2, 'Trung Bình', 2, 25, 'Bài tập yêu cầu tư duy thuật toán vừa phải.', NOW(), NOW()),
(3, 'Khó', 3, 50, 'Bài tập nâng cao, đòi hỏi kiến thức sâu rộng.', NOW(), NOW());

-- Insert Levels
INSERT INTO "levels" (id, name, min_exp, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Phàm Nhân', 0, 'Người bình thường, chưa bước vào con đường tu luyện.', NOW(), NOW()),
(2, 'Tán Tu', 1000, 'Tu sĩ tự do, không môn phái.', NOW(), NOW()),
(3, 'Ngoại Môn Đệ Tử', 5000, 'Đệ tử cấp thấp của tông môn, làm tạp dịch.', NOW(), NOW()),
(4, 'Nội Môn Đệ Tử', 20000, 'Đệ tử chính thức, được truyền thụ công pháp cơ bản.', NOW(), NOW()),
(5, 'Hạch Tâm Đệ Tử', 100000, 'Thành phần nòng cốt, nhận được nhiều tài nguyên.', NOW(), NOW()),
(6, 'Chân Truyền Đệ Tử', 500000, 'Đệ tử do trưởng lão hoặc chưởng môn trực tiếp chỉ dạy.', NOW(), NOW()),
(7, 'Chấp Sự', 2000000, 'Quản lý các công việc thường ngày trong tông môn.', NOW(), NOW()),
(8, 'Trưởng Lão', 10000000, 'Sở hữu quyền lực lớn, chỉ dạy đệ tử.', NOW(), NOW()),
(9, 'Thái Thượng Trưởng Lão', 50000000, 'Những tồn tại cường đại, ẩn tu sau màn.', NOW(), NOW()),
(10, 'Chưởng Môn', 100000000, 'Người đứng đầu một tông môn, quyền lực tối cao.', NOW(), NOW()),
(11, 'Vô Thượng Tiên Tôn', 500000000, 'Truyền thuyết tồn tại, vô địch thiên hạ.', NOW(), NOW());

-- Insert Ranks
INSERT INTO "ranks" (id, name, min_rating, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Luyện Khí Kỳ', 0, 'Cảnh giới sơ tuyển, hấp thu thiên địa linh khí vào cơ thể.', NOW(), NOW()),
(2, 'Trúc Cơ Kỳ', 1200, 'Xây dựng đạo thai, tuổi thọ tăng lên đáng kể.', NOW(), NOW()),
(3, 'Kết Đan Kỳ', 1400, 'Ngưng kết kim đan, có thể ngự kiếm phi hành.', NOW(), NOW()),
(4, 'Nguyên Anh Kỳ', 1600, 'Phá đan thành anh, linh hồn bất diệt.', NOW(), NOW()),
(5, 'Hóa Thần Kỳ', 1800, 'Cảm ngộ thiên địa quy tắc, nguyên thần ngoại xuất.', NOW(), NOW()),
(6, 'Luyện Hư Kỳ', 2000, 'Hòa mình vào hư không, không gian dung hợp.', NOW(), NOW()),
(7, 'Hợp Thể Kỳ', 2200, 'Nguyên anh và thể xác hoàn toàn hợp nhất.', NOW(), NOW()),
(8, 'Đại Thừa Kỳ', 2400, 'Tiếp cận tiên đạo, sắp phi thăng tiên giới.', NOW(), NOW()),
(9, 'Độ Kiếp Kỳ', 2600, 'Trải qua thiên kiếp để lột xác thành tiên.', NOW(), NOW()),
(10, 'Bán Tiên', 2800, 'Chuyển hóa một nửa tiên thể, chuẩn bị phi thăng tiên giới.', NOW(), NOW()),
(11, 'Địa Tiên', 3000, 'Phi thăng thành công, bước đầu làm quen với tiên khí.', NOW(), NOW()),
(12, 'Thiên Tiên', 3200, 'Tích lũy tiên nguyên, pháp lực vô biên.', NOW(), NOW()),
(13, 'Chân Tiên', 3400, 'Khấu triều đại đạo, nắm giữ quy tắc tiên giới.', NOW(), NOW()),
(14, 'Huyền Tiên', 3600, 'Siêu thoát tam giới, không nằm trong ngũ hành.', NOW(), NOW()),
(15, 'Đại La Kim Tiên', 3800, 'Vạn kiếp bất diệt, tồn tại vĩnh hằng.', NOW(), NOW()),
(16, 'Tiên Tôn', 4000, 'Hùng bá một phương tiên giới, vạn tiên triều bái.', NOW(), NOW()),
(17, 'Tiên Đế', 4200, 'Chúa tể tiên giới, cai quản càn khôn.', NOW(), NOW()),
(18, 'Đạo Tổ', 5000, 'Người dung hợp đại đạo, tồn tại tối thượng.', NOW(), NOW());

-- Insert Tags
INSERT INTO "tags" (id, name, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Array', NOW(), NOW()),
(2, 'String', NOW(), NOW()),
(3, 'Hash Map', NOW(), NOW()),
(4, 'Dynamic Programming', NOW(), NOW()),
(5, 'Math', NOW(), NOW()),
(6, 'Sorting', NOW(), NOW()),
(7, 'Greedy', NOW(), NOW()),
(8, 'Depth-First Search', NOW(), NOW()),
(9, 'Breadth-First Search', NOW(), NOW()),
(10, 'Tree', NOW(), NOW()),
(11, 'Binary Search', NOW(), NOW()),
(12, 'Two Pointers', NOW(), NOW()),
(13, 'Graph', NOW(), NOW()),
(14, 'Sliding Window', NOW(), NOW()),
(15, 'Linked List', NOW(), NOW()),
(16, 'Divide and Conquer', NOW(), NOW()),
(17, 'Dijkstra', NOW(), NOW()),
(18, 'Bit Manipulation', NOW(), NOW());

-- Insert Tag-Element Mapping (tag_id, element_id)
-- Kim (metal/logic): Math, Binary Search, Two Pointers, Sliding Window, Divide and Conquer
-- Mộc (wood/growth): Dynamic Programming, Tree, Greedy
-- Thủy (water/flow): String, Array, Sorting, Linked List
-- Hỏa (fire/explore): Depth-First Search, Breadth-First Search, Graph, Dijkstra
-- Thổ (earth/foundation): Hash Map, Bit Manipulation
INSERT INTO "tag_elements" (tag_id, element_id) VALUES
(5, 1),   -- Math → Kim
(11, 1),  -- Binary Search → Kim
(12, 1),  -- Two Pointers → Kim
(14, 1),  -- Sliding Window → Kim
(16, 1),  -- Divide and Conquer → Kim
(4, 2),   -- Dynamic Programming → Mộc
(10, 2),  -- Tree → Mộc
(7, 2),   -- Greedy → Mộc
(2, 3),   -- String → Thủy
(1, 3),   -- Array → Thủy
(6, 3),   -- Sorting → Thủy
(15, 3),  -- Linked List → Thủy
(8, 4),   -- Depth-First Search → Hỏa
(9, 4),   -- Breadth-First Search → Hỏa
(13, 4),  -- Graph → Hỏa
(17, 4),  -- Dijkstra → Hỏa
(3, 5),   -- Hash Map → Thổ
(18, 5);  -- Bit Manipulation → Thổ

-- Insert Rarities (Phẩm chất)
INSERT INTO "rarities" (id, name, code, weight, description, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Phàm Phẩm', 'mortal', 1000, 'Vật phẩm/Phẩm chất bình thường, có thể tìm thấy ở mọi nơi.', NOW(), NOW()),
(2, 'Địa Phẩm', 'earth', 300, 'Vật phẩm/Phẩm chất trung cấp, có linh khí đất trời.', NOW(), NOW()),
(3, 'Thiên Phẩm', 'heaven', 50, 'Vật phẩm/Phẩm chất cao cấp, ngưng tụ tinh hoa nhật nguyệt.', NOW(), NOW()),
(4, 'Thần Phẩm', 'divine', 5, 'Vật phẩm/Phẩm chất truyền thuyết, mang năng lượng nguyên thủy.', NOW(), NOW());

-- Insert Traits
-- Rarity: 1=mortal, 2=earth, 3=heaven, 4=divine
-- Type:
-- + root_bone: unique slot
-- + talent: gacha pool
INSERT INTO "traits" (id, type, name, rarity_id, description, metadata, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
-- Root Bones
(1, 'root_bone', 'Tạp Linh Căn', 1, 'Linh căn hỗn tạp, tốc độ tu luyện cực chậm.', '{"exp_multiplier": 0.5, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(2, 'root_bone', 'Ngũ Hành Phế Căn', 1, 'Năm luồng khí xung khắc, gần như không thể tu luyện.', '{"exp_multiplier": 0.3, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(3, 'root_bone', 'Tứ Linh Căn', 1, 'Linh căn thiếu hụt, tư chất thấp kém.', '{"exp_multiplier": 0.8, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(4, 'root_bone', 'Tam Linh Căn', 2, 'Linh căn trung bình, có thể miễn cưỡng bước vào con đường tu tiên.', '{"exp_multiplier": 1.0, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(5, 'root_bone', 'Chân Linh Căn', 2, 'Linh căn cơ bản, cơ sở của đại đa số tu sĩ.', '{"exp_multiplier": 1.1, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(6, 'root_bone', 'Song Linh Căn', 2, 'Linh căn khá tốt, được các môn phái nhỏ săn đón.', '{"exp_multiplier": 1.3, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(7, 'root_bone', 'Mộc Hỏa Tương Sinh Căn', 2, 'Mộc sinh Hỏa, linh căn tương hỗ tu luyện.', '{"exp_multiplier": 1.4, "exp_bonus": 10, "target_elements": ["wood", "fire"]}'::jsonb, NOW(), NOW()),
(8, 'root_bone', 'Thiên Linh Căn (Hỏa)', 3, 'Chỉ có hệ Hỏa tinh thuần, tốc độ tu luyện cực độ.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(9, 'root_bone', 'Thiên Linh Căn (Thủy)', 3, 'Hệ Thủy tinh thuần, ngộ tính hệt như dòng nước.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(10, 'root_bone', 'Thiên Linh Căn (Kim)', 3, 'Hệ Kim tinh thuần, sát phạt quyết đoán.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(11, 'root_bone', 'Thiên Linh Căn (Mộc)', 3, 'Hệ Mộc tinh thuần, sinh cơ bừng bừng vô tận.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(12, 'root_bone', 'Thiên Linh Căn (Thổ)', 3, 'Hệ Thổ tinh thuần, căn cơ vững vàng như bàn thạch.', '{"exp_multiplier": 1.8, "exp_bonus": 20, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(13, 'root_bone', 'Dị Linh Căn (Băng)', 3, 'Biến dị từ Thủy, cực hàn cực băng.', '{"exp_multiplier": 2.0, "exp_bonus": 30, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(14, 'root_bone', 'Dị Linh Căn (Lôi)', 3, 'Biến dị cực hiếm, mang sức mạnh của thiên kiếp.', '{"exp_multiplier": 2.2, "exp_bonus": 30, "target_elements": ["metal", "fire"]}'::jsonb, NOW(), NOW()),
(15, 'root_bone', 'Dị Linh Căn (Phong)', 3, 'Biến dị từ Mộc, nhanh nhẹn như gió.', '{"exp_multiplier": 2.0, "exp_bonus": 30, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(16, 'root_bone', 'Hỗn Độn Linh Căn', 4, 'Khởi nguyên vạn vật, cắn nuốt thiên địa.', '{"exp_multiplier": 3.0, "exp_bonus": 100}'::jsonb, NOW(), NOW()),
(17, 'root_bone', 'Thái Âm Thần Căn', 4, 'Sinh ra từ cực âm thiên địa, khống chế vạn vật.', '{"exp_multiplier": 2.8, "exp_bonus": 80}'::jsonb, NOW(), NOW()),
(18, 'root_bone', 'Thái Dương Thần Căn', 4, 'Mang sinh khí mặt trời, thiêu rụi chư thiên.', '{"exp_multiplier": 2.8, "exp_bonus": 80}'::jsonb, NOW(), NOW()),

-- Talents (Phàm Phẩm)
(19, 'talent', 'Trí Nhớ Tốt', 1, 'Học công pháp nhanh hơn người thường một chút.', '{"exp_multiplier": 1.05, "exp_bonus": 1}'::jsonb, NOW(), NOW()),
(20, 'talent', 'Khéo Tay', 1, 'Thao tác tay linh hoạt.', '{"exp_multiplier": 1.02, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(21, 'talent', 'Mọt Sách', 1, 'Thích đọc sách, kiên nhẫn hơn người.', '{"exp_multiplier": 1.05, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(22, 'talent', 'Khỏe Mạnh', 1, 'Sức khỏe dồi dào, ít ốm đau.', '{"exp_multiplier": 1.0, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(23, 'talent', 'Nhanh Trí', 1, 'Xử lý tình huống tốt.', '{"exp_multiplier": 1.08, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(24, 'talent', 'Gia Cảnh Bình Phàm', 1, 'Xuất thân bình thường, quen chịu khổ.', '{"exp_multiplier": 1.02, "exp_bonus": 3}'::jsonb, NOW(), NOW()),
(25, 'talent', 'Người Qua Đường', 1, 'Tướng mạo bình phàm, không ai để ý.', '{"exp_multiplier": 1.01, "exp_bonus": 1}'::jsonb, NOW(), NOW()),
(26, 'talent', 'Nông Dân Chăm Chỉ', 1, 'Quen với việc đồng áng, bền bỉ cực tốt.', '{"exp_multiplier": 1.0, "exp_bonus": 6}'::jsonb, NOW(), NOW()),
(27, 'talent', 'Thợ Rèn Tập Sự', 1, 'Tiếp xúc nhiều với kim khí.', '{"exp_multiplier": 1.05, "exp_bonus": 1, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(28, 'talent', 'Ngư Dân Biển Khơi', 1, 'Sống trên biển từ nhỏ.', '{"exp_multiplier": 1.05, "exp_bonus": 1, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(29, 'talent', 'Tiều Phu Rừng Sâu', 1, 'Kỹ năng sinh tồn nơi hoang dã.', '{"exp_multiplier": 1.05, "exp_bonus": 1, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(30, 'talent', 'Thợ Đào Mỏ', 1, 'Quen thuộc với lòng đất.', '{"exp_multiplier": 1.05, "exp_bonus": 1, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(31, 'talent', 'Tâm Trí Bình Thản', 1, 'Không hay bị giật mình.', '{"exp_multiplier": 1.03, "exp_bonus": 3}'::jsonb, NOW(), NOW()),
(32, 'talent', 'Tầm Nhìn Xa', 1, 'Mắt sáng, ít bị mỏi lúc tu luyện.', '{"exp_multiplier": 1.02, "exp_bonus": 4}'::jsonb, NOW(), NOW()),
(33, 'talent', 'Thể Lực Dẻo Dai', 1, 'Ngồi thiền được lâu.', '{"exp_multiplier": 1.0, "exp_bonus": 7}'::jsonb, NOW(), NOW()),
(34, 'talent', 'Sành Ăn', 1, 'Ăn nhiều chóng lớn.', '{"exp_multiplier": 1.01, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(35, 'talent', 'Thích Cuốc Đất', 1, 'Thú vui tao nhã thuận tự nhiên.', '{"exp_multiplier": 1.02, "exp_bonus": 2, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(36, 'talent', 'Tâm Cảnh Ổn Định', 1, 'Ít khi bị tẩu hỏa nhập ma.', '{"exp_multiplier": 1.04, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(37, 'talent', 'Biết Tự Lượng Sức', 1, 'Né tránh rắc rối tốt.', '{"exp_multiplier": 1.05, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(38, 'talent', 'Kẻ Vét Bát', 1, 'Ăn sạch mâm đồng đạo.', '{"exp_multiplier": 1.06, "exp_bonus": 1}'::jsonb, NOW(), NOW()),
(39, 'talent', 'Ngủ Say Như Chết', 1, 'Hồi phục thể lực siêu nhanh khi ngủ.', '{"exp_multiplier": 1.0, "exp_bonus": 8}'::jsonb, NOW(), NOW()),
(40, 'talent', 'Không Sợ Nóng', 1, 'Chịu nhiệt độ cao tốt.', '{"exp_multiplier": 1.0, "exp_bonus": 4, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(41, 'talent', 'Không Ngán Lạnh', 1, 'Chịu hàn khí giỏi.', '{"exp_multiplier": 1.0, "exp_bonus": 4, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(42, 'talent', 'Điềm Tĩnh', 1, 'Bình tĩnh dò bug thay vì đập bàn phím.', '{"exp_multiplier": 1.05, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(43, 'talent', 'Xóm Chợ Đầu Đình', 1, 'Bẩm sinh giao tiếp tốt.', '{"exp_multiplier": 1.03, "exp_bonus": 3}'::jsonb, NOW(), NOW()),
(44, 'talent', 'Khổng Võ Tất Can', 1, 'Tay to chân lớn.', '{"exp_multiplier": 1.0, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(45, 'talent', 'Thiên Lý Mã Cước', 1, 'Chạy trốn rất nhanh.', '{"exp_multiplier": 1.04, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(46, 'talent', 'Nhẫn Nại', 1, 'Có thể cắm cúi gõ phím cả ngày.', '{"exp_multiplier": 1.08, "exp_bonus": 2}'::jsonb, NOW(), NOW()),
(47, 'talent', 'Mũi Tách Mùi', 1, 'Ngửi thấy mùi linh thảo từ xa.', '{"exp_multiplier": 1.02, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(48, 'talent', 'Bụng Bầu', 1, 'Uống đan dược không sợ sình bụng.', '{"exp_multiplier": 1.02, "exp_bonus": 2}'::jsonb, NOW(), NOW()),

-- Talents (Địa Phẩm)
(49, 'talent', 'Hỏa Đức Chi Thân', 2, 'Thân thiện với hỏa nguyên tố.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(50, 'talent', 'Kim Canh Chi Khu', 2, 'Cơ thể như kim sắt, dẻo dai vô cùng.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(51, 'talent', 'Mộc Nguyên Bản Năng', 2, 'Sinh lực như cây cổ thụ.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(52, 'talent', 'Thủy Đạo Tiểu Thành', 2, 'Khống chế thủy nguyên tố dễ dàng.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(53, 'talent', 'Thổ Linh Thuẫn', 2, 'Phòng thủ vững chắc như vách núi.', '{"exp_multiplier": 1.2, "exp_bonus": 5, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(54, 'talent', 'Trí Nhớ Siêu Phàm', 2, 'Đọc qua một lần là nhớ mãi bí kíp.', '{"exp_multiplier": 1.15, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(55, 'talent', 'Tâm Chỉ Nhất Môn', 2, 'Tập trung tư tưởng cao độ, không xao nhãng.', '{"exp_multiplier": 1.18, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(56, 'talent', 'Nghiệp Tướng Gia Tộc', 2, 'Có bối cảnh hỗ trợ tài nguyên tu luyện.', '{"exp_multiplier": 1.1, "exp_bonus": 15}'::jsonb, NOW(), NOW()),
(57, 'talent', 'Ngộ Tính Khá Tốt', 2, 'Thời gian đốn ngộ nhanh hơn người thường.', '{"exp_multiplier": 1.2, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(58, 'talent', 'Mắt Nhìn Tinh Anh', 2, 'Kết giao được với nhiều tinh anh đệ tử.', '{"exp_multiplier": 1.1, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(59, 'talent', 'Khí Hải Khai Mở', 2, 'Đan điền chứa được nhiều chân khí hơn.', '{"exp_multiplier": 1.15, "exp_bonus": 8}'::jsonb, NOW(), NOW()),
(60, 'talent', 'Vận May Bộc Phát', 2, 'Thường vô tình nhặt được linh thảo.', '{"exp_multiplier": 1.12, "exp_bonus": 12}'::jsonb, NOW(), NOW()),
(61, 'talent', 'Nhạy Bén Khí Tức', 2, 'Dễ dàng cảm nhận được thuộc tính tự nhiên.', '{"exp_multiplier": 1.15, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(62, 'talent', 'Lôi Âm Cổ Bộ', 2, 'Sở hữu cước pháp lôi đình vi bộ.', '{"exp_multiplier": 1.25, "exp_bonus": 0, "target_elements": ["metal", "fire"]}'::jsonb, NOW(), NOW()),
(63, 'talent', 'Bách Khoa Toàn Thư', 2, 'Kiến thức thuật toán uyên bác vạn trượng.', '{"exp_multiplier": 1.15, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(64, 'talent', 'Y Thuật Bậc Thầy', 2, 'Tự hồi phục vết thương và luyện đan nhỏ.', '{"exp_multiplier": 1.1, "exp_bonus": 20}'::jsonb, NOW(), NOW()),
(65, 'talent', 'Xuyên Cốt Đằng', 2, 'Chịu đau giỏi, cắn răng nỗ lực giải bài khó.', '{"exp_multiplier": 1.2, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(66, 'talent', 'Kẻ Sống Sót Khắc Khổ', 2, 'Trải qua sinh tử, ý chí kiên định siêu quần.', '{"exp_multiplier": 1.25, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(67, 'talent', 'Tiềm Năng Đánh Thức', 2, 'Càng đua top đánh càng hăng.', '{"exp_multiplier": 1.22, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(68, 'talent', 'Kiến Trúc Sư Tông Môn', 2, 'Sáng tạo trận pháp và hệ thống cực giỏi.', '{"exp_multiplier": 1.25, "exp_bonus": 5, "target_elements": ["earth", "wood"]}'::jsonb, NOW(), NOW()),
(69, 'talent', 'Con Buôn Bí Kíp', 2, 'Biết cách khai thác Code kinh tế.', '{"exp_multiplier": 1.05, "exp_bonus": 30}'::jsonb, NOW(), NOW()),
(70, 'talent', 'Tu La Chuyển Thể', 2, 'Lòng đầy sát khí, múa phím ảo diệu.', '{"exp_multiplier": 1.3, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(71, 'talent', 'Cuồng Phong Chi Ý', 2, 'Sự nhạy bén với luồng khí thông.', '{"exp_multiplier": 1.2, "exp_bonus": 10, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(72, 'talent', 'Nắm Giữ Tinh Hoa', 2, 'Hiểu rõ các bài tập vỡ lòng một cách xuất thần.', '{"exp_multiplier": 1.25, "exp_bonus": 5}'::jsonb, NOW(), NOW()),
(73, 'talent', 'Đệ Nhị Môn Phái', 2, 'Tài năng không hẳn là Top 1 nhưng rất đa dụng.', '{"exp_multiplier": 1.15, "exp_bonus": 15}'::jsonb, NOW(), NOW()),
(74, 'talent', 'Trăm Trận Trăm Thắng', 2, 'Kinh nghiệm luyện tập cực cao.', '{"exp_multiplier": 1.28, "exp_bonus": 5}'::jsonb, NOW(), NOW()),

-- Talents (Thiên Phẩm)
(75, 'talent', 'Kiếm Tâm Thông Minh', 3, 'Sinh ra để dùng kiếm, ngộ tính tuyệt luân.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(76, 'talent', 'Băng Ngọc Chi Thân', 3, 'Thân thể tinh khiết, tu luyện thủy hệ tuyệt đỉnh.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["water"]}'::jsonb, NOW(), NOW()),
(77, 'talent', 'Thanh Trúc Linh Mạch', 3, 'Sinh mệnh dồi dào, code thâu đêm không mệt.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["wood"]}'::jsonb, NOW(), NOW()),
(78, 'talent', 'Thiên Hỏa Linh Đồng', 3, 'Đôi mắt nhìn thấu bugs tinh vi nhất.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(79, 'talent', 'Đại Địa Chi Tử', 3, 'Đứng trên mặt đất là nắm giữ mọi array mảng.', '{"exp_multiplier": 1.5, "exp_bonus": 30, "target_elements": ["earth"]}'::jsonb, NOW(), NOW()),
(80, 'talent', 'Đạo Cốt Trời Sinh', 3, 'Bẩm sinh đã nắm giữ đạo vận thuật toán.', '{"exp_multiplier": 1.6, "exp_bonus": 40}'::jsonb, NOW(), NOW()),
(81, 'talent', 'Phật Tâm Ma Mang', 3, 'Thiện ác nhất niệm, công pháp nào cũng tu được.', '{"exp_multiplier": 1.7, "exp_bonus": 20}'::jsonb, NOW(), NOW()),
(82, 'talent', 'Con Cưng Của Trời', 3, 'Đi đường vấp ngã cũng nhặt được AC (Accepted).', '{"exp_multiplier": 1.5, "exp_bonus": 80}'::jsonb, NOW(), NOW()),
(83, 'talent', 'Thập Điện Diêm Vương', 3, 'Linh hồn phán quan cực kì hùng mạnh phi phàm.', '{"exp_multiplier": 1.8, "exp_bonus": 0}'::jsonb, NOW(), NOW()),
(84, 'talent', 'Thần Giao Cách Cảm', 3, 'Giao tiếp với thiên địa, hiểu ngay ý tác giả đề.', '{"exp_multiplier": 1.6, "exp_bonus": 50}'::jsonb, NOW(), NOW()),
(85, 'talent', 'Chiến Thần Tái Thế', 3, 'Kinh nghiệm ACM tích lũy từ kiếp trước vô lường.', '{"exp_multiplier": 1.8, "exp_bonus": 10}'::jsonb, NOW(), NOW()),
(86, 'talent', 'Tinh Túc Mệnh Bàn', 3, 'Nắm rõ quy luật vận hành của vũ trụ Data.', '{"exp_multiplier": 1.6, "exp_bonus": 60}'::jsonb, NOW(), NOW()),
(87, 'talent', 'Tuyệt Đại Đan Sư', 3, 'Luyện đan xác suất thành công cực cao.', '{"exp_multiplier": 1.5, "exp_bonus": 0, "target_elements": ["fire", "wood"]}'::jsonb, NOW(), NOW()),
(88, 'talent', 'Tuyệt Đại Trận Pháp', 3, 'Vẽ trận phá vạn pháp hệ thống.', '{"exp_multiplier": 1.5, "exp_bonus": 0, "target_elements": ["earth", "metal"]}'::jsonb, NOW(), NOW()),
(89, 'talent', 'Bách Mạch Tụ Điển', 3, 'Kinh mạch thông suốt tới tột cùng.', '{"exp_multiplier": 1.8, "exp_bonus": 30}'::jsonb, NOW(), NOW()),
(90, 'talent', 'Quang Minh Thánh Cốt', 3, 'Ngập tràn ý niệm ánh sáng thánh vệ.', '{"exp_multiplier": 1.6, "exp_bonus": 50}'::jsonb, NOW(), NOW()),
(91, 'talent', 'U Ám Sát Thần', 3, 'Trong bóng râm ám sát định thế càn khôn.', '{"exp_multiplier": 1.7, "exp_bonus": 40}'::jsonb, NOW(), NOW()),
(92, 'talent', 'Đoạt Mệnh Lệnh Khách', 3, 'Khóa tử huyệt kẻ thù qua Data Structures.', '{"exp_multiplier": 1.6, "exp_bonus": 45}'::jsonb, NOW(), NOW()),

-- Talents (Thần Phẩm)
(93, 'talent', 'Hoang Cổ Thánh Thể', 4, 'Thể chất thuật toán vô địch, vạn tà bất xâm.', '{"exp_multiplier": 2.5, "exp_bonus": 100}'::jsonb, NOW(), NOW()),
(94, 'talent', 'Hỗn Độn Đạo Thể', 4, 'Vạn pháp bất xâm, điều khiển Time Complexity.', '{"exp_multiplier": 3.0, "exp_bonus": 150}'::jsonb, NOW(), NOW()),
(95, 'talent', 'Trọng Đồng (Hạng Vũ)', 4, 'Đôi mắt hai tròng, nhìn thấu gốc rễ mọi bài toán.', '{"exp_multiplier": 2.8, "exp_bonus": 80}'::jsonb, NOW(), NOW()),
(96, 'talent', 'Thiên Sinh Chí Tôn', 4, 'Sở hữu Chí Tôn Cốt, mang uy áp của các Grandmaster.', '{"exp_multiplier": 2.6, "exp_bonus": 200}'::jsonb, NOW(), NOW()),
(97, 'talent', 'Tiên Tôn Chuyển Thế', 4, 'Nhớ lại toàn bộ giải thuật kinh điển kiếp trước.', '{"exp_multiplier": 3.0, "exp_bonus": 500}'::jsonb, NOW(), NOW()),
(98, 'talent', 'Bất Diệt Kiếm Thể', 4, 'Bản thân là một thanh kiếm sắc bén nhất tam giới.', '{"exp_multiplier": 2.5, "exp_bonus": 150, "target_elements": ["metal"]}'::jsonb, NOW(), NOW()),
(99, 'talent', 'Vạn Hỏa Phần Thiên', 4, 'Thống ngự ngọn lửa của thần linh vĩnh hằng.', '{"exp_multiplier": 2.5, "exp_bonus": 150, "target_elements": ["fire"]}'::jsonb, NOW(), NOW()),
(100, 'talent', 'Đạo Đức Kinh Vạn Quyển', 4, 'Mang trong người tư chất của đấng sáng tạo Core Concept.', '{"exp_multiplier": 3.5, "exp_bonus": 0}'::jsonb, NOW(), NOW());

-- Reset Sequences (For PostgreSQL identity columns)
SELECT setval(pg_get_serial_sequence('roles', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "roles";
SELECT setval(pg_get_serial_sequence('users', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "users";
SELECT setval(pg_get_serial_sequence('resources', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "resources";
SELECT setval(pg_get_serial_sequence('attribute_definitions', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "attribute_definitions";
SELECT setval(pg_get_serial_sequence('elements', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "elements";
SELECT setval(pg_get_serial_sequence('levels', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "levels";
SELECT setval(pg_get_serial_sequence('ranks', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "ranks";
SELECT setval(pg_get_serial_sequence('tags', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "tags";
SELECT setval(pg_get_serial_sequence('difficulties', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "difficulties";
SELECT setval(pg_get_serial_sequence('rarities', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "rarities";
SELECT setval(pg_get_serial_sequence('traits', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM "traits";
