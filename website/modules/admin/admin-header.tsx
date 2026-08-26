"use client";

/**
 * Admin dashboard top header with breadcrumb and user actions.
 */

import { useMemo } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Loader2, LogOut, PanelLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-toggle";
import { ADMIN_NAV_SECTIONS } from "@/constants/admin-navigation";
import { APP_ROUTES } from "@/constants/routes";
import { text } from "@/i18n/text";
import { adminText } from "@/i18n/admin-text";
import { useLogoutAction } from "@/modules/auth/use-logout-action";

interface AdminHeaderProps {
  onToggleMobileSidebar: () => void;
}

export function AdminHeader({ onToggleMobileSidebar }: AdminHeaderProps) {
  const pathname = usePathname();
  const { isLoggingOut, performLogout } = useLogoutAction();

  const currentLabel = useMemo(() => {
    if (pathname === APP_ROUTES.ADMIN_PROBLEM_CREATE) {
      return adminText("PROBLEM_FORM.CREATE_TITLE");
    }
    if (/^\/admin\/problems\/[^/]+\/edit$/.test(pathname)) {
      return adminText("PROBLEM_FORM.EDIT_TITLE");
    }
    return (
      ADMIN_NAV_SECTIONS.flatMap((section) => section.items)
        .filter((item) =>
          item.href === APP_ROUTES.ADMIN
            ? pathname === APP_ROUTES.ADMIN
            : pathname === item.href || pathname.startsWith(`${item.href}/`),
        )
        .sort((left, right) => right.href.length - left.href.length)[0]?.label ??
      adminText("DASHBOARD_TITLE")
    );
  }, [pathname]);

  function handleLogout() {
    void performLogout(APP_ROUTES.ADMIN_LOGIN);
  }

  return (
    <header className="sticky top-0 z-20 flex h-16 items-center gap-2 border-b border-border bg-background/90 px-3 backdrop-blur-lg sm:gap-3 sm:px-6 lg:px-8">
      <Button
        variant="ghost"
        size="icon"
        className="lg:hidden"
        onClick={onToggleMobileSidebar}
        aria-label={text("NAV.OPEN_SIDEBAR")}
      >
        <PanelLeft className="size-5" aria-hidden="true" />
      </Button>

      <nav
        className="flex min-w-0 items-center gap-1.5 text-sm"
        aria-label={adminText("BREADCRUMB_LABEL")}
      >
        <Link
          href={APP_ROUTES.ADMIN}
          className="shrink-0 rounded-md px-1 py-2 text-muted-foreground outline-none transition-colors duration-150 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
        >
          {adminText("BREADCRUMB_ROOT")}
        </Link>
        <span className="text-muted-foreground/50" aria-hidden="true">
          /
        </span>
        <span
          className="truncate font-medium text-foreground"
          aria-current="page"
        >
          {currentLabel}
        </span>
      </nav>

      <div className="flex-1" />

      <ThemeToggle />
      <Button
        variant="ghost"
        size="sm"
        onClick={handleLogout}
        disabled={isLoggingOut}
        className="text-muted-foreground hover:text-foreground"
        aria-label={isLoggingOut ? text("NAV.LOGGING_OUT") : adminText("LOGOUT")}
      >
        {isLoggingOut ? (
          <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
        ) : (
          <LogOut className="size-4" aria-hidden="true" />
        )}
        <span className="hidden sm:inline">
          {isLoggingOut ? text("NAV.LOGGING_OUT") : adminText("LOGOUT")}
        </span>
      </Button>
    </header>
  );
}
