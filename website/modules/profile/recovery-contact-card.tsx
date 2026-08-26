"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CircleAlert,
  Clock3,
  KeyRound,
  Loader2,
  MailCheck,
  PencilLine,
  RefreshCw,
  Send,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  XCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ConfirmDialog } from "@/components/confirm-dialog";
import {
  RECOVERY_CONTACT_DELIVERY_STATE,
  RECOVERY_CONTACT_PURPOSE,
  RECOVERY_CONTACT_REAUTHENTICATION_STATUS,
  RECOVERY_CONTACT_UI,
} from "@/constants/identity";
import { TEXT } from "@/constants/text";
import { formatDateTime } from "@/lib/format";
import { notify } from "@/lib/toast";
import { authService } from "@/services/auth.service";
import type {
  RecoveryContactDeliveryState,
  RecoveryContactPurpose,
  RecoveryContactResponse,
} from "@/types/auth";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  classifyRecoveryContactError,
  formatCountdown,
  isRecentReauthenticationActive,
  secondsUntil,
} from "./recovery-contact-model";
import {
  ReauthenticationDialog,
  ReplaceRecoveryContactDialog,
} from "./recovery-contact-dialogs";

type SensitiveIntent = "replace" | "resend" | "cancel" | "remove";
type RecoveryAction = SensitiveIntent | "reauthenticate" | null;

function deliveryLabel(state: RecoveryContactDeliveryState): string {
  if (state === RECOVERY_CONTACT_DELIVERY_STATE.SENT) {
    return TEXT.PROFILE.RECOVERY_CONTACT.DELIVERY_SENT;
  }
  if (state === RECOVERY_CONTACT_DELIVERY_STATE.FAILED) {
    return TEXT.PROFILE.RECOVERY_CONTACT.DELIVERY_FAILED;
  }
  return TEXT.PROFILE.RECOVERY_CONTACT.DELIVERY_QUEUED;
}

function purposeDescription(purpose: RecoveryContactPurpose): string {
  return purpose === RECOVERY_CONTACT_PURPOSE.REPLACEMENT
    ? TEXT.PROFILE.RECOVERY_CONTACT.PENDING_REPLACEMENT
    : TEXT.PROFILE.RECOVERY_CONTACT.PENDING_REGISTRATION;
}

function actionErrorMessage(error: unknown): string {
  const kind = classifyRecoveryContactError(error);
  if (kind === "conflict") return TEXT.PROFILE.RECOVERY_CONTACT.VERSION_CONFLICT;
  if (kind === "rate_limited") return TEXT.PROFILE.RECOVERY_CONTACT.RATE_LIMITED;
  if (kind === "reauthentication_required") {
    return TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_REQUIRED;
  }
  return TEXT.PROFILE.RECOVERY_CONTACT.ACTION_ERROR;
}

