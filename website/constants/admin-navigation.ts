/**
 * Admin sidebar navigation configuration.
 */

import {
  LayoutDashboard,
  Users,
  FileCode2,
  Tags,
  ShieldCheck,
  Flame,
  Sparkles,
  Layers,
  Trophy,
  BarChart3,
  Gem,
  Swords,
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
      { label: "Độ Khó", href: "/admin/difficulties", icon: BarChart3 },
      { label: "Đại Hội", href: "/admin/contests", icon: Swords },
    ],
  },
  {
    title: "Tu Luyện",
    items: [
      { label: "Nguyên Tố", href: "/admin/elements", icon: Flame },
      { label: "Độ Hiếm", href: "/admin/rarities", icon: Gem },
      { label: "Đặc Tính", href: "/admin/traits", icon: Sparkles },
      { label: "Cảnh Giới", href: "/admin/levels", icon: Layers },
      { label: "Danh Hiệu", href: "/admin/ranks", icon: Trophy },
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
