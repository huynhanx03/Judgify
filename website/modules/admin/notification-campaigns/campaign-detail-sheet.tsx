"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import {
  CalendarClock,
  CircleAlert,
  ExternalLink,
  FilePenLine,
  Fingerprint,
  OctagonX,
  RefreshCw,
  Send,
  UsersRound,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  CAMPAIGN_LIMITS,
  CAMPAIGN_RECIPIENT_STATUS,
  CAMPAIGN_RECIPIENT_STATUSES,
  CAMPAIGN_STATUS,
} from "@/constants/notification-campaign";
import { OPERATION_REQUESTED_ACTION } from "@/constants/operation";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { formatDateTime, formatNumber } from "@/lib/format";
import {
  campaignAudienceLabel,
  campaignProcessedCount,
  campaignRecipientStatusLabel,
} from "@/lib/notifications/campaign-presentation";
import { notificationCampaignService } from "@/services/notification-campaign.service";
import type {
  CampaignRecipient,
  CampaignRecipientListQuery,
  CampaignRecipientPage,
  CampaignRecipientStatus,
  NotificationCampaign,
} from "@/types/notification-campaign";
import { CampaignStatusBadge } from "./campaign-status";
import type { Cursor } from "@/types/api";

interface CampaignDetailSheetProps {
  campaign: NotificationCampaign | null;
  canReadDelivery: boolean;
  onClose(): void;
  onEdit(campaign: NotificationCampaign): void;
  onReview(campaign: NotificationCampaign): void;
  onCancel(campaign: NotificationCampaign): void;
}

type RecipientFilter = CampaignRecipientStatus | "all";
type RecipientLoadState = "idle" | "loading" | "ready" | "error";
const EMPTY_RECIPIENT_PAGE: CampaignRecipientPage = { items: [] };

function recipientStatusClass(status: CampaignRecipientStatus): string {
  switch (status) {
    case CAMPAIGN_RECIPIENT_STATUS.PENDING:
      return "border-border text-muted-foreground";
    case CAMPAIGN_RECIPIENT_STATUS.CREATED:
      return "border-success/30 bg-success/10 text-success";
    case CAMPAIGN_RECIPIENT_STATUS.SKIPPED:
      return "border-warning/30 bg-warning/10 text-warning";
    case CAMPAIGN_RECIPIENT_STATUS.FAILED:
      return "border-destructive/35 bg-destructive/10 text-destructive";
  }
}