export function RecoveryContactCard() {
  const [projection, setProjection] = useState<RecoveryContactResponse | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [action, setAction] = useState<RecoveryAction>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [pendingIntent, setPendingIntent] = useState<SensitiveIntent | null>(null);
  const [reauthOpen, setReauthOpen] = useState(false);
  const [reauthError, setReauthError] = useState<string | null>(null);
  const [replaceOpen, setReplaceOpen] = useState(false);
  const [replaceError, setReplaceError] = useState<string | null>(null);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [removeOpen, setRemoveOpen] = useState(false);

  const resource = useRetryableResource<RecoveryContactResponse | null>({
    resetKey: "profile-recovery-contact",
    initialData: null,
    load: (signal) => authService.getRecoveryContact(signal),
    onSuccess: (nextProjection) => {
      setProjection(nextProjection);
      setNow(Date.now());
    },
  });

  const resendRemaining = projection?.pending
    ? secondsUntil(projection.pending.resend_available_at, now)
    : 0;
  const reauthenticationRemaining = projection?.reauthenticated_until
    ? secondsUntil(projection.reauthenticated_until, now)
    : 0;
  const shouldTick = resendRemaining > 0 || reauthenticationRemaining > 0;

  useEffect(() => {
    if (!shouldTick) return;
    const timer = window.setInterval(
      () => setNow(Date.now()),
      RECOVERY_CONTACT_UI.CLOCK_TICK_MS,
    );
    return () => window.clearInterval(timer);
  }, [shouldTick]);

  const hasRecentAuthentication = useMemo(
    () =>
      isRecentReauthenticationActive(
        projection?.reauthenticated_until ?? null,
        now,
      ),
    [now, projection?.reauthenticated_until],
  );

  function showReauthentication(intent: SensitiveIntent) {
    setPendingIntent(intent);
    setReauthError(null);
    setReauthOpen(true);
  }

  function continueIntent(intent: SensitiveIntent) {
    if (intent === "replace") {
      setReplaceError(null);
      setReplaceOpen(true);
      return;
    }
    if (intent === "cancel") {
      setCancelOpen(true);
      return;
    }
    if (intent === "remove") {
      setRemoveOpen(true);
      return;
    }
    void resendVerification();
  }

  function requestIntent(intent: SensitiveIntent) {
    if (action || !projection) return;
    setActionError(null);
    if (hasRecentAuthentication) {
      continueIntent(intent);
      return;
    }
    showReauthentication(intent);
  }

  async function runMutation(
    intent: SensitiveIntent,
    operation: () => Promise<RecoveryContactResponse>,
    successMessage: string,
  ): Promise<boolean> {
    setAction(intent);
    setActionError(null);
    try {
      const nextProjection = await operation();
      setProjection(nextProjection);
      setNow(Date.now());
      notify.success(successMessage);
      return true;
    } catch (error) {
      const kind = classifyRecoveryContactError(error);
      const message = actionErrorMessage(error);
      setActionError(message);
      if (
        intent === "replace" &&
        kind !== "reauthentication_required" &&
        kind !== "conflict"
      ) {
        setReplaceError(message);
      }
      if (kind === "reauthentication_required") {
        setReplaceOpen(false);
        setCancelOpen(false);
        setRemoveOpen(false);
        showReauthentication(intent);
      } else if (kind === "conflict") {
        setReplaceOpen(false);
        setCancelOpen(false);
        setRemoveOpen(false);
        resource.retry();
      }
      return false;
    } finally {
      setAction(null);
    }
  }

  async function resendVerification() {
    const pending = projection?.pending;
    if (!pending || secondsUntil(pending.resend_available_at) > 0) return;
    await runMutation(
      "resend",
      () => authService.resendRecoveryContactVerification(pending.version),
      TEXT.PROFILE.RECOVERY_CONTACT.RESEND_SUCCESS,
    );
  }

  async function handleReauthenticate(password: string) {
    setAction("reauthenticate");
    setReauthError(null);
    let intentToContinue: SensitiveIntent | null = null;
    try {
      const response = await authService.reauthenticateRecoveryContact(password);
      if (
        response.status !==
        RECOVERY_CONTACT_REAUTHENTICATION_STATUS.REAUTHENTICATED
      ) {
        throw new Error(TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_ERROR);
      }
      setProjection((current) =>
        current
          ? {
              ...current,
              reauthenticated_until: response.reauthenticated_until,
            }
          : current,
      );
      setNow(Date.now());
      intentToContinue = pendingIntent;
      setPendingIntent(null);
      setReauthOpen(false);
      setActionError(null);
      notify.success(TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_SUCCESS);
    } catch {
      setReauthError(TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_ERROR);
    }
    setAction(null);
    if (intentToContinue) {
      queueMicrotask(() => continueIntent(intentToContinue));
    }
  }

  async function handleReplace(email: string) {
    const current = projection?.current;
    if (!current) {
      setReplaceError(TEXT.PROFILE.RECOVERY_CONTACT.ACTION_ERROR);
      return;
    }
    const succeeded = await runMutation(
      "replace",
      () => authService.replaceRecoveryContact(email, current.version),
      TEXT.PROFILE.RECOVERY_CONTACT.REPLACE_SUCCESS,
    );
    if (succeeded) setReplaceOpen(false);
  }

  async function handleCancelPending() {
    const pending = projection?.pending;
    if (!pending || pending.purpose !== RECOVERY_CONTACT_PURPOSE.REPLACEMENT) {
      setCancelOpen(false);
      return;
    }
    await runMutation(
      "cancel",
      () => authService.cancelPendingRecoveryContact(pending.version),
      TEXT.PROFILE.RECOVERY_CONTACT.CANCEL_SUCCESS,
    );
    setCancelOpen(false);
  }

  async function handleRemoveCurrent() {
    const current = projection?.current;
    if (!current || !projection.can_remove) {
      setRemoveOpen(false);
      return;
    }
    await runMutation(
      "remove",
      () => authService.removeRecoveryContact(current.version),
      TEXT.PROFILE.RECOVERY_CONTACT.REMOVE_SUCCESS,
    );
    setRemoveOpen(false);
  }

  if (resource.status === "loading" && !projection) {
    return (
      <Card className="glass-card border-border/40" aria-busy="true">
        <CardHeader>
          <div className="flex items-center gap-3">
            <div className="size-11 animate-pulse rounded-xl bg-muted motion-reduce:animate-none" />
            <div className="flex-1 space-y-2">
              <div className="h-4 w-40 animate-pulse rounded bg-muted motion-reduce:animate-none" />
              <div className="h-3 w-full max-w-sm animate-pulse rounded bg-muted motion-reduce:animate-none" />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="h-28 animate-pulse rounded-xl bg-muted motion-reduce:animate-none" />
          <span className="sr-only">{TEXT.COMMON.LOADING}</span>
        </CardContent>
      </Card>
    );
  }

  if (resource.status === "error" && !projection) {
    return (
      <Card className="glass-card border-destructive/20">
        <CardContent className="flex flex-col items-center px-6 py-10 text-center" role="alert">
          <div className="mb-4 flex size-12 items-center justify-center rounded-xl bg-destructive/10 text-destructive ring-8 ring-destructive/5">
            <CircleAlert className="size-5" aria-hidden="true" />
          </div>
          <h2 className="text-lg font-bold tracking-tight">
            {TEXT.PROFILE.RECOVERY_CONTACT.LOAD_ERROR_TITLE}
          </h2>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {TEXT.PROFILE.RECOVERY_CONTACT.LOAD_ERROR_DESCRIPTION}
          </p>
          <Button
            type="button"
            onClick={resource.retry}
            className="mt-5 min-h-11 cursor-pointer px-4"
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.PROFILE.RECOVERY_CONTACT.RETRY}
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (!projection) return null;

  const pending = projection.pending;
  const current = projection.current;
  const pendingDeliveryFailed =
    pending?.delivery_state === RECOVERY_CONTACT_DELIVERY_STATE.FAILED;
  const isBusy = action !== null;

  return (
    <>
      <Card className="glass-card border-border/40" aria-busy={isBusy}>
        <CardHeader className="border-b border-border/40 pb-4">
          <div className="flex items-start gap-3">
            <div className="flex size-11 shrink-0 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary shadow-brand-subtle">
              <ShieldCheck className="size-5" aria-hidden="true" />
            </div>
            <div className="min-w-0 flex-1">
              <CardTitle className="text-lg font-bold tracking-tight">
                {TEXT.PROFILE.RECOVERY_CONTACT.TITLE}
              </CardTitle>
              <CardDescription className="mt-1 max-w-xl leading-6">
                {TEXT.PROFILE.RECOVERY_CONTACT.DESCRIPTION}
              </CardDescription>
            </div>
            {current ? (
              <Badge
                variant="outline"
                className="border-status-success/25 bg-status-success/10 text-status-success"
              >
                <MailCheck className="size-3" aria-hidden="true" />
                {TEXT.PROFILE.RECOVERY_CONTACT.VERIFIED}
              </Badge>
            ) : null}
          </div>
        </CardHeader>

        <CardContent className="space-y-5">
          {current ? (
            <section aria-labelledby="recovery-current-heading">
              <div className="flex flex-col gap-4 rounded-xl border border-border/60 bg-background/50 p-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <h3
                    id="recovery-current-heading"
                    className="text-xs font-semibold uppercase tracking-wider text-muted-foreground"
                  >
                    {TEXT.PROFILE.RECOVERY_CONTACT.CURRENT_LABEL}
                  </h3>
                  <p className="mt-2 truncate font-mono text-base font-semibold tracking-tight">
                    {current.masked_address}
                  </p>
                </div>
                {!pending ? (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => requestIntent("replace")}
                    disabled={isBusy}
                    className="min-h-11 cursor-pointer px-4"
                  >
                    <PencilLine className="size-4" aria-hidden="true" />
                    {TEXT.PROFILE.RECOVERY_CONTACT.REPLACE}
                  </Button>
                ) : null}
              </div>
            </section>
          ) : (
            <div className="rounded-xl border border-status-warning/25 bg-status-warning/10 p-4" role="status">
              <div className="flex items-start gap-3">
                <ShieldAlert className="mt-0.5 size-5 shrink-0 text-status-warning" aria-hidden="true" />
                <div>
                  <p className="font-semibold">
                    {TEXT.PROFILE.RECOVERY_CONTACT.NO_CURRENT}
                  </p>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">
                    {TEXT.PROFILE.RECOVERY_CONTACT.NO_CURRENT_DESCRIPTION}
                  </p>
                </div>
              </div>
            </div>
          )}

          {pending ? (
            <section
              className="rounded-xl border border-status-warning/25 bg-status-warning/5 p-4"
              aria-labelledby="recovery-pending-heading"
            >
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 id="recovery-pending-heading" className="font-semibold">
                      {TEXT.PROFILE.RECOVERY_CONTACT.PENDING_TITLE}
                    </h3>
                    <Badge
                      variant="outline"
                      className="border-status-warning/25 bg-status-warning/10 text-status-warning"
                    >
                      <Clock3 className="size-3" aria-hidden="true" />
                      {TEXT.PROFILE.RECOVERY_CONTACT.PENDING_TITLE}
                    </Badge>
                  </div>
                  <p className="mt-2 truncate font-mono text-sm font-semibold">
                    {pending.masked_address}
                  </p>
                  <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
                    {purposeDescription(pending.purpose)}
                  </p>
                  <dl className="mt-3 grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
                    <div className="flex items-center gap-2">
                      <dt>{TEXT.PROFILE.RECOVERY_CONTACT.DELIVERY_LABEL}</dt>
                      <dd
                        className={
                          pendingDeliveryFailed
                            ? "font-semibold text-destructive"
                            : "font-semibold text-foreground"
                        }
                      >
                        {deliveryLabel(pending.delivery_state)}
                      </dd>
                    </div>
                    <div className="flex items-center gap-2">
                      <dt>{TEXT.PROFILE.RECOVERY_CONTACT.EXPIRES_LABEL}</dt>
                      <dd className="font-semibold text-foreground">
                        {formatDateTime(pending.verification_expires_at)}
                      </dd>
                    </div>
                  </dl>
                </div>
                <div className="flex shrink-0 flex-col gap-2 sm:items-end">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => requestIntent("resend")}
                    disabled={isBusy || resendRemaining > 0}
                    className="min-h-11 cursor-pointer px-4"
                    aria-describedby={
                      resendRemaining > 0 ? "recovery-resend-countdown" : undefined
                    }
                  >
                    {action === "resend" ? (
                      <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                    ) : (
                      <Send className="size-4" aria-hidden="true" />
                    )}
                    {resendRemaining > 0
                      ? TEXT.PROFILE.RECOVERY_CONTACT.RESEND_AFTER(
                          formatCountdown(resendRemaining),
                        )
                      : TEXT.PROFILE.RECOVERY_CONTACT.RESEND}
                  </Button>
                  {resendRemaining > 0 ? (
                    <span id="recovery-resend-countdown" className="sr-only" aria-live="polite">
                      {TEXT.PROFILE.RECOVERY_CONTACT.RESEND_AFTER(
                        formatCountdown(resendRemaining),
                      )}
                    </span>
                  ) : null}
                  {pending.purpose === RECOVERY_CONTACT_PURPOSE.REPLACEMENT ? (
                    <Button
                      type="button"
                      variant="ghost"
                      onClick={() => requestIntent("cancel")}
                      disabled={isBusy}
                      className="min-h-11 cursor-pointer px-4 text-destructive hover:bg-destructive/10 hover:text-destructive"
                    >
                      <XCircle className="size-4" aria-hidden="true" />
                      {TEXT.PROFILE.RECOVERY_CONTACT.CANCEL_PENDING}
                    </Button>
                  ) : null}
                </div>
              </div>
            </section>
          ) : null}

          {hasRecentAuthentication && projection.reauthenticated_until ? (
            <div className="flex items-start gap-2 rounded-lg bg-muted/60 px-3 py-2 text-xs text-muted-foreground" role="status">
              <KeyRound className="mt-0.5 size-3.5 shrink-0 text-status-success" aria-hidden="true" />
              <div>
                <p className="font-semibold text-foreground">
                  {TEXT.PROFILE.RECOVERY_CONTACT.REAUTHENTICATED}
                </p>
                <p>
                  {TEXT.PROFILE.RECOVERY_CONTACT.REAUTHENTICATED_UNTIL(
                    formatDateTime(projection.reauthenticated_until),
                  )}
                </p>
              </div>
            </div>
          ) : null}

          {actionError ? (
            <div
              className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
              role="alert"
            >
              {actionError}
            </div>
          ) : null}

          {current ? (
            <div className="border-t border-border/40 pt-4">
              {projection.can_remove ? (
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => requestIntent("remove")}
                  disabled={isBusy}
                  aria-describedby="recovery-contact-remove-explanation"
                  className="min-h-11 cursor-pointer px-4"
                >
                  {action === "remove" ? (
                    <Loader2
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden="true"
                    />
                  ) : (
                    <Trash2 className="size-4" aria-hidden="true" />
                  )}
                  {TEXT.PROFILE.RECOVERY_CONTACT.REMOVE}
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="destructive"
                  disabled
                  aria-describedby="recovery-contact-remove-explanation"
                  className="min-h-11 px-4"
                >
                  <Trash2 className="size-4" aria-hidden="true" />
                  {TEXT.PROFILE.RECOVERY_CONTACT.REMOVE}
                </Button>
              )}
              <p
                id="recovery-contact-remove-explanation"
                className="mt-2 max-w-2xl text-xs leading-5 text-muted-foreground"
              >
                {projection.can_remove
                  ? TEXT.PROFILE.RECOVERY_CONTACT.REMOVE_DESCRIPTION
                  : TEXT.PROFILE.RECOVERY_CONTACT.REMOVE_DISABLED}
              </p>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {reauthOpen ? (
        <ReauthenticationDialog
          open={reauthOpen}
          error={reauthError}
          isSubmitting={action === "reauthenticate"}
          onOpenChange={(open) => {
            setReauthOpen(open);
            if (!open) {
              setPendingIntent(null);
              setReauthError(null);
            }
          }}
          onSubmit={handleReauthenticate}
        />
      ) : null}

      {replaceOpen ? (
        <ReplaceRecoveryContactDialog
          open={replaceOpen}
          error={replaceError}
          isSubmitting={action === "replace"}
          onOpenChange={(open) => {
            setReplaceOpen(open);
            if (!open) setReplaceError(null);
          }}
          onSubmit={handleReplace}
        />
      ) : null}

      <ConfirmDialog
        open={cancelOpen}
        onOpenChange={(open) => {
          if (action !== "cancel") setCancelOpen(open);
        }}
        onConfirm={() => void handleCancelPending()}
        title={TEXT.PROFILE.RECOVERY_CONTACT.CANCEL_TITLE}
        description={TEXT.PROFILE.RECOVERY_CONTACT.CANCEL_DESCRIPTION}
        confirmLabel={
          action === "cancel"
            ? TEXT.PROFILE.RECOVERY_CONTACT.PROCESSING
            : TEXT.PROFILE.RECOVERY_CONTACT.CANCEL_CONFIRM
        }
        loading={action === "cancel"}
      />

      <ConfirmDialog
        open={removeOpen}
        onOpenChange={(open) => {
          if (action !== "remove") setRemoveOpen(open);
        }}
        onConfirm={() => void handleRemoveCurrent()}
        title={TEXT.PROFILE.RECOVERY_CONTACT.REMOVE_TITLE}
        description={TEXT.PROFILE.RECOVERY_CONTACT.REMOVE_DESCRIPTION}
        confirmLabel={
          action === "remove"
            ? TEXT.PROFILE.RECOVERY_CONTACT.PROCESSING
            : TEXT.PROFILE.RECOVERY_CONTACT.REMOVE_CONFIRM
        }
        loading={action === "remove"}
      />
    </>
  );
}
