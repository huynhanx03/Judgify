"use client";

import { useEffect, useState } from "react";
import {
  Archive,
  ArrowRight,
  CheckCircle2,
  Clock3,
  Link2,
  Loader2,
  RotateCcw,
  ShieldCheck,
  Upload,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { useAuth } from "@/contexts/auth-context";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useCursorFeed } from "@/hooks/use-cursor-feed";
import { materialService } from "@/services/material.service";
import type { MaterialArticle, MaterialRevision } from "@/types/material";
import type {
  MaterialLifecyclePreview,
  MaterialLifecycleProjection,
  MaterialLifecycleRequest,
} from "@/types/material";
import { notify } from "@/lib/toast";
import { ApiError } from "@/lib/api/error";
import { formatDateTime, formatDateTimeMinute } from "@/lib/format";
import { createIdempotencyKey } from "@/lib/api/idempotency";
import type { EntityID } from "@/types/api";

type LifecycleAction = "publish" | "archive" | "rollback" | "submit_review" | "return_to_draft" | "rename";

export function MaterialLifecycleActions({
  material,
  onChanged,
}: {
  material: MaterialArticle;
  onChanged: () => void;
}) {
  const { can } = useAuth();
  const [action, setAction] = useState<LifecycleAction | null>(null);
  const [reason, setReason] = useState("");
  const [targetRevision, setTargetRevision] = useState("");
  const [commandId, setCommandId] = useState<EntityID | null>(null);
  const [preview, setPreview] = useState<MaterialLifecyclePreview | null>(null);
  const [inlineError, setInlineError] = useState<string | null>(null);
  const [newSlug, setNewSlug] = useState(material.slug);
  const [submitting, setSubmitting] = useState(false);
  const capability =
    action === "rename"
      ? AUTHORIZATION_ACTION.UPDATE
		: action === "submit_review"
			? AUTHORIZATION_ACTION.UPDATE
			: action === "return_to_draft"
				? AUTHORIZATION_ACTION.MANAGE
      : action
        ? AUTHORIZATION_ACTION[
            action.toUpperCase() as "PUBLISH" | "ARCHIVE" | "ROLLBACK"
          ]
        : AUTHORIZATION_ACTION.READ;

  const revisionFeed = useCursorFeed<MaterialRevision>({
    resetKey: material.id,
    enabled: action === "rollback",
    load: async (cursor, signal) => {
      const page = await materialService.listRevisions(
        material.id,
        cursor,
        signal,
      );
      return { items: page.records, next_cursor: page.next_cursor };
    },
    keyOf: (revision) => revision.id,
  });
  const revisions = revisionFeed.items;
  const revisionCursor = revisionFeed.nextCursor;
  const revisionsLoading =
    revisionFeed.status === "loading" ||
    revisionFeed.status === "refreshing" ||
    revisionFeed.status === "loading_more";
  const revisionsError = revisionFeed.status === "error";
  const hasPublishableDraft = Boolean(
    material.status !== "archived" &&
      material.revision_id &&
      material.published_revision_id !== material.revision_id,
  );

  useEffect(() => {
    if (!preview || preview.expected_version === material.version) return;
    setPreview(null);
    setCommandId(createIdempotencyKey());
    setInlineError(ADMIN_TEXT.MATERIALS.LIFECYCLE_REVIEW_INVALID);
  }, [material.version, preview]);

  function openAction(next: LifecycleAction) {
    setAction(next);
    setInlineError(null);
    setPreview(null);
    setCommandId(next === "rename" ? null : createIdempotencyKey());
    if (next === "rename") setNewSlug(material.slug);
  }

  function invalidatePreview() {
    if (preview) setCommandId(createIdempotencyKey());
    setPreview(null);
    setInlineError(null);
  }

  function resetDialog() {
    setAction(null);
    setReason("");
    setTargetRevision("");
    setCommandId(null);
    setPreview(null);
    setInlineError(null);
  }

  async function submit() {
    if (!action || reason.trim().length < 3 || submitting) return;
    setSubmitting(true);
    setInlineError(null);
    try {
      if (action === "rename") {
        if (!newSlug.trim()) return;
        await materialService.rename(material.id, newSlug.trim(), {
          expected_version: material.version,
          reason: reason.trim(),
        });
        notify.success(ADMIN_TEXT.MATERIALS.LIFECYCLE_SUCCESS);
        resetDialog();
        onChanged();
        return;
      }

      if (!commandId) return;
      const request = {
        command_id: commandId,
        expected_version: material.version,
        reason: reason.trim(),
      };
      if (!preview) {
        const reviewed = await createLifecyclePreview(
          action,
          material,
          targetRevision,
          request,
        );
        setPreview(reviewed);
        return;
      }
      const receipt = await applyLifecyclePreview(
        action,
        material,
        targetRevision,
        {
          ...request,
          confirmation_token: preview.confirmation_token,
        },
      );
      notify.success(
        receipt.idempotent_replay
          ? ADMIN_TEXT.MATERIALS.LIFECYCLE_REPLAY_SUCCESS
          : ADMIN_TEXT.MATERIALS.LIFECYCLE_SUCCESS,
      );
      resetDialog();
      onChanged();
    } catch (error) {
	  const recentAuthenticationRequired =
		error instanceof ApiError &&
		error.params?.reason === "recent_authentication_required";
	  const reviewedAction = action !== "rename";
	  const reviewInvalid =
		reviewedAction &&
		error instanceof ApiError &&
		(error.status === 409 || error.status === 403);
	  const mutationConflict =
		!reviewedAction && error instanceof ApiError && error.status === 409;
      if (reviewInvalid) {
        setPreview(null);
        setCommandId(createIdempotencyKey());
      }
      if (error instanceof ApiError && error.status === 409) onChanged();
	  const message = recentAuthenticationRequired
		? ADMIN_TEXT.MATERIALS.LIFECYCLE_RECENT_AUTH_REQUIRED
		: reviewInvalid
		  ? ADMIN_TEXT.MATERIALS.LIFECYCLE_REVIEW_INVALID
		  : mutationConflict
			? ADMIN_TEXT.MATERIALS.MUTATION_CONFLICT
		  : ADMIN_TEXT.MATERIALS.LIFECYCLE_ERROR;
      setInlineError(message);
      notify.error(message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex items-center justify-end gap-1">
      {hasPublishableDraft &&
      can(AUTHORIZATION_RESOURCE.MATERIAL, AUTHORIZATION_ACTION.PUBLISH) ? (
        <Button
          variant="ghost"
          size="icon"
          className="min-h-11 min-w-11 cursor-pointer transition-colors duration-200"
          onClick={() => openAction("publish")}
          aria-label={ADMIN_TEXT.MATERIALS.PUBLISH}
        >
          <Upload className="size-4" aria-hidden="true" />
        </Button>
      ) : null}
		{material.status === "draft" &&
		can(AUTHORIZATION_RESOURCE.MATERIAL, AUTHORIZATION_ACTION.UPDATE) ? (
			<Button variant="ghost" size="icon" className="min-h-11 min-w-11 cursor-pointer transition-colors duration-200" onClick={() => openAction("submit_review")} aria-label={ADMIN_TEXT.MATERIALS.SUBMIT_REVIEW}>
				<Upload className="size-4" aria-hidden="true" />
			</Button>
		) : null}
		{material.status === "in_review" &&
		can(AUTHORIZATION_RESOURCE.MATERIAL, AUTHORIZATION_ACTION.MANAGE) ? (
			<Button variant="ghost" size="icon" className="min-h-11 min-w-11 cursor-pointer transition-colors duration-200" onClick={() => openAction("return_to_draft")} aria-label={ADMIN_TEXT.MATERIALS.RETURN_TO_DRAFT}>
				<RotateCcw className="size-4" aria-hidden="true" />
			</Button>
		) : null}
      {material.status === "published" &&
      can(AUTHORIZATION_RESOURCE.MATERIAL, AUTHORIZATION_ACTION.ARCHIVE) ? (
        <Button
          variant="ghost"
          size="icon"
          className="min-h-11 min-w-11 cursor-pointer transition-colors duration-200"
          onClick={() => openAction("archive")}
          aria-label={ADMIN_TEXT.MATERIALS.ARCHIVE}
        >
          <Archive className="size-4" aria-hidden="true" />
        </Button>
      ) : null}
	  {material.status !== "archived" &&
	  can(AUTHORIZATION_RESOURCE.MATERIAL, AUTHORIZATION_ACTION.UPDATE) ? (
        <Button
          variant="ghost"
          size="icon"
          className="min-h-11 min-w-11 cursor-pointer transition-colors duration-200"
          onClick={() => openAction("rename")}
          aria-label={ADMIN_TEXT.MATERIALS.RENAME}
        >
          <Link2 className="size-4" aria-hidden="true" />
        </Button>
      ) : null}
      {material.status === "published" &&
      can(AUTHORIZATION_RESOURCE.MATERIAL, AUTHORIZATION_ACTION.ROLLBACK) ? (
        <Button
          variant="ghost"
          size="icon"
          className="min-h-11 min-w-11 cursor-pointer transition-colors duration-200"
          onClick={() => openAction("rollback")}
          aria-label={ADMIN_TEXT.MATERIALS.ROLLBACK}
        >
          <RotateCcw className="size-4" aria-hidden="true" />
        </Button>
      ) : null}

      <ConfirmDialog
        open={action !== null}
        onOpenChange={(open) => {
          if (!open && !submitting) resetDialog();
        }}
        onConfirm={submit}
        title={action ? ADMIN_TEXT.MATERIALS.LIFECYCLE_TITLE[action] : ""}
        description={
          preview
            ? ADMIN_TEXT.MATERIALS.LIFECYCLE_REVIEW_DESCRIPTION
            : action
              ? ADMIN_TEXT.MATERIALS.LIFECYCLE_DESCRIPTION[action]
              : ""
        }
        confirmLabel={
          action === "rename"
            ? ADMIN_TEXT.MATERIALS.LIFECYCLE_CONFIRM.rename
            : preview
              ? ADMIN_TEXT.MATERIALS.LIFECYCLE_APPLY_REVIEWED
              : ADMIN_TEXT.MATERIALS.LIFECYCLE_CREATE_REVIEW
        }
        confirmVariant={action === "archive" ? "destructive" : "default"}
        loading={submitting}
        contentClassName="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl"
        confirmDisabled={
          !action ||
          !can(AUTHORIZATION_RESOURCE.MATERIAL, capability) ||
          reason.trim().length < 3 ||
          (action === "publish" && !material.revision_id) ||
          (action === "rollback" && targetRevision.trim().length === 0) ||
          (action !== "rename" && commandId === null) ||
          (action === "rename" && newSlug.trim().length === 0)
        }
      >
        <div className="space-y-4">
          {action === "rollback" ? (
            <div className="space-y-2">
              <Label htmlFor="material-rollback-revision">
                {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION}
              </Label>
              <select
                id="material-rollback-revision"
                className="min-h-11 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={targetRevision}
                onChange={(event) => {
                  invalidatePreview();
                  setTargetRevision(event.target.value);
                }}
                required
                disabled={revisionsLoading && revisions.length === 0}
              >
                <option value="">
                  {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_PLACEHOLDER}
                </option>
                {revisions.map((revision) => (
				  <option
					key={revision.id}
					value={revision.id}
					disabled={rollbackRevisionUnavailable(material, revision)}
				  >
                    {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_NUMBER(
                      revision.revision_number,
                    )}{" "}
                    — {revision.title}
					{rollbackRevisionNote(material, revision)}
                  </option>
                ))}
              </select>
              {targetRevision ? (
                <RevisionSummary
                  revision={revisions.find(
                    (revision) => revision.id === targetRevision,
                  )}
                />
              ) : null}
              {revisionsLoading ? (
                <p
                  className="flex items-center gap-2 text-sm text-muted-foreground"
                  role="status"
                >
                  <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_LOADING}
                </p>
              ) : null}
              {revisionsError ? (
                <div
                  className="flex items-center justify-between gap-3 text-sm text-destructive"
                  role="alert"
                >
                  <span>{ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_LOAD_ERROR}</span>
                  {revisions.length === 0 ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={revisionFeed.reload}
                    >
                      {TEXT.COMMON.RETRY}
                    </Button>
                  ) : null}
                </div>
              ) : null}
              {!revisionsLoading && revisions.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_EMPTY}
                </p>
              ) : null}
              {revisionCursor ? (
                <Button
                  type="button"
                  variant="outline"
                  className="min-h-11 w-full cursor-pointer"
                  disabled={revisionsLoading}
                  onClick={() => void revisionFeed.loadMore()}
                >
                  {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_LOAD_MORE}
                </Button>
              ) : null}
            </div>
          ) : null}
          {action === "rename" ? (
            <div className="space-y-2">
              <Label htmlFor="material-new-slug">
                {ADMIN_TEXT.MATERIALS.FORM_SLUG}
              </Label>
              <Input
                id="material-new-slug"
                value={newSlug}
                onChange={(event) => setNewSlug(event.target.value)}
                placeholder={ADMIN_TEXT.MATERIALS.FORM_SLUG_PLACEHOLDER}
                required
              />
            </div>
          ) : null}
          <div className="space-y-2">
            <Label htmlFor="material-lifecycle-reason">
              {ADMIN_TEXT.MATERIALS.LIFECYCLE_REASON}
            </Label>
            <Textarea
              id="material-lifecycle-reason"
              value={reason}
              onChange={(event) => {
                invalidatePreview();
                setReason(event.target.value);
              }}
              maxLength={500}
              placeholder={ADMIN_TEXT.MATERIALS.LIFECYCLE_REASON_PLACEHOLDER}
              required
            />
          </div>
          {preview ? <LifecycleReview preview={preview} /> : null}
          {inlineError ? (
            <p
              className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
              role="alert"
            >
              {inlineError}
            </p>
          ) : null}
        </div>
      </ConfirmDialog>
    </div>
  );
}

