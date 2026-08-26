import Image from "next/image";
import Link from "next/link";

import { APP_ROUTES } from "@/constants/routes";
import { adminText } from "@/i18n/admin-text";
import { cn } from "@/lib/utils";

interface AdminBrandProps {
  className?: string;
  onNavigate?: () => void;
}

/** Shared, focusable admin product mark used by every sidebar variant. */
export function AdminBrand({ className, onNavigate }: AdminBrandProps) {
  return (
    <Link
      href={APP_ROUTES.ADMIN}
      onClick={onNavigate}
      className={cn(
        "group flex min-h-11 min-w-0 items-center gap-3 rounded-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
        className,
      )}
    >
      <span className="relative size-9 shrink-0 overflow-hidden rounded-xl border border-border-strong bg-surface-raised shadow-sm">
        <Image
          src="/images/logo.png"
          alt=""
          width={36}
          height={36}
          priority
          aria-hidden="true"
          className="size-full object-cover"
        />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-sm font-bold tracking-tight text-foreground transition-colors duration-150 group-hover:text-primary">
          {adminText("PRODUCT_NAME")}
        </span>
        <span className="block truncate text-[11px] font-medium text-muted-foreground">
          {adminText("DASHBOARD_SUBTITLE")}
        </span>
      </span>
    </Link>
  );
}
