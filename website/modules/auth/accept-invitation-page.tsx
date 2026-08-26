"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  CheckCircle2,
  Eye,
  EyeOff,
  KeyRound,
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
import { isPasswordValid } from "@/lib/auth/credentials";
import { consumeOneTimeTokenFragment } from "@/lib/auth/one-time-token";
import { authService } from "@/services/auth.service";

type InvitationTokenState =
  | { status: "preparing"; token: "" }
  | { status: "invalid"; token: "" }
  | { status: "ready"; token: string };

export default function AcceptInvitationPage() {
  const tokenConsumedRef = useRef(false);
  const submittingRef = useRef(false);
  const [tokenState, setTokenState] = useState<InvitationTokenState>({
    status: "preparing",
    token: "",
  });
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordTouched, setPasswordTouched] = useState(false);
  const [confirmationTouched, setConfirmationTouched] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmation, setShowConfirmation] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
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

  const passwordInvalid = passwordTouched && !isPasswordValid(newPassword);
  const confirmationInvalid =
    confirmationTouched &&
    (confirmPassword.length === 0 || confirmPassword !== newPassword);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) return;
    setPasswordTouched(true);
    setConfirmationTouched(true);
    setError("");

    if (tokenState.status !== "ready") {
      setError(TEXT.AUTH.INVITATION_ACTIVATION_INVALID);
      return;
    }
    if (!isPasswordValid(newPassword)) {
      setError(TEXT.AUTH.PASSWORD_REQUIREMENTS);
      return;
    }
    if (newPassword !== confirmPassword) {
      setError(TEXT.AUTH.PASSWORD_MISMATCH);
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    try {
      await authService.activateUserInvitation(tokenState.token, newPassword);
      setTokenState({ status: "invalid", token: "" });
      setNewPassword("");
      setConfirmPassword("");
      setSuccess(true);
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 400) {
        setTokenState({ status: "invalid", token: "" });
        setNewPassword("");
        setConfirmPassword("");
        setError("");
        return;
      }
      setError(TEXT.AUTH.INVITATION_ACTIVATION_FAILED);
    } finally {
      submittingRef.current = false;
      setIsSubmitting(false);
    }
  }

  const passwordVisibilityLabel = showPassword
    ? TEXT.AUTH.HIDE_PASSWORD
    : TEXT.AUTH.SHOW_PASSWORD;
  const confirmationVisibilityLabel = showConfirmation
    ? TEXT.AUTH.HIDE_PASSWORD
    : TEXT.AUTH.SHOW_PASSWORD;

  return (
    <Card className="mx-auto w-full max-w-lg overflow-hidden border-border/70 bg-background/90 shadow-2xl backdrop-blur-xl">
      <CardHeader className="space-y-4 px-5 pb-5 pt-6 text-center sm:px-8 sm:pt-8">
        <div className="mx-auto flex size-14 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20 shadow-brand-soft">
          <ShieldCheck aria-hidden="true" className="size-7" />
        </div>
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-primary">
            {TEXT.AUTH.INVITATION_ACTIVATION_EYEBROW}
          </p>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
            {TEXT.AUTH.INVITATION_ACTIVATION_TITLE}
          </h1>
          <CardDescription className="mx-auto max-w-sm leading-relaxed">
            {TEXT.AUTH.INVITATION_ACTIVATION_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent className="px-5 pb-6 sm:px-8 sm:pb-8">
        {tokenState.status === "preparing" ? (
          <div
            className="flex min-h-48 flex-col items-center justify-center gap-4 rounded-xl border border-border/70 bg-muted/35 px-6 text-center"
            role="status"
            aria-live="polite"
          >
            <Loader2
              aria-hidden="true"
              className="size-7 animate-spin text-primary motion-reduce:animate-none"
            />
            <p className="text-sm text-muted-foreground">
              {TEXT.AUTH.INVITATION_ACTIVATION_PREPARING}
            </p>
          </div>
        ) : success ? (
          <div className="space-y-6 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2">
            <div
              className="flex flex-col items-center gap-3 rounded-xl border border-success/25 bg-success/10 px-5 py-6 text-center"
              role="status"
              aria-live="polite"
            >
              <CheckCircle2 aria-hidden="true" className="size-8 text-success" />
              <div className="space-y-1.5">
                <h2 className="font-semibold text-foreground">
                  {TEXT.AUTH.INVITATION_ACTIVATION_SUCCESS_TITLE}
                </h2>
                <p className="text-sm leading-relaxed text-muted-foreground">
                  {TEXT.AUTH.INVITATION_ACTIVATION_SUCCESS_DESCRIPTION}
                </p>
              </div>
            </div>
            <Link
              href={APP_ROUTES.LOGIN}
              className={buttonVariants({ className: "w-full" })}
            >
              {TEXT.AUTH.INVITATION_ACTIVATION_LOGIN}
            </Link>
          </div>
        ) : tokenState.status === "invalid" ? (
          <div className="space-y-6 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2">
            <div
              className="flex items-start gap-3 rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-4 text-sm text-destructive"
              role="alert"
            >
              <ShieldAlert aria-hidden="true" className="mt-0.5 size-5 shrink-0" />
              <p className="leading-relaxed">
                {TEXT.AUTH.INVITATION_ACTIVATION_INVALID}
              </p>
            </div>
            <Link
              href={APP_ROUTES.LOGIN}
              className={buttonVariants({
                variant: "outline",
                className: "w-full",
              })}
            >
              {TEXT.AUTH.BACK_TO_LOGIN}
            </Link>
          </div>
        ) : (
          <form
            onSubmit={handleSubmit}
            className="space-y-5"
            aria-busy={isSubmitting}
            noValidate
          >
            <div className="flex items-start gap-3 rounded-xl border border-primary/20 bg-primary/8 px-4 py-3.5 text-sm text-muted-foreground">
              <KeyRound aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-primary" />
              <p className="leading-relaxed">
                {TEXT.AUTH.INVITATION_ACTIVATION_SECURITY_NOTE}
              </p>
            </div>

            {error ? (
              <div
                className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive motion-safe:animate-in motion-safe:fade-in"
                role="alert"
              >
                {error}
              </div>
            ) : null}

            <div className="space-y-2">
              <Label
                htmlFor="invitation-new-password"
                className="text-xs font-semibold uppercase tracking-wider text-muted-foreground"
              >
                {TEXT.AUTH.NEW_PASSWORD}
              </Label>
              <div className="relative">
                <Input
                  id="invitation-new-password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(event) => {
                    setNewPassword(event.target.value);
                    setError("");
                  }}
                  onBlur={() => setPasswordTouched(true)}
                  minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
                  maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
                  aria-invalid={passwordInvalid}
                  aria-describedby="invitation-password-description"
                  required
                  disabled={isSubmitting}
                  className="pr-12"
                />
                <button
                  type="button"
                  aria-label={passwordVisibilityLabel}
                  title={passwordVisibilityLabel}
                  onClick={() => setShowPassword((visible) => !visible)}
                  disabled={isSubmitting}
                  className="absolute right-0 top-1/2 inline-flex size-11 -translate-y-1/2 items-center justify-center rounded-[var(--control-radius)] text-muted-foreground transition-colors duration-200 hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
                >
                  {showPassword ? (
                    <EyeOff aria-hidden="true" className="size-4" />
                  ) : (
                    <Eye aria-hidden="true" className="size-4" />
                  )}
                </button>
              </div>
              <p
                id="invitation-password-description"
                aria-live="polite"
                className={
                  passwordInvalid
                    ? "text-xs text-destructive"
                    : "text-xs text-muted-foreground"
                }
              >
                {TEXT.AUTH.PASSWORD_REQUIREMENTS}
              </p>
            </div>

            <div className="space-y-2">
              <Label
                htmlFor="invitation-confirm-password"
                className="text-xs font-semibold uppercase tracking-wider text-muted-foreground"
              >
                {TEXT.AUTH.CONFIRM_PASSWORD}
              </Label>
              <div className="relative">
                <Input
                  id="invitation-confirm-password"
                  type={showConfirmation ? "text" : "password"}
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(event) => {
                    setConfirmPassword(event.target.value);
                    setError("");
                  }}
                  onBlur={() => setConfirmationTouched(true)}
                  minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
                  maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
                  aria-invalid={confirmationInvalid}
                  aria-describedby={
                    confirmationInvalid
                      ? "invitation-confirmation-error"
                      : undefined
                  }
                  required
                  disabled={isSubmitting}
                  className="pr-12"
                />
                <button
                  type="button"
                  aria-label={confirmationVisibilityLabel}
                  title={confirmationVisibilityLabel}
                  onClick={() =>
                    setShowConfirmation((visible) => !visible)
                  }
                  disabled={isSubmitting}
                  className="absolute right-0 top-1/2 inline-flex size-11 -translate-y-1/2 items-center justify-center rounded-[var(--control-radius)] text-muted-foreground transition-colors duration-200 hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
                >
                  {showConfirmation ? (
                    <EyeOff aria-hidden="true" className="size-4" />
                  ) : (
                    <Eye aria-hidden="true" className="size-4" />
                  )}
                </button>
              </div>
              {confirmationInvalid ? (
                <p
                  id="invitation-confirmation-error"
                  className="text-xs text-destructive"
                  role="alert"
                >
                  {TEXT.AUTH.PASSWORD_MISMATCH}
                </p>
              ) : null}
            </div>

            <Button
              type="submit"
              className="w-full button-primary-elevation"
              disabled={isSubmitting}
            >
              {isSubmitting ? (
                <Loader2
                  aria-hidden="true"
                  className="size-5 animate-spin motion-reduce:animate-none"
                />
              ) : null}
              {isSubmitting
                ? TEXT.AUTH.INVITATION_ACTIVATION_SUBMITTING
                : TEXT.AUTH.INVITATION_ACTIVATION_SUBMIT}
            </Button>

            <Link
              href={APP_ROUTES.LOGIN}
              className="block min-h-11 rounded-[var(--control-radius)] py-3 text-center text-sm font-medium text-muted-foreground transition-colors duration-200 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {TEXT.AUTH.BACK_TO_LOGIN}
            </Link>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
