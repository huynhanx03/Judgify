"use client";

import { type FormEvent, useEffect, useMemo, useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  KeyRound,
  Loader2,
  PauseCircle,
  PlayCircle,
  ShieldAlert,
  Trash2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
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
  ReviewedUserLifecycleCommand,
  UserLifecycleAction,
  UserLifecycleCommandInput,
  UserLifecycleStatus,
} from "@/types/admin";

interface UserLifecycleDialogProps {
  target: AdminUser | null;
  action: UserLifecycleAction | null;
  reviewed: ReviewedUserLifecycleCommand | null;
  reviewing: boolean;
  applying: boolean;
  reauthenticating: boolean;
  requiresReauthentication: boolean;
  operationError: string | null;
  onReview(input: UserLifecycleCommandInput): void;
  onApply(currentPassword: string): void;
  onBack(): void;
  onClose(): void;
}

function actionIcon(action: UserLifecycleAction) {
  if (action === "suspend") {
    return <PauseCircle className="size-5" aria-hidden="true" />;
  }
  if (action === "reactivate") {
    return <PlayCircle className="size-5" aria-hidden="true" />;
  }
  return <Trash2 className="size-5" aria-hidden="true" />;
}

function statusTone(status: UserLifecycleStatus): string {
  if (status === "active") {
    return "border-success/30 bg-success/10 text-success";
  }
  if (status === "suspended") {
    return "border-warning/30 bg-warning/10 text-warning";
  }
  if (status === "invited") {
    return "border-info/30 bg-info/10 text-info";
  }
  return "border-destructive/30 bg-destructive/10 text-destructive";
}

