"use client";

import { Card } from "@/components/ui/card";
import LoginForm from "./LoginForm";

/**
 * Auth container for login page — glassmorphism card with decorative corners.
 * Only holds login form now. Register has its own full page.
 */
export default function AuthContainer() {
  return (
    <Card className="glass-card shadow-2xl border-white/10 dark:border-white/5 bg-background/60 dark:bg-zinc-950/60 backdrop-blur-xl overflow-hidden relative w-full">
      {/* Decorative corners */}
      <div className="absolute top-0 left-0 w-16 h-16 border-t-2 border-l-2 border-primary/30 rounded-tl-xl pointer-events-none" />
      <div className="absolute top-0 right-0 w-16 h-16 border-t-2 border-r-2 border-primary/30 rounded-tr-xl pointer-events-none" />
      <div className="absolute bottom-0 left-0 w-16 h-16 border-b-2 border-l-2 border-primary/30 rounded-bl-xl pointer-events-none" />
      <div className="absolute bottom-0 right-0 w-16 h-16 border-b-2 border-r-2 border-primary/30 rounded-br-xl pointer-events-none" />

      <div className="p-6 lg:p-10 relative z-10">
        <LoginForm />
      </div>
    </Card>
  );
}
