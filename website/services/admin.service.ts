/**
 * Admin service for dashboard management.
 * Uses mock data — replace with apiClient calls when backend is ready.
 */

import type { AdminUser, DashboardStats, Role, Permission, Resource } from "@/types/admin";
import type { Problem } from "@/types/problem";
import type { Tag } from "@/types/tag";

const delay = (ms: number) => new Promise((r) => setTimeout(r, ms));

const MOCK_ROLES: Role[] = [
  { id: 1, name: "Admin", level: 0, parent_id: -1, lft: 1, rgt: 6, created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 2, name: "Teacher", level: 1, parent_id: 1, lft: 2, rgt: 5, created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 3, name: "Student", level: 2, parent_id: 2, lft: 3, rgt: 4, created_at: "2025-01-01", updated_at: "2025-01-01" },
];

const MOCK_RESOURCES: Resource[] = [
  { id: 1, key: "user", description: "Tài khoản người dùng trong hệ thống", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 2, key: "role", description: "Vai trò và phân cấp quyền hạn", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 3, key: "permission", description: "Quyền truy cập tài nguyên theo vai trò", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 4, key: "resource", description: "Danh mục tài nguyên được bảo vệ", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 5, key: "problem", description: "Đề bài lập trình và mô tả yêu cầu", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 6, key: "test_case", description: "Bộ test đầu vào/đầu ra cho bài tập", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 7, key: "tag", description: "Nhãn phân loại bài tập theo chủ đề", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 8, key: "element", description: "Nguyên tố tu luyện của người dùng", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 9, key: "trait", description: "Đặc tính và thuộc tính đặc biệt", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 10, key: "level", description: "Cấp độ tu luyện và kinh nghiệm", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 11, key: "rank", description: "Danh hiệu và bảng xếp hạng", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 12, key: "user_stats", description: "Thống kê hoạt động và tiến độ người dùng", created_at: "2025-01-01", updated_at: "2025-01-01" },
];

const MOCK_PERMISSIONS: Permission[] = [
  // Admin (role 1) — full access everything
  { id: 1, role_id: 1, resource_id: 1, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 2, role_id: 1, resource_id: 2, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 3, role_id: 1, resource_id: 3, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 4, role_id: 1, resource_id: 4, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 5, role_id: 1, resource_id: 5, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 6, role_id: 1, resource_id: 6, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 7, role_id: 1, resource_id: 7, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 8, role_id: 1, resource_id: 8, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 9, role_id: 1, resource_id: 9, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 10, role_id: 1, resource_id: 10, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 11, role_id: 1, resource_id: 11, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 12, role_id: 1, resource_id: 12, scopes: 15, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  // Teacher (role 2) — CRU problems, CRU test_cases, CRU tags, R users
  { id: 13, role_id: 2, resource_id: 1, scopes: 2, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 14, role_id: 2, resource_id: 5, scopes: 7, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 15, role_id: 2, resource_id: 6, scopes: 7, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 16, role_id: 2, resource_id: 7, scopes: 7, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  // Student (role 3) — R problems, R tags
  { id: 17, role_id: 3, resource_id: 5, scopes: 2, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 18, role_id: 3, resource_id: 7, scopes: 2, description: "", created_at: "2025-01-01", updated_at: "2025-01-01" },
];

const MOCK_USERS: AdminUser[] = [
  { id: 1, username: "admin", role_id: 1, role: MOCK_ROLES[0], created_at: "2025-01-01", updated_at: "2025-01-01" },
  { id: 2, username: "teacher01", role_id: 2, role: MOCK_ROLES[1], created_at: "2025-02-15", updated_at: "2025-02-15" },
  { id: 3, username: "student01", role_id: 3, role: MOCK_ROLES[2], created_at: "2025-03-01", updated_at: "2025-03-01" },
  { id: 4, username: "student02", role_id: 3, role: MOCK_ROLES[2], created_at: "2025-03-05", updated_at: "2025-03-05" },
  { id: 5, username: "student03", role_id: 3, role: MOCK_ROLES[2], created_at: "2025-03-10", updated_at: "2025-03-10" },
];

const MOCK_PROBLEMS: Problem[] = [
  { id: 1, title: "Two Sum", description: "Tìm hai số có tổng bằng target", difficulty: "easy", time_limit_ms: 1000, memory_limit_kb: 262144, author_id: 2, is_published: true, tags: [{ id: 1, name: "Array" }], created_at: "2025-01-15", updated_at: "2025-01-15" },
  { id: 2, title: "Longest Substring", description: "Tìm chuỗi con dài nhất không lặp ký tự", difficulty: "medium", time_limit_ms: 2000, memory_limit_kb: 262144, author_id: 2, is_published: true, tags: [{ id: 2, name: "String" }, { id: 3, name: "Sliding Window" }], created_at: "2025-02-01", updated_at: "2025-02-01" },
  { id: 3, title: "Merge K Sorted Lists", description: "Gộp K danh sách đã sắp xếp", difficulty: "hard", time_limit_ms: 3000, memory_limit_kb: 524288, author_id: 1, is_published: false, tags: [{ id: 4, name: "Heap" }, { id: 5, name: "Linked List" }], created_at: "2025-02-20", updated_at: "2025-02-20" },
  { id: 4, title: "Binary Search", description: "Tìm kiếm nhị phân cơ bản", difficulty: "easy", time_limit_ms: 1000, memory_limit_kb: 262144, author_id: 2, is_published: true, tags: [{ id: 1, name: "Array" }], created_at: "2025-03-01", updated_at: "2025-03-01" },
];

const MOCK_TAGS: Tag[] = [
  { id: 1, name: "Array" },
  { id: 2, name: "String" },
  { id: 3, name: "Sliding Window" },
  { id: 4, name: "Heap" },
  { id: 5, name: "Linked List" },
  { id: 6, name: "Dynamic Programming" },
  { id: 7, name: "Graph" },
  { id: 8, name: "Tree" },
];

export const adminService = {
  async getDashboardStats(): Promise<DashboardStats> {
    await delay(300);
    return {
      totalUsers: MOCK_USERS.length,
      totalProblems: MOCK_PROBLEMS.length,
      totalTags: MOCK_TAGS.length,
      totalRoles: MOCK_ROLES.length,
    };
  },

  async getUsers(): Promise<AdminUser[]> {
    await delay(400);
    return MOCK_USERS;
  },

  async getProblems(): Promise<Problem[]> {
    await delay(400);
    return MOCK_PROBLEMS;
  },

  async getTags(): Promise<Tag[]> {
    await delay(300);
    return MOCK_TAGS;
  },

  async getRoles(): Promise<Role[]> {
    await delay(300);
    return MOCK_ROLES;
  },

  async getPermissions(): Promise<Permission[]> {
    await delay(300);
    return MOCK_PERMISSIONS;
  },

  async getResources(): Promise<Resource[]> {
    await delay(300);
    return MOCK_RESOURCES;
  },
};
