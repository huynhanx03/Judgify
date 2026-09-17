/**
 * AppShell provides the cohesive main layout.
 * Wraps all authenticated pages with the top Header.
 */

import { Header } from "./header";
import { SkipLink } from "@/components/skip-link";

interface AppShellProps {
  children: React.ReactNode;
}

/** Main application shell with top header navigation. */
export function AppShell({ children }: AppShellProps) {
  return (
    <div className="relative flex min-h-screen flex-col bg-background text-foreground selection:bg-primary/20 selection:text-foreground">
      <SkipLink />
      <Header />
      <main
        id="main-content"
        tabIndex={-1}
        className="relative mx-auto w-full max-w-[1400px] flex-1 p-4 py-8 outline-none sm:p-6 sm:py-10 lg:p-8"
      >
        {children}
      </main>
    </div>
  );
}
