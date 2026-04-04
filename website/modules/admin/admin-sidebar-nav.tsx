"use client";

/**
 * Admin sidebar navigation links — collapsible sections.
 * Click section header to expand/collapse items.
 */

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronDown } from "lucide-react";
import { ADMIN_NAV_SECTIONS } from "@/constants/admin-navigation";
import { cn } from "@/lib/utils";

interface AdminSidebarNavProps {
  onNavigate?: () => void;
}

export function AdminSidebarNav({ onNavigate }: AdminSidebarNavProps) {
  const pathname = usePathname();

  // Initialize: all sections expanded by default
  const [openSections, setOpenSections] = useState<Record<string, boolean>>(
    () => Object.fromEntries(ADMIN_NAV_SECTIONS.map((s) => [s.title, true]))
  );

  function toggleSection(title: string) {
    setOpenSections((prev) => ({ ...prev, [title]: !prev[title] }));
  }

  return (
    <nav className="space-y-2">
      {ADMIN_NAV_SECTIONS.map((section) => {
        const isOpen = openSections[section.title] ?? true;
        const hasActiveItem = section.items.some((item) =>
          item.href === "/admin"
            ? pathname === "/admin"
            : pathname.startsWith(item.href)
        );

        return (
          <div key={section.title}>
            {/* Collapsible section header */}
            <button
              onClick={() => toggleSection(section.title)}
              className={cn(
                "flex w-full items-center justify-between rounded-lg px-3 py-2 text-[11px] font-semibold uppercase tracking-wider transition-colors cursor-pointer",
                hasActiveItem && !isOpen
                  ? "text-primary"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              <span>{section.title}</span>
              <ChevronDown
                className={cn(
                  "h-3.5 w-3.5 transition-transform duration-200",
                  isOpen ? "rotate-0" : "-rotate-90"
                )}
              />
            </button>

            {/* Collapsible items */}
            <div
              className={cn(
                "overflow-hidden transition-all duration-200",
                isOpen ? "max-h-96 opacity-100" : "max-h-0 opacity-0"
              )}
            >
              <div className="space-y-0.5 pt-1">
                {section.items.map((item) => {
                  const isActive =
                    item.href === "/admin"
                      ? pathname === "/admin"
                      : pathname.startsWith(item.href);

                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={onNavigate}
                      className={cn(
                        "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                        isActive
                          ? "bg-primary/10 text-primary"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground"
                      )}
                    >
                      <item.icon className="h-4 w-4 shrink-0" />
                      <span>{item.label}</span>
                    </Link>
                  );
                })}
              </div>
            </div>
          </div>
        );
      })}
    </nav>
  );
}
