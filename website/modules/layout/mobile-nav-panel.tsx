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
import { cn } from "@/lib/utils";
import { TenantSelector } from "@/modules/layout/tenant-selector";
import { useLocale } from "@/contexts/locale-context";

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
  const { language, setLanguage, t } = useLocale();
  return (
    <Sheet open={open} onOpenChange={onOpenChange} triggerId={triggerId}>
      <SheetContent
        side="left"
        className="w-[86vw] max-w-sm gap-0 border-border/60 bg-background/95 p-0 backdrop-blur-xl"
      >
        <SheetHeader className="border-b border-border/60 p-5 pr-14">
          <SheetTitle className="text-lg font-bold">
            {t("NAV.MOBILE_MENU_TITLE")}
          </SheetTitle>
          <SheetDescription>
            {t("NAV.MOBILE_MENU_DESCRIPTION")}
          </SheetDescription>
        </SheetHeader>
        <TenantSelector mobile />
        <div className="px-4 pt-3">
          <button type="button" className="min-h-10 rounded-md border border-border px-3 text-sm" onClick={() => setLanguage(language === "vi" ? "en" : "vi")}>
            {language === "vi" ? "English" : "Tiếng Việt"}
          </button>
        </div>
        <nav
          className="flex flex-col gap-1 p-3"
          aria-label={t("NAV.MOBILE_MENU_TITLE")}
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
                {t(item.labelKey)}
              </SheetClose>
            );
          })}
        </nav>
      </SheetContent>
    </Sheet>
  );
}
