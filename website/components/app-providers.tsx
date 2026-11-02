"use client";

/**
 * Client-side providers wrapper.
 * Combines all client providers in one component
 * so the root layout stays a Server Component.
 */

import type { ReactNode } from "react";
import { NotificationProvider } from "@/contexts/notification-context";
import {
  AuthorizationRealtimeSynchronizer,
  RealtimeProvider,
} from "@/contexts/realtime-context";
import { SessionProvider } from "@/contexts/session-context";
import { TenantProvider } from "@/contexts/tenant-context";
import { LocaleProvider } from "@/contexts/locale-context";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <LocaleProvider>
    <SessionProvider>
      <TenantProvider>
        <RealtimeProvider>
          <AuthorizationRealtimeSynchronizer />
          <NotificationProvider>{children}</NotificationProvider>
        </RealtimeProvider>
      </TenantProvider>
    </SessionProvider>
    </LocaleProvider>
  );
}
