"use client";

/**
 * Admin dashboard layout — auth guard + sidebar shell.
 * Redirects to /admin/login if no access token found in localStorage.
 */

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { AdminShell } from "@/modules/admin/admin-shell";
import { Loader2 } from "lucide-react";

export default function AdminDashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const [isChecking, setIsChecking] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem("judgify_access_token");
    if (!token) {
      router.replace("/admin/login");
    } else {
      setIsChecking(false);
    }
  }, [router]);

  if (isChecking) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  return <AdminShell>{children}</AdminShell>;
}
