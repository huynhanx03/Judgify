"use client";

/**
 * Top Header Navigation component.
 * Auth-aware: shows login/register when unauthenticated, avatar/notification when authenticated.
 */

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { Bell, LogIn, UserPlus, LogOut } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { buttonVariants } from "@/components/ui/button";
import { MAIN_NAV_ITEMS } from "@/constants/navigation";
import { TEXT } from "@/constants/text";
import { useAuth } from "@/contexts/auth-context";

export function Header() {
  const pathname = usePathname();
  const { isAuthenticated, isLoading } = useAuth();

  return (
    <div className="sticky top-4 z-40 w-full px-4 sm:px-6 lg:px-8 flex justify-center transition-all">
      <header className="flex h-16 w-full max-w-[1400px] items-center justify-between rounded-2xl border border-border/40 bg-background/70 px-4 sm:px-6 backdrop-blur-xl shadow-sm dark:shadow-none transition-all">
        {/* Left: Logo & Main Navigation */}
        <div className="flex items-center gap-6 lg:gap-10">
          <Link href="/arena" className="flex items-center gap-3 shrink-0 group">
            <div className="relative w-8 h-8 rounded-lg overflow-hidden border border-primary/50 shadow-[0_0_10px_rgba(245,158,11,0.5)]">
              <img src="/images/logo.png" alt="Judgify" className="w-full h-full object-cover" />
            </div>
            <h1 className="text-xl font-bold tracking-tight heading-gaming text-transparent bg-clip-text bg-gradient-to-r from-amber-500 via-amber-400 to-amber-200 drop-shadow-[0_0_8px_rgba(245,158,11,0.5)] transition-transform group-hover:scale-105 hidden sm:block">
              {TEXT.APP_NAME}
            </h1>
          </Link>

          <div className="h-6 w-px bg-border/60 hidden md:block"></div>

          <nav className="flex items-center gap-3 sm:gap-6 overflow-x-auto no-scrollbar mask-edges">
            {MAIN_NAV_ITEMS.map((item) => {
              const isActive = pathname.startsWith(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={cn(
                    "relative flex items-center gap-2 text-sm font-medium transition-all duration-300 py-2 whitespace-nowrap group",
                    isActive ? "text-primary drop-shadow-[0_0_8px_rgba(245,158,11,0.5)]" : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  {item.label}
                  {isActive && (
                    <span className="absolute -bottom-[2px] left-1/2 -translate-x-1/2 w-4/5 h-[2px] bg-gradient-to-r from-transparent via-primary to-transparent rounded-full shadow-[0_0_10px_2px_rgba(245,158,11,0.6)] animate-pulse" />
                  )}
                  {!isActive && (
                    <span className="absolute -bottom-[2px] left-1/2 -translate-x-1/2 w-0 h-[2px] bg-primary/50 rounded-full transition-all duration-300 group-hover:w-1/2" />
                  )}
                </Link>
              );
            })}
          </nav>
        </div>

        {/* Right: Actions */}
        <div className="flex items-center gap-2 sm:gap-4 shrink-0 ml-auto">
          {/* <ThemeToggle /> */}

          {!isLoading && (
            isAuthenticated ? (
              <AuthenticatedActions />
            ) : (
              <GuestActions />
            )
          )}
        </div>
      </header>
    </div>
  );
}

/** Notification bell + avatar + username for logged-in users. */
function AuthenticatedActions() {
  const { user, logout } = useAuth();
  const displayName = user?.username || TEXT.HEADER.AVATAR_FALLBACK;
  const initials = displayName.slice(0, 2).toUpperCase();

  return (
    <>
      <button className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground transition-colors focus:outline-none focus:ring-2 focus:ring-ring relative">
        <Bell className="h-5 w-5" />
        <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-primary glow-amber"></span>
      </button>

      <div className="h-6 w-px bg-border/60 mx-1 hidden sm:block"></div>

      <Link href="/profile" className="flex items-center gap-2 rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring transition-transform hover:scale-105">
        <Avatar className="h-8 w-8 border border-primary/30 cursor-pointer">
          <AvatarImage src="/images/default_avatar.png" alt={displayName} />
          <AvatarFallback className="bg-primary/20 text-primary font-bold text-xs">
            {initials}
          </AvatarFallback>
        </Avatar>
        <span className="text-sm font-medium text-foreground hidden sm:inline max-w-[120px] truncate">
          {displayName}
        </span>
      </Link>

      <button
        onClick={logout}
        className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
        title="Đăng xuất"
      >
        <LogOut className="h-4 w-4" />
      </button>
    </>
  );
}

/** Login + Register buttons for guests. */
function GuestActions() {
  return (
    <>
      <div className="h-6 w-px bg-border/60 mx-1 hidden sm:block"></div>

      <Link href="/login" className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "text-muted-foreground hover:text-foreground gap-1.5")}>
        <LogIn className="h-4 w-4" />
        <span className="hidden sm:inline">{TEXT.AUTH.LOGIN}</span>
      </Link>

      <Link href="/register" className={cn(buttonVariants({ size: "sm" }), "bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-600 hover:to-amber-700 text-white shadow-[0_0_12px_rgba(245,158,11,0.3)] gap-1.5")}>
        <UserPlus className="h-4 w-4" />
        <span className="hidden sm:inline">{TEXT.AUTH.REGISTER}</span>
      </Link>
    </>
  );
}
