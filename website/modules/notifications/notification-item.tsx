"use client";

import { useState } from "react";
import Link from "next/link";
import {
  Bell,
  BookOpen,
  Check,
  ChevronRight,
  Code2,
  Megaphone,
  ShieldCheck,
  Trash2,
  Trophy,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { TEXT } from "@/constants/text";
import type { NotificationCategory } from "@/constants/notification";
import {
  formatNotificationAge,
  notificationCategoryCopy,
  notificationCopy,
} from "@/lib/notifications/presentation";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import type { Notification } from "@/types/notification";
import { NotificationCampaignDialog } from "./notification-campaign-dialog";

const CATEGORY_ICON: Readonly<Record<NotificationCategory, LucideIcon>> = {
  authorization: ShieldCheck,
  campaign: Megaphone,
  contest: Trophy,
  material: BookOpen,
  operation: Wrench,
  security: ShieldCheck,
  submission: Code2,
};

const CATEGORY_TONE: Readonly<Record<NotificationCategory, string>> = {
  authorization: "bg-primary/10 text-primary",
  campaign: "bg-info/10 text-info",
  contest: "bg-cultivation/10 text-cultivation",
  material: "bg-success/10 text-success",
  operation: "bg-warning/10 text-warning",
  security: "bg-danger/10 text-danger",
  submission: "bg-primary/10 text-primary",
};

export function NotificationCategoryIcon({
  category,
  className,
}: {
  category: NotificationCategory;
  className?: string;
}) {
  const Icon = CATEGORY_ICON[category] ?? Bell;
  return (
    <span
      className={cn(
        "flex size-10 shrink-0 items-center justify-center rounded-xl",
        CATEGORY_TONE[category],
        className,
      )}
      aria-hidden="true"
    >
      <Icon className="size-4.5" />
    </span>
  );
}

interface NotificationItemProps {
  notification: Notification;
  pending: boolean;
  onMarkRead(id: string): Promise<void>;
  onArchive(id: string): Promise<void>;
}

export function NotificationItem({
  notification,
  pending,
  onMarkRead,
  onArchive,
}: NotificationItemProps) {
  const [campaignOpen, setCampaignOpen] = useState(false);
  const copy = notificationCopy(notification);
  const category = notificationCategoryCopy(notification.category);
  const isUnread = !notification.read_at;

  const markRead = async () => {
    if (!isUnread) return;
    try {
      await onMarkRead(notification.id);
    } catch {
      notify.error(TEXT.NOTIFICATION.ACTION_ERROR);
    }
  };

  const archive = async () => {
    try {
      await onArchive(notification.id);
    } catch {
      notify.error(TEXT.NOTIFICATION.ACTION_ERROR);
    }
  };

  const openCampaign = () => {
    setCampaignOpen(true);
    void markRead();
  };

  return (
    <>
    <article
      className={cn(
        "group relative grid grid-cols-[auto_minmax(0,1fr)] gap-3 rounded-xl border p-4 transition-colors duration-150 motion-reduce:transition-none sm:grid-cols-[auto_minmax(0,1fr)_auto] sm:gap-4",
        isUnread
          ? "border-primary/20 bg-primary/[0.045] shadow-sm"
          : "border-border/70 bg-card hover:border-border-strong",
      )}
    >
      <NotificationCategoryIcon category={notification.category} />
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h2 className="min-w-0 text-sm font-semibold leading-5 text-foreground sm:text-base">
            {copy.title}
          </h2>
          {isUnread ? (
            <Badge variant="secondary" className="bg-primary/10 text-primary">
              {TEXT.NOTIFICATION.NEW}
            </Badge>
          ) : null}
        </div>
        <p className="mt-1 text-sm leading-6 text-muted-foreground">
          {copy.body}
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
          <span>{category.label}</span>
          <span aria-hidden="true">·</span>
          <time dateTime={notification.created_at}>
            {formatNotificationAge(notification.created_at)}
          </time>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-4">
          {notification.content_kind === "campaign" ? (
            <button
              type="button"
              onClick={openCampaign}
              className="inline-flex min-h-11 items-center gap-1.5 rounded-md text-sm font-semibold text-primary outline-none transition-colors hover:text-primary/80 focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {TEXT.NOTIFICATION.CAMPAIGN.READ}
              <ChevronRight className="size-4" aria-hidden="true" />
            </button>
          ) : null}
          {notification.action_path ? (
            <Link
              href={notification.action_path}
              onClick={() => void markRead()}
              className="inline-flex min-h-11 items-center gap-1.5 rounded-md text-sm font-semibold text-primary outline-none transition-colors hover:text-primary/80 focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {TEXT.NOTIFICATION.OPEN_ACTION}
              <ChevronRight className="size-4" aria-hidden="true" />
            </Link>
          ) : null}
        </div>
      </div>
      <div className="col-span-2 flex items-center justify-end gap-1 sm:col-span-1 sm:self-start">
        {isUnread ? (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={pending}
                  onClick={() => void markRead()}
                  aria-label={TEXT.NOTIFICATION.MARK_READ}
                />
              }
            >
              <Check className="size-4" aria-hidden="true" />
            </TooltipTrigger>
            <TooltipContent>{TEXT.NOTIFICATION.MARK_READ}</TooltipContent>
          </Tooltip>
        ) : null}
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                type="button"
                variant="ghost"
                size="icon"
                disabled={pending}
                onClick={() => void archive()}
                aria-label={TEXT.NOTIFICATION.ARCHIVE}
                className="text-muted-foreground hover:text-destructive"
              />
            }
          >
            <Trash2 className="size-4" aria-hidden="true" />
          </TooltipTrigger>
          <TooltipContent>{TEXT.NOTIFICATION.ARCHIVE}</TooltipContent>
        </Tooltip>
      </div>
    </article>
    {notification.content_kind === "campaign" ? (
      <NotificationCampaignDialog
        open={campaignOpen}
        notification={notification}
        onOpenChange={setCampaignOpen}
      />
    ) : null}
    </>
  );
}
