"use client";

/**
 * Theme-aware toast notification container.
 * Reads current theme from next-themes and passes it to sonner.
 */

import { Toaster } from "sonner";
import { useTheme } from "next-themes";
import { text } from "@/i18n/text";

export function AppToasterContent() {
  const { theme } = useTheme();
  const toasterTheme = theme === "light" || theme === "dark" ? theme : "system";

  return (
    <Toaster
      position="bottom-right"
      richColors
      closeButton
      containerAriaLabel={text("COMMON.NOTIFICATIONS")}
      theme={toasterTheme}
      toastOptions={{
        closeButtonAriaLabel: text("COMMON.CLOSE_NOTIFICATION"),
        classNames: {
          toast: "border-border bg-surface-raised text-foreground",
          description: "text-muted-foreground",
        },
      }}
    />
  );
}
