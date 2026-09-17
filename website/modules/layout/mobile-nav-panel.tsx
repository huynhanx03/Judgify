"use client";

import Link from "next/link";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { MAIN_NAV_ITEMS } from "@/constants/navigation";
import { text } from "@/i18n/text";
import { cn } from "@/lib/utils";

interface MobileNavPanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  triggerId: string;
  pathname: string;
}

export function MobileNavPanel({
  open,
  onOpenChange,
  triggerId,
  pathname,
}: MobileNavPanelProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange} triggerId={triggerId}>
      <SheetContent
        side="left"
        className="w-[86vw] max-w-sm gap-0 border-border/60 bg-background/95 p-0 backdrop-blur-xl"
      >
        <SheetHeader className="border-b border-border/60 p-5 pr-14">
          <SheetTitle className="text-lg font-bold">
            {text("NAV.MOBILE_MENU_TITLE")}
          </SheetTitle>
          <SheetDescription>
            {text("NAV.MOBILE_MENU_DESCRIPTION")}
          </SheetDescription>
        </SheetHeader>
        <nav
          className="flex flex-col gap-1 p-3"
          aria-label={text("NAV.MOBILE_MENU_TITLE")}
        >
          {MAIN_NAV_ITEMS.map((item) => {
            const isActive = pathname.startsWith(item.href);
            return (
              <SheetClose
                key={item.href}
                nativeButton={false}
                render={
                  <Link
                    href={item.href}
                    className={cn(
                      "flex min-h-12 items-center rounded-xl px-4 text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                      isActive
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground",
                    )}
                    aria-current={isActive ? "page" : undefined}
                  />
                }
              >
                {item.label}
              </SheetClose>
            );
          })}
        </nav>
      </SheetContent>
    </Sheet>
  );
}
