"use client";

/**
 * Top Header Navigation component.
 * Premium floating capsule layout with logo, main navigation links, and right-aligned actions.
 */

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { Bell } from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { MAIN_NAV_ITEMS } from "@/constants/navigation";
import { TEXT } from "@/constants/text";

export function Header() {
  const pathname = usePathname();

  return (
    <div className="sticky top-4 z-40 w-full px-4 sm:px-6 lg:px-8 flex justify-center transition-all">
      <header className="flex h-16 w-full max-w-[1400px] items-center justify-between rounded-2xl border border-border/40 bg-background/70 px-4 sm:px-6 backdrop-blur-xl shadow-sm dark:shadow-none transition-all">
        {/* Left: Logo & Main Navigation */}
        <div className="flex items-center gap-6 lg:gap-10">
          <Link href="/arena" className="flex items-center gap-3 shrink-0 group">
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
          <ThemeToggle />
          
          <button className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground transition-colors focus:outline-none focus:ring-2 focus:ring-ring relative">
            <Bell className="h-5 w-5" />
            <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-primary glow-amber"></span>
          </button>

          <div className="h-6 w-px bg-border/60 mx-1 hidden sm:block"></div>

          {/* User avatar — click to go to profile */}
          <Link href="/profile" className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring transition-transform hover:scale-105">
            <Avatar className="h-8 w-8 border border-border/50 cursor-pointer">
              <AvatarFallback className="bg-primary/20 text-primary font-bold text-xs">
                {TEXT.HEADER.AVATAR_FALLBACK}
              </AvatarFallback>
            </Avatar>
          </Link>
        </div>
      </header>
    </div>
  );
}
