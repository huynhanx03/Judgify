-- Stable identifiers keep local catalog references reproducible.
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

-- Seed only missing system catalog rows; never clear user data.

-- Insert Elements
INSERT INTO "elements" (id, name, code, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(5, 1), 'Kim', 'metal', 'Thuộc tính Kim, sắc bén và cứng rắn.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 2), 'Mộc', 'wood', 'Thuộc tính Mộc, sinh trưởng và dẻo dai.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 3), 'Thủy', 'water', 'Thuộc tính Thủy, mềm mại và uyển chuyển.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 4), 'Hỏa', 'fire', 'Thuộc tính Hỏa, bùng nổ và thiêu rụi.', NOW(), NOW()),
(pg_temp.seed_uuid(5, 5), 'Thổ', 'earth', 'Thuộc tính Thổ, vững chắc và bao dung.', NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Insert Difficulties
INSERT INTO "difficulties" (id, name, level, exp_reward, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(6, 1), 'Dễ', 1, 10, 'Bài tập cơ bản, phù hợp cho người mới bắt đầu.', NOW(), NOW()),
(pg_temp.seed_uuid(6, 2), 'Trung Bình', 2, 25, 'Bài tập yêu cầu tư duy thuật toán vừa phải.', NOW(), NOW()),
(pg_temp.seed_uuid(6, 3), 'Khó', 3, 50, 'Bài tập nâng cao, đòi hỏi kiến thức sâu rộng.', NOW(), NOW()) ON CONFLICT DO NOTHING;

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
(pg_temp.seed_uuid(7, 11), 'Vô Thượng Tiên Tôn', 500000000, 'Truyền thuyết tồn tại, vô địch thiên hạ.', NOW(), NOW()) ON CONFLICT DO NOTHING;

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
(pg_temp.seed_uuid(8, 18), 'Đạo Tổ', 5000, 'Người dung hợp đại đạo, tồn tại tối thượng.', NOW(), NOW()) ON CONFLICT DO NOTHING;

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
(pg_temp.seed_uuid(9, 18), 'Bit Manipulation', NOW(), NOW()) ON CONFLICT DO NOTHING;

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
(pg_temp.seed_uuid(9, 18), pg_temp.seed_uuid(5, 5)) ON CONFLICT DO NOTHING;  -- Bit Manipulation → Thổ

-- Insert Rarities (Phẩm chất)
INSERT INTO "rarities" (id, name, code, weight, description, created_at, updated_at) VALUES
(pg_temp.seed_uuid(10, 1), 'Phàm Phẩm', 'common', 75, 'Phẩm chất bèo bọt, nhan nhản mọi nơi.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 2), 'Nhân Phẩm', 'uncommon', 50, 'Dành cho nhân kiệt, có tư chất tu luyện.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 3), 'Địa Phẩm', 'rare', 35, 'Phẩm chất trung cấp, có linh khí đại địa.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 4), 'Thiên Phẩm', 'epic', 20, 'Phẩm chất cao cấp, ngưng tụ tinh hoa nhật nguyệt.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 5), 'Tiên Phẩm', 'legendary', 12, 'Phẩm chất tiên giới, thoát thai hoán cốt.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 6), 'Thần Phẩm', 'mythic', 5, 'Phẩm chất truyền thuyết, mang uy áp thần minh.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 7), 'Thánh Phẩm', 'transcendent', 2, 'Phẩm chất của thánh nhân, vạn thế vô nhất.', NOW(), NOW()),
(pg_temp.seed_uuid(10, 8), 'Hỗn Độn Phẩm', 'origin', 1, 'Khởi nguyên vạn vật, cắn nuốt từ cõi vô hư.', NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Minimal immutable trait catalog needed for onboarding offers.
INSERT INTO "traits" (
  id, code, type, name, rarity_id, description,
  display_order, active, version, created_at, updated_at
) VALUES
(pg_temp.seed_uuid(11, 1), 'local_root_bone', 'root_bone', 'Tạp Linh Căn', pg_temp.seed_uuid(10, 1), 'Căn cốt mặc định cho môi trường local.', 0, true, 1, NOW(), NOW()),
(pg_temp.seed_uuid(11, 2), 'local_talent_1', 'talent', 'Trí Nhớ Tốt', pg_temp.seed_uuid(10, 1), 'Ghi nhớ công pháp nhanh hơn.', 1, true, 1, NOW(), NOW()),
(pg_temp.seed_uuid(11, 3), 'local_talent_2', 'talent', 'Khéo Tay', pg_temp.seed_uuid(10, 1), 'Thao tác linh hoạt.', 2, true, 1, NOW(), NOW()),
(pg_temp.seed_uuid(11, 4), 'local_talent_3', 'talent', 'Mọt Sách', pg_temp.seed_uuid(10, 1), 'Kiên nhẫn học hỏi.', 3, true, 1, NOW(), NOW()),
(pg_temp.seed_uuid(11, 5), 'local_talent_4', 'talent', 'Sức Khỏe Dẻo Dai', pg_temp.seed_uuid(10, 1), 'Bền bỉ trên đường tu luyện.', 4, true, 1, NOW(), NOW()),
(pg_temp.seed_uuid(11, 6), 'local_talent_5', 'talent', 'Thức Đêm', pg_temp.seed_uuid(10, 1), 'Tập trung tốt khi đêm xuống.', 5, true, 1, NOW(), NOW()),
(pg_temp.seed_uuid(11, 7), 'local_talent_6', 'talent', 'Chúa Tể Cà Phê', pg_temp.seed_uuid(10, 1), 'Biến caffeine thành động lực.', 6, true, 1, NOW(), NOW())
ON CONFLICT DO NOTHING;

-- Checksums use the same immutable revision payload as the cultivation domain.
INSERT INTO "trait_effect_revisions" (
  id, trait_id, revision, schema_version, effect,
  target_element_ids, checksum, effective_at, created_at
) VALUES
(pg_temp.seed_uuid(12, 1), pg_temp.seed_uuid(11, 1), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":100}'::jsonb, '[]'::jsonb, decode('cdee93a51b4c8687af5fa4fdd9c18079713c0b5ef6ad2d437a6a7aa5fcab78ac', 'hex'), '2026-01-01 00:00:00+00', NOW()),
(pg_temp.seed_uuid(12, 2), pg_temp.seed_uuid(11, 2), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":50}'::jsonb, '[]'::jsonb, decode('90a0c0f097aa11cd43c69c2b1d28e98abfa842b1016568c25787fc6765684e11', 'hex'), '2026-01-01 00:00:00+00', NOW()),
(pg_temp.seed_uuid(12, 3), pg_temp.seed_uuid(11, 3), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":50}'::jsonb, '[]'::jsonb, decode('a3128fdf161b43f7ef75c747f24a0ac261e27a0db6a6413e425a6f4231c296cc', 'hex'), '2026-01-01 00:00:00+00', NOW()),
(pg_temp.seed_uuid(12, 4), pg_temp.seed_uuid(11, 4), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":50}'::jsonb, '[]'::jsonb, decode('c9bec1bd4fae9d4681da4801891d904d1de359797ce386a7754700aa2f47f27e', 'hex'), '2026-01-01 00:00:00+00', NOW()),
(pg_temp.seed_uuid(12, 5), pg_temp.seed_uuid(11, 5), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":50}'::jsonb, '[]'::jsonb, decode('429366a857a435120ece2e93d4b048bf298c1c614b54cd8ef43d8907ca555069', 'hex'), '2026-01-01 00:00:00+00', NOW()),
(pg_temp.seed_uuid(12, 6), pg_temp.seed_uuid(11, 6), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":50}'::jsonb, '[]'::jsonb, decode('8eeda99722f2cb80b3d8ebcb2ebc12383a62142bbcb6dc5f703f002622393088', 'hex'), '2026-01-01 00:00:00+00', NOW()),
(pg_temp.seed_uuid(12, 7), pg_temp.seed_uuid(11, 7), 1, 1, '{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":50}'::jsonb, '[]'::jsonb, decode('f2fe91bb25e53a58e1c58944d75e5851baa1cd9862e83553d83523b0e395ed28', 'hex'), '2026-01-01 00:00:00+00', NOW())
ON CONFLICT DO NOTHING;

