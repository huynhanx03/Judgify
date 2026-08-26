"use client";

/**
 * Admin shell layout — sidebar + header + main content area.
 */

import { useState } from "react";
import { AdminSidebar } from "./admin-sidebar";
import { AdminSidebarNav } from "./admin-sidebar-nav";
import { AdminHeader } from "./admin-header";
import { AdminBrand } from "./admin-brand";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { adminText } from "@/i18n/admin-text";
import { SkipLink } from "@/components/skip-link";

export function AdminShell({ children }: { children: React.ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SkipLink />

      <AdminSidebar />

      <Sheet
        open={mobileOpen}
        onOpenChange={(open) => setMobileOpen(open)}
      >
        <SheetContent
          side="left"
          showCloseButton
          className="w-[min(88vw,18rem)] max-w-none gap-0 border-border bg-surface p-0 lg:hidden"
        >
          <SheetHeader className="border-b border-border px-4 py-2 pr-14">
            <SheetTitle className="sr-only">
              {adminText("PRODUCT_NAME")}
            </SheetTitle>
            <SheetDescription className="sr-only">
              {adminText("MOBILE_NAV_DESCRIPTION")}
            </SheetDescription>
            <AdminBrand onNavigate={() => setMobileOpen(false)} />
          </SheetHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-3 py-4">
            <AdminSidebarNav onNavigate={() => setMobileOpen(false)} />
          </div>
        </SheetContent>
      </Sheet>

      <div className="lg:pl-64">
        <AdminHeader onToggleMobileSidebar={() => setMobileOpen(true)} />
        <main
          id="main-content"
          tabIndex={-1}
          className="mx-auto w-full max-w-[1600px] p-4 outline-none sm:p-6 lg:p-8"
        >
          {children}
        </main>
      </div>
    </div>
  );
}
