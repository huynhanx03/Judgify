"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  Check,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Laptop,
  Loader2,
  LogOut,
  RefreshCw,
  Shield,
  Smartphone,
  Tablet,
  XCircle,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { formatDateTime } from "@/lib/format";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { useSession } from "@/contexts/session-context";
import { ApiError } from "@/lib/api/error";
import {
  SESSION_PAGE_SIZE,
  sessionService,
} from "@/services/session.service";
import type {
  AuthSession,
  AuthSessionListResponse,
  AuthSessionStatus,
} from "@/types/session";

type SessionAction = "revoke" | "logout-all" | null;

const EMPTY_PAGE: AuthSessionListResponse = {
  items: [],
  next_cursor: null,
};

function safeDate(value: string): string {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? TEXT.COMMON.UNKNOWN : formatDateTime(value);
}

function userAgentLabel(userAgent: string | undefined): string {
  if (!userAgent) return TEXT.PROFILE.SESSIONS.UNKNOWN_DEVICE;
  const value = userAgent.toLowerCase();
  const browser = value.includes("edg/")
    ? TEXT.PROFILE.SESSIONS.BROWSER_EDGE
    : value.includes("opr/")
      ? TEXT.PROFILE.SESSIONS.BROWSER_OPERA
      : value.includes("firefox/")
        ? TEXT.PROFILE.SESSIONS.BROWSER_FIREFOX
        : value.includes("chrome/")
          ? TEXT.PROFILE.SESSIONS.BROWSER_CHROME
        : value.includes("safari/")
          ? TEXT.PROFILE.SESSIONS.BROWSER_SAFARI
          : TEXT.PROFILE.SESSIONS.BROWSER_GENERIC;
  const platform = value.includes("android")
    ? TEXT.PROFILE.SESSIONS.PLATFORM_ANDROID
    : value.includes("iphone") || value.includes("ipad")
      ? TEXT.PROFILE.SESSIONS.PLATFORM_IOS
      : value.includes("mac os")
        ? TEXT.PROFILE.SESSIONS.PLATFORM_MACOS
        : value.includes("windows")
          ? TEXT.PROFILE.SESSIONS.PLATFORM_WINDOWS
          : value.includes("linux")
            ? TEXT.PROFILE.SESSIONS.PLATFORM_LINUX
            : TEXT.PROFILE.SESSIONS.PLATFORM_WEB;
  return `${browser} • ${platform}`;
}

function DeviceIcon({ userAgent }: { userAgent?: string }) {
  const value = userAgent?.toLowerCase() ?? "";
  if (value.includes("ipad") || value.includes("tablet")) {
    return <Tablet className="size-5" aria-hidden="true" />;
  }
  if (value.includes("android") || value.includes("iphone")) {
    return <Smartphone className="size-5" aria-hidden="true" />;
  }
  return <Laptop className="size-5" aria-hidden="true" />;
}

function statusLabel(status: AuthSessionStatus): string {
  if (status === "active") return TEXT.PROFILE.SESSIONS.ACTIVE;
  if (status === "revoked") return TEXT.PROFILE.SESSIONS.REVOKED;
  return TEXT.PROFILE.SESSIONS.EXPIRED;
}

function statusVariant(
  status: AuthSessionStatus,
): "default" | "secondary" | "destructive" | "outline" {
  if (status === "active") return "default";
  if (status === "revoked") return "destructive";
  return "secondary";
}

function actionErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 404) {
    return TEXT.PROFILE.SESSIONS.NOT_FOUND;
  }
  // API messages are diagnostic/server-owned copy. Keep the UI vocabulary in
  // the single text catalog instead of leaking transport wording into it.
  return TEXT.PROFILE.SESSIONS.ACTION_ERROR;
}

function loadErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 404) {
    return TEXT.PROFILE.SESSIONS.NOT_FOUND;
  }
  return TEXT.PROFILE.SESSIONS.LOAD_ERROR_DESCRIPTION;
}

