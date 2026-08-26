"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  Eye,
  EyeOff,
  Loader2,
  ShieldAlert,
  ShieldCheck,
} from "lucide-react";

import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { ApiError } from "@/lib/api/error";
import { consumeOneTimeTokenFragment } from "@/lib/auth/one-time-token";
import { authService } from "@/services/auth.service";

const LOGIN_REDIRECT_DELAY_MS = 2_000;

type ResetTokenState =
  | { status: "loading"; token: "" }
  | { status: "invalid"; token: "" }
  | { status: "ready"; token: string };

export default function ResetPasswordPage() {
  const router = useRouter();
  const tokenConsumedRef = useRef(false);
  const [tokenState, setTokenState] = useState<ResetTokenState>({
    status: "loading",
    token: "",
  });
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (tokenConsumedRef.current) return;
    tokenConsumedRef.current = true;
    const token = consumeOneTimeTokenFragment();
    setTokenState(
      token
        ? { status: "ready", token }
        : { status: "invalid", token: "" },
    );
  }, []);

  useEffect(() => {
    if (!success) return;
    const redirectTimer = window.setTimeout(
      () => router.replace(APP_ROUTES.LOGIN),
      LOGIN_REDIRECT_DELAY_MS,
    );
    return () => window.clearTimeout(redirectTimer);
  }, [router, success]);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");

    if (tokenState.status !== "ready") {
      setError(TEXT.AUTH.RESET_LINK_INVALID);
      return;
    }
    if (
      newPassword.length < IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH ||
      newPassword.length > IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH
    ) {
      setError(TEXT.AUTH.PASSWORD_REQUIREMENTS);
      return;
    }
    if (newPassword !== confirmPassword) {
      setError(TEXT.AUTH.PASSWORD_MISMATCH);
      return;
    }

    setIsLoading(true);
    try {
      await authService.resetPassword(tokenState.token, newPassword);
      setSuccess(true);
      setTokenState({ status: "invalid", token: "" });
      setNewPassword("");
      setConfirmPassword("");
    } catch (caught) {
      setError(
        caught instanceof ApiError && caught.status !== undefined && caught.status < 500
          ? TEXT.AUTH.RESET_LINK_INVALID
          : TEXT.AUTH.RESET_PASSWORD_FAILED,
      );
    } finally {
      setIsLoading(false);
    }
  }

  const passwordVisibilityLabel = showPassword
    ? TEXT.AUTH.HIDE_PASSWORD
    : TEXT.AUTH.SHOW_PASSWORD;
  const confirmationVisibilityLabel = showConfirm
    ? TEXT.AUTH.HIDE_PASSWORD
    : TEXT.AUTH.SHOW_PASSWORD;

  return (
    <Card className="border-border/70 bg-background/85 shadow-2xl backdrop-blur-xl">
      <CardHeader className="space-y-4 pb-6 text-center">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-brand-soft">
          <ShieldCheck aria-hidden="true" className="h-7 w-7" />
        </div>
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.RESET_PASSWORD_TITLE}
          </h1>
          <CardDescription>{TEXT.AUTH.RESET_PASSWORD_SUBTITLE}</CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        {tokenState.status === "loading" ? (
          <div
            className="flex min-h-40 items-center justify-center gap-3 text-sm text-muted-foreground"
            role="status"
          >
            <Loader2 aria-hidden="true" className="h-5 w-5 animate-spin motion-reduce:animate-none" />
            {TEXT.AUTH.RESET_LINK_PREPARING}
          </div>
        ) : tokenState.status === "invalid" && !success ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none">
            <div
              className="flex items-start gap-3 rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-4 text-sm text-destructive"
              role="alert"
            >
              <ShieldAlert aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0" />
              <p>{TEXT.AUTH.RESET_LINK_INVALID}</p>
            </div>
            <Link
              href={APP_ROUTES.FORGOT_PASSWORD}
              className={buttonVariants({
                variant: "outline",
                className: "h-11 w-full",
              })}
            >
              {TEXT.AUTH.SEND_RESET_LINK}
            </Link>
            <Link
              href={APP_ROUTES.LOGIN}
              className="block text-center text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              {TEXT.AUTH.BACK_TO_LOGIN}
            </Link>
          </div>
        ) : success ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none">
            <div
              className="flex items-center gap-3 rounded-xl border border-success/25 bg-success/10 px-4 py-4 text-sm text-success"
              role="status"
            >
              <CheckCircle2 aria-hidden="true" className="h-5 w-5 shrink-0" />
              <p>{TEXT.AUTH.RESET_PASSWORD_SUCCESS}</p>
            </div>
            <p className="text-center text-sm text-muted-foreground">
              {TEXT.AUTH.RESET_REDIRECTING}
            </p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            {error ? (
              <div
                className="rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive animate-in fade-in"
                role="alert"
              >
                {error}
              </div>
            ) : null}

            <div className="space-y-2">
              <Label
                htmlFor="newPassword"
                className="text-xs font-semibold uppercase tracking-wider text-muted-foreground"
              >
                {TEXT.AUTH.NEW_PASSWORD}
              </Label>
              <div className="relative">
                <Input
                  id="newPassword"
                  type={showPassword ? "text" : "password"}
                  autoComplete="new-password"
                  placeholder={TEXT.AUTH.NEW_PASSWORD_PLACEHOLDER}
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
                  maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
                  required
                  className="h-11 bg-muted/50 pr-11 transition-colors focus-visible:bg-background focus-visible:ring-primary"
                />
                <button
                  type="button"
                  aria-label={passwordVisibilityLabel}
                  title={passwordVisibilityLabel}
                  onClick={() => setShowPassword((visible) => !visible)}
                  className="absolute right-1 top-1/2 inline-flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {showPassword ? (
                    <EyeOff aria-hidden="true" className="h-4 w-4" />
                  ) : (
                    <Eye aria-hidden="true" className="h-4 w-4" />
                  )}
                </button>
              </div>
              <p className="text-xs text-muted-foreground">
                {TEXT.AUTH.PASSWORD_REQUIREMENTS}
              </p>
            </div>

            <div className="space-y-2">
              <Label
                htmlFor="confirmPassword"
                className="text-xs font-semibold uppercase tracking-wider text-muted-foreground"
              >
                {TEXT.AUTH.CONFIRM_PASSWORD}
              </Label>
              <div className="relative">
                <Input
                  id="confirmPassword"
                  type={showConfirm ? "text" : "password"}
                  autoComplete="new-password"
                  placeholder={TEXT.AUTH.CONFIRM_PASSWORD_PLACEHOLDER}
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
                  maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
                  required
                  className="h-11 bg-muted/50 pr-11 transition-colors focus-visible:bg-background focus-visible:ring-primary"
                />
                <button
                  type="button"
                  aria-label={confirmationVisibilityLabel}
                  title={confirmationVisibilityLabel}
                  onClick={() => setShowConfirm((visible) => !visible)}
                  className="absolute right-1 top-1/2 inline-flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {showConfirm ? (
                    <EyeOff aria-hidden="true" className="h-4 w-4" />
                  ) : (
                    <Eye aria-hidden="true" className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>

            <Button
              type="submit"
              className="h-11 w-full font-semibold button-primary-elevation"
              disabled={isLoading}
            >
              {isLoading ? (
                <Loader2 aria-hidden="true" className="mr-2 h-5 w-5 animate-spin motion-reduce:animate-none" />
              ) : null}
              {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.RESET_PASSWORD}
            </Button>

            <Link
              href={APP_ROUTES.LOGIN}
              className="block text-center text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              {TEXT.AUTH.BACK_TO_LOGIN}
            </Link>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
