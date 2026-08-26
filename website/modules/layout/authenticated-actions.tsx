"use client";

import Link from "next/link";
import { Loader2, LogOut } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { useAuth } from "@/contexts/auth-context";
import { APP_ROUTES } from "@/constants/routes";
import { text } from "@/i18n/text";
import { useLogoutAction } from "@/modules/auth/use-logout-action";
import { NotificationMenu } from "@/modules/notifications/notification-menu";

export function AuthenticatedActions() {
  const { user } = useAuth();
  const { isLoggingOut, performLogout } = useLogoutAction();
  const displayName = user?.username || text("HEADER.AVATAR_FALLBACK");
  const initials = displayName.slice(0, 2).toUpperCase();

  return (
    <>
      <div className="h-6 w-px bg-border/60 mx-1 hidden sm:block"></div>

      <NotificationMenu />

      <Link
        href={APP_ROUTES.PROFILE}
        className="flex min-h-11 items-center gap-2 rounded-lg px-1 outline-none transition-colors duration-150 hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/50"
      >
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
        type="button"
        onClick={() => void performLogout()}
        disabled={isLoggingOut}
        className="flex size-11 items-center justify-center rounded-lg text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        title={isLoggingOut ? text("NAV.LOGGING_OUT") : text("NAV.LOGOUT")}
        aria-label={isLoggingOut ? text("NAV.LOGGING_OUT") : text("NAV.LOGOUT")}
      >
        {isLoggingOut ? (
          <Loader2
            className="h-4 w-4 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />
        ) : (
          <LogOut className="h-4 w-4" aria-hidden="true" />
        )}
      </button>
    </>
  );
}
