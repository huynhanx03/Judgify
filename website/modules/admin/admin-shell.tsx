"use client";

/**
 * Admin shell layout — sidebar + header + main content area.
 */

import { useState } from "react";
import { X } from "lucide-react";
import { AdminSidebar } from "./admin-sidebar";
import { AdminSidebarNav } from "./admin-sidebar-nav";
import { AdminHeader } from "./admin-header";

export function AdminShell({ children }: { children: React.ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <div className="min-h-screen bg-background">
      {/* Desktop sidebar */}
      <AdminSidebar />

      {/* Mobile sidebar overlay */}
      {mobileOpen && (
        <>
          <div
            className="fixed inset-0 z-40 bg-black/40 lg:hidden"
            onClick={() => setMobileOpen(false)}
          />
          <div className="fixed inset-y-0 left-0 z-50 w-60 bg-card border-r border-border lg:hidden">
            <div className="flex h-14 items-center justify-between border-b border-border px-5">
              <div className="flex items-center gap-2">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground text-sm font-bold">
                  J
                </div>
                <span className="text-sm font-bold tracking-tight">Judgify Admin</span>
              </div>
              <button
                onClick={() => setMobileOpen(false)}
                className="text-muted-foreground hover:text-foreground cursor-pointer"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <div className="px-3 py-4">
              <AdminSidebarNav onNavigate={() => setMobileOpen(false)} />
            </div>
          </div>
        </>
      )}

      {/* Main area (offset by sidebar on desktop) */}
      <div className="lg:pl-60">
        <AdminHeader onToggleMobileSidebar={() => setMobileOpen(true)} />
        <main className="p-4 lg:p-6">{children}</main>
      </div>
    </div>
  );
}
