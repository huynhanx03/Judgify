"use client";

import { type FormEvent, useMemo, useState } from "react";
import {
  CalendarClock,
  CheckCircle2,
  KeyRound,
  LoaderCircle,
  RefreshCw,
  Send,
  ShieldAlert,
  UsersRound,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  CAMPAIGN_AUDIENCE,
  CAMPAIGN_LIMITS,
} from "@/constants/notification-campaign";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { formatDateTime } from "@/lib/format";
import { campaignAudienceKindLabel } from "@/lib/notifications/campaign-presentation";
import { authService } from "@/services/auth.service";
import { notificationCampaignService } from "@/services/notification-campaign.service";
import type {
  CampaignDryRun,
  CampaignScheduleResult,
  NotificationCampaign,
} from "@/types/notification-campaign";

interface CampaignReviewDialogProps {
  open: boolean;
  campaign: NotificationCampaign | null;
  onOpenChange(open: boolean): void;
  onScheduled(result: CampaignScheduleResult): void;
}

type ScheduleMode = "now" | "later";
type ReviewState = "idle" | "reviewing" | "reviewed" | "scheduling";

const encoder = new TextEncoder();

function localDateTimeValue(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function scheduledFor(mode: ScheduleMode, value: string): string | undefined {
  if (mode === "now") return undefined;
  const parsed = new Date(value);
  if (!value || Number.isNaN(parsed.getTime()) || parsed.getTime() <= Date.now()) {
    throw new TypeError("invalid campaign schedule");
  }
  const maximum = Date.now() + CAMPAIGN_LIMITS.SCHEDULE_AHEAD_DAYS * 86_400_000;
  if (parsed.getTime() > maximum) {
    throw new TypeError("campaign schedule exceeds policy");
  }
  return parsed.toISOString();
}

export function CampaignReviewDialog({
  open,
  campaign,
  onOpenChange,
  onScheduled,
}: CampaignReviewDialogProps) {
  const [mode, setMode] = useState<ScheduleMode>("now");
  const [scheduledAt, setScheduledAt] = useState(() =>
    localDateTimeValue(new Date(Date.now() + 5 * 60_000)),
  );
  const [password, setPassword] = useState("");
  const [reason, setReason] = useState("");
  const [review, setReview] = useState<CampaignDryRun | null>(null);
  const [state, setState] = useState<ReviewState>("idle");
  const [error, setError] = useState<string | null>(null);

  const broadAudience =
    campaign?.revision.audience.kind !== CAMPAIGN_AUDIENCE.EXPLICIT_USERS;
  const busy = state === "reviewing" || state === "scheduling";
  const reasonBytes = encoder.encode(reason.trim()).byteLength;
  const minimumSchedule = useMemo(
    () => localDateTimeValue(new Date(Date.now() + 60_000)),
    [],
  );

  function invalidateReview() {
    setReview(null);
    setState("idle");
    setError(null);
  }

  async function runReview() {
    if (!campaign || campaign.status !== "draft") return;
    setError(null);
    let schedule: string | undefined;
    try {
      schedule = scheduledFor(mode, scheduledAt);
    } catch {
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SCHEDULE_INVALID);
      return;
    }
    if (broadAudience && !password) {
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.PASSWORD_REQUIRED);
      return;
    }
    setState("reviewing");
    try {
      if (broadAudience) {
        await authService.reauthenticateRecoveryContact(password);
      }
      const result = await notificationCampaignService.dryRun(campaign.id, {
        expected_version: campaign.version,
        ...(schedule ? { scheduled_for: schedule } : {}),
      });
      setPassword("");
      setReview(result);
      setState("reviewed");
    } catch {
      setReview(null);
      setState("idle");
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REVIEW_ERROR);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!campaign || !review || state !== "reviewed") return;
    if (
      reasonBytes < 1 ||
      reasonBytes > CAMPAIGN_LIMITS.REASON_BYTES
    ) {
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REASON_INVALID);
      return;
    }
    if (Date.parse(review.confirmation_expires_at) <= Date.now()) {
      setReview(null);
      setState("idle");
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.CONFIRMATION_EXPIRED);
      return;
    }
    setError(null);
    setState("scheduling");
    try {
      const result = await notificationCampaignService.schedule(campaign.id, {
        confirmation_token: review.confirmation_token,
        reason,
      });
      onScheduled(result);
      onOpenChange(false);
    } catch {
      setState("reviewed");
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SCHEDULE_ERROR);
    }
  }

  if (!campaign) return null;

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!busy) onOpenChange(next);
      }}
    >
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-warning/30 bg-warning/10 text-warning">
            <ShieldAlert className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.TITLE}
          </DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-5">
          <section className="rounded-2xl border border-border bg-muted/25 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="text-xs font-semibold tracking-[0.14em] text-primary uppercase">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.IMMUTABLE_REVISION}
                </p>
                <h3 className="mt-2 truncate text-lg font-semibold">
                  {campaign.revision.title}
                </h3>
                <p className="mt-1 text-xs text-muted-foreground">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVISION(
                    campaign.revision.revision_number,
                  )}{" "}
                  · {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.VERSION(campaign.version)}
                </p>
              </div>
              <Badge variant="outline">
                {campaignAudienceKindLabel(campaign.revision.audience.kind)}
              </Badge>
            </div>
          </section>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="campaign-schedule-mode">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SCHEDULE_MODE}
              </Label>
              <Select
                value={mode}
                disabled={busy}
                onValueChange={(value) => {
                  setMode(value as ScheduleMode);
                  invalidateReview();
                }}
              >
                <SelectTrigger id="campaign-schedule-mode" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="now">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SEND_NOW}
                  </SelectItem>
                  <SelectItem value="later">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SEND_LATER}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            {mode === "later" ? (
              <div className="space-y-2">
                <Label htmlFor="campaign-scheduled-at">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SCHEDULED_FOR}
                </Label>
                <Input
                  id="campaign-scheduled-at"
                  type="datetime-local"
                  min={minimumSchedule}
                  value={scheduledAt}
                  disabled={busy}
                  onChange={(event) => {
                    setScheduledAt(event.target.value);
                    invalidateReview();
                  }}
                />
              </div>
            ) : (
              <div className="flex items-end">
                <div className="flex min-h-10 w-full items-center gap-2 rounded-lg border border-border bg-card px-3 text-sm text-muted-foreground">
                  <CalendarClock className="size-4 text-primary" aria-hidden="true" />
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.IMMEDIATE_HINT}
                </div>
              </div>
            )}
          </div>

          {broadAudience ? (
            <section className="space-y-3 rounded-2xl border border-warning/30 bg-warning/10 p-4">
              <div className="flex items-start gap-3">
                <KeyRound
                  className="mt-0.5 size-5 shrink-0 text-warning"
                  aria-hidden="true"
                />
                <div>
                  <h3 className="text-sm font-semibold text-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REAUTH_TITLE}
                  </h3>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REAUTH_DESCRIPTION}
                  </p>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="campaign-current-password">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.CURRENT_PASSWORD}
                </Label>
                <Input
                  id="campaign-current-password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  disabled={busy || Boolean(review)}
                  onChange={(event) => {
                    setPassword(event.target.value);
                    invalidateReview();
                  }}
                  placeholder={
                    ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.PASSWORD_PLACEHOLDER
                  }
                />
              </div>
            </section>
          ) : null}

          {review ? (
            <section
              className="space-y-4 rounded-2xl border border-success/30 bg-success/10 p-4"
              aria-live="polite"
            >
              <div className="flex items-start gap-3">
                <CheckCircle2
                  className="mt-0.5 size-5 shrink-0 text-success"
                  aria-hidden="true"
                />
                <div>
                  <h3 className="font-semibold text-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.READY_TITLE}
                  </h3>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.RECIPIENT_COUNT(
                      review.recipient_count,
                    )}
                  </p>
                </div>
              </div>
              <dl className="grid gap-3 text-sm sm:grid-cols-2">
                <div>
                  <dt className="text-xs text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.ESTIMATED_AT}
                  </dt>
                  <dd className="mt-1 tabular-nums">
                    {formatDateTime(review.estimated_at)}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.CONFIRMATION_EXPIRES}
                  </dt>
                  <dd className="mt-1 tabular-nums">
                    {formatDateTime(review.confirmation_expires_at)}
                  </dd>
                </div>
              </dl>
              {review.sample.length > 0 ? (
                <div>
                  <p className="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                    <UsersRound className="size-4" aria-hidden="true" />
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SAMPLE}
                  </p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {review.sample.map((member) => (
                      <Badge key={member.user_id} variant="outline">
                        {member.username}
                      </Badge>
                    ))}
                  </div>
                </div>
              ) : null}
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={busy}
                onClick={() => {
                  setReview(null);
                  setState("idle");
                }}
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REVIEW_AGAIN}
              </Button>
            </section>
          ) : (
            <Button
              type="button"
              variant="outline"
              className="w-full"
              disabled={busy}
              onClick={() => void runReview()}
            >
              {state === "reviewing" ? (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : (
                <ShieldAlert className="size-4" aria-hidden="true" />
              )}
              {state === "reviewing"
                ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REVIEWING
                : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.RUN_REVIEW}
            </Button>
          )}

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="campaign-schedule-reason">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REASON}
              </Label>
              <span className="text-xs tabular-nums text-muted-foreground">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BYTE_COUNT(
                  reasonBytes,
                  CAMPAIGN_LIMITS.REASON_BYTES,
                )}
              </span>
            </div>
            <Textarea
              id="campaign-schedule-reason"
              value={reason}
              disabled={busy}
              onChange={(event) => setReason(event.target.value)}
              placeholder={
                ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.REASON_PLACEHOLDER
              }
              className="min-h-24 resize-y"
            />
          </div>

          {error ? (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          ) : null}

          <DialogFooter className="sticky bottom-0 bg-background/95 supports-backdrop-filter:backdrop-blur">
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => onOpenChange(false)}
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button type="submit" disabled={!review || busy}>
              {state === "scheduling" ? (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : (
                <Send className="size-4" aria-hidden="true" />
              )}
              {state === "scheduling"
                ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.SCHEDULING
                : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW.CONFIRM}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
