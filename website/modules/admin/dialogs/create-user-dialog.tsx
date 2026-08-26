"use client";

import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  ArrowLeft,
  Check,
  CheckCircle2,
  KeyRound,
  Loader2,
  MailCheck,
  Send,
  ShieldCheck,
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
import {
  isUsernameValid,
  normalizeUsername,
  usernameHTMLPattern,
} from "@/lib/auth/credentials";
import {
  isRecoveryEmailValid,
  normalizeRecoveryEmail,
} from "@/lib/auth/recovery-email";
import { roleDisplayName } from "@/lib/auth/role-presentation";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useOnboardingProfileForm } from "@/modules/auth/hooks/use-onboarding-profile-form";
import { OnboardingProfileFields } from "@/modules/auth/onboarding-profile-fields";
import type {
  ReviewedUserInvitationCommand,
  Role,
  UserInvitationCommandInput,
} from "@/types/admin";

interface InvitationDraft {
  username: string;
  recoveryEmail: string;
  roles: string[];
  reason: string;
}

export type InviteUserReviewResult =
  | { ok: true }
  | { ok: false; error: unknown };

interface InviteUserDialogProps {
  open: boolean;
  roles: Role[];
  reviewed: ReviewedUserInvitationCommand | null;
  isReviewing: boolean;
  isApplying: boolean;
  isReauthenticating: boolean;
  requiresReauthentication: boolean;
  operationError: string | null;
  onReview: (
    input: UserInvitationCommandInput,
  ) => Promise<InviteUserReviewResult>;
  onApply: (currentPassword: string) => void;
  onBack: () => void;
  onClose: () => void;
}

const INITIAL_DRAFT: InvitationDraft = {
  username: "",
  recoveryEmail: "",
  roles: [],
  reason: "",
};

