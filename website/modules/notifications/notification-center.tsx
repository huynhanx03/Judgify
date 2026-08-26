"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Bell,
  CheckCheck,
  CircleAlert,
  Loader2,
  LogIn,
  RefreshCw,
  Settings2,
  Wifi,
  WifiOff,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { useAuth } from "@/contexts/auth-context";
import { useNotifications } from "@/contexts/notification-context";
import { useRealtime } from "@/contexts/realtime-context";
import { notificationCategoryCopy } from "@/lib/notifications/presentation";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import {
  NotificationCategoryIcon,
  NotificationItem,
} from "@/modules/notifications/notification-item";

const TAB_INBOX = "inbox";
const TAB_PREFERENCES = "preferences";

export function NotificationCenter() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { isAuthenticated, isLoading } = useAuth();
  const realtime = useRealtime();
  const notifications = useNotifications();
  const activeTab =
    searchParams.get("tab") === TAB_PREFERENCES
      ? TAB_PREFERENCES
      : TAB_INBOX;

  if (isLoading) {
    return (
      <div className="flex min-h-[50dvh] items-center justify-center" role="status">
        <Loader2 className="size-7 animate-spin text-primary motion-reduce:animate-none" />
        <span className="sr-only">{TEXT.COMMON.LOADING}</span>
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <Card className="mx-auto mt-12 max-w-xl border-border/70 bg-card/90 shadow-xl">
        <CardContent className="flex flex-col items-center px-6 py-12 text-center sm:px-10">
          <div className="flex size-16 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/5">
            <Bell className="size-7" aria-hidden="true" />
          </div>
          <h1 className="mt-6 text-2xl font-bold tracking-tight">
            {TEXT.NOTIFICATION.TITLE}
          </h1>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {TEXT.NOTIFICATION.DESCRIPTION}
          </p>
          <Link
            href={APP_ROUTES.LOGIN}
            className={cn(buttonVariants(), "mt-7")}
          >
            <LogIn className="size-4" aria-hidden="true" />
            {TEXT.AUTH.LOGIN}
          </Link>
        </CardContent>
      </Card>
    );
  }

  return (
    <section className="mx-auto w-full max-w-5xl" aria-labelledby="notification-title">
      <div className="flex flex-col gap-5 rounded-2xl border border-border/70 bg-card/80 p-5 shadow-sm backdrop-blur-sm sm:p-7 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/5">
            <Bell className="size-5" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <p className="text-xs font-bold uppercase tracking-[0.18em] text-primary">
              {TEXT.NOTIFICATION.EYEBROW}
            </p>
            <h1 id="notification-title" className="mt-1 text-2xl font-black tracking-tight sm:text-3xl">
              {TEXT.NOTIFICATION.TITLE}
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground sm:text-base">
              {TEXT.NOTIFICATION.DESCRIPTION}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="min-h-8 gap-1.5 px-3">
            {realtime.state === "open" ? (
              <Wifi className="size-3.5 text-success" aria-hidden="true" />
            ) : (
              <WifiOff className="size-3.5 text-muted-foreground" aria-hidden="true" />
            )}
            {realtime.state === "open"
              ? TEXT.NOTIFICATION.LIVE
              : TEXT.NOTIFICATION.RECONNECTING}
          </Badge>
          <Badge className="min-h-8 px-3" aria-live="polite">
            {TEXT.NOTIFICATION.UNREAD_SHORT(notifications.unreadCount)}
          </Badge>
        </div>
      </div>

      <Tabs
        value={activeTab}
        onValueChange={(value) => {
          const destination =
            value === TAB_PREFERENCES
              ? `${APP_ROUTES.NOTIFICATIONS}?tab=${TAB_PREFERENCES}`
              : APP_ROUTES.NOTIFICATIONS;
          router.replace(destination, { scroll: false });
        }}
        className="mt-6"
      >
        <TabsList className="grid w-full grid-cols-2 sm:w-auto">
          <TabsTrigger value={TAB_INBOX}>
            <Bell className="size-4" aria-hidden="true" />
            {TEXT.NOTIFICATION.TITLE}
          </TabsTrigger>
          <TabsTrigger value={TAB_PREFERENCES}>
            <Settings2 className="size-4" aria-hidden="true" />
            {TEXT.NOTIFICATION.SETTINGS}
          </TabsTrigger>
        </TabsList>
        <TabsContent value={TAB_INBOX} className="mt-4">
          <InboxPanel />
        </TabsContent>
        <TabsContent value={TAB_PREFERENCES} className="mt-4">
          <PreferencePanel />
        </TabsContent>
      </Tabs>
    </section>
  );
}

