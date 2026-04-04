/**
 * Admin-related types for dashboard management.
 */

/** Role entity — mirrors BE RoleResponse. */
export interface Role {
  id: number;
  name: string;
  level: number;
}

/** Resource entity — mirrors BE ResourceResponse. */
export interface Resource {
  id: number;
  key: string;
  description?: string;
}

/** Permission entity — mirrors BE PermissionResponse. */
export interface Permission {
  id: number;
  role_id: number;
  resource_id: number;
  description?: string;
  scopes: number;
}

/** Admin user listing. */
export interface AdminUser {
  id: number;
  username: string;
  role_id: number;
  role_name: string;
  created_at: string;
  updated_at: string;
}

/** Dashboard stats overview. */
export interface DashboardStats {
  totalProblems: number;
  totalTags: number;
  totalRoles: number;
  totalUsers: number;
}
