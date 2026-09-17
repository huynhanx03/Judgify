"use client";

/**
 * Top Header Navigation component.
 * Auth-aware: shows login/register when unauthenticated and profile actions when authenticated.
 */

import { useState, useCallback } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import dynamic from "next/dynamic";
import { cn } from "@/lib/utils";
import { LogIn, Menu, UserPlus } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { MAIN_NAV_ITEMS } from "@/constants/navigation";
import { useAuth } from "@/contexts/auth-context";
import { BrandMark } from "@/components/brand-mark";
import { ThemeToggle } from "@/components/theme-toggle";
import { APP_ROUTES } from "@/constants/routes";
import { text } from "@/i18n/text";

const MOBILE_NAV_TRIGGER_ID = "mobile-nav-trigger";

const MobileNavPanel = dynamic(
  () =>
    import("@/modules/layout/mobile-nav-panel").then((m) => m.MobileNavPanel),
  { ssr: false },
);

const AuthenticatedActions = dynamic(
  () =>
    import("@/modules/layout/authenticated-actions").then(
      (m) => m.AuthenticatedActions,
    ),
  { ssr: false },
);

function preloadMobileNavPanel() {
  void import("@/modules/layout/mobile-nav-panel");
}

function preloadAuthenticatedActions() {
  void import("@/modules/layout/authenticated-actions");
}

export function Header() {
  const pathname = usePathname();
  const { isAuthenticated, isLoading } = useAuth();

  return (
    <div className="sticky top-0 z-40 flex w-full justify-center border-b border-border bg-surface/95 backdrop-blur-lg">
      <header className="flex min-h-16 w-full max-w-[1400px] items-center gap-3 px-4 sm:px-6 lg:px-8">
        {/* Left: Logo & Main Navigation */}
        <div className="flex min-w-0 flex-1 items-center gap-4 lg:gap-9">
          <Link
            href={APP_ROUTES.ARENA}
            className="group flex min-h-11 shrink-0 items-center gap-2.5 rounded-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <BrandMark className="size-8" priority />
            <span className="hidden text-lg font-semibold tracking-tight text-foreground transition-colors duration-150 group-hover:text-primary sm:block">
              {text("APP_NAME")}
            </span>
          </Link>

          <nav
            className="no-scrollbar mask-edges hidden min-w-0 flex-1 items-center gap-1 overflow-x-auto md:flex"
            aria-label={text("NAV.PRIMARY_LABEL")}
          >
            {MAIN_NAV_ITEMS.map((item) => {
              const isActive = pathname.startsWith(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={cn(
                    "group relative flex min-h-11 items-center px-3 py-2 text-sm font-medium whitespace-nowrap transition-colors duration-150 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                    isActive
                      ? "text-primary"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                  aria-current={isActive ? "page" : undefined}
                >
                  {item.label}
                  {isActive && (
                    <span className="absolute inset-x-3 bottom-0 h-0.5 rounded-full bg-primary" />
                  )}
                </Link>
              );
            })}
          </nav>

          <MobileNavigation pathname={pathname} />
        </div>

        {/* Right: Actions */}
        <div className="ml-auto flex shrink-0 items-center gap-2 sm:gap-3">
          {!isLoading &&
            (isAuthenticated ? (
              <AuthenticatedActions />
            ) : (
              <GuestActions onPointerEnter={preloadAuthenticatedActions} />
            ))}
          <ThemeToggle />
        </div>
      </header>
    </div>
  );
}

function MobileNavigation({ pathname }: { pathname: string }) {
  const [open, setOpen] = useState(false);
  const [panelMounted, setPanelMounted] = useState(false);

  const handleOpen = useCallback(() => {
    setPanelMounted(true);
    setOpen(true);
  }, []);

  const handlePreload = useCallback(() => {
    preloadMobileNavPanel();
  }, []);

  return (
    <>
      <Button
        id={MOBILE_NAV_TRIGGER_ID}
        type="button"
        variant="ghost"
        size="icon"
        className="size-11 cursor-pointer md:hidden"
        aria-label={text("NAV.OPEN_MENU")}
        aria-expanded={open}
        aria-controls={open ? "mobile-nav-panel" : undefined}
        onClick={handleOpen}
        onPointerEnter={handlePreload}
        onFocus={handlePreload}
      >
        <Menu className="size-5" aria-hidden="true" />
      </Button>
      {panelMounted && (
        <MobileNavPanel
          open={open}
          onOpenChange={setOpen}
          triggerId={MOBILE_NAV_TRIGGER_ID}
          pathname={pathname}
        />
      )}
    </>
  );
}

/** Login + Register buttons for guests. */
function GuestActions({ onPointerEnter }: { onPointerEnter?: () => void }) {
  return (
    <>
      <Link
        href={APP_ROUTES.LOGIN}
        className={cn(
          buttonVariants({ variant: "ghost", size: "sm" }),
          "text-muted-foreground hover:text-foreground",
        )}
        onPointerEnter={onPointerEnter}
      >
        <LogIn className="size-4" aria-hidden="true" />
        <span>{text("AUTH.LOGIN")}</span>
      </Link>

      <Link
        href={APP_ROUTES.REGISTER}
        className={buttonVariants({ size: "sm" })}
      >
        <UserPlus className="size-4" aria-hidden="true" />
        <span>{text("AUTH.REGISTER")}</span>
      </Link>
    </>
  );
}
