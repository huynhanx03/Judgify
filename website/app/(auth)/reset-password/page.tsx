"use client";

/**
 * Reset Password page — set new password with token from URL.
 * Premium glassmorphism design matching auth pages.
 */

import { useState, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
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
import { resetPassword } from "@/services/auth.service";
import { TEXT } from "@/constants/text";
import { Loader2, CheckCircle2, ShieldCheck, Eye, EyeOff } from "lucide-react";

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (newPassword !== confirmPassword) {
      setError(TEXT.AUTH.PASSWORD_MISMATCH);
      return;
    }

    setIsLoading(true);
    try {
      await resetPassword(token, newPassword);
      setSuccess(true);
      setTimeout(() => router.push("/login"), 2000);
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
          <ShieldCheck className="h-7 w-7" />
        </div>
        <div className="space-y-1">
          <CardTitle className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.RESET_PASSWORD_TITLE}
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            {TEXT.AUTH.RESET_PASSWORD_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        {success ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2">
            <div className="rounded-lg bg-emerald-500/10 px-4 py-4 text-sm text-emerald-500 border border-emerald-500/20 flex items-center gap-3">
              <CheckCircle2 className="h-5 w-5 shrink-0" />
              <p>{TEXT.AUTH.RESET_PASSWORD_SUCCESS}</p>
            </div>
            <p className="text-center text-sm text-muted-foreground">
              Đang chuyển hướng đến trang đăng nhập...
            </p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            {error && (
              <div className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive border border-destructive/20 animate-in fade-in">
                {error}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="newPassword" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.NEW_PASSWORD}
              </Label>
              <div className="relative">
                <Input
                  id="newPassword"
                  type={showPassword ? "text" : "password"}
                  placeholder={TEXT.AUTH.NEW_PASSWORD_PLACEHOLDER}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="confirmPassword" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.CONFIRM_PASSWORD}
              </Label>
              <div className="relative">
                <Input
                  id="confirmPassword"
                  type={showConfirm ? "text" : "password"}
                  placeholder={TEXT.AUTH.CONFIRM_PASSWORD_PLACEHOLDER}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowConfirm(!showConfirm)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  {showConfirm ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>

            <Button
              type="submit"
              className="w-full h-11 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-md shadow-[0_4px_14px_0_rgba(245,158,11,0.39)] transition-all cursor-pointer"
              disabled={isLoading}
            >
              {isLoading ? <Loader2 className="mr-2 h-5 w-5 animate-spin" /> : null}
              {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.RESET_PASSWORD}
            </Button>

            <div className="text-center mt-4">
              <Link
                href="/login"
                className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                {TEXT.AUTH.BACK_TO_LOGIN}
              </Link>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense>
      <ResetPasswordForm />
    </Suspense>
  );
}
