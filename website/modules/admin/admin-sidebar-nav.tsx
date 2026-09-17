"use client";

/**
 * Admin sidebar navigation links — collapsible sections.
 * Click section header to expand/collapse items.
 */

import { useMemo } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Collapsible } from "@base-ui/react/collapsible";
import { ChevronDown } from "lucide-react";
import { ADMIN_NAV_SECTIONS } from "@/constants/admin-navigation";
import { cn } from "@/lib/utils";
import { useAuth } from "@/contexts/auth-context";
import { filterAuthorizedSections } from "@/lib/auth/admin-policy";
import { adminText } from "@/i18n/admin-text";
import { APP_ROUTES } from "@/constants/routes";

interface AdminSidebarNavProps {
  onNavigate?: () => void;
}

export function AdminSidebarNav({ onNavigate }: AdminSidebarNavProps) {
  const pathname = usePathname();
  const { capabilityIndex } = useAuth();
  const visibleSections = useMemo(
    () => filterAuthorizedSections(ADMIN_NAV_SECTIONS, capabilityIndex),
    [capabilityIndex],
  );
  const activeHref = useMemo(
    () =>
      visibleSections
        .flatMap((section) => section.items)
        .filter((item) =>
          item.href === APP_ROUTES.ADMIN
            ? pathname === APP_ROUTES.ADMIN
            : pathname === item.href || pathname.startsWith(`${item.href}/`),
        )
        .sort((left, right) => right.href.length - left.href.length)[0]?.href,
    [pathname, visibleSections],
  );

  return (
    <nav
      className="space-y-2"
      aria-label={adminText("NAVIGATION_LABEL")}
    >
      {visibleSections.map((section) => {
        const hasActiveItem = section.items.some((item) => item.href === activeHref);

        return (
          <Collapsible.Root key={section.titleKey} defaultOpen>
            <Collapsible.Trigger
              type="button"
              className={cn(
                "group flex min-h-10 w-full items-center justify-between rounded-lg px-3 py-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground outline-none transition-colors duration-150 hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50",
                hasActiveItem && "data-[panel-open]:text-primary",
              )}
            >
              <span>{section.title}</span>
              <ChevronDown
                className="size-3.5 -rotate-90 transition-transform duration-200 group-data-[panel-open]:rotate-0"
                aria-hidden="true"
              />
            </Collapsible.Trigger>

            <Collapsible.Panel
              className="h-[var(--collapsible-panel-height)] overflow-hidden transition-[height,opacity] duration-200 data-ending-style:h-0 data-ending-style:opacity-0 data-starting-style:h-0 data-starting-style:opacity-0"
            >
              <div className="space-y-0.5 pt-1">
                {section.items.map((item) => {
                  const isActive = item.href === activeHref;

                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={onNavigate}
                      className={cn(
                        "flex min-h-11 items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-ring/50",
                        isActive
                          ? "bg-primary/10 text-primary"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground",
                      )}
                      aria-current={isActive ? "page" : undefined}
                    >
                      <item.icon
                        className="size-4 shrink-0"
                        aria-hidden="true"
                      />
                      <span>{item.label}</span>
                    </Link>
                  );
                })}
              </div>
            </Collapsible.Panel>
          </Collapsible.Root>
        );
      })}
    </nav>
  );
}
