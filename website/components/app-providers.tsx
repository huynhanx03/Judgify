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

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <SessionProvider>
      <RealtimeProvider>
        <AuthorizationRealtimeSynchronizer />
        <NotificationProvider>{children}</NotificationProvider>
      </RealtimeProvider>
    </SessionProvider>
  );
}