function RevisionSummary({
  revision,
}: {
  revision: MaterialRevision | undefined;
}) {
  if (!revision) return null;
  return (
    <div className="rounded-md border border-border bg-muted/30 p-3 text-sm">
      <p className="font-medium">{revision.change_summary}</p>
      <p className="mt-1 text-muted-foreground">
        {ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_CREATED_AT(
          formatDateTimeMinute(revision.created_at),
        )}
      </p>
      <p className="mt-1 font-mono text-xs text-muted-foreground">
        {revision.normalized_source_checksum.slice(0, 16)}
      </p>
    </div>
  );
}

async function createLifecyclePreview(
  action: Exclude<LifecycleAction, "rename">,
  material: MaterialArticle,
  targetRevision: string,
  request: MaterialLifecycleRequest,
): Promise<MaterialLifecyclePreview> {
  switch (action) {
	case "submit_review":
		return materialService.previewSubmitReview(material.id, request);
	case "return_to_draft":
		return materialService.previewReturnToDraft(material.id, request);
    case "publish":
      if (!material.revision_id) {
        throw new TypeError("material revision is unavailable");
      }
      return materialService.previewPublish(
        material.id,
        material.revision_id,
        request,
      );
    case "archive":
      return materialService.previewArchive(material.id, request);
    case "rollback":
      return materialService.previewRollback(
        material.id,
        targetRevision,
        request,
      );
  }
}

