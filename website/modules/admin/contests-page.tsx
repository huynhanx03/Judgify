"use client";

/**
 * Admin contests page — paginated list with search, create, edit, delete.
 */

import { useMemo, useState } from "react";
import {
  AlertTriangle,
  Ban,
  CircleStop,
  RefreshCw,
  Rocket,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import {
  ContestDialog,
  type ContestEditorInput,
} from "@/modules/admin/dialogs/contest-dialog";
import { contestService } from "@/services/contest.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { getErrorMessage, notify } from "@/lib/toast";
import type { Contest } from "@/types/contest";
import { CONTEST_REASON_LIMITS } from "@/constants/contest";
import { APP_ROUTES } from "@/constants/routes";
import {
  isContestPublishReasonValid,
  isContestReasonValid,
  isRatingReratingReasonValid,
} from "@/lib/contest/publish";
import { formatDateTimeMinute } from "@/lib/format";

const STATUS_STYLES: Record<string, string> = {
  draft: "bg-muted text-muted-foreground",
  upcoming: "bg-info/10 text-info",
  running: "bg-success/10 text-success",
  ended: "bg-muted text-muted-foreground",
  cancelled: "bg-danger/10 text-danger",
};

const STATUS_LABELS: Record<string, string> = {
  draft: TEXT.CONTEST.STATUS_DRAFT,
  upcoming: TEXT.CONTEST.STATUS_UPCOMING,
  running: TEXT.CONTEST.STATUS_RUNNING,
  ended: TEXT.CONTEST.STATUS_ENDED,
  cancelled: TEXT.CONTEST.STATUS_CANCELLED,
};

type LifecycleIntent = {
  contest: Contest;
  target: "ended" | "cancelled";
};

export default function AdminContestsPage() {
  const { can } = useAuth();
  const router = useRouter();
  const resource = AUTHORIZATION_RESOURCE.CONTEST;
  const canRead = can(resource, AUTHORIZATION_ACTION.READ);
  const canPublish = can(resource, AUTHORIZATION_ACTION.UPDATE);
  const canRerate =
    can(resource, AUTHORIZATION_ACTION.UPDATE) &&
    can(
      AUTHORIZATION_RESOURCE.OPERATION,
      AUTHORIZATION_ACTION.EXECUTE,
    );
  const canViewOperations = can(
    AUTHORIZATION_RESOURCE.OPERATION,
    AUTHORIZATION_ACTION.READ,
  );
  const [publishTarget, setPublishTarget] = useState<Contest | null>(null);
  const [publishReason, setPublishReason] = useState("");
  const [isPublishing, setIsPublishing] = useState(false);
  const [lifecycleIntent, setLifecycleIntent] =
    useState<LifecycleIntent | null>(null);
  const [lifecycleReason, setLifecycleReason] = useState("");
  const [isChangingLifecycle, setIsChangingLifecycle] = useState(false);
  const [deleteReason, setDeleteReason] = useState("");
  const [reratingTarget, setReratingTarget] =
    useState<Contest | null>(null);
  const [reratingReason, setReratingReason] = useState("");
  const [isStartingRerating, setIsStartingRerating] =
    useState(false);
  const publishReasonValid = useMemo(
    () => isContestPublishReasonValid(publishReason),
    [publishReason],
  );
  const publishReasonBytes = useMemo(
    () => new TextEncoder().encode(publishReason.trim()).byteLength,
    [publishReason],
  );
  const lifecycleReasonValid = useMemo(
    () => isContestReasonValid(lifecycleReason),
    [lifecycleReason],
  );
  const lifecycleReasonBytes = useMemo(
    () => new TextEncoder().encode(lifecycleReason.trim()).byteLength,
    [lifecycleReason],
  );
  const reratingReasonValid = useMemo(
    () => isRatingReratingReasonValid(reratingReason),
    [reratingReason],
  );
  const reratingReasonBytes = useMemo(
    () => new TextEncoder().encode(reratingReason.trim()).byteLength,
    [reratingReason],
  );
  const service = useMemo(() => ({
    find: contestService.findAdmin.bind(contestService),
    create: (input: ContestEditorInput) => {
      const { expected_version: expectedVersion, ...draft } = input;
      if (expectedVersion !== undefined) {
        throw new TypeError("new contest cannot carry a prior version");
      }
      return contestService.create(draft);
    },
    update: (id: string, input: ContestEditorInput) => {
      const { expected_version: expectedVersion, ...draft } = input;
      if (expectedVersion === undefined) {
        throw new TypeError("contest editor version is required");
      }
      return contestService.update(id, {
        ...draft,
        expected_version: expectedVersion,
      });
    },
    delete: (id: string) => contestService.delete(id, deleteReason),
  }), [deleteReason]);

  const crud = usePaginatedCRUD({ service, resource });

  async function handlePublish() {
    if (!publishTarget || !canPublish || !publishReasonValid) return;
    setIsPublishing(true);
    try {
      await contestService.publish(
        publishTarget.id,
        publishTarget.version,
        publishReason,
      );
      notify.success(ADMIN_TEXT.CONTESTS.PUBLISH_SUCCESS);
      setPublishTarget(null);
      setPublishReason("");
      crud.refresh();
    } catch (error) {
      notify.error(
        getErrorMessage(error, ADMIN_TEXT.CONTESTS.PUBLISH_ERROR),
      );
    } finally {
      setIsPublishing(false);
    }
  }

  async function handleRerating() {
    if (
      !reratingTarget ||
      !canRerate ||
      !reratingReasonValid
    ) {
      return;
    }
    setIsStartingRerating(true);
    try {
      await contestService.startRatingRerating(
        reratingTarget.id,
        reratingReason,
      );
      notify.success(ADMIN_TEXT.CONTESTS.RERATE_SUCCESS);
      setReratingTarget(null);
      setReratingReason("");
      if (canViewOperations) {
        router.push(APP_ROUTES.ADMIN_OPERATIONS);
      }
    } catch (error) {
      notify.error(
        getErrorMessage(error, ADMIN_TEXT.CONTESTS.RERATE_ERROR),
      );
    } finally {
      setIsStartingRerating(false);
    }
  }

  async function handleLifecycleTransition() {
    if (
      !lifecycleIntent ||
      !canPublish ||
      !lifecycleReasonValid
    ) {
      return;
    }
    setIsChangingLifecycle(true);
    try {
      const { contest, target } = lifecycleIntent;
      if (target === "ended") {
        await contestService.end(
          contest.id,
          contest.version,
          lifecycleReason,
        );
        notify.success(ADMIN_TEXT.CONTESTS.END_SUCCESS);
      } else {
        await contestService.cancel(
          contest.id,
          contest.version,
          lifecycleReason,
        );
        notify.success(ADMIN_TEXT.CONTESTS.CANCEL_SUCCESS);
      }
      setLifecycleIntent(null);
      setLifecycleReason("");
      crud.refresh();
    } catch (error) {
      notify.error(
        getErrorMessage(
          error,
          lifecycleIntent.target === "ended"
            ? ADMIN_TEXT.CONTESTS.END_ERROR
            : ADMIN_TEXT.CONTESTS.CANCEL_ERROR,
        ),
      );
    } finally {
      setIsChangingLifecycle(false);
    }
  }

  const columns: AdminColumn<Contest>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (c) => <span className="text-xs tabular-nums text-muted-foreground">{c.id}</span> },
    { key: "title", label: ADMIN_TEXT.CONTESTS.COL_TITLE,
      render: (c) => (
        <Link
          href={APP_ROUTES.ADMIN_CONTEST(c.id)}
          className="font-medium text-foreground transition-colors hover:text-primary focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {c.title}
        </Link>
      ) },
    { key: "status", label: ADMIN_TEXT.CONTESTS.COL_STATUS, className: "w-32",
      render: (c) => (
        <Badge variant="secondary" className={STATUS_STYLES[c.status] ?? ""}>
          {STATUS_LABELS[c.status] ?? TEXT.COMMON.UNKNOWN}
        </Badge>
      ) },
    { key: "start_time", label: ADMIN_TEXT.CONTESTS.COL_START, className: "w-36",
      render: (c) => <span className="text-sm text-muted-foreground">{formatDateTimeMinute(c.start_time)}</span> },
    { key: "end_time", label: ADMIN_TEXT.CONTESTS.COL_END, className: "w-36",
      render: (c) => <span className="text-sm text-muted-foreground">{formatDateTimeMinute(c.end_time)}</span> },
    { key: "participants", label: ADMIN_TEXT.CONTESTS.COL_PARTICIPANTS, className: "w-28",
      render: (c) => (
        <span className="text-sm tabular-nums">
          {c.participant_count}{c.max_participants !== undefined ? `/${c.max_participants}` : ""}
        </span>
      ) },
    { key: "rating", label: ADMIN_TEXT.CONTESTS.COL_RATING, className: "w-28",
      render: (c) => {
        const label = !c.rated
          ? ADMIN_TEXT.CONTESTS.RATING_DISABLED
          : c.rating_completed_at
            ? ADMIN_TEXT.CONTESTS.RATING_APPLIED
            : c.status === "ended"
              ? ADMIN_TEXT.CONTESTS.RATING_PENDING
              : ADMIN_TEXT.CONTESTS.RATING_ENABLED;
        return (
          <Badge
            variant="secondary"
            className={
              c.rating_completed_at
                ? "bg-success/10 text-success"
                : ""
            }
          >
            {label}
          </Badge>
        );
      } },
    { key: "actions", label: "", className: "w-72",
      render: (c) => (
        <div className="flex justify-end gap-1">
          {canRead ? <Link href={APP_ROUTES.ADMIN_CONTEST_COMMUNICATIONS(c.id)} className="inline-flex min-h-10 items-center rounded-[var(--control-radius)] border border-border px-3 text-[0.8rem] font-semibold hover:bg-muted">{TEXT.CONTEST.COMMUNICATIONS_TITLE}</Link> : null}
          {canPublish && c.status === "draft" ? (
            <Button
              variant="outline"
              size="sm"
              className="cursor-pointer text-primary"
              aria-label={ADMIN_TEXT.CONTESTS.PUBLISH_ACTION}
              onClick={() => {
                setPublishReason("");
                setPublishTarget(c);
              }}
            >
              <Rocket className="h-3.5 w-3.5" />
              {ADMIN_TEXT.CONTESTS.PUBLISH}
            </Button>
          ) : null}
          {canPublish && c.status === "running" ? (
            <Button
              variant="outline"
              size="sm"
              className="cursor-pointer text-warning"
              aria-label={ADMIN_TEXT.CONTESTS.END_ACTION}
              onClick={() => {
                setLifecycleReason("");
                setLifecycleIntent({ contest: c, target: "ended" });
              }}
            >
              <CircleStop className="h-3.5 w-3.5" />
              {ADMIN_TEXT.CONTESTS.END}
            </Button>
          ) : null}
          {canPublish &&
          (c.status === "upcoming" || c.status === "running") ? (
            <Button
              variant="outline"
              size="sm"
              className="cursor-pointer text-destructive"
              aria-label={ADMIN_TEXT.CONTESTS.CANCEL_ACTION}
              onClick={() => {
                setLifecycleReason("");
                setLifecycleIntent({ contest: c, target: "cancelled" });
              }}
            >
              <Ban className="h-3.5 w-3.5" />
              {ADMIN_TEXT.CONTESTS.CANCEL}
            </Button>
          ) : null}
          {canRerate &&
          c.status === "ended" &&
          c.rated &&
          c.rating_completed_at ? (
            <Button
              variant="outline"
              size="sm"
              className="cursor-pointer"
              aria-label={ADMIN_TEXT.CONTESTS.RERATE_ACTION}
              onClick={() => {
                setReratingReason("");
                setReratingTarget(c);
              }}
            >
              <RefreshCw className="h-3.5 w-3.5" />
              {ADMIN_TEXT.CONTESTS.RERATE}
            </Button>
          ) : null}
          <AdminResourceActions
            resource={resource}
            onEdit={c.status === "draft" ? () => crud.openEdit(c) : undefined}
            onDelete={
              c.status === "draft"
                ? () => {
                    setDeleteReason("");
                    crud.confirmDelete(c.id);
                  }
                : undefined
            }
          />
        </div>
      ) },
  ];

  if (crud.isLoading && !crud.hasSettled) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={ADMIN_TEXT.CONTESTS.TITLE}
      subtitle={ADMIN_TEXT.CONTESTS.SUBTITLE}
      createLabel={ADMIN_TEXT.CONTESTS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      searchPlaceholder={ADMIN_TEXT.CONTESTS.SEARCH_PLACEHOLDER}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(c) => c.id}
          emptyMessage={ADMIN_TEXT.CONTESTS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <ContestDialog
          key={`${crud.dialogOpen}:${crud.editing?.id ?? "create"}`}
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
        />
      }
      confirmDialog={
        <>
          <ConfirmDialog
            open={crud.deleteId !== null}
            onOpenChange={(open) => {
              if (!open) {
                setDeleteReason("");
                crud.cancelDelete();
              }
            }}
            onConfirm={crud.handleDelete}
            confirmDisabled={!isContestReasonValid(deleteReason)}
          >
            <div className="space-y-2">
              <Label htmlFor="contest-delete-reason">
                {ADMIN_TEXT.CONTESTS.DELETE_REASON_LABEL}
              </Label>
              <Textarea
                id="contest-delete-reason"
                value={deleteReason}
                maxLength={CONTEST_REASON_LIMITS.MAXIMUM_CHARACTERS}
                placeholder={ADMIN_TEXT.CONTESTS.DELETE_REASON_PLACEHOLDER}
                aria-invalid={deleteReason.length > 0 && !isContestReasonValid(deleteReason)}
                onChange={(event) => setDeleteReason(event.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                {ADMIN_TEXT.CONTESTS.DELETE_REASON_HINT(
                  new TextEncoder().encode(deleteReason.trim()).byteLength,
                  CONTEST_REASON_LIMITS.MAXIMUM_UTF8_BYTES,
                )}
              </p>
            </div>
          </ConfirmDialog>
          <ConfirmDialog
            open={publishTarget !== null}
            onOpenChange={(open) => {
              if (!open && !isPublishing) {
                setPublishTarget(null);
                setPublishReason("");
              }
            }}
            onConfirm={handlePublish}
            title={ADMIN_TEXT.CONTESTS.PUBLISH_CONFIRM_TITLE}
            description={ADMIN_TEXT.CONTESTS.PUBLISH_CONFIRM_DESCRIPTION(publishTarget?.title ?? "")}
            confirmLabel={ADMIN_TEXT.CONTESTS.PUBLISH}
            confirmVariant="default"
            loading={isPublishing}
            confirmDisabled={!publishReasonValid}
          >
            <div className="space-y-2">
              <Label htmlFor="contest-publish-reason">
                {ADMIN_TEXT.CONTESTS.PUBLISH_REASON_LABEL}
              </Label>
              <Textarea
                id="contest-publish-reason"
                value={publishReason}
                maxLength={CONTEST_REASON_LIMITS.MAXIMUM_CHARACTERS}
                placeholder={ADMIN_TEXT.CONTESTS.PUBLISH_REASON_PLACEHOLDER}
                disabled={isPublishing}
                aria-invalid={publishReason.length > 0 && !publishReasonValid}
                onChange={(event) => setPublishReason(event.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                {ADMIN_TEXT.CONTESTS.PUBLISH_REASON_HINT(
                  publishReasonBytes,
                  CONTEST_REASON_LIMITS.MAXIMUM_UTF8_BYTES,
                )}
              </p>
            </div>
          </ConfirmDialog>
          <ConfirmDialog
            open={reratingTarget !== null}
            onOpenChange={(open) => {
              if (!open && !isStartingRerating) {
                setReratingTarget(null);
                setReratingReason("");
              }
            }}
            onConfirm={handleRerating}
            title={ADMIN_TEXT.CONTESTS.RERATE_CONFIRM_TITLE}
            description={ADMIN_TEXT.CONTESTS.RERATE_CONFIRM_DESCRIPTION(
              reratingTarget?.title ?? "",
            )}
            confirmLabel={ADMIN_TEXT.CONTESTS.RERATE_START}
            confirmVariant="default"
            loading={isStartingRerating}
            confirmDisabled={!reratingReasonValid}
          >
            <div className="space-y-4">
              <div className="flex gap-3 rounded-[var(--control-radius)] border border-warning/25 bg-warning/8 p-3 text-sm text-foreground">
                <AlertTriangle
                  className="mt-0.5 size-4 shrink-0 text-warning"
                  aria-hidden="true"
                />
                <p>{ADMIN_TEXT.CONTESTS.RERATE_SAFETY_NOTE}</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="contest-rerating-reason">
                  {ADMIN_TEXT.CONTESTS.RERATE_REASON_LABEL}
                </Label>
                <Textarea
                  id="contest-rerating-reason"
                  value={reratingReason}
                  maxLength={
                    CONTEST_REASON_LIMITS.MAXIMUM_CHARACTERS
                  }
                  placeholder={
                    ADMIN_TEXT.CONTESTS.RERATE_REASON_PLACEHOLDER
                  }
                  disabled={isStartingRerating}
                  aria-invalid={
                    reratingReason.length > 0 &&
                    !reratingReasonValid
                  }
                  onChange={(event) =>
                    setReratingReason(event.target.value)
                  }
                />
                <p className="text-xs text-muted-foreground">
                  {ADMIN_TEXT.CONTESTS.RERATE_REASON_HINT(
                    reratingReasonBytes,
                    CONTEST_REASON_LIMITS.MAXIMUM_UTF8_BYTES,
                  )}
                </p>
              </div>
            </div>
          </ConfirmDialog>
          <ConfirmDialog
            open={lifecycleIntent !== null}
            onOpenChange={(open) => {
              if (!open && !isChangingLifecycle) {
                setLifecycleIntent(null);
                setLifecycleReason("");
              }
            }}
            onConfirm={handleLifecycleTransition}
            title={
              lifecycleIntent?.target === "ended"
                ? ADMIN_TEXT.CONTESTS.END_CONFIRM_TITLE
                : ADMIN_TEXT.CONTESTS.CANCEL_CONFIRM_TITLE
            }
            description={
              lifecycleIntent?.target === "ended"
                ? ADMIN_TEXT.CONTESTS.END_CONFIRM_DESCRIPTION(
                    lifecycleIntent.contest.title,
                  )
                : ADMIN_TEXT.CONTESTS.CANCEL_CONFIRM_DESCRIPTION(
                    lifecycleIntent?.contest.title ?? "",
                  )
            }
            confirmLabel={
              lifecycleIntent?.target === "ended"
                ? ADMIN_TEXT.CONTESTS.END
                : ADMIN_TEXT.CONTESTS.CANCEL
            }
            confirmVariant="destructive"
            loading={isChangingLifecycle}
            confirmDisabled={!lifecycleReasonValid}
          >
            <div className="space-y-4">
              <div className="flex gap-3 rounded-[var(--control-radius)] border border-warning/25 bg-warning/8 p-3 text-sm text-foreground">
                <AlertTriangle
                  className="mt-0.5 size-4 shrink-0 text-warning"
                  aria-hidden="true"
                />
                <p>
                  {lifecycleIntent?.target === "ended"
                    ? ADMIN_TEXT.CONTESTS.END_CONFIRM_DESCRIPTION(
                        lifecycleIntent.contest.title,
                      )
                    : ADMIN_TEXT.CONTESTS.CANCEL_CONFIRM_DESCRIPTION(
                        lifecycleIntent?.contest.title ?? "",
                      )}
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="contest-lifecycle-reason">
                  {ADMIN_TEXT.CONTESTS.LIFECYCLE_REASON_LABEL}
                </Label>
                <Textarea
                  id="contest-lifecycle-reason"
                  value={lifecycleReason}
                  maxLength={CONTEST_REASON_LIMITS.MAXIMUM_CHARACTERS}
                  placeholder={
                    ADMIN_TEXT.CONTESTS.LIFECYCLE_REASON_PLACEHOLDER
                  }
                  disabled={isChangingLifecycle}
                  aria-invalid={
                    lifecycleReason.length > 0 && !lifecycleReasonValid
                  }
                  onChange={(event) =>
                    setLifecycleReason(event.target.value)
                  }
                />
                <p className="text-xs text-muted-foreground">
                  {ADMIN_TEXT.CONTESTS.LIFECYCLE_REASON_HINT(
                    lifecycleReasonBytes,
                    CONTEST_REASON_LIMITS.MAXIMUM_UTF8_BYTES,
                  )}
                </p>
              </div>
            </div>
          </ConfirmDialog>
        </>
      }
    />
  );
}
