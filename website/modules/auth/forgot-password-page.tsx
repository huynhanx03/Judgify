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
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authService } from "@/services/auth.service";
import { TEXT } from "@/constants/text";
import { PASSWORD_RESET_REQUEST_STATUS } from "@/constants/identity";
import { APP_ROUTES } from "@/constants/routes";
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
      const response = await authService.forgotPassword(username);
      if (response.status !== PASSWORD_RESET_REQUEST_STATUS.ACCEPTED) {
        throw new Error(TEXT.AUTH.FORGOT_PASSWORD_FAILED);
      }
      setSuccess(true);
    } catch {
      setError(TEXT.AUTH.FORGOT_PASSWORD_FAILED);
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <Card className="glass-card border-border/50 bg-background/80 shadow-2xl">
      <CardHeader className="text-center space-y-4 pb-6">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-brand-soft">
          <KeyRound className="h-7 w-7" aria-hidden="true" />
        </div>
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.FORGOT_PASSWORD_TITLE}
          </h1>
          <CardDescription className="text-muted-foreground">
            {TEXT.AUTH.FORGOT_PASSWORD_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        {success ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none">
            <div className="flex items-start gap-3 rounded-lg border border-status-success/20 bg-status-success/10 px-4 py-4 text-sm text-status-success" role="status">
              <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
              <p>{TEXT.AUTH.FORGOT_PASSWORD_SUCCESS}</p>
            </div>
            <Link href={APP_ROUTES.LOGIN}>
              <Button
                variant="outline"
                className="w-full h-11 cursor-pointer"
              >
                <ArrowLeft className="mr-2 h-4 w-4" aria-hidden="true" />
                {TEXT.AUTH.BACK_TO_LOGIN}
              </Button>
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            {error && (
              <div className="animate-in fade-in rounded-lg border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive motion-reduce:animate-none" role="alert">
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
                autoComplete="username"
                required
                className="h-11 bg-muted/50 transition-colors focus-visible:bg-background focus-visible:ring-primary"
              />
            </div>

            <Button
              type="submit"
              className="button-primary-elevation h-11 w-full cursor-pointer bg-primary font-semibold text-primary-foreground transition-colors hover:bg-primary/90"
              disabled={isLoading}
            >
              {isLoading ? <Loader2 className="mr-2 h-5 w-5 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
              {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.SEND_RESET_LINK}
            </Button>

            <div className="text-center mt-4">
              <Link
                href={APP_ROUTES.LOGIN}
                className="inline-flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                <ArrowLeft className="h-4 w-4" aria-hidden="true" />
                {TEXT.AUTH.BACK_TO_LOGIN}
              </Link>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