export function UserLifecycleDialog({
  target,
  action,
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
}: UserLifecycleDialogProps) {
  const [reason, setReason] = useState("");
  const [reasonTouched, setReasonTouched] = useState(false);
  const [password, setPassword] = useState("");
  const [passwordTouched, setPasswordTouched] = useState(false);
  const busy = reviewing || applying || reauthenticating;

  useEffect(() => {
    setReason("");
    setReasonTouched(false);
    setPassword("");
    setPasswordTouched(false);
  }, [target?.id, action]);

  useEffect(() => {
    if (!requiresReauthentication) {
      setPassword("");
      setPasswordTouched(false);
    }
  }, [requiresReauthentication]);

  const normalizedReason = reason.trim();
  const reasonValid = useMemo(
    () =>
      normalizedReason.length >=
        IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH &&
      normalizedReason.length <=
        IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH,
    [normalizedReason],
  );

  if (!target || !action) return null;
  const activeTarget = target;

  const copy = ADMIN_TEXT.USERS_LIFECYCLE;
  const review = reviewed?.review;
  const displayedReason = reviewed?.input.reason ?? normalizedReason;
  const destructive = action === "delete";

  function close(): void {
    if (!busy) onClose();
  }

  function submitDraft(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    setReasonTouched(true);
    if (!reasonValid || busy) return;
    onReview({
      expected_version: activeTarget.version,
      reason: normalizedReason,
    });
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

  return (
    <Dialog open onOpenChange={(open) => !open && close()}>
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl"
        aria-busy={busy}
        showCloseButton={!busy}
      >
        <DialogHeader>
          <div
            className={`mb-1 flex size-11 items-center justify-center rounded-xl border ${
              destructive
                ? "border-destructive/30 bg-destructive/10 text-destructive"
                : "border-warning/30 bg-warning/10 text-warning"
            }`}
          >
            {actionIcon(action)}
          </div>
          <DialogTitle>{copy.TITLE(action)}</DialogTitle>
          <DialogDescription>
            {copy.DESCRIPTION(action, target.username)}
          </DialogDescription>
        </DialogHeader>

        {!reviewed || !review ? (
          <form onSubmit={submitDraft} className="space-y-5">
            <section className="rounded-2xl border border-border bg-muted/25 p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {copy.TARGET_ACCOUNT}
                  </p>
                  <p className="mt-1 font-semibold">{target.username}</p>
                </div>
                <Badge variant="outline" className={statusTone(target.status)}>
                  {copy.STATUS(target.status)}
                </Badge>
              </div>
              <p className="mt-3 text-xs leading-5 text-muted-foreground">
                {copy.CURRENT_VERSION(target.version)}
              </p>
            </section>

            <div className="space-y-2">
              <Label htmlFor="user-lifecycle-reason">{copy.REASON}</Label>
              <Textarea
                id="user-lifecycle-reason"
                value={reason}
                minLength={
                  IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH
                }
                maxLength={
                  IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH
                }
                rows={4}
                disabled={busy}
                aria-invalid={reasonTouched && !reasonValid}
                aria-describedby="user-lifecycle-reason-help user-lifecycle-reason-count"
                placeholder={copy.REASON_PLACEHOLDER}
                onBlur={() => setReasonTouched(true)}
                onChange={(event) => setReason(event.target.value)}
              />
              <div className="flex items-start justify-between gap-4 text-xs">
                <p
                  id="user-lifecycle-reason-help"
                  className={
                    reasonTouched && !reasonValid
                      ? "text-destructive"
                      : "text-muted-foreground"
                  }
                  role={reasonTouched && !reasonValid ? "alert" : undefined}
                >
                  {reasonTouched && !reasonValid
                    ? copy.REASON_INVALID
                    : copy.REASON_HELP}
                </p>
                <span
                  id="user-lifecycle-reason-count"
                  className="shrink-0 tabular-nums text-muted-foreground"
                >
                  {copy.REASON_COUNT(
                    normalizedReason.length,
                    IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH,
                  )}
                </span>
              </div>
            </div>

            <p
              className={`rounded-xl border px-4 py-3 text-sm leading-6 ${
                destructive
                  ? "border-destructive/25 bg-destructive/5 text-foreground"
                  : "border-warning/25 bg-warning/5 text-foreground"
              }`}
            >
              {copy.EFFECT(action, target.status)}
            </p>

            {operationError ? (
              <p
                className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
                role="alert"
                aria-live="assertive"
              >
                {operationError}
              </p>
            ) : null}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                className="min-h-11"
                disabled={busy}
                onClick={close}
              >
                {TEXT.COMMON.CANCEL}
              </Button>
              <Button type="submit" className="min-h-11" disabled={busy}>
                {reviewing ? (
                  <Loader2
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                ) : (
                  <ShieldAlert className="size-4" aria-hidden="true" />
                )}
                {reviewing ? copy.REVIEWING : copy.REVIEW}
              </Button>
            </DialogFooter>
          </form>
        ) : (
          <form onSubmit={submitReviewed} className="space-y-5">
            <section className="rounded-2xl border border-success/30 bg-success/10 p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <CheckCircle2 className="size-4 text-success" aria-hidden="true" />
                  {copy.SERVER_REVIEWED}
                </div>
                <Badge variant="outline" className="bg-background/60 font-mono">
                  {target.username}
                </Badge>
              </div>
              <p className="mt-2 text-xs leading-5 text-muted-foreground">
                {copy.EXPIRES_AT(formatDateTime(review.expires_at))}
              </p>
            </section>

            <dl className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">
                  {copy.STATUS_TRANSITION}
                </dt>
                <dd className="mt-2 flex flex-wrap items-center gap-2">
                  <Badge variant="outline" className={statusTone(review.preview.before.status)}>
                    {copy.STATUS(review.preview.before.status)}
                  </Badge>
                  <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
                  <Badge variant="outline" className={statusTone(review.preview.after.status)}>
                    {copy.STATUS(review.preview.after.status)}
                  </Badge>
                </dd>
              </div>
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">
                  {copy.VERSION_TRANSITION}
                </dt>
                <dd className="mt-2 flex items-center gap-2 font-bold tabular-nums">
                  {review.preview.expected_version}
                  <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
                  {review.preview.resulting_version}
                </dd>
              </div>
            </dl>

            <section className="rounded-2xl border border-border bg-muted/25 p-4">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {copy.REASON_REVIEWED}
              </h3>
              <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6">
                {displayedReason}
              </p>
            </section>

            <p
              className={`rounded-xl border px-4 py-3 text-sm leading-6 ${
                destructive
                  ? "border-destructive/25 bg-destructive/5"
                  : "border-warning/25 bg-warning/5"
              }`}
            >
              {copy.IMMUTABLE_NOTICE(action)}
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
                  <Label htmlFor="user-lifecycle-current-password">
                    {copy.CURRENT_PASSWORD}
                  </Label>
                  <Input
                    id="user-lifecycle-current-password"
                    type="password"
                    autoComplete="current-password"
                    value={password}
                    maxLength={256}
                    disabled={busy}
                    autoFocus
                    aria-invalid={passwordTouched && password.length === 0}
                    aria-describedby={
                      passwordTouched && password.length === 0
                        ? "user-lifecycle-password-error"
                        : undefined
                    }
                    placeholder={copy.CURRENT_PASSWORD_PLACEHOLDER}
                    onChange={(event) => {
                      setPassword(event.target.value);
                      setPasswordTouched(false);
                    }}
                  />
                  {passwordTouched && password.length === 0 ? (
                    <p
                      id="user-lifecycle-password-error"
                      className="text-xs text-destructive"
                      role="alert"
                    >
                      {copy.PASSWORD_REQUIRED}
                    </p>
                  ) : null}
                </div>
              </section>
            ) : null}

            {operationError ? (
              <p
                className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
                role="alert"
                aria-live="assertive"
              >
                {operationError}
              </p>
            ) : null}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                className="min-h-11"
                disabled={busy}
                onClick={onBack}
              >
                {copy.BACK}
              </Button>
              <Button
                type="submit"
                variant={destructive ? "destructive" : "default"}
                className="min-h-11"
                disabled={busy}
              >
                {busy ? (
                  <Loader2
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                ) : (
                  actionIcon(action)
                )}
                {reauthenticating
                  ? copy.REAUTHENTICATING
                  : applying
                    ? copy.APPLYING
                    : copy.APPLY(action)}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
