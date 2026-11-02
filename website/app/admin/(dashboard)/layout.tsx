"use client";

/** Admin dashboard shell with explicit authentication and capability states. */

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/contexts/auth-context";
import { AdminShell } from "@/modules/admin/admin-shell";
import {
  AdminAccessLoading,
  AdminAccessState,
} from "@/modules/admin/admin-access-state";
import { adminRouteAllowed } from "@/lib/auth/admin-policy";
import {
  AUTH_BOOTSTRAP_STATUS,
  CAPABILITY_STATUS,
} from "@/constants/authorization";
import { APP_ROUTES } from "@/constants/routes";

export default function AdminDashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const {
    isAuthenticated,
    isLoading,
    bootstrapStatus,
    capabilityStatus,
    capabilityIndex,
    refreshCapabilities,
    retryBootstrap,
  } = useAuth();

  useEffect(() => {
    if (
      bootstrapStatus === AUTH_BOOTSTRAP_STATUS.READY &&
      !isAuthenticated
    ) {
      router.replace(APP_ROUTES.ADMIN_LOGIN);
    }
  }, [bootstrapStatus, isAuthenticated, router]);

  if (isLoading || bootstrapStatus === AUTH_BOOTSTRAP_STATUS.LOADING) {
    return <AdminAccessLoading fullScreen />;
  }

  if (bootstrapStatus === AUTH_BOOTSTRAP_STATUS.ERROR) {
    return (
      <AdminAccessState
        kind="unavailable"
        onRetry={retryBootstrap}
        fullScreen
      />
    );
  }

  if (!isAuthenticated) return <AdminAccessLoading fullScreen />;

  if (capabilityStatus === CAPABILITY_STATUS.ERROR) {
    return (
      <AdminShell>
        <AdminAccessState
          kind="unavailable"
          onRetry={() => void refreshCapabilities()}
        />
      </AdminShell>
    );
  }

  if (capabilityStatus !== CAPABILITY_STATUS.READY) {
    return (
      <AdminShell>
        <AdminAccessLoading />
      </AdminShell>
    );
  }

  if (!adminRouteAllowed(pathname, capabilityIndex)) {
    return (
      <AdminShell>
        <AdminAccessState kind="forbidden" />
      </AdminShell>
    );
  }

  return <AdminShell>{children}</AdminShell>;
}
