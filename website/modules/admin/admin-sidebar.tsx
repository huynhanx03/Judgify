"use client";

/**
 * Desktop fixed admin sidebar — logo + navigation.
 */

import { AdminSidebarNav } from "./admin-sidebar-nav";

export function AdminSidebar() {
  return (
    <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 border-r border-border bg-card/50 backdrop-blur-sm lg:block">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2 border-b border-border px-5">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground text-sm font-bold">
          J
        </div>
        <span className="text-sm font-bold tracking-tight">Judgify Admin</span>
      </div>

      {/* Navigation */}
      <div className="overflow-y-auto px-3 py-4">
        <AdminSidebarNav />
      </div>
    </aside>
  );
}
