"use client";

import { useMemo, useState } from "react";
import { Archive, CheckCircle2, History, Pencil } from "lucide-react";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/loading-spinner";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useSession } from "@/contexts/session-context";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { AttributeLifecycleReviewDialog } from "@/modules/admin/attribute-lifecycle-review-dialog";
import { AttributeRevisionHistorySheet } from "@/modules/admin/attribute-revision-history-sheet";
import { AttributeDefinitionDialog } from "@/modules/admin/dialogs/attribute-definition-dialog";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { ApiError } from "@/lib/api/error";
import { attributeDataTypeLabel } from "@/lib/admin/auxiliary-form";
import { getErrorMessage, notify } from "@/lib/toast";
import { authService } from "@/services/auth.service";
import { attributeDefinitionService } from "@/services/attribute-definition.service";
import type {
  AttributeDefinition,
  AttributeDefinitionRevision,
  AttributeLifecycleAction,
  CreateAttributeDefinitionInput,
  ReviewedAttributeLifecycleCommand,
  UpdateAttributeDefinitionInput,
} from "@/types/admin-auxiliary";

export default function AttributeDefinitionsPage() {
  const { can } = useAuth();
  const session = useSession();
  const resource = AUTHORIZATION_RESOURCE.ATTRIBUTE_DEFINITION;
  const service = useMemo(() => ({
    find: attributeDefinitionService.find,
    create: attributeDefinitionService.create,
    update: attributeDefinitionService.update,
  }), []);
  const crud = usePaginatedCRUD<
    AttributeDefinition,
    CreateAttributeDefinitionInput,
    UpdateAttributeDefinitionInput
  >({ service, resource, searchKey: "key" });
  const [lifecycleTarget, setLifecycleTarget] = useState<AttributeDefinition | null>(null);
  const [lifecycleCandidate, setLifecycleCandidate] = useState<AttributeDefinitionRevision | null>(null);
  const [historyTarget, setHistoryTarget] = useState<AttributeDefinition | null>(null);
  const [lifecycleAction, setLifecycleAction] = useState<AttributeLifecycleAction | null>(null);
  const [reviewedLifecycle, setReviewedLifecycle] = useState<ReviewedAttributeLifecycleCommand | null>(null);
  const [isPreviewingLifecycle, setIsPreviewingLifecycle] = useState(false);
  const [isApplyingLifecycle, setIsApplyingLifecycle] = useState(false);
  const [lifecycleError, setLifecycleError] = useState<string | null>(null);
  const [reauthenticationRequired, setReauthenticationRequired] = useState(false);

  const canUpdate = can(resource, AUTHORIZATION_ACTION.UPDATE);
  const canArchive = can(resource, AUTHORIZATION_ACTION.DELETE);

  function openLifecycle(
    item: AttributeDefinition,
    action: AttributeLifecycleAction,
    candidate: AttributeDefinitionRevision | null = null,
  ) {
    const candidateID = candidate?.id ?? item.draft_revision_id;
    if (
      item.status !== "active" ||
      (action === "activate" &&
        (!canUpdate || !candidateID || candidateID === item.active_revision_id)) ||
      (action === "archive" && (!canArchive || item.kind === "system" || item.required_on_onboarding))
    ) {
      return;
    }
    setLifecycleTarget(item);
    setLifecycleCandidate(action === "activate" ? candidate : null);
    setLifecycleAction(action);
    setReviewedLifecycle(null);
    setLifecycleError(null);
    setReauthenticationRequired(false);
  }

  function resetLifecycle() {
    setLifecycleTarget(null);
    setLifecycleCandidate(null);
    setLifecycleAction(null);
    setReviewedLifecycle(null);
    setLifecycleError(null);
    setReauthenticationRequired(false);
  }

  function closeLifecycle() {
    if (isPreviewingLifecycle || isApplyingLifecycle) return;
    resetLifecycle();
  }

  async function previewLifecycle(reason: string) {
    const target = lifecycleTarget;
    const action = lifecycleAction;
    if (!target || !action || isPreviewingLifecycle || isApplyingLifecycle) return;
    setIsPreviewingLifecycle(true);
    setLifecycleError(null);
    setReauthenticationRequired(false);
    try {
      const reviewed = await attributeDefinitionService.previewLifecycle(
        target.id,
        action,
        {
          expected_version: target.version,
          candidate_revision_id:
            action === "activate"
              ? (lifecycleCandidate?.id ?? target.draft_revision_id)
              : undefined,
          reason,
        },
      );
      setReviewedLifecycle(reviewed);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) crud.refresh();
      setLifecycleError(
        getErrorMessage(error, ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REVIEW_ERROR),
      );
    } finally {
      setIsPreviewingLifecycle(false);
    }
  }

  async function applyLifecycle(password: string) {
    const reviewed = reviewedLifecycle;
    if (!reviewed || isApplyingLifecycle || isPreviewingLifecycle) return;
    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      setReviewedLifecycle(null);
      setLifecycleError(ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.CONFIRMATION_EXPIRED);
      return;
    }
    setIsApplyingLifecycle(true);
    setLifecycleError(null);
    try {
      if (reauthenticationRequired) {
        await authService.reauthenticateRecoveryContact(password);
        const refreshed = await session.refresh({ forceRotation: true });
        if (!refreshed) throw new TypeError("session refresh rejected");
        setReauthenticationRequired(false);
      }
      const receipt = await attributeDefinitionService.applyLifecycle(reviewed);
      notify.success(
        receipt.status === "migration_scheduled"
          ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.MIGRATION_SCHEDULED_SUCCESS
          : receipt.action === "archive"
          ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ARCHIVE_SUCCESS
          : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ACTIVATE_SUCCESS,
      );
      resetLifecycle();
      crud.refresh();
    } catch (error) {
      if (error instanceof ApiError && error.status === 403) {
        setReauthenticationRequired(true);
        setLifecycleError(ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REAUTH_REQUIRED);
      } else {
        if (error instanceof ApiError && error.status === 409) crud.refresh();
        setLifecycleError(
          getErrorMessage(error, ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.APPLY_ERROR),
        );
      }
    } finally {
      setIsApplyingLifecycle(false);
    }
  }

  const columns: AdminColumn<AttributeDefinition>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "min-w-48", render: (item) => <span className="text-xs tabular-nums text-muted-foreground">{item.id}</span> },
    { key: "key", label: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.COL_KEY, className: "min-w-36", render: (item) => <span className="font-medium">{item.key}</span> },
    { key: "revision_data_type", label: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.COL_DATA_TYPE, className: "min-w-32", render: (item) => <span className="rounded bg-muted px-1.5 py-0.5 text-xs">{attributeDataTypeLabel(item.draft_revision.data_type)}</span> },
    { key: "status", label: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.COL_STATUS, className: "min-w-32", render: (item) => {
      const hasDraft = item.draft_revision_id !== item.active_revision_id;
      return <div className="flex flex-wrap gap-1.5"><span className="rounded-full border border-border bg-background px-2 py-0.5 text-xs">{item.status === "active" ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.STATUS_ACTIVE : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.STATUS_ARCHIVED}</span>{hasDraft ? <span className="rounded-full border border-primary/25 bg-primary/10 px-2 py-0.5 text-xs text-primary">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.STATUS_DRAFT_READY}</span> : null}</div>;
    } },
    { key: "revision_description", label: ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.COL_DESCRIPTION, className: "min-w-56", render: (item) => <span className="text-sm text-muted-foreground">{item.draft_revision.description || TEXT.COMMON.NOT_AVAILABLE}</span> },
    { key: "actions", label: "", className: "w-48", render: (item) => {
      const draftReady = item.status === "active" && item.draft_revision_id && item.draft_revision_id !== item.active_revision_id;
      const archivable = item.status === "active" && item.kind === "custom" && !item.required_on_onboarding;
      return <div className="flex items-center justify-end gap-1">
        <Button type="button" variant="ghost" size="icon" className="h-10 w-10 text-muted-foreground hover:text-foreground" onClick={() => setHistoryTarget(item)} aria-label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_ACTION}><History className="h-4 w-4" aria-hidden="true" /></Button>
        {canUpdate && item.status === "active" ? <Button type="button" variant="ghost" size="icon" className="h-10 w-10 text-muted-foreground hover:text-foreground" onClick={() => crud.openEdit(item)} aria-label={TEXT.COMMON.EDIT}><Pencil className="h-4 w-4" aria-hidden="true" /></Button> : null}
        {canUpdate && draftReady ? <Button type="button" variant="ghost" size="icon" className="h-10 w-10 text-muted-foreground hover:text-primary" onClick={() => openLifecycle(item, "activate")} aria-label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ACTIVATE_ACTION}><CheckCircle2 className="h-4 w-4" aria-hidden="true" /></Button> : null}
        {canArchive && archivable ? <Button type="button" variant="ghost" size="icon" className="h-10 w-10 text-muted-foreground hover:text-destructive" onClick={() => openLifecycle(item, "archive")} aria-label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ARCHIVE_ACTION}><Archive className="h-4 w-4" aria-hidden="true" /></Button> : null}
      </div>;
    } },
  ];

  if (!crud.hasSettled) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.TITLE}
      subtitle={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.SUBTITLE}
      createLabel={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchLabel={ADMIN_TEXT.FIELDS.SEARCH}
      searchPlaceholder={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.SEARCH_PLACEHOLDER}
      searchError={crud.searchError}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      table={<AdminDataTable columns={columns} data={crud.data} keyExtractor={(item) => item.id} emptyMessage={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.EMPTY} pagination={crud.pagination} onPageChange={crud.onPageChange} />}
      dialog={<AttributeDefinitionDialog key={`${crud.dialogOpen}:${crud.editing?.id ?? "create"}`} open={crud.dialogOpen} editing={crud.editing} onCreate={crud.handleCreate} onUpdate={crud.handleUpdate} onClose={crud.closeDialog} isSaving={crud.isSaving} />}
      confirmDialog={
        <>
          <AttributeLifecycleReviewDialog
            key={`${lifecycleTarget?.id ?? "closed"}:${lifecycleAction ?? "none"}:${reviewedLifecycle?.command_id ?? "draft"}`}
            definition={lifecycleTarget}
            candidateRevision={lifecycleCandidate}
            action={lifecycleAction}
            reviewed={reviewedLifecycle}
            isPreviewing={isPreviewingLifecycle}
            isApplying={isApplyingLifecycle}
            reauthenticationRequired={reauthenticationRequired}
            error={lifecycleError}
            onPreview={previewLifecycle}
            onApply={applyLifecycle}
            onClose={closeLifecycle}
          />
          <AttributeRevisionHistorySheet
            definition={historyTarget}
            open={Boolean(historyTarget)}
            canRestore={canUpdate}
            onClose={() => setHistoryTarget(null)}
            onRestore={(revision) => {
              const target = historyTarget;
              if (!target) return;
              setHistoryTarget(null);
              openLifecycle(target, "activate", revision);
            }}
          />
        </>
      }
    />
  );
}
