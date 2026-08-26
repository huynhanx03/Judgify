/**
 * Focused authentication layout with restrained, motion-free brand context.
 */

import type { Metadata } from "next";

export const metadata: Metadata = {
  referrer: "no-referrer",
};

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main
      id="main-content"
      className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background"
    >
      <div
        aria-hidden="true"
        className="absolute inset-0 bg-cover bg-center opacity-[0.08] grayscale dark:opacity-[0.12]"
        style={{
          backgroundImage: "url('/images/auth_bg.png')",
        }}
      />
      <div
        aria-hidden="true"
        className="absolute inset-0 bg-gradient-to-br from-primary/8 via-background/96 to-cultivation/8"
      />
      <div className="relative z-10 w-full max-w-6xl px-4 py-8 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-200 sm:px-8">
        {children}
      </div>
    </main>
  );
}
