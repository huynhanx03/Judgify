"use client";

import LoginForm from "./LoginForm";
import { AuthSurface } from "./auth-surface";

/**
 * Auth container for login page. Register shares the same focused surface.
 */
export default function AuthContainer() {
  return (
    <AuthSurface className="mx-auto max-w-lg overflow-hidden" framed>
      <div className="p-6 lg:p-10 relative z-10">
        <LoginForm />
      </div>
    </AuthSurface>
  );
}
