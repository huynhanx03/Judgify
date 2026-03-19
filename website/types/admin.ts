/**
 * Admin-related types for dashboard management.
 */

/** Role entity from backend. */
export interface Role {
  id: number;
  name: string;
  level: number;
  parent_id: number;
  lft: number;
  rgt: number;
  created_at: string;
  updated_at: string;
}

/** Resource entity from backend. */
export interface Resource {
  id: number;
  key: string;
  description: string;
  created_at: string;
  updated_at: string;
}

/** Permission entity from backend. */
export interface Permission {
  id: number;
  role_id: number;
  resource_id: number;
  scopes: number;
  description: string;
  role?: Role;
  resource?: Resource;
  created_at: string;
  updated_at: string;
}

/** Admin user listing (extended from UserProfile). */
export interface AdminUser {
  id: number;
  username: string;
  role_id: number;
  role?: Role;
  created_at: string;
  updated_at: string;
}

/** Dashboard stats overview. */
export interface DashboardStats {
  totalUsers: number;
  totalProblems: number;
  totalTags: number;
  totalRoles: number;
}
