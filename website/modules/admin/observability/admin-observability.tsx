"use client";

import {
  type FormEvent,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  AlertTriangle,
  Bug,
  Clock3,
  RefreshCw,
  ShieldCheck,
  TimerOff,
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
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  DEBUG_WINDOW_DURATION_OPTIONS,
  DEBUG_WINDOW_REASON_LIMITS,
} from "@/constants/observability";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { formatDateTime } from "@/lib/format";
import { isDebugWindowReasonValid } from "@/lib/observability/control";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { observabilityService } from "@/services/observability.service";
import type { DebugWindow } from "@/types/observability";

const EMPTY_DEBUG_WINDOW: DebugWindow = { active: false };
const DEFAULT_DURATION_SECONDS = DEBUG_WINDOW_DURATION_OPTIONS[1];
const EXPIRY_REFRESH_GRACE_MILLISECONDS = 250;
const MAXIMUM_BROWSER_TIMEOUT_MILLISECONDS = 2_147_000_000;

function commandErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 403) {
    return ADMIN_TEXT.OBSERVABILITY.REAUTHENTICATION_REQUIRED;
  }
  if (error instanceof ApiError && error.status === 409) {
    return ADMIN_TEXT.OBSERVABILITY.CONFLICT;
  }
  return ADMIN_TEXT.OBSERVABILITY.COMMAND_ERROR;
}

