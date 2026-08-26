"use client";

import { type FormEvent, useMemo, useState } from "react";
import {
  Activity,
  CalendarClock,
  CheckCircle2,
  CircleAlert,
  Eye,
  FilePenLine,
  Filter,
  LoaderCircle,
  Megaphone,
  OctagonX,
  Plus,
  RefreshCw,
  Send,
  ShieldCheck,
  UsersRound,
  type LucideIcon,
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  CAMPAIGN_LIMITS,
  CAMPAIGN_STATUS,
  CAMPAIGN_STATUSES,
} from "@/constants/notification-campaign";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { formatDateTime, formatNumber } from "@/lib/format";
import {
  campaignAudienceLabel,
  campaignProcessedCount,
} from "@/lib/notifications/campaign-presentation";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { notificationCampaignService } from "@/services/notification-campaign.service";
import { roleService } from "@/services/role.service";
import type { Role } from "@/types/admin";
import type { Cursor } from "@/types/api";
import type {
  CampaignListQuery,
  CampaignCancelResult,
  CampaignPage,
  CampaignScheduleResult,
  CampaignStatus,
  CreateCampaignInput,
  NotificationCampaign,
} from "@/types/notification-campaign";
import { CampaignComposerDialog } from "./campaign-composer-dialog";
import { CampaignCancelDialog } from "./campaign-cancel-dialog";
import { CampaignDetailSheet } from "./campaign-detail-sheet";
import { CampaignReviewDialog } from "./campaign-review-dialog";
import { CampaignStatusBadge } from "./campaign-status";

type LoadState = "loading" | "refreshing" | "ready" | "error";
type StatusFilter = CampaignStatus | "all";

interface AppliedFilters {
  status: StatusFilter;
}

const ALL_STATUSES = "all" as const;
const EMPTY_CAMPAIGN_PAGE: CampaignPage = { items: [] };

function statusLabel(status: CampaignStatus): string {
  switch (status) {
    case CAMPAIGN_STATUS.DRAFT:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.DRAFT;
    case CAMPAIGN_STATUS.SCHEDULED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.SCHEDULED;
    case CAMPAIGN_STATUS.SENDING:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.SENDING;
    case CAMPAIGN_STATUS.COMPLETED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.COMPLETED;
    case CAMPAIGN_STATUS.FAILED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.FAILED;
    case CAMPAIGN_STATUS.CANCELLED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.CANCELLED;
  }
}

function CampaignSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label={TEXT.COMMON.LOADING}>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map((item) => (
          <div
            key={item}
            className="h-32 animate-pulse rounded-xl border border-border bg-muted/40 motion-reduce:animate-none"
          />
        ))}
      </div>
      <div className="h-80 animate-pulse rounded-xl border border-border bg-muted/40 motion-reduce:animate-none" />
    </div>
  );
}

function MetricCard({
  label,
  value,
  icon: Icon,
  tone,
}: {
  label: string;
  value: number;
  icon: LucideIcon;
  tone: "primary" | "warning" | "success" | "danger";
}) {
  return (
    <Card className="relative min-h-32 overflow-hidden border-border">
      <div
        className={cn(
          "absolute inset-x-0 top-0 h-0.5",
          tone === "warning"
            ? "bg-warning"
            : tone === "success"
              ? "bg-success"
              : tone === "danger"
                ? "bg-destructive"
                : "bg-primary",
        )}
      />
      <CardContent className="flex h-full items-center justify-between gap-4 pt-3">
        <div>
          <p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            {label}
          </p>
          <p className="mt-2 text-3xl font-bold tabular-nums">
            {formatNumber(value)}
          </p>
        </div>
        <div className="flex size-11 items-center justify-center rounded-2xl border border-primary/15 bg-primary/10 text-primary">
          <Icon className="size-5" aria-hidden="true" />
        </div>
      </CardContent>
    </Card>
  );
}

function CampaignIdentity({ campaign }: { campaign: NotificationCampaign }) {
  return (
    <div className="max-w-80">
      <p className="line-clamp-2 font-semibold leading-5">
        {campaign.revision.title}
      </p>
      <p className="mt-1 text-xs text-muted-foreground">
        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVISION(
          campaign.revision.revision_number,
        )}{" "}
        · {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.VERSION(campaign.version)}
      </p>
    </div>
  );
}

