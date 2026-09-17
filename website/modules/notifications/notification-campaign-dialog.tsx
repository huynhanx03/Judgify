"use client";

import Link from "next/link";
import {
  CircleAlert,
  ExternalLink,
  LoaderCircle,
  Megaphone,
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
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { notificationService } from "@/services/notification.service";
import type {
  Notification,
  NotificationCampaignContent,
} from "@/types/notification";

interface NotificationCampaignDialogProps {
  open: boolean;
  notification: Notification;
  onOpenChange(open: boolean): void;
}

export function NotificationCampaignDialog({
  open,
  notification,
  onOpenChange,
}: NotificationCampaignDialogProps) {
  const resource = useRetryableResource<NotificationCampaignContent | null>({
    resetKey: `${notification.id}:${notification.campaign_revision_id}`,
    enabled: open,
    initialData: null,
    load: async (signal) => {
      const result = await notificationService.campaignContent(
        notification.id,
        signal,
      );
      if (
        result.notification_id !== notification.id ||
        result.revision_id !== notification.campaign_revision_id
      ) {
        throw new TypeError("notification campaign identity mismatch");
      }
      return result;
    },
  });
  const content = resource.data;
  const state = resource.status;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[calc(100dvh-2rem)] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border px-5 py-5 pr-12 sm:px-7">
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl bg-info/10 text-info">
            <Megaphone className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>
            {content?.title ?? TEXT.NOTIFICATION.CAMPAIGN.DIALOG_TITLE}
          </DialogTitle>
          <DialogDescription>
            {TEXT.NOTIFICATION.CAMPAIGN.DIALOG_DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 overflow-y-auto px-5 py-5 sm:px-7 sm:py-6">
          {state === "loading" ? (
            <div
              className="flex min-h-64 flex-col items-center justify-center text-center"
              role="status"
            >
              <LoaderCircle
                className="size-7 animate-spin text-primary motion-reduce:animate-none"
                aria-hidden="true"
              />
              <p className="mt-3 text-sm text-muted-foreground">
                {TEXT.NOTIFICATION.CAMPAIGN.LOADING}
              </p>
            </div>
          ) : null}

          {state === "error" ? (
            <div
              className="flex min-h-64 flex-col items-center justify-center text-center"
              role="alert"
            >
              <div className="flex size-12 items-center justify-center rounded-xl bg-destructive/10 text-destructive">
                <CircleAlert className="size-5" aria-hidden="true" />
              </div>
              <h2 className="mt-4 font-semibold">
                {TEXT.NOTIFICATION.CAMPAIGN.ERROR_TITLE}
              </h2>
              <p className="mt-1 max-w-md text-sm leading-6 text-muted-foreground">
                {TEXT.NOTIFICATION.CAMPAIGN.ERROR_DESCRIPTION}
              </p>
              <Button
                type="button"
                variant="outline"
                className="mt-4"
                onClick={resource.retry}
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.COMMON.RETRY}
              </Button>
            </div>
          ) : null}

          {state === "ready" && content ? (
            <>
              {/* The API serves an immutable artifact sanitized by the server. */}
              <article
                className="prose max-w-none dark:prose-invert"
                dangerouslySetInnerHTML={{ __html: content.sanitized_html }}
              />
              <p className="mt-8 border-t border-border pt-4 font-mono text-[11px] text-muted-foreground">
                {TEXT.NOTIFICATION.CAMPAIGN.REVISION(content.revision_id)}
              </p>
            </>
          ) : null}
        </div>

        <DialogFooter className="border-t border-border bg-background/95 px-5 py-4 sm:px-7">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {TEXT.COMMON.CLOSE}
          </Button>
          {content?.action_path ? (
            <Button render={<Link href={content.action_path} />}>
              <ExternalLink className="size-4" aria-hidden="true" />
              {TEXT.NOTIFICATION.OPEN_ACTION}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
