"use client";

import { useRouter } from "next/navigation";
import { Bell, CheckCheck, ChevronRight, RefreshCw, Settings2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { NOTIFICATION_POLICY } from "@/constants/notification";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { useNotifications } from "@/contexts/notification-context";
import {
  formatNotificationAge,
  notificationCopy,
} from "@/lib/notifications/presentation";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { NotificationCategoryIcon } from "@/modules/notifications/notification-item";
import type { Notification } from "@/types/notification";

function compactUnreadLabel(count: number): string {
  return count > 99
    ? TEXT.NOTIFICATION.UNREAD_OVERFLOW(99)
    : TEXT.NOTIFICATION.UNREAD_BADGE(count);
}

export function NotificationMenu() {
  const router = useRouter();
  const {
    items,
    unreadCount,
    status,
    error,
    isMarkingAllRead,
    refresh,
    markRead,
    markAllRead,
  } = useNotifications();
  const previews = items.slice(0, NOTIFICATION_POLICY.DROPDOWN_PREVIEW_SIZE);

  const openNotification = (notification: Notification) => {
    if (!notification.read_at) {
      void markRead(notification.id).catch(() =>
        notify.error(TEXT.NOTIFICATION.ACTION_ERROR),
      );
    }
    if (notification.action_path) router.push(notification.action_path);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="relative"
            aria-label={
              unreadCount > 0
                ? compactUnreadLabel(unreadCount)
                : TEXT.NOTIFICATION.OPEN
            }
            title={TEXT.NOTIFICATION.OPEN}
          />
        }
      >
        <Bell className="size-5" aria-hidden="true" />
        {unreadCount > 0 ? (
          <span
            className="absolute right-0.5 top-0.5 flex min-w-5 translate-x-0.5 -translate-y-0.5 items-center justify-center rounded-full border-2 border-surface bg-primary px-1 text-[10px] font-bold leading-4 text-primary-foreground tabular-nums"
            aria-hidden="true"
          >
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        ) : null}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        sideOffset={8}
        className="w-[min(25rem,calc(100vw-1rem))] overflow-hidden p-0"
      >
        <div className="flex items-center justify-between gap-3 border-b border-border/70 px-4 py-3">
          <div className="min-w-0">
            <p className="font-semibold text-foreground">
              {TEXT.NOTIFICATION.TITLE}
            </p>
            <p className="mt-0.5 text-xs text-muted-foreground" aria-live="polite">
              {TEXT.NOTIFICATION.UNREAD_SHORT(unreadCount)}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={unreadCount === 0 || isMarkingAllRead}
            onClick={() =>
              void markAllRead().catch(() =>
                notify.error(TEXT.NOTIFICATION.ACTION_ERROR),
              )
            }
            className="shrink-0"
          >
            <CheckCheck className="size-4" aria-hidden="true" />
            <span className="hidden sm:inline">
              {isMarkingAllRead
                ? TEXT.NOTIFICATION.MARKING_ALL_READ
                : TEXT.NOTIFICATION.MARK_ALL_READ}
            </span>
          </Button>
        </div>

        <div className="max-h-[min(66dvh,34rem)] overflow-y-auto p-1.5">
          {status === "loading" && previews.length === 0 ? (
            <div className="space-y-2 p-2" role="status">
              {[0, 1, 2].map((item) => (
                <div
                  key={item}
                  className="h-20 animate-pulse rounded-xl bg-muted motion-reduce:animate-none"
                />
              ))}
              <span className="sr-only">{TEXT.NOTIFICATION.LOADING}</span>
            </div>
          ) : null}
          {status === "error" && previews.length === 0 ? (
            <div className="p-5 text-center" role="alert">
              <p className="text-sm font-semibold">{TEXT.NOTIFICATION.ERROR_TITLE}</p>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {TEXT.NOTIFICATION.ERROR_DESCRIPTION}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void refresh()}
                className="mt-3"
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.NOTIFICATION.RETRY}
              </Button>
            </div>
          ) : null}
          {status !== "loading" && !error && previews.length === 0 ? (
            <div className="px-5 py-8 text-center">
              <div className="mx-auto flex size-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Bell className="size-5" aria-hidden="true" />
              </div>
              <p className="mt-3 text-sm font-semibold">
                {TEXT.NOTIFICATION.EMPTY_TITLE}
              </p>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {TEXT.NOTIFICATION.EMPTY_DESCRIPTION}
              </p>
            </div>
          ) : null}
          {previews.map((notification) => {
            const copy = notificationCopy(notification);
            return (
              <DropdownMenuItem
                key={notification.id}
                onClick={() => openNotification(notification)}
                className={cn(
                  "h-auto items-start gap-3 rounded-xl px-3 py-3",
                  !notification.read_at && "bg-primary/[0.045]",
                )}
              >
                <NotificationCategoryIcon
                  category={notification.category}
                  className="mt-0.5 size-9 rounded-lg"
                />
                <span className="min-w-0 flex-1">
                  <span className="flex items-start gap-2">
                    <span className="line-clamp-1 flex-1 text-sm font-semibold text-foreground">
                      {copy.title}
                    </span>
                    {!notification.read_at ? (
                      <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary" />
                    ) : null}
                  </span>
                  <span className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted-foreground">
                    {copy.body}
                  </span>
                  <span className="mt-1 block text-[11px] text-muted-foreground">
                    {formatNotificationAge(notification.created_at)}
                  </span>
                </span>
                <ChevronRight className="mt-3 size-4 text-muted-foreground" aria-hidden="true" />
              </DropdownMenuItem>
            );
          })}
        </div>

        <DropdownMenuSeparator className="m-0" />
        <div className="grid grid-cols-2 gap-1 p-1.5">
          <DropdownMenuItem
            onClick={() => router.push(APP_ROUTES.NOTIFICATIONS)}
            className="justify-center text-center font-semibold"
          >
            {TEXT.NOTIFICATION.VIEW_ALL}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => router.push(`${APP_ROUTES.NOTIFICATIONS}?tab=preferences`)}
            className="justify-center text-center font-semibold"
          >
            <Settings2 className="size-4" aria-hidden="true" />
            {TEXT.NOTIFICATION.SETTINGS}
          </DropdownMenuItem>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
