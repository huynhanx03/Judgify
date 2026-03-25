/**
 * Main app layout with sidebar and header.
 * All authenticated pages under (main) route group use this layout.
 */

import { AppShell } from "@/modules/layout/app-shell";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AppShell>{children}</AppShell>;
}