async function applyLifecyclePreview(
  action: Exclude<LifecycleAction, "rename">,
  material: MaterialArticle,
  targetRevision: string,
  request: MaterialLifecycleRequest,
) {
  switch (action) {
	case "submit_review":
		return materialService.applySubmitReview(material.id, request);
	case "return_to_draft":
		return materialService.applyReturnToDraft(material.id, request);
    case "publish":
      if (!material.revision_id) {
        throw new TypeError("material revision is unavailable");
      }
      return materialService.applyPublish(
        material.id,
        material.revision_id,
        request,
      );
    case "archive":
      return materialService.applyArchive(material.id, request);
    case "rollback":
      return materialService.applyRollback(
        material.id,
        targetRevision,
        request,
      );
  }
}

function LifecycleReview({
  preview,
}: {
  preview: MaterialLifecyclePreview;
}) {
  const expiresAt = formatDateTime(preview.confirmation_expires_at);
  return (
    <section
      className="space-y-4 rounded-xl border border-primary/20 bg-primary/[0.035] p-4 shadow-sm"
      aria-labelledby="material-lifecycle-review-title"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
            <ShieldCheck className="size-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h3
              id="material-lifecycle-review-title"
              className="font-semibold text-foreground"
            >
              {ADMIN_TEXT.MATERIALS.LIFECYCLE_REVIEW_TITLE}
            </h3>
            <p className="mt-1 text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.MATERIALS.LIFECYCLE_REVIEW_HELP}
            </p>
          </div>
        </div>
        <div className="inline-flex shrink-0 items-center gap-2 rounded-full border bg-background px-3 py-1.5 text-xs font-medium text-muted-foreground">
          <Clock3 className="size-3.5" aria-hidden="true" />
          {ADMIN_TEXT.MATERIALS.LIFECYCLE_EXPIRES_AT(expiresAt)}
        </div>
      </div>

      <div className="grid items-stretch gap-3 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] sm:gap-4">
        <LifecycleProjectionCard
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_BEFORE}
          projection={preview.before}
        />
        <div className="flex items-center justify-center text-muted-foreground">
          <ArrowRight
            className="size-5 rotate-90 motion-reduce:transition-none sm:rotate-0"
            aria-hidden="true"
          />
          <span className="sr-only">
            {ADMIN_TEXT.MATERIALS.LIFECYCLE_CHANGE_DIRECTION}
          </span>
        </div>
        <LifecycleProjectionCard
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_AFTER}
          projection={preview.after}
          emphasized
        />
      </div>

      <div className="flex items-start gap-2 rounded-lg border border-success/25 bg-success/10 px-3 py-2 text-sm text-foreground">
        <CheckCircle2
          className="mt-0.5 size-4 shrink-0 text-success"
          aria-hidden="true"
        />
        <span>{ADMIN_TEXT.MATERIALS.LIFECYCLE_SERVER_VERIFIED}</span>
      </div>
    </section>
  );
}