function InboxPanel() {
  const {
    items,
    unreadCount,
    nextCursor,
    status,
    isLoadingMore,
    isMarkingAllRead,
    pendingNotificationIDs,
    refresh,
    loadMore,
    markRead,
    markAllRead,
    archive,
  } = useNotifications();

  return (
    <Card className="border-border/70">
      <CardHeader className="border-b border-border/70 sm:grid-cols-[1fr_auto]">
        <div>
          <CardTitle>{TEXT.NOTIFICATION.TITLE}</CardTitle>
          <CardDescription className="mt-1">
            {TEXT.NOTIFICATION.UNREAD_BADGE(unreadCount)}
          </CardDescription>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={unreadCount === 0 || isMarkingAllRead}
          onClick={() =>
            void markAllRead().catch(() =>
              notify.error(TEXT.NOTIFICATION.ACTION_ERROR),
            )
          }
          className="mt-3 w-full sm:mt-0 sm:w-auto"
        >
          {isMarkingAllRead ? (
            <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
          ) : (
            <CheckCheck className="size-4" aria-hidden="true" />
          )}
          {isMarkingAllRead
            ? TEXT.NOTIFICATION.MARKING_ALL_READ
            : TEXT.NOTIFICATION.MARK_ALL_READ}
        </Button>
      </CardHeader>
      <CardContent className="p-3 sm:p-4">
        {status === "loading" && items.length === 0 ? (
          <div className="space-y-3" role="status">
            {[0, 1, 2, 3].map((item) => (
              <div
                key={item}
                className="h-32 animate-pulse rounded-xl bg-muted motion-reduce:animate-none"
              />
            ))}
            <span className="sr-only">{TEXT.NOTIFICATION.LOADING}</span>
          </div>
        ) : null}
        {status === "error" && items.length === 0 ? (
          <ResourceError onRetry={refresh} />
        ) : null}
        {status === "ready" && items.length === 0 ? (
          <div className="flex flex-col items-center px-5 py-14 text-center">
            <div className="flex size-14 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/5">
              <Bell className="size-6" aria-hidden="true" />
            </div>
            <h2 className="mt-5 text-lg font-bold">{TEXT.NOTIFICATION.EMPTY_TITLE}</h2>
            <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
              {TEXT.NOTIFICATION.EMPTY_DESCRIPTION}
            </p>
          </div>
        ) : null}
        {items.length > 0 ? (
          <div className="space-y-3">
            {items.map((notification) => (
              <NotificationItem
                key={notification.id}
                notification={notification}
                pending={pendingNotificationIDs.has(notification.id)}
                onMarkRead={markRead}
                onArchive={archive}
              />
            ))}
          </div>
        ) : null}
        {nextCursor ? (
          <div className="flex justify-center pt-5">
            <Button
              type="button"
              variant="outline"
              disabled={isLoadingMore}
              onClick={() =>
                void loadMore().catch(() =>
                  notify.error(TEXT.NOTIFICATION.ERROR_DESCRIPTION),
                )
              }
              className="w-full sm:w-auto"
            >
              {isLoadingMore ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {isLoadingMore
                ? TEXT.NOTIFICATION.LOADING_MORE
                : TEXT.NOTIFICATION.LOAD_MORE}
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

function ResourceError({ onRetry }: { onRetry(): Promise<void> }) {
  return (
    <div className="flex flex-col items-center px-5 py-14 text-center" role="alert">
      <div className="flex size-14 items-center justify-center rounded-2xl bg-destructive/10 text-destructive ring-8 ring-destructive/5">
        <CircleAlert className="size-6" aria-hidden="true" />
      </div>
      <h2 className="mt-5 text-lg font-bold">{TEXT.NOTIFICATION.ERROR_TITLE}</h2>
      <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
        {TEXT.NOTIFICATION.ERROR_DESCRIPTION}
      </p>
      <Button type="button" onClick={() => void onRetry()} className="mt-5">
        <RefreshCw className="size-4" aria-hidden="true" />
        {TEXT.NOTIFICATION.RETRY}
      </Button>
    </div>
  );
}

function PreferencePanel() {
  const {
    preferences,
    preferenceStatus,
    pendingPreferenceCategories,
    loadPreferences,
    updatePreference,
  } = useNotifications();

  useEffect(() => {
    void loadPreferences();
  }, [loadPreferences]);

  return (
    <Card className="border-border/70">
      <CardHeader className="border-b border-border/70">
        <CardTitle>{TEXT.NOTIFICATION.PREFERENCES.TITLE}</CardTitle>
        <CardDescription className="max-w-2xl leading-6">
          {TEXT.NOTIFICATION.PREFERENCES.DESCRIPTION}
        </CardDescription>
      </CardHeader>
      <CardContent className="p-3 sm:p-4">
        {preferenceStatus === "loading" && preferences.length === 0 ? (
          <div className="space-y-3" role="status">
            {[0, 1, 2, 3].map((item) => (
              <div
                key={item}
                className="h-24 animate-pulse rounded-xl bg-muted motion-reduce:animate-none"
              />
            ))}
            <span className="sr-only">{TEXT.NOTIFICATION.PREFERENCES.LOADING}</span>
          </div>
        ) : null}
        {preferenceStatus === "error" && preferences.length === 0 ? (
          <div className="rounded-xl border border-destructive/20 bg-destructive/[0.04] p-5 text-center" role="alert">
            <CircleAlert className="mx-auto size-6 text-destructive" aria-hidden="true" />
            <h2 className="mt-3 font-semibold">
              {TEXT.NOTIFICATION.PREFERENCES.ERROR_TITLE}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {TEXT.NOTIFICATION.PREFERENCES.ERROR_DESCRIPTION}
            </p>
            <Button
              type="button"
              variant="outline"
              onClick={() => void loadPreferences({ force: true })}
              className="mt-4"
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.NOTIFICATION.PREFERENCES.RETRY}
            </Button>
          </div>
        ) : null}
        {preferences.length > 0 ? (
          <div className="space-y-3">
            {preferences.map((preference) => {
              const copy = notificationCategoryCopy(preference.category);
              const pending = pendingPreferenceCategories.has(preference.category);
              return (
                <div
                  key={preference.category}
                  className="flex min-h-24 items-center gap-3 rounded-xl border border-border/70 bg-card p-4 transition-colors hover:border-border-strong motion-reduce:transition-none sm:gap-4"
                >
                  <NotificationCategoryIcon category={preference.category} />
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="font-semibold">{copy.label}</h2>
                      {preference.required ? (
                        <Badge variant="outline">
                          {TEXT.NOTIFICATION.PREFERENCES.REQUIRED}
                        </Badge>
                      ) : null}
                    </div>
                    <p className="mt-1 text-sm leading-5 text-muted-foreground">
                      {preference.required
                        ? TEXT.NOTIFICATION.PREFERENCES.REQUIRED_DESCRIPTION
                        : copy.description}
                    </p>
                  </div>
                  <div className="flex min-w-16 shrink-0 flex-col items-end gap-2">
                    <span className="hidden text-xs font-medium text-muted-foreground sm:block">
                      {TEXT.NOTIFICATION.PREFERENCES.CHANNEL}
                    </span>
                    {pending ? (
                      <Loader2 className="mr-2 size-4 animate-spin text-muted-foreground motion-reduce:animate-none" aria-hidden="true" />
                    ) : (
                      <Switch
                        checked={preference.in_app}
                        disabled={preference.required}
                        onCheckedChange={(checked) => {
                          void updatePreference(preference.category, checked).catch(() =>
                            notify.error(TEXT.NOTIFICATION.PREFERENCE_ACTION_ERROR),
                          );
                        }}
                        aria-label={
                          preference.in_app
                            ? TEXT.NOTIFICATION.PREFERENCES.DISABLE(copy.label)
                            : TEXT.NOTIFICATION.PREFERENCES.ENABLE(copy.label)
                        }
                      />
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
