/**
 * Admin sidebar navigation configuration.
 */

import {
  LayoutDashboard,
  Users,
  FileCode2,
  Tags,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";

export interface AdminNavItem {
  label: string;
  href: string;
  icon: LucideIcon;
}

export interface AdminNavSection {
  title: string;
  items: AdminNavItem[];
}

export const ADMIN_NAV_SECTIONS: AdminNavSection[] = [
  {
    title: "Tổng Quan",
    items: [
      { label: "Dashboard", href: "/admin", icon: LayoutDashboard },
    ],
  },
  {
    title: "Quản Lý Nội Dung",
    items: [
      { label: "Bài Tập", href: "/admin/problems", icon: FileCode2 },
      { label: "Tags", href: "/admin/tags", icon: Tags },
    ],
  },
  {
    title: "Quản Lý Hệ Thống",
    items: [
      { label: "Người Dùng", href: "/admin/users", icon: Users },
      { label: "Phân Quyền", href: "/admin/roles", icon: ShieldCheck },
    ],
  },
];