function SessionRow({
  session,
  disabled,
  onRevoke,
}: {
  session: AuthSession;
  disabled: boolean;
  onRevoke: (session: AuthSession) => void;
}) {
  const canRevoke = session.status === "active";
  return (
    <li
      className="group rounded-2xl border border-border/70 bg-background/70 p-4 transition-colors duration-200 hover:border-primary/35 hover:bg-muted/25 motion-reduce:transition-none"
    >
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <DeviceIcon userAgent={session.user_agent} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate text-sm font-semibold text-foreground">
              {userAgentLabel(session.user_agent)}
            </p>
            {session.current ? (
              <Badge variant="outline" className="border-primary/40 text-primary">
                <Check className="size-3" aria-hidden="true" />
                {TEXT.PROFILE.SESSIONS.CURRENT}
              </Badge>
            ) : null}
            <Badge variant={statusVariant(session.status)}>
              {statusLabel(session.status)}
            </Badge>
          </div>
          <dl className="mt-3 grid gap-x-4 gap-y-1 text-xs text-muted-foreground sm:grid-cols-2">
            <div className="flex min-w-0 items-center gap-1.5">
              <dt className="shrink-0">{TEXT.PROFILE.SESSIONS.LAST_SEEN}:</dt>
              <dd className="truncate tabular-nums text-foreground/80">
                {safeDate(session.last_seen_at)}
              </dd>
            </div>
            <div className="flex min-w-0 items-center gap-1.5">
              <dt className="shrink-0">{TEXT.PROFILE.SESSIONS.CREATED}:</dt>
              <dd className="truncate tabular-nums text-foreground/80">
                {safeDate(session.created_at)}
              </dd>
            </div>
            <div className="flex min-w-0 items-center gap-1.5">
              <dt className="shrink-0">{TEXT.PROFILE.SESSIONS.REAUTHENTICATED}:</dt>
              <dd className="truncate tabular-nums text-foreground/80">
                {safeDate(session.last_reauthenticated_at)}
              </dd>
            </div>
            <div className="flex min-w-0 items-center gap-1.5 sm:col-span-2">
              <dt className="shrink-0">{TEXT.PROFILE.SESSIONS.IDLE_EXPIRES}:</dt>
              <dd className="truncate tabular-nums text-foreground/80">
                {safeDate(session.idle_expires_at)}
              </dd>
            </div>
            <div className="flex min-w-0 items-center gap-1.5 sm:col-span-2">
              <dt className="shrink-0">{TEXT.PROFILE.SESSIONS.EXPIRES}:</dt>
              <dd className="truncate tabular-nums text-foreground/80">
                {safeDate(session.absolute_expires_at)}
              </dd>
            </div>
          </dl>
        </div>
        {canRevoke ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-10 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            disabled={disabled}
            onClick={() => onRevoke(session)}
            aria-label={`${TEXT.PROFILE.SESSIONS.REVOKE}: ${userAgentLabel(session.user_agent)}`}
            title={TEXT.PROFILE.SESSIONS.REVOKE}
          >
            <XCircle aria-hidden="true" />
          </Button>
        ) : null}
      </div>
    </li>
  );
}

