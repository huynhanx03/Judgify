"use client";

/**
 * Admin dashboard top header with breadcrumb and user actions.
 */

import { usePathname, useRouter } from "next/navigation";
import { LogOut, PanelLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { clearTokens } from "@/lib/api-client";

/** Map route segments to display labels. */
const SEGMENT_LABELS: Record<string, string> = {
  admin: "Dashboard",
  problems: "Bài Tập",
  tags: "Tags",
  users: "Người Dùng",
  roles: "Phân Quyền",
};

interface AdminHeaderProps {
  onToggleMobileSidebar: () => void;
}

export function AdminHeader({ onToggleMobileSidebar }: AdminHeaderProps) {
  const pathname = usePathname();
  const router = useRouter();

  const segments = pathname.split("/").filter(Boolean);
  const currentLabel = SEGMENT_LABELS[segments[segments.length - 1]] ?? "Dashboard";

  function handleLogout() {
    clearTokens();
    router.push("/admin/login");
  }

  return (
    <header className="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-border bg-background/80 backdrop-blur-sm px-4 lg:px-6">
      {/* Mobile sidebar toggle */}
      <Button
        variant="ghost"
        size="icon"
        className="lg:hidden h-8 w-8 cursor-pointer"
        onClick={onToggleMobileSidebar}
      >
        <PanelLeft className="h-4 w-4" />
      </Button>

      {/* Breadcrumb */}
      <div className="flex items-center gap-1.5 text-sm">
        <span className="text-muted-foreground">Admin</span>
        <span className="text-muted-foreground/50">/</span>
        <span className="font-medium text-foreground">{currentLabel}</span>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Actions */}
      <Button
        variant="ghost"
        size="sm"
        onClick={handleLogout}
        className="gap-2 text-muted-foreground hover:text-foreground cursor-pointer"
      >
        <LogOut className="h-4 w-4" />
        <span className="hidden sm:inline">Đăng Xuất</span>
      </Button>
    </header>
  );
}
