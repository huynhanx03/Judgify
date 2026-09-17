"use client";

/**
 * Theme-aware toast notification container.
 * Reads current theme from next-themes and passes it to sonner.
 */

import dynamic from "next/dynamic";

const AppToasterContent = dynamic(
  () =>
    import("@/components/app-toaster-content").then(
      (m) => m.AppToasterContent,
    ),
  { ssr: false },
);

export function AppToaster() {
  return <AppToasterContent />;
}
