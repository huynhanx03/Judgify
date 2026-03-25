/**
 * Admin dashboard layout — wraps all admin pages with sidebar + header.
 */

import { AdminShell } from "@/modules/admin/admin-shell";

export default function AdminDashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AdminShell>{children}</AdminShell>;
}