function LifecycleProjectionCard({
  label,
  projection,
  emphasized = false,
}: {
  label: string;
  projection: MaterialLifecycleProjection;
  emphasized?: boolean;
}) {
  return (
    <div
      className={
        emphasized
          ? "rounded-xl border border-primary/30 bg-background p-4 ring-1 ring-primary/10"
          : "rounded-xl border bg-muted/30 p-4"
      }
    >
      <p className="text-xs font-semibold uppercase tracking-[0.12em] text-muted-foreground">
        {label}
      </p>
      <dl className="mt-3 space-y-2.5 text-sm">
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_STATUS}
          value={materialStatus(projection.status)}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_VERSION}
          value={String(projection.version)}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_REVISION}
          value={
            projection.revision_number
              ? ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_NUMBER(
                  projection.revision_number,
                )
              : TEXT.COMMON.NOT_AVAILABLE
          }
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_TITLE}
          value={projection.title ?? TEXT.COMMON.NOT_AVAILABLE}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_VISIBILITY}
          value={materialVisibility(projection.visibility)}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_TAGS}
          value={ADMIN_TEXT.MATERIALS.LIFECYCLE_TAG_COUNT(
            projection.tag_ids.length,
          )}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_ARTIFACT}
          value={shortEvidenceID(projection.render_artifact_id)}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_PIPELINE}
          value={`${projection.renderer_version} · ${projection.sanitizer_policy_version}`}
        />
        <ProjectionRow
          label={ADMIN_TEXT.MATERIALS.LIFECYCLE_FIELD_CHECKSUM}
          value={projection.source_checksum.slice(0, 16)}
        />
      </dl>
    </div>
  );
}

function ProjectionRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)] gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="break-words text-right font-medium text-foreground">
        {value}
      </dd>
    </div>
  );
}

function materialStatus(status: MaterialArticle["status"]): string {
  return {
    draft: ADMIN_TEXT.MATERIALS.STATUS_DRAFT,
    in_review: ADMIN_TEXT.MATERIALS.STATUS_IN_REVIEW,
    published: ADMIN_TEXT.MATERIALS.STATUS_PUBLISHED,
    archived: ADMIN_TEXT.MATERIALS.STATUS_ARCHIVED,
  }[status];
}

function materialVisibility(
  visibility: MaterialArticle["visibility"],
): string {
  return visibility === "public"
    ? ADMIN_TEXT.MATERIALS.VISIBILITY_PUBLIC
    : ADMIN_TEXT.MATERIALS.VISIBILITY_AUTHENTICATED;
}

function shortEvidenceID(value: string): string {
	return value.length <= 18 ? value : `${value.slice(0, 8)}…${value.slice(-6)}`;
}

function rollbackRevisionUnavailable(
	material: MaterialArticle,
	revision: MaterialRevision,
): boolean {
	return (
	  revision.id === material.revision_id ||
	  revision.id === material.published_revision_id ||
	  (material.published_revision_number !== undefined &&
		revision.revision_number >= material.published_revision_number)
	);
}

function rollbackRevisionNote(
	material: MaterialArticle,
	revision: MaterialRevision,
): string {
	if (
	  revision.id === material.revision_id ||
	  revision.id === material.published_revision_id
	) {
	  return ` (${ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_CURRENT})`;
	}
	if (
	  material.published_revision_number !== undefined &&
	  revision.revision_number >= material.published_revision_number
	) {
	  return ` (${ADMIN_TEXT.MATERIALS.ROLLBACK_REVISION_NOT_PRIOR})`;
	}
	return "";
}