function DeliveryProgress({ campaign }: { campaign: NotificationCampaign }) {
  const counts = campaign.recipients;
  const processed = campaignProcessedCount(counts);
  const percentage =
    counts.total > 0 ? Math.min(100, (processed / counts.total) * 100) : 0;
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-medium text-foreground">
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.DELIVERY_PROGRESS}
        </span>
        <span className="tabular-nums text-muted-foreground">
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.PROGRESS_COUNT(
            processed,
            counts.total,
          )}
        </span>
      </div>
      <div
        className="h-2 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.DELIVERY_PROGRESS}
        aria-valuemin={0}
        aria-valuemax={counts.total}
        aria-valuenow={processed}
      >
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-300 motion-reduce:transition-none"
          style={{ width: `${percentage}%` }}
        />
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {[
          {
            label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.PENDING,
            value: counts.pending,
          },
          {
            label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.CREATED,
            value: counts.created,
          },
          {
            label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.SKIPPED,
            value: counts.skipped,
          },
          {
            label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.FAILED,
            value: counts.failed,
          },
        ].map((metric) => (
          <div
            key={metric.label}
            className="rounded-xl border border-border bg-muted/25 p-3"
          >
            <p className="text-[11px] text-muted-foreground">{metric.label}</p>
            <p className="mt-1 text-lg font-semibold tabular-nums">
              {formatNumber(metric.value)}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}

function RecipientList({
  campaignID,
}: {
  campaignID: string;
}) {
  const [filter, setFilter] = useState<RecipientFilter>("all");
  const [pageIndex, setPageIndex] = useState(0);
  const [cursors, setCursors] = useState<Array<Cursor | undefined>>([
    undefined,
  ]);
  const cursor = cursors[pageIndex];
  const query = useMemo<CampaignRecipientListQuery>(
    () => ({
      limit: CAMPAIGN_LIMITS.PAGE_SIZE,
      ...(filter === "all" ? {} : { status: filter }),
      ...(cursor ? { cursor } : {}),
    }),
    [cursor, filter],
  );
  const recipientResource = useRetryableResource<
    CampaignRecipientPage | null
  >({
    resetKey: `${campaignID}:${JSON.stringify(query)}`,
    initialData: null,
    keepPreviousData: true,
    load: (signal) =>
      notificationCampaignService.recipients(campaignID, query, signal),
    onSuccess: (result) => {
      if (!result) return;
      setCursors((current) => {
        const next = current.slice(0, pageIndex + 1);
        if (result.next_cursor && result.next_cursor !== cursor) {
          next[pageIndex + 1] = result.next_cursor;
        }
        return next;
      });
    },
  });
  const currentPage = recipientResource.isPreviousData
    ? null
    : recipientResource.data;
  const page = currentPage ?? EMPTY_RECIPIENT_PAGE;
  const state: RecipientLoadState =
    recipientResource.status === "loading"
      ? "loading"
      : recipientResource.status === "error"
        ? "error"
        : recipientResource.status === "ready"
          ? "ready"
          : "idle";

  const hasNext = Boolean(page.next_cursor);
  const loading = state === "loading";

  function changeFilter(value: string) {
    setCursors([undefined]);
    setPageIndex(0);
    setFilter(value as RecipientFilter);
  }

  function goToPreviousPage() {
    if (loading || pageIndex === 0) return;
    setPageIndex((index) => Math.max(0, index - 1));
  }

  function goToNextPage() {
    if (loading || !page.next_cursor) return;
    const nextCursor = page.next_cursor;
    setCursors((current) => {
      const next = current.slice(0, pageIndex + 1);
      next[pageIndex + 1] = nextCursor;
      return next;
    });
    setPageIndex((index) => index + 1);
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h3 className="font-semibold">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.TITLE}
          </h3>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.DESCRIPTION}
          </p>
        </div>
        <Select
          value={filter}
          disabled={loading}
          onValueChange={changeFilter}
        >
          <SelectTrigger
            className="w-full sm:w-48"
            aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.FILTER}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.ALL_STATUSES}
            </SelectItem>
            {CAMPAIGN_RECIPIENT_STATUSES.map((status) => (
              <SelectItem key={status} value={status}>
                {campaignRecipientStatusLabel(status)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {state === "error" ? (
        <div
          className="rounded-xl border border-destructive/25 bg-destructive/[0.04] p-5 text-center"
          role="alert"
        >
          <CircleAlert
            className="mx-auto size-5 text-destructive"
            aria-hidden="true"
          />
          <p className="mt-2 text-sm font-semibold">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.LOAD_ERROR}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="mt-3"
            onClick={recipientResource.retry}
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}

      {loading && !currentPage ? (
        <div className="space-y-2" role="status">
          {[0, 1, 2, 3].map((item) => (
            <div
              key={item}
              className="h-16 animate-pulse rounded-xl bg-muted motion-reduce:animate-none"
            />
          ))}
          <span className="sr-only">{TEXT.COMMON.LOADING}</span>
        </div>
      ) : null}

      {state === "ready" && page.items.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border px-5 py-10 text-center text-sm text-muted-foreground">
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.EMPTY}
        </div>
      ) : null}

      {page.items.length > 0 ? (
        <>
          <div className="space-y-2 lg:hidden">
            {page.items.map((recipient) => (
              <RecipientCard
                key={recipient.user_id}
                recipient={recipient}
              />
            ))}
          </div>
          <div className="hidden overflow-x-auto rounded-xl border border-border lg:block">
            <Table
              aria-label={
                ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.TABLE_LABEL
              }
            >
              <TableHeader>
                <TableRow>
                  <TableHead>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.USER}
                  </TableHead>
                  <TableHead>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATE}
                  </TableHead>
                  <TableHead>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.RESULT}
                  </TableHead>
                  <TableHead>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.ATTEMPTS}
                  </TableHead>
                  <TableHead>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.PROCESSED_AT}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {page.items.map((recipient) => (
                  <TableRow key={recipient.user_id}>
                    <TableCell className="font-mono text-xs">
                      {recipient.user_id}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={recipientStatusClass(recipient.status)}
                      >
                        {campaignRecipientStatusLabel(recipient.status)}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-56 font-mono text-xs">
                      {recipient.notification_id ??
                        recipient.skip_code ??
                        recipient.failure_code ??
                        TEXT.COMMON.NOT_AVAILABLE}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {formatNumber(recipient.attempt_count)}
                    </TableCell>
                    <TableCell className="text-xs tabular-nums">
                      {recipient.processed_at
                        ? formatDateTime(recipient.processed_at)
                        : TEXT.COMMON.NOT_AVAILABLE}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </>
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={loading || pageIndex === 0}
          onClick={goToPreviousPage}
        >
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.PREVIOUS_PAGE}
        </Button>
        <span className="text-xs tabular-nums text-muted-foreground">
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.PAGE_NUMBER(pageIndex + 1)}
        </span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={loading || !hasNext}
          onClick={goToNextPage}
        >
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.NEXT_PAGE}
        </Button>
      </div>
    </section>
  );
}

