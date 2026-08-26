"use client";

/**
 * Admin login page — separate from user login.
 * Clean minimal design for admin access.
 */

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/contexts/auth-context";
import { Loader2, Shield } from "lucide-react";
import { TEXT } from "@/constants/text";
import { APP_ROUTES } from "@/constants/routes";

export default function AdminLoginPage() {
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      await login({ username, password }, APP_ROUTES.ADMIN);
    } catch {
      setError(TEXT.AUTH.ADMIN_INVALID_CREDENTIALS);
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <main
      id="main-content"
      className="relative flex min-h-screen items-center justify-center bg-background"
    >
      {/* Subtle background grid */}
      <div className="absolute inset-0 surface-grid-wide" />

      <div className="z-10 w-full max-w-sm px-4">
        <Card className="border-border shadow-xl">
          <CardHeader className="text-center space-y-3 pb-4">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Shield className="h-6 w-6" aria-hidden="true" />
            </div>
            <div className="space-y-1">
              <h1 className="text-xl font-bold tracking-tight">
                {TEXT.AUTH.ADMIN_TITLE}
              </h1>
              <CardDescription className="text-muted-foreground text-sm">
                {TEXT.AUTH.ADMIN_SUBTITLE}
              </CardDescription>
            </div>
          </CardHeader>

          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4" aria-busy={isLoading}>
              {error && (
                <div className="rounded-lg bg-destructive/10 px-3 py-2.5 text-sm text-destructive border border-destructive/20" role="alert">
                  {error}
                </div>
              )}

              <div className="space-y-2">
                <Label htmlFor="username" className="text-xs font-medium text-muted-foreground">
                  {TEXT.AUTH.ADMIN_USERNAME}
                </Label>
                <Input
                  id="username"
                  type="text"
                  placeholder={TEXT.AUTH.ADMIN_USERNAME_PLACEHOLDER}
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  disabled={isLoading}
                  autoComplete="username"
                  className="h-10"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="password" className="text-xs font-medium text-muted-foreground">
                  {TEXT.AUTH.ADMIN_PASSWORD}
                </Label>
                <Input
                  id="password"
                  type="password"
                  placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  disabled={isLoading}
                  autoComplete="current-password"
                  className="h-10"
                />
              </div>

              <Button
                type="submit"
                className="w-full h-10 font-semibold cursor-pointer"
                disabled={isLoading}
              >
                {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
                {isLoading ? TEXT.AUTH.ADMIN_VERIFYING : TEXT.AUTH.LOGIN}
              </Button>
            </form>
          </CardContent>
        </Card>

        <p className="text-center text-xs text-muted-foreground/50 mt-4">
          {TEXT.AUTH.ADMIN_PRODUCT_NAME}
        </p>
      </div>
    </main>
  );
}
