"use client";

import { useCallback, useRef, useState } from "react";
import { useAuth } from "@/contexts/auth-context";
import { text } from "@/i18n/text";
import { getErrorMessage, notify } from "@/lib/toast";

/** One guarded, observable logout path shared by every product shell. */
export function useLogoutAction() {
  const { logout } = useAuth();
  const pendingRef = useRef(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const performLogout = useCallback(
    async (destination?: string): Promise<void> => {
      if (pendingRef.current) return;
      pendingRef.current = true;
      setIsLoggingOut(true);
      try {
        await logout(destination);
      } catch (error) {
        notify.error(getErrorMessage(error, text("NAV.LOGOUT_FAILED")));
      } finally {
        pendingRef.current = false;
        setIsLoggingOut(false);
      }
    },
    [logout],
  );

  return { isLoggingOut, performLogout } as const;
}
