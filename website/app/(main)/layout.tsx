/**
 * Main product layout for public and authenticated workbench routes.
 */

import { AppShell } from "@/modules/layout/app-shell";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AppShell>{children}</AppShell>;
}