export function InviteUserDialog({
  open,
  roles,
  reviewed,
  isReviewing,
  isApplying,
  isReauthenticating,
  requiresReauthentication,
  operationError,
  onReview,
  onApply,
  onBack,
  onClose,
}: InviteUserDialogProps) {
  const copy = ADMIN_TEXT.USERS_INVITATION;
  const [draft, setDraft] = useState<InvitationDraft>(INITIAL_DRAFT);
  const [submitted, setSubmitted] = useState(false);
  const [password, setPassword] = useState("");
  const [passwordTouched, setPasswordTouched] = useState(false);
  // Keep the reviewed draft hydrated while the confirmation step is open so
  // "Back" restores the exact profile values the administrator reviewed.
  const profileForm = useOnboardingProfileForm({ enabled: open });
  const busy = isReviewing || isApplying || isReauthenticating;

  useEffect(() => {
    if (open) return;
    setDraft(INITIAL_DRAFT);
    setSubmitted(false);
    setPassword("");
    setPasswordTouched(false);
  }, [open]);

  useEffect(() => {
    if (!requiresReauthentication) {
      setPassword("");
      setPasswordTouched(false);
    }
  }, [requiresReauthentication]);

  const normalizedReason = draft.reason.trim();
  const reasonValid = useMemo(
    () =>
      normalizedReason.length >=
        IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH &&
      normalizedReason.length <=
        IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH,
    [normalizedReason],
  );
  const usernameValid = isUsernameValid(draft.username);
  const recoveryEmailValid = isRecoveryEmailValid(draft.recoveryEmail);
  const rolesValid = draft.roles.length > 0;

  function updateDraft(
    field: "username" | "recoveryEmail" | "reason",
    value: string,
  ): void {
    setDraft((current) => ({ ...current, [field]: value }));
  }

  function toggleRole(roleKey: string): void {
    setDraft((current) => ({
      ...current,
      roles: current.roles.includes(roleKey)
        ? current.roles.filter((key) => key !== roleKey)
        : [...current.roles, roleKey],
    }));
  }

  async function submitDraft(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setSubmitted(true);
    const profileEvidence = profileForm.buildEvidence();
    if (
      !usernameValid ||
      !recoveryEmailValid ||
      !rolesValid ||
      !reasonValid ||
      !profileEvidence ||
      busy
    ) {
      return;
    }
    const result = await onReview({
      username: normalizeUsername(draft.username),
      recovery_email: normalizeRecoveryEmail(draft.recoveryEmail),
      roles: [...draft.roles].sort(),
      reason: normalizedReason,
      ...profileEvidence,
    });
    if (!result.ok) profileForm.handleSubmissionError(result.error);
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
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && close()}>
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl"
        aria-busy={busy}
        showCloseButton={!busy}
      >
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-primary/25 bg-primary/10 text-primary">
            {reviewed ? (
              <MailCheck className="size-5" aria-hidden="true" />
            ) : (
              <Send className="size-5" aria-hidden="true" />
            )}
          </div>
          <DialogTitle>{copy.TITLE}</DialogTitle>
          <DialogDescription>{copy.DESCRIPTION}</DialogDescription>
        </DialogHeader>

        {!reviewed ? (
          <form onSubmit={submitDraft} className="space-y-5">
            <section className="flex gap-3 rounded-2xl border border-success/25 bg-success/10 p-4">
              <ShieldCheck
                className="mt-0.5 size-5 shrink-0 text-success"
                aria-hidden="true"
              />
              <div>
                <h3 className="text-sm font-semibold">{copy.SECURITY_TITLE}</h3>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {copy.SECURITY_DESCRIPTION}
                </p>
              </div>
            </section>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="invite-username">{copy.USERNAME}</Label>
                <Input
                  id="invite-username"
                  autoComplete="off"
                  value={draft.username}
                  minLength={IDENTITY_INPUT_LIMITS.USERNAME_MIN_LENGTH}
                  maxLength={IDENTITY_INPUT_LIMITS.USERNAME_MAX_LENGTH}
                  pattern={usernameHTMLPattern()}
                  disabled={busy}
                  required
                  aria-invalid={submitted && !usernameValid}
                  aria-describedby="invite-username-help"
                  placeholder={copy.USERNAME_PLACEHOLDER}
                  onChange={(event) => updateDraft("username", event.target.value)}
                />
                <p
                  id="invite-username-help"
                  className={cn(
                    "text-xs",
                    submitted && !usernameValid
                      ? "text-destructive"
                      : "text-muted-foreground",
                  )}
                >
                  {submitted && !usernameValid
                    ? copy.USERNAME_INVALID
                    : TEXT.AUTH.USERNAME_REQUIREMENTS}
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="invite-email">{copy.RECOVERY_EMAIL}</Label>
                <Input
                  id="invite-email"
                  type="email"
                  inputMode="email"
                  autoComplete="email"
                  value={draft.recoveryEmail}
                  maxLength={IDENTITY_INPUT_LIMITS.RECOVERY_EMAIL_MAX_LENGTH}
                  disabled={busy}
                  required
                  aria-invalid={submitted && !recoveryEmailValid}
                  aria-describedby="invite-email-help"
                  placeholder={copy.RECOVERY_EMAIL_PLACEHOLDER}
                  onChange={(event) =>
                    updateDraft("recoveryEmail", event.target.value)
                  }
                />
                <p
                  id="invite-email-help"
                  className={cn(
                    "text-xs",
                    submitted && !recoveryEmailValid
                      ? "text-destructive"
                      : "text-muted-foreground",
                  )}
                >
                  {submitted && !recoveryEmailValid
                    ? copy.RECOVERY_EMAIL_INVALID
                    : copy.RECOVERY_EMAIL_HELP}
                </p>
              </div>

              <OnboardingProfileFields
                form={profileForm}
                disabled={busy}
                className="sm:col-span-2"
              />

              <fieldset
                className="space-y-2 sm:col-span-2"
                disabled={busy}
                aria-invalid={submitted && !rolesValid}
              >
                <legend className="text-sm font-medium">{copy.ROLES}</legend>
                <div className="grid gap-2 rounded-xl border border-border p-2 sm:grid-cols-2">
                  {roles.map((role) => {
                    const selected = draft.roles.includes(role.key);
                    return (
                      <button
                        key={role.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleRole(role.key)}
                        className={cn(
                          "flex min-h-11 cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-left text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                          selected
                            ? "border-primary/30 bg-primary/10 text-primary"
                            : "border-transparent bg-muted/30 text-muted-foreground hover:bg-muted hover:text-foreground",
                        )}
                      >
                        <span
                          className={cn(
                            "flex size-5 shrink-0 items-center justify-center rounded-md border",
                            selected
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-border bg-background",
                          )}
                        >
                          {selected ? (
                            <Check
                              className="size-3.5"
                              strokeWidth={3}
                              aria-hidden="true"
                            />
                          ) : null}
                        </span>
                        <span className="truncate font-medium">
                          {roleDisplayName(role)}
                        </span>
                      </button>
                    );
                  })}
                </div>
                {submitted && !rolesValid ? (
                  <p className="text-xs font-medium text-destructive" role="alert">
                    {copy.ROLES_REQUIRED}
                  </p>
                ) : null}
              </fieldset>

              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="invite-reason">{copy.REASON}</Label>
                <Textarea
                  id="invite-reason"
                  value={draft.reason}
                  rows={3}
                  minLength={
                    IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH
                  }
                  maxLength={
                    IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH
                  }
                  disabled={busy}
                  required
                  aria-invalid={submitted && !reasonValid}
                  placeholder={copy.REASON_PLACEHOLDER}
                  onChange={(event) => updateDraft("reason", event.target.value)}
                />
                <div className="flex justify-between gap-4 text-xs">
                  <p
                    className={
                      submitted && !reasonValid
                        ? "text-destructive"
                        : "text-muted-foreground"
                    }
                  >
                    {submitted && !reasonValid
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
            </div>

            {operationError ? (
              <p
                className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
                role="alert"
              >
                {operationError}
              </p>
            ) : null}

            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={close}>
                {TEXT.COMMON.CANCEL}
              </Button>
              <Button
                type="submit"
                disabled={busy || profileForm.status !== "ready"}
              >
                {isReviewing ? (
                  <Loader2
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                ) : (
                  <ShieldCheck className="size-4" aria-hidden="true" />
                )}
                {isReviewing ? copy.REVIEWING : copy.REVIEW}
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
                {copy.CONFIRMATION_EXPIRES(
                  formatDateTime(reviewed.review.expires_at),
                )}
              </p>
            </section>

            <dl className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">{copy.ACCOUNT}</dt>
                <dd className="mt-1 font-semibold">
                  {reviewed.review.preview.username}
                </dd>
              </div>
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">{copy.DESTINATION}</dt>
                <dd className="mt-1 font-medium">
                  {reviewed.review.preview.masked_destination}
                </dd>
              </div>
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">{copy.PROFILE_FIELDS}</dt>
                <dd className="mt-1 font-bold tabular-nums">
                  {reviewed.review.preview.profile_value_count}
                </dd>
              </div>
              <div className="rounded-xl border border-border bg-muted/25 p-3">
                <dt className="text-xs text-muted-foreground">{copy.ROLES}</dt>
                <dd className="mt-2 flex flex-wrap gap-1.5">
                  {reviewed.review.preview.roles.map((role) => (
                    <Badge key={role} variant="secondary">
                      {role}
                    </Badge>
                  ))}
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

            <p className="rounded-xl border border-primary/20 bg-primary/5 px-4 py-3 text-sm leading-6">
              {copy.IMMUTABLE_NOTICE}
            </p>

            {requiresReauthentication ? (
              <section className="space-y-3 rounded-2xl border border-warning/30 bg-warning/10 p-4">
                <div className="flex items-start gap-3">
                  <KeyRound
                    className="mt-0.5 size-5 shrink-0 text-warning"
                    aria-hidden="true"
                  />
                  <div>
                    <h3 className="text-sm font-semibold">{copy.REAUTH_TITLE}</h3>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {copy.REAUTH_DESCRIPTION}
                    </p>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="invite-current-password">
                    {copy.CURRENT_PASSWORD}
                  </Label>
                  <Input
                    id="invite-current-password"
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
              <p
                className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
                role="alert"
              >
                {operationError}
              </p>
            ) : null}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={onBack}
              >
                <ArrowLeft className="size-4" aria-hidden="true" />
                {copy.BACK}
              </Button>
              <Button type="submit" disabled={busy}>
                {busy ? (
                  <Loader2
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                ) : (
                  <Send className="size-4" aria-hidden="true" />
                )}
                {isApplying || isReauthenticating
                  ? copy.SENDING
                  : copy.SEND_INVITATION}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
