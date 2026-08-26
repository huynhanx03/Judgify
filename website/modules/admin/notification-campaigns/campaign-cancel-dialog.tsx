"use client";

import { type FormEvent, useState } from "react";
import { KeyRound, LoaderCircle, OctagonX, ShieldAlert } from "lucide-react";
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
import { CAMPAIGN_LIMITS, CAMPAIGN_STATUS } from "@/constants/notification-campaign";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { ApiError } from "@/lib/api/error";
import { authService } from "@/services/auth.service";
import { notificationCampaignService } from "@/services/notification-campaign.service";
import type {
  CampaignCancelResult,
  NotificationCampaign,
} from "@/types/notification-campaign";

interface CampaignCancelDialogProps {
  campaign: NotificationCampaign | null;
  onOpenChange(open: boolean): void;
  onCancelled(result: CampaignCancelResult): void;
}

const encoder = new TextEncoder();

export function CampaignCancelDialog({
  campaign,
  onOpenChange,
  onCancelled,
}: CampaignCancelDialogProps) {
  const [password, setPassword] = useState("");
  const [reason, setReason] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!campaign?.operation) return null;
  const activeCampaign = campaign;
  const operation = campaign.operation;
  const cancellable =
    activeCampaign.status === CAMPAIGN_STATUS.SCHEDULED ||
    activeCampaign.status === CAMPAIGN_STATUS.SENDING;
  const reasonBytes = encoder.encode(reason.trim()).byteLength;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!cancellable || pending) return;
    if (!password) {
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.PASSWORD_REQUIRED);
      return;
    }
    if (reasonBytes < 1 || reasonBytes > CAMPAIGN_LIMITS.REASON_BYTES) {
      setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.REASON_INVALID);
      return;
    }
    setPending(true);
    setError(null);
    try {
      await authService.reauthenticateRecoveryContact(password);
      setPassword("");
      const result = await notificationCampaignService.cancel(activeCampaign.id, {
        expected_campaign_version: activeCampaign.version,
        expected_operation_version: operation.version,
        reason,
      });
      onCancelled(result);
      onOpenChange(false);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.CONFLICT);
      } else if (cause instanceof ApiError && cause.status === 403) {
        setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.FORBIDDEN);
      } else {
        setError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.ERROR);
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!pending) onOpenChange(open);
      }}
    >
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-destructive/30 bg-destructive/10 text-destructive">
            <OctagonX className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.TITLE}
          </DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-5">
          <section className="rounded-2xl border border-destructive/25 bg-destructive/[0.04] p-4">
            <div className="flex items-start gap-3">
              <ShieldAlert
                className="mt-0.5 size-5 shrink-0 text-destructive"
                aria-hidden="true"
              />
              <div>
                <h3 className="text-sm font-semibold">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.WARNING_TITLE}
                </h3>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.WARNING_DESCRIPTION}
                </p>
              </div>
            </div>
            <p className="mt-4 line-clamp-2 font-semibold">
              {campaign.revision.title}
            </p>
            <div className="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
              <span>
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.CAMPAIGN_VERSION(
                  campaign.version,
                )}
              </span>
              <span aria-hidden="true">·</span>
              <span>
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.OPERATION_VERSION(
                  campaign.operation.version,
                )}
              </span>
            </div>
          </section>

          <div className="space-y-2">
            <Label htmlFor="campaign-cancel-password">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.CURRENT_PASSWORD}
            </Label>
            <div className="relative">
              <KeyRound
                className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
                aria-hidden="true"
              />
              <Input
                id="campaign-cancel-password"
                type="password"
                autoComplete="current-password"
                className="pl-9"
                value={password}
                disabled={pending}
                placeholder={
                  ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.PASSWORD_PLACEHOLDER
                }
                onChange={(event) => setPassword(event.target.value)}
              />
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="campaign-cancel-reason">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.REASON}
              </Label>
              <span className="text-xs tabular-nums text-muted-foreground">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BYTE_COUNT(
                  reasonBytes,
                  CAMPAIGN_LIMITS.REASON_BYTES,
                )}
              </span>
            </div>
            <Textarea
              id="campaign-cancel-reason"
              rows={4}
              value={reason}
              disabled={pending}
              aria-describedby={error ? "campaign-cancel-error" : undefined}
              placeholder={
                ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.REASON_PLACEHOLDER
              }
              onChange={(event) => setReason(event.target.value)}
            />
          </div>

          {error ? (
            <p
              id="campaign-cancel-error"
              className="text-sm text-destructive"
              role="alert"
            >
              {error}
            </p>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => onOpenChange(false)}
            >
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.DISMISS}
            </Button>
            <Button type="submit" variant="destructive" disabled={pending}>
              {pending ? (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : (
                <OctagonX className="size-4" aria-hidden="true" />
              )}
              {pending
                ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.CANCELLING
                : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL.CONFIRM}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