export function SessionSecurityCard() {
  const router = useRouter();
  const { revalidate } = useSession();
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [actionError, setActionError] = useState<string | null>(null);
  const [action, setAction] = useState<SessionAction>(null);
  const [selectedSession, setSelectedSession] = useState<AuthSession | null>(null);
  const [logoutAllOpen, setLogoutAllOpen] = useState(false);

  const cursor = cursorStack.at(-1);
  const resourceKey = cursor ?? "profile-sessions-first-page";
  const sessionsResource = useRetryableResource<AuthSessionListResponse>({
    resetKey: resourceKey,
    initialData: EMPTY_PAGE,
    load: (signal) => {
    const query = cursor ? { limit: SESSION_PAGE_SIZE, cursor } : { limit: SESSION_PAGE_SIZE };
      return sessionService.list(query, signal);
    },
    onSuccess: () => setActionError(null),
  });
  const page = sessionsResource.data;
  const status = sessionsResource.status;

  function openRevoke(session: AuthSession) {
    setActionError(null);
    setSelectedSession(session);
  }

  async function confirmLocalSessionEnded() {
    const current = await revalidate();
    if (current) {
      throw new TypeError("session revocation was not confirmed");
    }
    router.replace(APP_ROUTES.LOGIN);
  }

  async function revokeSelected() {
    if (!selectedSession || action) return;
    const target = selectedSession;
    setAction("revoke");
    setActionError(null);
    try {
      await sessionService.revoke(target.id);
      setSelectedSession(null);
      if (target.current) {
        await confirmLocalSessionEnded();
        return;
      }
      sessionsResource.retry();
    } catch (mutationError: unknown) {
      setActionError(actionErrorMessage(mutationError));
    } finally {
      setAction(null);
    }
  }

  async function logoutAll() {
    if (action) return;
    setAction("logout-all");
    setActionError(null);
    try {
      await sessionService.logoutAll();
      setLogoutAllOpen(false);
      await confirmLocalSessionEnded();
    } catch (mutationError: unknown) {
      setActionError(actionErrorMessage(mutationError));
    } finally {
      setAction(null);
    }
  }

  const hasPrevious = cursorStack.length > 0;
  const hasNext = Boolean(page.next_cursor);

  return (
    <>
      <Card className="glass-card overflow-hidden border-border/40">
        <CardHeader className="gap-3 border-b border-border/60 bg-muted/15 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 gap-3">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Shield className="size-5" aria-hidden="true" />
            </div>
            <div>
              <CardTitle className="text-lg">{TEXT.PROFILE.SESSIONS.TITLE}</CardTitle>
              <CardDescription className="mt-1 leading-5">
                {TEXT.PROFILE.SESSIONS.DESCRIPTION}
              </CardDescription>
            </div>
          </div>
          <Button
            type="button"
            variant="destructive"
            className="min-h-11 w-full shrink-0 sm:w-auto"
            disabled={status !== "ready" || Boolean(action)}
            onClick={() => {
              setActionError(null);
              setLogoutAllOpen(true);
            }}
          >
            <LogOut aria-hidden="true" />
            {TEXT.PROFILE.SESSIONS.LOGOUT_ALL}
          </Button>
        </CardHeader>
        <CardContent className="space-y-4 p-4 sm:p-5">
          {actionError ? (
            <div
              className="flex items-start gap-2 rounded-xl border border-destructive/25 bg-destructive/10 p-3 text-sm text-destructive"
              role="alert"
            >
              <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
              <span>{actionError}</span>
            </div>
          ) : null}

          {status === "loading" ? (
            <div className="space-y-3" role="status" aria-live="polite" aria-busy="true">
              <span className="sr-only">{TEXT.PROFILE.SESSIONS.LOADING}</span>
              {[0, 1].map((item) => (
                <div
                  key={item}
                  className="h-24 animate-pulse rounded-2xl bg-muted/50 motion-reduce:animate-none"
                />
              ))}
            </div>
          ) : status === "error" ? (
            <div className="flex min-h-36 flex-col items-center justify-center text-center" role="alert">
              <AlertTriangle className="size-5 text-destructive" aria-hidden="true" />
              <p className="mt-3 max-w-sm text-sm text-muted-foreground">
                {sessionsResource.error
                  ? loadErrorMessage(sessionsResource.error)
                  : TEXT.PROFILE.SESSIONS.LOAD_ERROR_TITLE}
              </p>
              <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
                {hasPrevious ? (
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11"
                    onClick={() => setCursorStack((stack) => stack.slice(0, -1))}
                  >
                    <ChevronLeft aria-hidden="true" />
                    {TEXT.PROFILE.SESSIONS.PREVIOUS}
                  </Button>
                ) : null}
                <Button
                  type="button"
                  variant="outline"
                  className="min-h-11"
                  onClick={sessionsResource.retry}
                >
                  <RefreshCw aria-hidden="true" />
                  {TEXT.PROFILE.SESSIONS.RETRY}
                </Button>
              </div>
            </div>
          ) : page.items.length === 0 ? (
            <div className="flex min-h-36 flex-col items-center justify-center text-center">
              <Clock3 className="size-5 text-muted-foreground" aria-hidden="true" />
              <p className="mt-3 text-sm text-muted-foreground">{TEXT.PROFILE.SESSIONS.EMPTY}</p>
            </div>
          ) : (
            <>
              <ul className="space-y-3" aria-label={TEXT.PROFILE.SESSIONS.TITLE}>
                {page.items.map((session) => (
                  <SessionRow
                    key={session.id}
                    session={session}
                    disabled={Boolean(action)}
                    onRevoke={openRevoke}
                  />
                ))}
              </ul>
              <div className="flex flex-col gap-3 border-t border-border/60 pt-4 sm:flex-row sm:items-center sm:justify-between">
                <p className="text-xs leading-5 text-muted-foreground">
                  {TEXT.PROFILE.SESSIONS.SECURITY_NOTE}
                </p>
                <nav
                  className="flex shrink-0 items-center justify-end gap-2"
                  aria-label={TEXT.PROFILE.SESSIONS.PAGE_LABEL}
                >
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="min-h-10"
                    disabled={!hasPrevious || Boolean(action)}
                    onClick={() => setCursorStack((stack) => stack.slice(0, -1))}
                    aria-label={TEXT.PROFILE.SESSIONS.PREVIOUS}
                  >
                    <ChevronLeft aria-hidden="true" />
                    <span className="hidden sm:inline">{TEXT.PROFILE.SESSIONS.PREVIOUS}</span>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="min-h-10"
                    disabled={!hasNext || Boolean(action)}
                    aria-label={TEXT.PROFILE.SESSIONS.NEXT}
                    onClick={() => {
                      if (page.next_cursor) setCursorStack((stack) => [...stack, page.next_cursor!]);
                    }}
                  >
                    <span className="hidden sm:inline">{TEXT.PROFILE.SESSIONS.NEXT}</span>
                    <ChevronRight aria-hidden="true" />
                  </Button>
                </nav>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={selectedSession !== null}
        onOpenChange={(open) => {
          if (!open && !action) setSelectedSession(null);
        }}
        onConfirm={() => void revokeSelected()}
        title={TEXT.PROFILE.SESSIONS.REVOKE_TITLE}
        description={[
          selectedSession?.current
            ? TEXT.PROFILE.SESSIONS.CURRENT_REVOKE_DESCRIPTION
            : TEXT.PROFILE.SESSIONS.REVOKE_DESCRIPTION,
          actionError,
        ]
          .filter(Boolean)
          .join(" ")}
        confirmLabel={TEXT.PROFILE.SESSIONS.REVOKE_CONFIRM}
        loading={action === "revoke"}
      />
      <ConfirmDialog
        open={logoutAllOpen}
        onOpenChange={(open) => {
          if (!open && !action) setLogoutAllOpen(false);
        }}
        onConfirm={() => void logoutAll()}
        title={TEXT.PROFILE.SESSIONS.LOGOUT_ALL_TITLE}
        description={[TEXT.PROFILE.SESSIONS.LOGOUT_ALL_DESCRIPTION, actionError]
          .filter(Boolean)
          .join(" ")}
        confirmLabel={TEXT.PROFILE.SESSIONS.LOGOUT_ALL_CONFIRM}
        loading={action === "logout-all"}
      />
      {action ? (
        <span className="sr-only" role="status" aria-live="polite">
          <Loader2 className="inline size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
          {TEXT.COMMON.LOADING}
        </span>
      ) : null}
    </>
  );
}
