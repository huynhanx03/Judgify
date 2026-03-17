/**
 * AppShell provides the cohesive main layout.
 * Wraps all authenticated pages with the top Header.
 */

import { Header } from "./header";

interface AppShellProps {
  children: React.ReactNode;
}

/** Main application shell with top header navigation. */
export function AppShell({ children }: AppShellProps) {
  return (
    <div className="min-h-screen bg-background text-foreground selection:bg-primary/30 selection:text-primary flex flex-col relative">
      {/* Background ambient light */}
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-primary/5 blur-[120px] pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[30%] h-[30%] rounded-full bg-secondary/5 blur-[120px] pointer-events-none" />
      
      <Header />
      <main className="relative flex-1 p-6 pt-12 md:p-8 md:pt-16 max-w-[1400px] mx-auto w-full z-10">
        {children}
      </main>
    </div>
  );
}
