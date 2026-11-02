"use client";

/**
 * Desktop fixed admin sidebar — logo + navigation.
 */

import { AdminSidebarNav } from "./admin-sidebar-nav";
import { AdminBrand } from "./admin-brand";
import { adminText } from "@/i18n/admin-text";

export function AdminSidebar() {
  return (
    <aside
      className="fixed inset-y-0 left-0 z-30 hidden w-64 border-r border-border bg-surface/95 backdrop-blur-lg lg:flex lg:flex-col"
      aria-label={adminText("NAVIGATION_LABEL")}
    >
      <div className="flex h-16 shrink-0 items-center border-b border-border px-4">
        <AdminBrand />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-3 py-4">
        <AdminSidebarNav />
      </div>
    </aside>
  );
}