export function AdminObservability() {
  const { can } = useAuth();
  const canUpdate = can(
    AUTHORIZATION_RESOURCE.OBSERVABILITY,
    AUTHORIZATION_ACTION.UPDATE,
  );
  const [hasLoaded, setHasLoaded] = useState(false);
  const [commandWindow, setCommandWindow] =
    useState<DebugWindow | null>(null);
  const [durationSeconds, setDurationSeconds] = useState<number>(
    DEFAULT_DURATION_SECONDS,
  );
  const [reason, setReason] = useState("");
  const [isMutating, setIsMutating] = useState(false);
  const resource = useRetryableResource<DebugWindow>({
    resetKey: "admin-observability-debug-window",
    initialData: EMPTY_DEBUG_WINDOW,
    keepPreviousData: true,
    load: (signal) => observabilityService.getDebugWindow(signal),
    onSuccess: () => {
      setHasLoaded(true);
      setCommandWindow(null);
    },
  });
  const retryDebugWindow = resource.retry;
  const debugWindow = commandWindow ?? resource.data;
  const reasonValid = useMemo(
    () => isDebugWindowReasonValid(reason),
    [reason],
  );

  useEffect(() => {
    if (!debugWindow.active || !debugWindow.expires_at) return;
    const delay = new Date(debugWindow.expires_at).getTime() - Date.now();
    const timeout = window.setTimeout(() => {
      setCommandWindow(null);
      retryDebugWindow();
    }, Math.max(
      0,
      Math.min(
        delay + EXPIRY_REFRESH_GRACE_MILLISECONDS,
        MAXIMUM_BROWSER_TIMEOUT_MILLISECONDS,
      ),
    ));
    return () => window.clearTimeout(timeout);
  }, [debugWindow.active, debugWindow.expires_at, retryDebugWindow]);

  function refresh() {
    setCommandWindow(null);
    resource.retry();
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canUpdate || !reasonValid || isMutating) return;
    setIsMutating(true);
    try {
      const result = debugWindow.active && debugWindow.key
        ? await observabilityService.deactivateDebugWindow(
            debugWindow.key,
            { reason },
          )
        : await observabilityService.activateDebugWindow({
            duration_seconds: durationSeconds,
            reason,
          });
      setCommandWindow(result);
      setReason("");
      notify.success(
        debugWindow.active
          ? ADMIN_TEXT.OBSERVABILITY.DEACTIVATE_SUCCESS
          : ADMIN_TEXT.OBSERVABILITY.ACTIVATE_SUCCESS,
      );
    } catch (error) {
      notify.error(commandErrorMessage(error));
      if (error instanceof ApiError && error.status === 409) refresh();
    } finally {
      setIsMutating(false);
    }
  }

  const initialLoading = resource.status === "loading" && !hasLoaded;
  const unavailable = resource.status === "error" && !hasLoaded;
  const refreshing = resource.status === "loading" && hasLoaded;

  return (
    <div className="space-y-6 pb-8">
      <header className="flex flex-col gap-4 border-b border-border pb-6 lg:flex-row lg:items-end lg:justify-between">
        <div className="max-w-3xl">
          <div className="mb-3 flex size-11 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary">
            <Bug className="size-5" aria-hidden="true" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            {ADMIN_TEXT.OBSERVABILITY.TITLE}
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            {ADMIN_TEXT.OBSERVABILITY.SUBTITLE}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          disabled={refreshing}
          onClick={refresh}
        >
          <RefreshCw
            className={cn(
              "size-4",
              refreshing &&
                "animate-spin motion-reduce:animate-none",
            )}
            aria-hidden="true"
          />
          {refreshing
            ? ADMIN_TEXT.OBSERVABILITY.REFRESHING
            : ADMIN_TEXT.OBSERVABILITY.REFRESH}
        </Button>
      </header>

      <div className="flex gap-3 rounded-2xl border border-primary/20 bg-primary/5 p-4">
        <ShieldCheck
          className="mt-0.5 size-5 shrink-0 text-primary"
          aria-hidden="true"
        />
        <div>
          <p className="text-sm font-semibold text-foreground">
            {ADMIN_TEXT.OBSERVABILITY.TRUST_LABEL}
          </p>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">
            {ADMIN_TEXT.OBSERVABILITY.TRUST_DESCRIPTION}
          </p>
        </div>
      </div>

      {initialLoading ? (
        <div
          className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.75fr)]"
          role="status"
          aria-label={TEXT.COMMON.LOADING}
        >
          <div className="h-72 animate-pulse rounded-2xl border border-border bg-muted/35 motion-reduce:animate-none" />
          <div className="h-72 animate-pulse rounded-2xl border border-border bg-muted/35 motion-reduce:animate-none" />
        </div>
      ) : unavailable ? (
        <Card className="border-destructive/25 bg-destructive/5">
          <CardContent className="flex min-h-64 flex-col items-center justify-center p-6 text-center">
            <AlertTriangle
              className="size-7 text-destructive"
              aria-hidden="true"
            />
            <h2 className="mt-3 font-semibold">
              {ADMIN_TEXT.OBSERVABILITY.LOAD_ERROR_TITLE}
            </h2>
            <p className="mt-1 max-w-xl text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.OBSERVABILITY.LOAD_ERROR_DESCRIPTION}
            </p>
            <Button
              type="button"
              variant="outline"
              className="mt-4"
              onClick={refresh}
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.COMMON.RETRY}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          {resource.status === "error" ? (
            <div
              className="flex gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4"
              role="alert"
            >
              <AlertTriangle
                className="mt-0.5 size-5 shrink-0 text-destructive"
                aria-hidden="true"
              />
              <p className="text-sm leading-6 text-muted-foreground">
                {ADMIN_TEXT.OBSERVABILITY.STALE_SNAPSHOT}
              </p>
            </div>
          ) : null}

          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.75fr)]">
            <Card className="overflow-hidden border-border">
              <CardHeader className="border-b border-border bg-muted/20">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <CardTitle>
                      {ADMIN_TEXT.OBSERVABILITY.STATUS_TITLE}
                    </CardTitle>
                    <CardDescription className="mt-1">
                      {ADMIN_TEXT.OBSERVABILITY.STATUS_DESCRIPTION}
                    </CardDescription>
                  </div>
                  <Badge
                    variant={debugWindow.active ? "default" : "outline"}
                    className={cn(
                      "gap-1.5",
                      debugWindow.active &&
                        "bg-warning/15 text-warning",
                    )}
                  >
                    {debugWindow.active ? (
                      <Bug className="size-3.5" aria-hidden="true" />
                    ) : (
                      <ShieldCheck
                        className="size-3.5"
                        aria-hidden="true"
                      />
                    )}
                    {debugWindow.active
                      ? ADMIN_TEXT.OBSERVABILITY.ACTIVE
                      : ADMIN_TEXT.OBSERVABILITY.INACTIVE}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="grid gap-4 p-5 sm:grid-cols-2">
                <StatusField
                  label={ADMIN_TEXT.OBSERVABILITY.COMPONENT}
                  value={
                    debugWindow.component
                      ? ADMIN_TEXT.OBSERVABILITY.COMPONENT_LABEL(
                          debugWindow.component,
                        )
                      : ADMIN_TEXT.OBSERVABILITY.COMPONENT_API
                  }
                />
                <StatusField
                  label={ADMIN_TEXT.OBSERVABILITY.LEVEL}
                  value={
                    debugWindow.active
                      ? ADMIN_TEXT.OBSERVABILITY.LEVEL_DEBUG
                      : ADMIN_TEXT.OBSERVABILITY.LEVEL_BASELINE
                  }
                />
                <StatusField
                  label={ADMIN_TEXT.OBSERVABILITY.STARTED_AT}
                  value={
                    debugWindow.started_at
                      ? formatDateTime(debugWindow.started_at)
                      : TEXT.COMMON.NOT_AVAILABLE
                  }
                />
                <StatusField
                  label={ADMIN_TEXT.OBSERVABILITY.EXPIRES_AT}
                  value={
                    debugWindow.expires_at
                      ? formatDateTime(debugWindow.expires_at)
                      : TEXT.COMMON.NOT_AVAILABLE
                  }
                />
              </CardContent>
            </Card>

            <Card className="border-border">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  {debugWindow.active ? (
                    <TimerOff
                      className="size-4 text-destructive"
                      aria-hidden="true"
                    />
                  ) : (
                    <Clock3
                      className="size-4 text-primary"
                      aria-hidden="true"
                    />
                  )}
                  {debugWindow.active
                    ? ADMIN_TEXT.OBSERVABILITY.DEACTIVATE_TITLE
                    : ADMIN_TEXT.OBSERVABILITY.ACTIVATE_TITLE}
                </CardTitle>
                <CardDescription>
                  {debugWindow.active
                    ? ADMIN_TEXT.OBSERVABILITY.DEACTIVATE_DESCRIPTION
                    : ADMIN_TEXT.OBSERVABILITY.ACTIVATE_DESCRIPTION}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form className="space-y-4" onSubmit={submit}>
                  {!debugWindow.active ? (
                    <div className="space-y-2">
                      <Label htmlFor="debug-window-duration">
                        {ADMIN_TEXT.OBSERVABILITY.DURATION_LABEL}
                      </Label>
                      <Select
                        value={String(durationSeconds)}
                        disabled={!canUpdate || isMutating}
                        onValueChange={(value) =>
                          setDurationSeconds(Number(value))
                        }
                      >
                        <SelectTrigger id="debug-window-duration">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {DEBUG_WINDOW_DURATION_OPTIONS.map(
                            (seconds) => (
                              <SelectItem
                                key={seconds}
                                value={String(seconds)}
                              >
                                {ADMIN_TEXT.OBSERVABILITY.DURATION_OPTION(
                                  seconds,
                                )}
                              </SelectItem>
                            ),
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                  ) : null}

                  <div className="space-y-2">
                    <Label htmlFor="debug-window-reason">
                      {ADMIN_TEXT.OBSERVABILITY.REASON_LABEL}
                    </Label>
                    <Textarea
                      id="debug-window-reason"
                      value={reason}
                      disabled={!canUpdate || isMutating}
                      maxLength={
                        DEBUG_WINDOW_REASON_LIMITS.MAXIMUM_CHARACTERS
                      }
                      aria-invalid={reason.length > 0 && !reasonValid}
                      placeholder={
                        debugWindow.active
                          ? ADMIN_TEXT.OBSERVABILITY.DEACTIVATE_REASON_PLACEHOLDER
                          : ADMIN_TEXT.OBSERVABILITY.ACTIVATE_REASON_PLACEHOLDER
                      }
                      onChange={(event) => setReason(event.target.value)}
                    />
                    <p className="text-xs leading-5 text-muted-foreground">
                      {ADMIN_TEXT.OBSERVABILITY.REASON_HELP(
                        DEBUG_WINDOW_REASON_LIMITS.MINIMUM_CHARACTERS,
                        DEBUG_WINDOW_REASON_LIMITS.MAXIMUM_UTF8_BYTES,
                      )}
                    </p>
                  </div>

                  {!canUpdate ? (
                    <p className="rounded-lg border border-border bg-muted/30 p-3 text-xs leading-5 text-muted-foreground">
                      {ADMIN_TEXT.OBSERVABILITY.READ_ONLY}
                    </p>
                  ) : null}

                  <Button
                    type="submit"
                    variant={
                      debugWindow.active ? "destructive" : "default"
                    }
                    className="w-full"
                    disabled={!canUpdate || !reasonValid || isMutating}
                  >
                    {debugWindow.active ? (
                      <TimerOff className="size-4" aria-hidden="true" />
                    ) : (
                      <Bug className="size-4" aria-hidden="true" />
                    )}
                    {isMutating
                      ? ADMIN_TEXT.OBSERVABILITY.COMMAND_PENDING
                      : debugWindow.active
                        ? ADMIN_TEXT.OBSERVABILITY.DEACTIVATE
                        : ADMIN_TEXT.OBSERVABILITY.ACTIVATE}
                  </Button>
                </form>
              </CardContent>
            </Card>
          </div>
        </>
      )}
    </div>
  );
}

function StatusField({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-xl border border-border bg-muted/20 p-4">
      <p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {label}
      </p>
      <p className="mt-2 text-sm font-semibold text-foreground">
        {value}
      </p>
    </div>
  );
}