function CampaignProgress({ campaign }: { campaign: NotificationCampaign }) {
  const processed = campaignProcessedCount(campaign.recipients);
  const total = campaign.recipients.total;
  const percentage = total > 0 ? Math.min(100, (processed / total) * 100) : 0;
  return (
    <div className="min-w-44 space-y-2">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="text-muted-foreground">
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.DELIVERED}
        </span>
        <span className="font-medium tabular-nums">
          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.PROGRESS_COUNT(processed, total)}
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-300 motion-reduce:transition-none"
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  );
}

interface CampaignCollectionProps {
  campaigns: NotificationCampaign[];
  onSelect(campaign: NotificationCampaign): void;
  onEdit(campaign: NotificationCampaign): void;
  onReview(campaign: NotificationCampaign): void;
  onCancel(campaign: NotificationCampaign): void;
}

function CampaignCards({
  campaigns,
  onSelect,
  onEdit,
  onReview,
  onCancel,
}: CampaignCollectionProps) {
  return (
    <div className="grid gap-3 lg:hidden">
      {campaigns.map((campaign) => (
        <Card key={campaign.id} className="border-border">
          <CardContent className="space-y-4 p-4">
            <div className="flex items-start justify-between gap-3">
              <CampaignIdentity campaign={campaign} />
              <CampaignStatusBadge status={campaign.status} />
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <UsersRound className="size-4" aria-hidden="true" />
              <span className="truncate">
                {campaignAudienceLabel(campaign.revision.audience)}
              </span>
            </div>
            <CampaignProgress campaign={campaign} />
            <p className="text-xs tabular-nums text-muted-foreground">
              {campaign.scheduled_for
                ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SCHEDULE_VALUE(
                    formatDateTime(campaign.scheduled_for),
                  )
                : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CREATED_VALUE(
                    formatDateTime(campaign.created_at),
                  )}
            </p>
            <div className="grid grid-cols-2 gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => onSelect(campaign)}
              >
                <Eye className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.VIEW_DETAILS}
              </Button>
              {campaign.status === CAMPAIGN_STATUS.DRAFT ? (
                <Button type="button" onClick={() => onReview(campaign)}>
                  <Send className="size-4" aria-hidden="true" />
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW_ACTION}
                </Button>
              ) : campaign.status === CAMPAIGN_STATUS.SCHEDULED ||
                campaign.status === CAMPAIGN_STATUS.SENDING ? (
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => onCancel(campaign)}
                >
                  <OctagonX className="size-4" aria-hidden="true" />
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL_ACTION}
                </Button>
              ) : (
                <Button type="button" variant="outline" disabled>
                  <ShieldCheck className="size-4" aria-hidden="true" />
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.IMMUTABLE}
                </Button>
              )}
            </div>
            {campaign.status === CAMPAIGN_STATUS.DRAFT ? (
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                onClick={() => onEdit(campaign)}
              >
                <FilePenLine className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.EDIT}
              </Button>
            ) : null}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function CampaignTable({
  campaigns,
  onSelect,
  onEdit,
  onReview,
  onCancel,
}: CampaignCollectionProps) {
  return (
    <div className="hidden overflow-x-auto lg:block">
      <Table aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.TABLE_LABEL}>
        <TableHeader>
          <TableRow>
            <TableHead>
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COLUMN_CAMPAIGN}
            </TableHead>
            <TableHead>
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COLUMN_AUDIENCE}
            </TableHead>
            <TableHead>
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COLUMN_DELIVERY}
            </TableHead>
            <TableHead>
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COLUMN_STATUS}
            </TableHead>
            <TableHead>
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COLUMN_TIMING}
            </TableHead>
            <TableHead className="text-right">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COLUMN_ACTIONS}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {campaigns.map((campaign) => (
            <TableRow key={campaign.id}>
              <TableCell>
                <CampaignIdentity campaign={campaign} />
              </TableCell>
              <TableCell>
                <div className="max-w-56">
                  <p className="text-sm">
                    {campaignAudienceLabel(campaign.revision.audience)}
                  </p>
                  <p className="mt-1 truncate font-mono text-xs text-muted-foreground">
                    {campaign.id}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <CampaignProgress campaign={campaign} />
              </TableCell>
              <TableCell>
                <CampaignStatusBadge status={campaign.status} />
                {campaign.last_failure_code ? (
                  <p className="mt-2 max-w-40 truncate font-mono text-xs text-destructive">
                    {campaign.last_failure_code}
                  </p>
                ) : null}
              </TableCell>
              <TableCell className="text-xs tabular-nums">
                <p>{formatDateTime(campaign.created_at)}</p>
                <p className="mt-1 text-muted-foreground">
                  {campaign.scheduled_for
                    ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SCHEDULE_VALUE(
                        formatDateTime(campaign.scheduled_for),
                      )
                    : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.NOT_SCHEDULED}
                </p>
              </TableCell>
              <TableCell>
                <div className="flex justify-end gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => onSelect(campaign)}
                    aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.VIEW_DETAILS_ARIA(
                      campaign.revision.title,
                    )}
                  >
                    <Eye className="size-4" aria-hidden="true" />
                  </Button>
                  {campaign.status === CAMPAIGN_STATUS.DRAFT ? (
                    <>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => onEdit(campaign)}
                        aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.EDIT_ARIA(
                          campaign.revision.title,
                        )}
                      >
                        <FilePenLine className="size-4" aria-hidden="true" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => onReview(campaign)}
                        aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REVIEW_ARIA(
                          campaign.revision.title,
                        )}
                      >
                        <Send className="size-4" aria-hidden="true" />
                      </Button>
                    </>
                  ) : campaign.status === CAMPAIGN_STATUS.SCHEDULED ||
                    campaign.status === CAMPAIGN_STATUS.SENDING ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="text-destructive hover:text-destructive"
                      onClick={() => onCancel(campaign)}
                      aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL_ARIA(
                        campaign.revision.title,
                      )}
                    >
                      <OctagonX className="size-4" aria-hidden="true" />
                    </Button>
                  ) : null}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export function AdminNotificationCampaigns() {
  const { authorizationRevision, can } = useAuth();
  const canBroadcastRole = can(
    AUTHORIZATION_RESOURCE.NOTIFICATION,
    AUTHORIZATION_ACTION.BROADCAST_ROLE,
  );
  const canBroadcastAllActive = can(
    AUTHORIZATION_RESOURCE.NOTIFICATION,
    AUTHORIZATION_ACTION.BROADCAST_ALL_ACTIVE,
  );
  const canReadDelivery = can(
    AUTHORIZATION_RESOURCE.NOTIFICATION,
    AUTHORIZATION_ACTION.READ_DELIVERY,
  );

  const [filters, setFilters] = useState<AppliedFilters>({
    status: ALL_STATUSES,
  });
  const [draftStatus, setDraftStatus] = useState<StatusFilter>(ALL_STATUSES);
  const [pageIndex, setPageIndex] = useState(0);
  const [cursors, setCursors] = useState<Array<Cursor | undefined>>([
    undefined,
  ]);
  const [updatedAt, setUpdatedAt] = useState<string | null>(null);
  const [selected, setSelected] = useState<NotificationCampaign | null>(null);
  const [editing, setEditing] = useState<NotificationCampaign | null>(null);
  const [reviewing, setReviewing] = useState<NotificationCampaign | null>(null);
  const [cancelling, setCancelling] = useState<NotificationCampaign | null>(null);
  const [composerOpen, setComposerOpen] = useState(false);
  const [mutationPending, setMutationPending] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const cursor = cursors[pageIndex];
  const query = useMemo<CampaignListQuery>(
    () => {
      const next: CampaignListQuery = {
        limit: CAMPAIGN_LIMITS.PAGE_SIZE,
        ...(cursor ? { cursor } : {}),
      };
      if (filters.status !== ALL_STATUSES) next.status = filters.status;
      return next;
    },
    [cursor, filters.status],
  );
  const campaignResource = useRetryableResource<CampaignPage | null>({
    resetKey: `${authorizationRevision}:${JSON.stringify(query)}`,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => notificationCampaignService.list(query, signal),
    onSuccess: (result) => {
      if (!result) return;
      setUpdatedAt(new Date().toISOString());
      setSelected((current) =>
        current
          ? result.items.find(({ id }) => id === current.id) ?? current
          : null,
      );
      setCursors((current) => {
        const next = current.slice(0, pageIndex + 1);
        if (result.next_cursor && result.next_cursor !== cursor) {
          next[pageIndex + 1] = result.next_cursor;
        }
        return next;
      });
    },
  });
  const rolesResource = useRetryableResource<Role[]>({
    resetKey: authorizationRevision,
    enabled: canBroadcastRole,
    initialData: [],
    load: (signal) => roleService.getAll(signal),
  });
  const currentPage = campaignResource.isPreviousData
    ? null
    : campaignResource.data;
  const page = currentPage ?? EMPTY_CAMPAIGN_PAGE;
  const roles = rolesResource.data;
  const rolesLoading = rolesResource.status === "loading";
  const loadState: LoadState =
    campaignResource.status === "loading"
      ? currentPage
        ? "refreshing"
        : "loading"
      : campaignResource.status === "error" && !currentPage
        ? "error"
        : "ready";

  const metrics = useMemo(() => {
    const campaigns = page.items;
    return {
      drafts: campaigns.filter(({ status }) => status === CAMPAIGN_STATUS.DRAFT)
        .length,
      queued: campaigns.filter(
        ({ status }) =>
          status === CAMPAIGN_STATUS.SCHEDULED ||
          status === CAMPAIGN_STATUS.SENDING,
      ).length,
      delivered: campaigns.reduce(
        (sum, campaign) => sum + campaign.recipients.created,
        0,
      ),
      failures: campaigns.reduce(
        (sum, campaign) =>
          sum +
          campaign.recipients.failed +
          (campaign.status === CAMPAIGN_STATUS.FAILED ? 1 : 0),
        0,
      ),
    };
  }, [page.items]);

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCursors([undefined]);
    setPageIndex(0);
    setSelected(null);
    setFilters({ status: draftStatus });
    campaignResource.retry();
  }

  function resetFilters() {
    setDraftStatus(ALL_STATUSES);
    setCursors([undefined]);
    setPageIndex(0);
    setSelected(null);
    setFilters({ status: ALL_STATUSES });
    campaignResource.retry();
  }

  function openCreate() {
    setEditing(null);
    setMutationError(null);
    setComposerOpen(true);
  }

  function openEdit(campaign: NotificationCampaign) {
    setSelected(null);
    setEditing(campaign);
    setMutationError(null);
    setComposerOpen(true);
  }

  function openReview(campaign: NotificationCampaign) {
    setSelected(null);
    setReviewing(campaign);
  }

  function openCancel(campaign: NotificationCampaign) {
    setSelected(null);
    setCancelling(campaign);
  }

  async function saveCampaign(input: CreateCampaignInput) {
    setMutationPending(true);
    setMutationError(null);
    try {
      const saved = editing
        ? await notificationCampaignService.revise(editing.id, {
            ...input,
            expected_version: editing.version,
          })
        : await notificationCampaignService.create(input);
      setComposerOpen(false);
      setEditing(null);
      setSelected(saved);
      notify.success(
        editing
          ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.UPDATE_SUCCESS
          : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CREATE_SUCCESS,
      );
      campaignResource.retry();
    } catch (error) {
      const conflict = error instanceof ApiError && error.status === 409;
      setMutationError(
        conflict
          ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CONFLICT
          : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SAVE_ERROR,
      );
      if (conflict) {
        campaignResource.retry();
      }
      throw error;
    } finally {
      setMutationPending(false);
    }
  }

  function scheduled(result: CampaignScheduleResult) {
    setReviewing(null);
    setSelected(result.campaign);
    campaignResource.retry();
    if (result.replayed) {
      notify.info(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SCHEDULE_REPLAYED);
    } else {
      notify.success(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SCHEDULE_SUCCESS);
    }
  }

  function cancelled(result: CampaignCancelResult) {
    setCancelling(null);
    setSelected(result.campaign);
    campaignResource.retry();
    if (result.replayed) {
      notify.info(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL_REPLAYED);
    } else {
      notify.success(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CANCEL_SUCCESS);
    }
  }

  const initialLoading = loadState === "loading";
  const refreshing = loadState === "refreshing";

  function goToPreviousPage() {
    if (campaignResource.status === "loading" || pageIndex === 0) return;
    setSelected(null);
    setPageIndex((index) => Math.max(0, index - 1));
  }

  function goToNextPage() {
    if (campaignResource.status === "loading" || !page.next_cursor) return;
    const nextCursor = page.next_cursor;
    setSelected(null);
    setCursors((current) => {
      const next = current.slice(0, pageIndex + 1);
      next[pageIndex + 1] = nextCursor;
      return next;
    });
    setPageIndex((index) => index + 1);
  }

  return (
    <section
      className="mx-auto w-full max-w-[100rem] space-y-6"
      aria-labelledby="notification-campaign-title"
    >
      <div className="flex flex-col gap-5 rounded-2xl border border-border/70 bg-card/80 p-5 shadow-sm backdrop-blur-sm sm:p-7 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/5">
            <Megaphone className="size-5" aria-hidden="true" />
          </div>
          <div>
            <p className="text-xs font-bold tracking-[0.18em] text-primary uppercase">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.EYEBROW}
            </p>
            <h1
              id="notification-campaign-title"
              className="mt-1 text-2xl font-black tracking-tight sm:text-3xl"
            >
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.TITLE}
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground sm:text-base">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.SUBTITLE}
            </p>
            {updatedAt ? (
              <p className="mt-2 text-xs text-muted-foreground">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.UPDATED_AT(
                  formatDateTime(updatedAt),
                )}
              </p>
            ) : null}
          </div>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <Button
            type="button"
            variant="outline"
            disabled={campaignResource.status === "loading"}
            onClick={campaignResource.retry}
          >
            <RefreshCw
              className={cn(
                "size-4",
                refreshing && "animate-spin motion-reduce:animate-none",
              )}
              aria-hidden="true"
            />
            {refreshing
              ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REFRESHING
              : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REFRESH}
          </Button>
          <Button type="button" onClick={openCreate}>
            <Plus className="size-4" aria-hidden="true" />
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CREATE}
          </Button>
        </div>
      </div>

      <div className="flex items-start gap-3 rounded-2xl border border-primary/20 bg-primary/[0.045] p-4">
        <ShieldCheck
          className="mt-0.5 size-5 shrink-0 text-primary"
          aria-hidden="true"
        />
        <div>
          <h2 className="text-sm font-semibold">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.TRUST_LABEL}
          </h2>
          <p className="mt-1 text-xs leading-5 text-muted-foreground sm:text-sm sm:leading-6">
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.TRUST_DESCRIPTION}
          </p>
        </div>
      </div>

      <Card className="border-border">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Filter className="size-4 text-primary" aria-hidden="true" />
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.FILTERS_TITLE}
          </CardTitle>
          <CardDescription>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.FILTERS_DESCRIPTION}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={applyFilters}
            className="flex flex-col gap-3 sm:flex-row sm:items-end"
          >
            <div className="w-full space-y-2 sm:max-w-xs">
              <Label htmlFor="campaign-status-filter">
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS_LABEL}
              </Label>
              <Select
                value={draftStatus}
                onValueChange={(value) =>
                  setDraftStatus(value as StatusFilter)
                }
              >
                <SelectTrigger id="campaign-status-filter">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL_STATUSES}>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.ALL_STATUSES}
                  </SelectItem>
                  {CAMPAIGN_STATUSES.map((status) => (
                    <SelectItem key={status} value={status}>
                      {statusLabel(status)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button type="submit">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.APPLY_FILTERS}
            </Button>
            <Button type="button" variant="ghost" onClick={resetFilters}>
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RESET_FILTERS}
            </Button>
          </form>
        </CardContent>
      </Card>

      {initialLoading ? (
        <CampaignSkeleton />
      ) : loadState === "error" && !currentPage ? (
        <Card className="border-destructive/25">
          <CardContent className="flex flex-col items-center px-6 py-14 text-center">
            <div className="flex size-14 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
              <CircleAlert className="size-6" aria-hidden="true" />
            </div>
            <h2 className="mt-5 text-lg font-semibold">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.LOAD_ERROR_TITLE}
            </h2>
            <p className="mt-2 max-w-lg text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.LOAD_ERROR_DESCRIPTION}
            </p>
            <Button
              type="button"
              className="mt-5"
              onClick={campaignResource.retry}
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.COMMON.RETRY}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          {campaignResource.status === "error" && currentPage ? (
            <div
              className="flex items-start gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4"
              role="alert"
            >
              <CircleAlert
                className="mt-0.5 size-5 shrink-0 text-destructive"
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-semibold">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.LOAD_ERROR_TITLE}
                </p>
                <p className="mt-1 text-sm leading-6 text-muted-foreground">
                  {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.LOAD_ERROR_DESCRIPTION}
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                onClick={campaignResource.retry}
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                <span className="sr-only">{TEXT.COMMON.RETRY}</span>
              </Button>
            </div>
          ) : null}
          <div
            className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
            aria-label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CURRENT_PAGE}
          >
            <MetricCard
              label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.METRIC_DRAFTS}
              value={metrics.drafts}
              icon={FilePenLine}
              tone="primary"
            />
            <MetricCard
              label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.METRIC_QUEUED}
              value={metrics.queued}
              icon={CalendarClock}
              tone="warning"
            />
            <MetricCard
              label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.METRIC_DELIVERED}
              value={metrics.delivered}
              icon={CheckCircle2}
              tone="success"
            />
            <MetricCard
              label={ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.METRIC_FAILURES}
              value={metrics.failures}
              icon={Activity}
              tone="danger"
            />
          </div>

          <Card className="overflow-hidden border-border">
            <CardHeader className="border-b border-border">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <CardTitle>
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.LIST_TITLE}
                  </CardTitle>
                  <CardDescription className="mt-1">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.LIST_COUNT(
                      page.items.length,
                    )}
                  </CardDescription>
                </div>
                {refreshing ? (
                  <Badge variant="outline" aria-live="polite">
                    <LoaderCircle
                      className="size-3.5 animate-spin motion-reduce:animate-none"
                      aria-hidden="true"
                    />
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.REFRESHING}
                  </Badge>
                ) : null}
              </div>
            </CardHeader>
            <CardContent className="p-0">
              {page.items.length === 0 ? (
                <div className="flex flex-col items-center px-6 py-16 text-center">
                  <div className="flex size-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                    <Megaphone className="size-6" aria-hidden="true" />
                  </div>
                  <h2 className="mt-5 text-lg font-semibold">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.EMPTY_TITLE}
                  </h2>
                  <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.EMPTY_DESCRIPTION}
                  </p>
                  <Button type="button" className="mt-5" onClick={openCreate}>
                    <Plus className="size-4" aria-hidden="true" />
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.CREATE}
                  </Button>
                </div>
              ) : (
                <>
                  <div className="p-3 lg:hidden">
                    <CampaignCards
                      campaigns={page.items}
                      onSelect={setSelected}
                      onEdit={openEdit}
                      onReview={openReview}
                      onCancel={openCancel}
                    />
                  </div>
                  <CampaignTable
                    campaigns={page.items}
                    onSelect={setSelected}
                    onEdit={openEdit}
                    onReview={openReview}
                    onCancel={openCancel}
                  />
                </>
              )}
            </CardContent>
          </Card>

          <div className="flex items-center justify-between gap-3">
            <Button
              type="button"
              variant="outline"
              disabled={campaignResource.status === "loading" || pageIndex === 0}
              onClick={goToPreviousPage}
            >
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.PREVIOUS_PAGE}
            </Button>
            <span className="text-sm tabular-nums text-muted-foreground">
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.PAGE_NUMBER(pageIndex + 1)}
            </span>
            <Button
              type="button"
              variant="outline"
              disabled={
                campaignResource.status === "loading" || !page.next_cursor
              }
              onClick={goToNextPage}
            >
              {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.NEXT_PAGE}
            </Button>
          </div>
        </>
      )}

      {composerOpen ? (
        <CampaignComposerDialog
          key={`${editing?.id ?? "create"}:${editing?.version ?? 0}:${canBroadcastRole}:${canBroadcastAllActive}`}
          open
          campaign={editing}
          roles={roles}
          rolesLoading={rolesLoading}
          rolesError={rolesResource.status === "error"}
          onRetryRoles={rolesResource.retry}
          canBroadcastRole={canBroadcastRole}
          canBroadcastAllActive={canBroadcastAllActive}
          pending={mutationPending}
          error={mutationError}
          onOpenChange={(open) => {
            if (!mutationPending && !open) {
              setComposerOpen(false);
              setEditing(null);
              setMutationError(null);
            }
          }}
          onSave={saveCampaign}
        />
      ) : null}

      {reviewing ? (
        <CampaignReviewDialog
          key={`${reviewing.id}:${reviewing.version}`}
          open
          campaign={reviewing}
          onOpenChange={(open) => {
            if (!open) setReviewing(null);
          }}
          onScheduled={scheduled}
        />
      ) : null}

      {cancelling ? (
        <CampaignCancelDialog
          key={`${cancelling.id}:${cancelling.version}:${cancelling.operation?.version ?? 0}`}
          campaign={cancelling}
          onOpenChange={(open) => {
            if (!open) setCancelling(null);
          }}
          onCancelled={cancelled}
        />
      ) : null}

      {selected ? (
        <CampaignDetailSheet
          key={`${selected.id}:${selected.version}:${canReadDelivery}`}
          campaign={selected}
          canReadDelivery={canReadDelivery}
          onClose={() => setSelected(null)}
          onEdit={openEdit}
          onReview={openReview}
          onCancel={openCancel}
        />
      ) : null}
    </section>
  );
}
