"use client";

/**
 * Forgot Password page — request password reset link.
 * Premium glassmorphism design matching auth pages.
 */

import { useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authService } from "@/services/auth.service";
import { TEXT } from "@/constants/text";
import { Loader2, ArrowLeft, CheckCircle2, KeyRound } from "lucide-react";

export default function ForgotPasswordPage() {
  const [username, setUsername] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      await authService.forgotPassword(username);
      setSuccess(true);
    } catch {
      setError(TEXT.COMMON.ERROR);
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <Card className="glass-card shadow-2xl border-white/10 dark:border-white/5 bg-background/60 dark:bg-zinc-950/60 transition-all">
      <CardHeader className="text-center space-y-4 pb-6">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-amber-700 text-primary-foreground shadow-[0_0_20px_var(--color-primary)]">
          <KeyRound className="h-7 w-7" />
        </div>
        <div className="space-y-1">
          <CardTitle className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.FORGOT_PASSWORD_TITLE}
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            {TEXT.AUTH.FORGOT_PASSWORD_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        {success ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2">
            <div className="rounded-lg bg-emerald-500/10 px-4 py-4 text-sm text-emerald-500 border border-emerald-500/20 flex items-start gap-3">
              <CheckCircle2 className="h-5 w-5 mt-0.5 shrink-0" />
              <p>{TEXT.AUTH.FORGOT_PASSWORD_SUCCESS}</p>
            </div>
            <Link href="/login">
              <Button
                variant="outline"
                className="w-full h-11 cursor-pointer"
              >
                <ArrowLeft className="mr-2 h-4 w-4" />
                {TEXT.AUTH.BACK_TO_LOGIN}
              </Button>
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            {error && (
              <div className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive border border-destructive/20 animate-in fade-in">
                {error}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="username" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.USERNAME}
              </Label>
              <Input
                id="username"
                type="text"
                placeholder={TEXT.AUTH.USERNAME_PLACEHOLDER}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
              />
            </div>

            <Button
              type="submit"
              className="w-full h-11 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-md shadow-[0_4px_14px_0_rgba(245,158,11,0.39)] transition-all cursor-pointer"
              disabled={isLoading}
            >
              {isLoading ? <Loader2 className="mr-2 h-5 w-5 animate-spin" /> : null}
              {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.SEND_RESET_LINK}
            </Button>

            <div className="text-center mt-4">
              <Link
                href="/login"
                className="inline-flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                <ArrowLeft className="h-4 w-4" />
                {TEXT.AUTH.BACK_TO_LOGIN}
              </Link>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