function RecipientCard({ recipient }: { recipient: CampaignRecipient }) {
  return (
    <div className="space-y-3 rounded-xl border border-border p-3">
      <div className="flex items-start justify-between gap-3">
        <p className="break-all font-mono text-xs">{recipient.user_id}</p>
        <Badge
          variant="outline"
          className={recipientStatusClass(recipient.status)}
        >
          {campaignRecipientStatusLabel(recipient.status)}
        </Badge>
      </div>
      <dl className="grid grid-cols-2 gap-3 text-xs">
        <div>
          <dt className="text-muted-foreground">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.ATTEMPTS}
          </dt>
          <dd className="mt-1 tabular-nums">{recipient.attempt_count}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.RESULT}
          </dt>
          <dd className="mt-1 break-all font-mono">
            {recipient.notification_id ??
              recipient.skip_code ??
              recipient.failure_code ??
              TEXT.COMMON.NOT_AVAILABLE}
          </dd>
        </div>
      </dl>
    </div>
  );
}

export function CampaignDetailSheet({
  campaign,
  canReadDelivery,
  onClose,
  onEdit,
  onReview,
  onCancel,
}: CampaignDetailSheetProps) {
  const [activeTab, setActiveTab] = useState("overview");

  if (!campaign) return null;
  const draft = campaign.status === CAMPAIGN_STATUS.DRAFT;
  const cancellable =
    Boolean(campaign.operation) &&
    (campaign.status === CAMPAIGN_STATUS.SCHEDULED ||
      campaign.status === CAMPAIGN_STATUS.SENDING);

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full overflow-y-auto border-border bg-background p-0 sm:max-w-3xl">
        <SheetHeader className="border-b border-border px-5 py-5 sm:px-6">
          <div className="flex flex-wrap items-center gap-2">
            <CampaignStatusBadge status={campaign.status} />
            <Badge variant="outline">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVISION(
                campaign.revision.revision_number,
              )}
            </Badge>
          </div>
          <SheetTitle className="mt-3 pr-8 text-xl">
            {campaign.revision.title}
          </SheetTitle>
          <SheetDescription>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.DESCRIPTION}
          </SheetDescription>
        </SheetHeader>

        <Tabs
          value={activeTab}
          onValueChange={setActiveTab}
          className="px-5 py-5 sm:px-6"
        >
          <TabsList
            className={
              canReadDelivery && campaign.recipients.total > 0
                ? "grid w-full grid-cols-2"
                : "w-full"
            }
          >
            <TabsTrigger value="overview">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.OVERVIEW_TAB}
            </TabsTrigger>
            {canReadDelivery && campaign.recipients.total > 0 ? (
              <TabsTrigger value="recipients">
                <UsersRound className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.RECIPIENTS_TAB}
              </TabsTrigger>
            ) : null}
          </TabsList>

          <TabsContent value="overview" className="mt-5 space-y-6">
            <DeliveryProgress campaign={campaign} />

            <section className="space-y-3">
              <h3 className="text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.CONTENT}
              </h3>
              <div className="rounded-2xl border border-border bg-card p-5">
                {/* This HTML is the immutable, server-sanitized render artifact. */}
                <div
                  className="prose prose-sm max-w-none dark:prose-invert"
                  dangerouslySetInnerHTML={{
                    __html: campaign.revision.sanitized_html,
                  }}
                />
                {campaign.revision.action_path ? (
                  <div className="mt-5 border-t border-border pt-4">
                    <p className="text-xs font-medium text-muted-foreground">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.ACTION_PATH}
                    </p>
                    <code className="mt-1 block break-all text-xs text-primary">
                      {campaign.revision.action_path}
                    </code>
                  </div>
                ) : null}
              </div>
            </section>

            <section className="space-y-3">
              <h3 className="flex items-center gap-2 text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                <UsersRound className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.AUDIENCE}
              </h3>
              <dl className="grid gap-4 rounded-2xl border border-border bg-card p-4 sm:grid-cols-2">
                <div>
                  <dt className="text-xs text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.SELECTOR}
                  </dt>
                  <dd className="mt-1 text-sm font-medium">
                    {campaignAudienceLabel(campaign.revision.audience)}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.AUTHOR}
                  </dt>
                  <dd className="mt-1 break-all font-mono text-xs">
                    {campaign.revision.author_id}
                  </dd>
                </div>
              </dl>
            </section>

            <section className="space-y-3">
              <h3 className="flex items-center gap-2 text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                <CalendarClock className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.TIMELINE}
              </h3>
              <dl className="grid gap-4 rounded-2xl border border-border bg-card p-4 sm:grid-cols-2">
                {[
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CREATED_AT,
                    value: campaign.created_at,
                  },
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.UPDATED_AT_LABEL,
                    value: campaign.updated_at,
                  },
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SCHEDULED_FOR,
                    value: campaign.scheduled_for,
                  },
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.STARTED_AT,
                    value: campaign.started_at,
                  },
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.COMPLETED_AT,
                    value: campaign.completed_at,
                  },
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.CANCELLED_AT,
                    value: campaign.cancelled_at,
                  },
                ].map((entry) => (
                  <div key={entry.label}>
                    <dt className="text-xs text-muted-foreground">
                      {entry.label}
                    </dt>
                    <dd className="mt-1 text-sm tabular-nums">
                      {entry.value
                        ? formatDateTime(entry.value)
                        : TEXT.COMMON.NOT_AVAILABLE}
                    </dd>
                  </div>
                ))}
              </dl>
              {campaign.cancel_reason ? (
                <div className="rounded-2xl border border-destructive/20 bg-destructive/[0.035] p-4">
                  <p className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.CANCEL_REASON}
                  </p>
                  <p className="mt-1 text-sm leading-6">
                    {campaign.cancel_reason}
                  </p>
                  {campaign.cancelled_by ? (
                    <p className="mt-2 break-all font-mono text-xs text-muted-foreground">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.CANCELLED_BY_VALUE(
                        campaign.cancelled_by,
                      )}
                    </p>
                  ) : null}
                </div>
              ) : null}
            </section>

            <section className="space-y-3">
              <h3 className="flex items-center gap-2 text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                <Fingerprint className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.IDENTIFIERS}
              </h3>
              <dl className="space-y-3 rounded-2xl border border-border bg-card p-4">
                {[
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.CAMPAIGN_ID,
                    value: campaign.id,
                  },
                  {
                    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.REVISION_ID,
                    value: campaign.revision.id,
                  },
                  ...(campaign.operation
                    ? [
                        {
                          label:
                            ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.OPERATION_ID,
                          value: campaign.operation.id,
                        },
                      ]
                    : []),
                ].map((entry) => (
                  <div key={entry.label}>
                    <dt className="text-xs text-muted-foreground">
                      {entry.label}
                    </dt>
                    <dd className="mt-1 break-all font-mono text-xs">
                      {entry.value}
                    </dd>
                  </div>
                ))}
              </dl>
              {campaign.operation ? (
                <Button
                  render={<Link href={APP_ROUTES.ADMIN_OPERATIONS} />}
                  variant="outline"
                  size="sm"
                >
                  <ExternalLink className="size-4" aria-hidden="true" />
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.OPEN_OPERATION}
                </Button>
              ) : null}
            </section>
          </TabsContent>

          {canReadDelivery && campaign.recipients.total > 0 ? (
            <TabsContent value="recipients" className="mt-5">
              <RecipientList campaignID={campaign.id} />
            </TabsContent>
          ) : null}
        </Tabs>

        <SheetFooter className="sticky bottom-0 border-t border-border bg-background/95 px-5 py-4 backdrop-blur-sm supports-backdrop-filter:bg-background/85 sm:px-6">
          {draft ? (
            <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button
                type="button"
                variant="outline"
                onClick={() => onEdit(campaign)}
              >
                <FilePenLine className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.EDIT}
              </Button>
              <Button type="button" onClick={() => onReview(campaign)}>
                <Send className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW_ACTION}
              </Button>
            </div>
          ) : cancellable ? (
            <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-sm leading-6 text-muted-foreground">
                {campaign.operation?.requested_action ===
                OPERATION_REQUESTED_ACTION.CANCEL
                  ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.CANCELLATION_PENDING
                  : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.IMMUTABLE_NOTICE}
              </p>
              <Button
                type="button"
                variant="destructive"
                className="shrink-0"
                onClick={() => onCancel(campaign)}
              >
                <OctagonX className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL_ACTION}
              </Button>
            </div>
          ) : (
            <p className="text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DETAIL.IMMUTABLE_NOTICE}
            </p>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
