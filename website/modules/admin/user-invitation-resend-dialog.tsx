"use client";

import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  KeyRound,
  Loader2,
  MailCheck,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { formatDateTime } from "@/lib/format";
import type {
  AdminUser,
  ReviewedUserInvitationResendCommand,
} from "@/types/admin";

interface UserInvitationResendDialogProps {
  target: AdminUser | null;
  reviewed: ReviewedUserInvitationResendCommand | null;
  reviewing: boolean;
  applying: boolean;
  reauthenticating: boolean;
  requiresReauthentication: boolean;
  operationError: string | null;
  onReview: (reason: string) => void;
  onApply: (currentPassword: string) => void;
  onBack: () => void;
  onClose: () => void;
}

export function UserInvitationResendDialog({
  target,
  reviewed,
  reviewing,
  applying,
  reauthenticating,
  requiresReauthentication,
  operationError,
  onReview,
  onApply,
  onBack,
  onClose,
}: UserInvitationResendDialogProps) {
  const copy = ADMIN_TEXT.USERS_INVITATION_RESEND;
  const [reason, setReason] = useState("");
  const [reasonTouched, setReasonTouched] = useState(false);
  const [password, setPassword] = useState("");
  const [passwordTouched, setPasswordTouched] = useState(false);
  const busy = reviewing || applying || reauthenticating;
  const normalizedReason = reason.trim();
  const reasonValid = useMemo(
    () =>
      normalizedReason.length >=
        IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH &&
      normalizedReason.length <=
        IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH,
    [normalizedReason],
  );

  useEffect(() => {
    setReason("");
    setReasonTouched(false);
    setPassword("");
    setPasswordTouched(false);
  }, [target?.id]);

  useEffect(() => {
    if (!requiresReauthentication) {
      setPassword("");
      setPasswordTouched(false);
    }
  }, [requiresReauthentication]);

  if (!target) return null;

  function submitDraft(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    setReasonTouched(true);
    if (!reasonValid || busy) return;
    onReview(normalizedReason);
  }

  function submitReviewed(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (busy) return;
    if (requiresReauthentication && password.length === 0) {
      setPasswordTouched(true);
      return;
    }
    onApply(password);
  }

  function close(): void {
    if (!busy) onClose();
  }

  return (
    <Dialog open onOpenChange={(open) => !open && close()}>
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl"
        aria-busy={busy}
        showCloseButton={!busy}
      >
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-info/30 bg-info/10 text-info">
            <RefreshCw className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>{copy.TITLE}</DialogTitle>
          <DialogDescription>
            {copy.DESCRIPTION(target.username)}
          </DialogDescription>
        </DialogHeader>

        {!reviewed ? (
          <form onSubmit={submitDraft} className="space-y-5">
            <p className="rounded-xl border border-info/25 bg-info/5 px-4 py-3 text-sm leading-6">
              {copy.EFFECT}
            </p>
            <div className="space-y-2">
              <Label htmlFor="invitation-resend-reason">{copy.REASON}</Label>
              <Textarea
                id="invitation-resend-reason"
                value={reason}
                rows={4}
                minLength={
                  IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH
                }
                maxLength={
                  IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH
                }
                disabled={busy}
                required
                aria-invalid={reasonTouched && !reasonValid}
                placeholder={copy.REASON_PLACEHOLDER}
                onBlur={() => setReasonTouched(true)}
                onChange={(event) => setReason(event.target.value)}
              />
              <div className="flex justify-between gap-4 text-xs">
                <p
                  className={
                    reasonTouched && !reasonValid
                      ? "text-destructive"
                      : "text-muted-foreground"
                  }
                >
                  {reasonTouched && !reasonValid
                    ? copy.REASON_INVALID
                    : copy.REASON_HELP}
                </p>
                <span className="shrink-0 tabular-nums text-muted-foreground">
                  {copy.REASON_COUNT(
                    normalizedReason.length,
                    IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH,
                  )}
                </span>
              </div>
            </div>
            {operationError ? (
              <p className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive" role="alert">
                {operationError}
              </p>
            ) : null}
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={close}>
                {TEXT.COMMON.CANCEL}
              </Button>
              <Button type="submit" disabled={busy}>
                {reviewing ? (
                  <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                ) : (
                  <MailCheck className="size-4" aria-hidden="true" />
                )}
                {reviewing ? copy.REVIEWING : copy.REVIEW}
              </Button>
            </DialogFooter>
          </form>
        ) : (
          <form onSubmit={submitReviewed} className="space-y-5">
            <section className="rounded-2xl border border-success/30 bg-success/10 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold">
                <CheckCircle2 className="size-4 text-success" aria-hidden="true" />
                {copy.SERVER_REVIEWED}
              </div>
              <p className="mt-2 text-xs leading-5 text-muted-foreground">
                {copy.CONFIRMATION_EXPIRES(formatDateTime(reviewed.review.expires_at))}
              </p>
            </section>
            <dl className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">{copy.DESTINATION}</dt>
                <dd className="mt-1 font-medium">
                  {reviewed.review.preview.masked_destination}
                </dd>
              </div>
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">{copy.CURRENT_EXPIRY}</dt>
                <dd className="mt-1 text-sm font-medium">
                  {formatDateTime(reviewed.review.preview.current_expires_at)}
                </dd>
              </div>
              <div className="rounded-xl border border-border bg-muted/25 p-3 sm:col-span-2">
                <dt className="text-xs text-muted-foreground">{copy.VERSION}</dt>
                <dd className="mt-2 flex items-center gap-2 font-bold tabular-nums">
                  {reviewed.review.preview.expected_invitation_version}
                  <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
                  {reviewed.review.preview.resulting_invitation_version}
                </dd>
              </div>
            </dl>
            <section className="rounded-xl border border-border bg-muted/25 p-4">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {copy.REASON_REVIEWED}
              </h3>
              <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6">
                {reviewed.input.reason}
              </p>
            </section>
            <p className="rounded-xl border border-warning/25 bg-warning/5 px-4 py-3 text-sm leading-6">
              {copy.IMMUTABLE_NOTICE}
            </p>
            {requiresReauthentication ? (
              <section className="space-y-3 rounded-2xl border border-warning/30 bg-warning/10 p-4">
                <div className="flex items-start gap-3">
                  <KeyRound className="mt-0.5 size-5 shrink-0 text-warning" aria-hidden="true" />
                  <div>
                    <h3 className="text-sm font-semibold">{copy.REAUTH_TITLE}</h3>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {copy.REAUTH_DESCRIPTION}
                    </p>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="invitation-resend-password">{copy.CURRENT_PASSWORD}</Label>
                  <Input
                    id="invitation-resend-password"
                    type="password"
                    autoComplete="current-password"
                    value={password}
                    maxLength={256}
                    disabled={busy}
                    autoFocus
                    aria-invalid={passwordTouched && password.length === 0}
                    onChange={(event) => {
                      setPassword(event.target.value);
                      setPasswordTouched(false);
                    }}
                  />
                  {passwordTouched && password.length === 0 ? (
                    <p className="text-xs text-destructive" role="alert">
                      {copy.PASSWORD_REQUIRED}
                    </p>
                  ) : null}
                </div>
              </section>
            ) : null}
            {operationError ? (
              <p className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive" role="alert">
                {operationError}
              </p>
            ) : null}
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={onBack}>
                <ArrowLeft className="size-4" aria-hidden="true" />
                {copy.BACK}
              </Button>
              <Button type="submit" disabled={busy}>
                {busy ? (
                  <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                ) : (
                  <RefreshCw className="size-4" aria-hidden="true" />
                )}
                {applying || reauthenticating ? copy.SENDING : copy.SEND}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
