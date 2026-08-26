"use client";

import { FormEvent, useMemo, useState, type ReactNode } from "react";
import {
  AlertTriangle,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  FilePenLine,
  History,
  Loader2,
  Megaphone,
  Plus,
  RefreshCw,
  Send,
  Undo2,
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { CONTEST_CONTENT_LIMITS } from "@/constants/contest-content";
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  announcementEditorInput,
  announcementEditorIsValid,
  announcementScheduleLocal,
  utf8Bytes,
  type AnnouncementEditorValue,
} from "@/lib/contest/content-input";
import { formatDateTimeMinute } from "@/lib/format";
import { getErrorMessage, notify } from "@/lib/toast";
import {
  contestContentService,
  type AdminAnnouncementListQuery,
} from "@/services/contest-content.service";
import type {
  ContestAnnouncement,
  ContestContentPage,
  ContestContentRevision,
} from "@/types/contest";

type AnnouncementFilter = "all" | ContestAnnouncement["status"];
type AnnouncementAction = "publish" | "withdraw";

const EMPTY_PAGE: ContestContentPage<ContestAnnouncement> = { items: [] };
const EMPTY_REVISIONS: ContestContentRevision[] = [];

const STATUS_COPY: Record<ContestAnnouncement["status"], string> = {
  draft: TEXT.CONTEST.ANNOUNCEMENT_STATUS_DRAFT,
  published: TEXT.CONTEST.ANNOUNCEMENT_STATUS_PUBLISHED,
  withdrawn: TEXT.CONTEST.ANNOUNCEMENT_STATUS_WITHDRAWN,
};

const AUDIENCE_COPY: Record<ContestAnnouncement["audience"], string> = {
  public: TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_PUBLIC,
  participants: TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_PARTICIPANTS,
  jury: TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_JURY,
};

function statusVariant(
  status: ContestAnnouncement["status"],
): "default" | "secondary" | "outline" {
  if (status === "published") return "default";
  if (status === "draft") return "secondary";
  return "outline";
}

export function AnnouncementManager({
  contestID,
  canUpdate,
  realtimeRevision,
}: {
  contestID: string;
  canUpdate: boolean;
  realtimeRevision: number;
}) {
  const [filter, setFilter] = useState<AnnouncementFilter>("all");
  const [cursorStack, setCursorStack] = useState<(string | undefined)[]>([
    undefined,
  ]);
  const [pageIndex, setPageIndex] = useState(0);
  const [editing, setEditing] = useState<ContestAnnouncement | null>(null);
  const [editorNonce, setEditorNonce] = useState(0);
  const [actionTarget, setActionTarget] = useState<{
    item: ContestAnnouncement;
    action: AnnouncementAction;
  } | null>(null);

  const cursor = cursorStack[pageIndex];
  const query = useMemo<AdminAnnouncementListQuery>(
    () => ({
      ...(filter === "all" ? {} : { status: filter }),
      ...(cursor ? { cursor } : {}),
    }),
    [cursor, filter],
  );
  const list = useRetryableResource({
    resetKey:
      `${contestID}:${filter}:${pageIndex}:${cursor ?? "first"}:` +
      realtimeRevision,
    initialData: EMPTY_PAGE,
    keepPreviousData: true,
    load: (signal) =>
      contestContentService.adminAnnouncements(contestID, query, signal),
  });

  function changeFilter(value: AnnouncementFilter) {
    setFilter(value);
    setCursorStack([undefined]);
    setPageIndex(0);
    setEditing(null);
  }

  function nextPage() {
    if (!list.data.next_cursor) return;
    setCursorStack((current) => {
      const next = current.slice(0, pageIndex + 1);
      next.push(list.data.next_cursor);
      return next;
    });
    setPageIndex((current) => current + 1);
  }

  function previousPage() {
    setPageIndex((current) => Math.max(0, current - 1));
  }

  function finishEditing() {
    setEditing(null);
    setEditorNonce((current) => current + 1);
    list.retry();
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(20rem,0.82fr)_minmax(28rem,1.18fr)]">
      <AnnouncementEditor
        key={`${editing?.id ?? "new"}:${editing?.version ?? editorNonce}`}
        contestID={contestID}
        editing={editing}
        canUpdate={canUpdate}
        onCancel={() => setEditing(null)}
        onSaved={finishEditing}
      />

      <Card className="min-w-0">
        <CardHeader className="border-b">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <CardTitle>{TEXT.CONTEST.ANNOUNCEMENT_LIST_TITLE}</CardTitle>
              <CardDescription>
                {TEXT.CONTEST.ANNOUNCEMENT_EDITOR_DESCRIPTION}
              </CardDescription>
            </div>
            <Select
              value={filter}
              onValueChange={(value) =>
                changeFilter(value as AnnouncementFilter)
              }
            >
              <SelectTrigger className="w-full sm:w-52">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {TEXT.CONTEST.ANNOUNCEMENT_FILTER_ALL}
                </SelectItem>
                <SelectItem value="draft">
                  {TEXT.CONTEST.ANNOUNCEMENT_FILTER_DRAFT}
                </SelectItem>
                <SelectItem value="published">
                  {TEXT.CONTEST.ANNOUNCEMENT_FILTER_PUBLISHED}
                </SelectItem>
                <SelectItem value="withdrawn">
                  {TEXT.CONTEST.ANNOUNCEMENT_FILTER_WITHDRAWN}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className="space-y-4 pt-1">
          {list.status === "loading" && list.data.items.length === 0 ? (
            <AnnouncementListSkeleton />
          ) : null}

          {list.status === "error" && list.data.items.length === 0 ? (
            <div
              className="flex min-h-52 flex-col items-center justify-center gap-4 text-center"
              role="alert"
            >
              <AlertTriangle
                className="size-7 text-destructive"
                aria-hidden="true"
              />
              <p className="text-sm text-muted-foreground">
                {TEXT.CONTEST.ANNOUNCEMENT_LIST_ERROR}
              </p>
              <Button type="button" variant="outline" onClick={list.retry}>
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.CONTEST.RETRY}
              </Button>
            </div>
          ) : null}

          {list.data.items.length === 0 && list.status === "ready" ? (
            <div className="flex min-h-52 flex-col items-center justify-center gap-3 text-center">
              <div className="rounded-2xl bg-muted p-4 text-muted-foreground">
                <Megaphone className="size-6" aria-hidden="true" />
              </div>
              <p className="max-w-sm text-sm text-muted-foreground">
                {TEXT.CONTEST.ANNOUNCEMENT_LIST_EMPTY}
              </p>
            </div>
          ) : null}

          <div className="space-y-3" aria-busy={list.status === "loading"}>
            {list.data.items.map((item) => (
              <article
                key={item.id}
                className="rounded-xl border border-border bg-background/70 p-4 transition-colors hover:border-primary/30"
              >
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant={statusVariant(item.status)}>
                        {STATUS_COPY[item.status]}
                      </Badge>
                      <Badge variant="outline">
                        {AUDIENCE_COPY[item.audience]}
                      </Badge>
                    </div>
                    <h3 className="text-base font-semibold leading-snug">
                      {item.title}
                    </h3>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span>
                        {TEXT.CONTEST.ANNOUNCEMENT_REVISION(item.revision)}
                      </span>
                      <span>
                        {TEXT.CONTEST.ANNOUNCEMENT_UPDATED_AT(
                          formatDateTimeMinute(item.updated_at),
                        )}
                      </span>
                      {item.scheduled_for ? (
                        <span className="inline-flex items-center gap-1 text-primary">
                          <CalendarClock
                            className="size-3.5"
                            aria-hidden="true"
                          />
                          {TEXT.CONTEST.ANNOUNCEMENT_SCHEDULED_FOR(
                            formatDateTimeMinute(item.scheduled_for),
                          )}
                        </span>
                      ) : null}
                    </div>
                  </div>
                  {canUpdate && item.status !== "withdrawn" ? (
                    <div className="flex shrink-0 flex-wrap gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => setEditing(item)}
                      >
                        <FilePenLine className="size-4" aria-hidden="true" />
                        {TEXT.CONTEST.ANNOUNCEMENT_EDIT_ACTION}
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        onClick={() =>
                          setActionTarget({ item, action: "publish" })
                        }
                      >
                        <Send className="size-4" aria-hidden="true" />
                        {item.status === "published"
                          ? TEXT.CONTEST.ANNOUNCEMENT_PUBLISH_UPDATE_ACTION
                          : TEXT.CONTEST.ANNOUNCEMENT_PUBLISH_ACTION}
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="text-destructive hover:text-destructive"
                        onClick={() =>
                          setActionTarget({ item, action: "withdraw" })
                        }
                      >
                        <Undo2 className="size-4" aria-hidden="true" />
                        {TEXT.CONTEST.ANNOUNCEMENT_WITHDRAW_ACTION}
                      </Button>
                    </div>
                  ) : null}
                </div>
              </article>
            ))}
          </div>

          {(pageIndex > 0 || Boolean(list.data.next_cursor)) && (
            <nav
              className="flex items-center justify-between border-t pt-4"
              aria-label={TEXT.COMMON.PAGINATION}
            >
              <Button
                type="button"
                variant="outline"
                disabled={pageIndex === 0 || list.status === "loading"}
                onClick={previousPage}
              >
                <ChevronLeft className="size-4" aria-hidden="true" />
                {TEXT.CONTEST.JURY_LOAD_PREVIOUS}
              </Button>
              <span className="text-sm font-medium tabular-nums text-muted-foreground">
                {TEXT.CONTEST.JURY_PAGE(pageIndex + 1)}
              </span>
              <Button
                type="button"
                variant="outline"
                disabled={!list.data.next_cursor || list.status === "loading"}
                onClick={nextPage}
              >
                {TEXT.CONTEST.JURY_LOAD_NEXT}
                <ChevronRight className="size-4" aria-hidden="true" />
              </Button>
            </nav>
          )}
        </CardContent>
      </Card>

      <AnnouncementActionDialog
        contestID={contestID}
        target={actionTarget}
        onOpenChange={(open) => {
          if (!open) setActionTarget(null);
        }}
        onCompleted={() => {
          setActionTarget(null);
          setEditing(null);
          list.retry();
        }}
      />
    </div>
  );
}

function AnnouncementEditor({
  contestID,
  editing,
  canUpdate,
  onCancel,
  onSaved,
}: {
  contestID: string;
  editing: ContestAnnouncement | null;
  canUpdate: boolean;
  onCancel: () => void;
  onSaved: () => void;
}) {
  const [value, setValue] = useState<AnnouncementEditorValue>({
    audience: editing?.audience ?? "participants",
    title: editing?.title ?? "",
    markdown: editing?.markdown ?? "",
    scheduledForLocal: announcementScheduleLocal(editing?.scheduled_for),
    reason: "",
  });
  const [saving, setSaving] = useState(false);
  const valid = useMemo(() => announcementEditorIsValid(value), [value]);
  const history = useRetryableResource({
    resetKey: editing?.id ?? null,
    enabled: editing !== null,
    initialData: EMPTY_REVISIONS,
    load: (signal) =>
      contestContentService.announcementRevisions(
        contestID,
        editing?.id ?? "",
        signal,
      ),
  });

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canUpdate || saving || !valid) return;
    setSaving(true);
    try {
      const input = announcementEditorInput(value);
      if (editing) {
        await contestContentService.editAnnouncement(
          contestID,
          editing.id,
          editing.version,
          input,
        );
        notify.success(TEXT.CONTEST.ANNOUNCEMENT_UPDATE_SUCCESS);
      } else {
        await contestContentService.createAnnouncement(contestID, input);
        notify.success(TEXT.CONTEST.ANNOUNCEMENT_CREATE_SUCCESS);
      }
      onSaved();
    } catch (error) {
      notify.error(
        getErrorMessage(error, TEXT.CONTEST.ANNOUNCEMENT_SAVE_ERROR),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className="h-fit xl:sticky xl:top-24">
      <CardHeader className="border-b">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle>
              {editing
                ? TEXT.CONTEST.ANNOUNCEMENT_EDIT
                : TEXT.CONTEST.ANNOUNCEMENT_CREATE}
            </CardTitle>
            <CardDescription>
              {TEXT.CONTEST.ANNOUNCEMENT_EDITOR_DESCRIPTION}
            </CardDescription>
          </div>
          <div className="rounded-xl bg-primary/10 p-2.5 text-primary">
            {editing ? (
              <FilePenLine className="size-5" aria-hidden="true" />
            ) : (
              <Plus className="size-5" aria-hidden="true" />
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={save}>
          <div className="space-y-2">
            <Label htmlFor="announcement-audience">
              {TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_LABEL}
            </Label>
            <Select
              value={value.audience}
              onValueChange={(audience) =>
                setValue((current) => ({
                  ...current,
                  audience: audience as ContestAnnouncement["audience"],
                }))
              }
              disabled={!canUpdate || saving}
            >
              <SelectTrigger id="announcement-audience">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="public">
                  {TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_PUBLIC}
                </SelectItem>
                <SelectItem value="participants">
                  {TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_PARTICIPANTS}
                </SelectItem>
                <SelectItem value="jury">
                  {TEXT.CONTEST.ANNOUNCEMENT_AUDIENCE_JURY}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <BoundedField
            id="announcement-title"
            label={TEXT.CONTEST.ANNOUNCEMENT_TITLE_LABEL}
            value={value.title}
            maximumBytes={CONTEST_CONTENT_LIMITS.ANNOUNCEMENT_TITLE_BYTES}
          >
            <Input
              id="announcement-title"
              value={value.title}
              maxLength={CONTEST_CONTENT_LIMITS.ANNOUNCEMENT_TITLE_BYTES}
              disabled={!canUpdate || saving}
              placeholder={TEXT.CONTEST.ANNOUNCEMENT_TITLE_PLACEHOLDER}
              onChange={(event) =>
                setValue((current) => ({
                  ...current,
                  title: event.target.value,
                }))
              }
            />
          </BoundedField>

          <BoundedField
            id="announcement-markdown"
            label={TEXT.CONTEST.ANNOUNCEMENT_CONTENT_LABEL}
            value={value.markdown}
            maximumBytes={CONTEST_CONTENT_LIMITS.ANNOUNCEMENT_MARKDOWN_BYTES}
          >
            <Textarea
              id="announcement-markdown"
              value={value.markdown}
              maxLength={CONTEST_CONTENT_LIMITS.ANNOUNCEMENT_MARKDOWN_BYTES}
              disabled={!canUpdate || saving}
              className="min-h-44 resize-y font-mono text-sm"
              placeholder={TEXT.CONTEST.ANNOUNCEMENT_CONTENT_PLACEHOLDER}
              onChange={(event) =>
                setValue((current) => ({
                  ...current,
                  markdown: event.target.value,
                }))
              }
            />
          </BoundedField>

          <div className="space-y-2">
            <Label htmlFor="announcement-schedule">
              {TEXT.CONTEST.ANNOUNCEMENT_SCHEDULE_LABEL}
            </Label>
            <Input
              id="announcement-schedule"
              type="datetime-local"
              value={value.scheduledForLocal}
              disabled={!canUpdate || saving}
              onChange={(event) =>
                setValue((current) => ({
                  ...current,
                  scheduledForLocal: event.target.value,
                }))
              }
            />
            <p className="text-xs leading-5 text-muted-foreground">
              {TEXT.CONTEST.ANNOUNCEMENT_SCHEDULE_HELP}
            </p>
          </div>

          <BoundedField
            id="announcement-reason"
            label={TEXT.CONTEST.ANNOUNCEMENT_REASON_LABEL}
            value={value.reason}
            maximumBytes={CONTEST_CONTENT_LIMITS.REASON_BYTES}
          >
            <Textarea
              id="announcement-reason"
              value={value.reason}
              maxLength={CONTEST_CONTENT_LIMITS.REASON_BYTES}
              disabled={!canUpdate || saving}
              className="min-h-24 resize-y"
              placeholder={TEXT.CONTEST.ANNOUNCEMENT_REASON_PLACEHOLDER}
              onChange={(event) =>
                setValue((current) => ({
                  ...current,
                  reason: event.target.value,
                }))
              }
            />
          </BoundedField>

          <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            {editing ? (
              <Button
                type="button"
                variant="outline"
                disabled={saving}
                onClick={onCancel}
              >
                {TEXT.CONTEST.ANNOUNCEMENT_CANCEL_EDIT}
              </Button>
            ) : null}
            <Button
              type="submit"
              disabled={!canUpdate || !valid || saving}
            >
              {saving ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {saving
                ? TEXT.CONTEST.ANNOUNCEMENT_SAVING
                : editing
                  ? TEXT.CONTEST.ANNOUNCEMENT_SAVE_CHANGES
                  : TEXT.CONTEST.ANNOUNCEMENT_SAVE_DRAFT}
            </Button>
          </div>
        </form>
        {editing ? (
          <section className="mt-6 border-t pt-5">
            <div className="mb-3">
              <h3 className="flex items-center gap-2 font-semibold">
                <History className="size-4 text-primary" aria-hidden="true" />
                {TEXT.CONTEST.JURY_HISTORY}
              </h3>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {TEXT.CONTEST.ANNOUNCEMENT_HISTORY_DESCRIPTION}
              </p>
            </div>
            {history.status === "loading" ? (
              <div
                className="h-20 animate-pulse rounded-lg bg-muted motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : history.status === "error" ? (
              <div className="flex items-center justify-between gap-3" role="alert">
                <p className="text-xs text-muted-foreground">
                  {TEXT.CONTEST.JURY_HISTORY_ERROR}
                </p>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={history.retry}
                >
                  {TEXT.CONTEST.RETRY}
                </Button>
              </div>
            ) : (
              <div className="space-y-2">
                {history.data.map((revision) => (
                  <details
                    key={revision.id}
                    className="group rounded-lg border border-border bg-muted/20"
                  >
                    <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 px-3 text-sm font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
                      <span>
                        {TEXT.CONTEST.ANNOUNCEMENT_REVISION(
                          revision.revision_number,
                        )}
                      </span>
                      <span className="text-xs font-normal text-muted-foreground">
                        {formatDateTimeMinute(revision.created_at)}
                      </span>
                    </summary>
                    <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-words border-t bg-background p-3 font-mono text-xs leading-5 text-muted-foreground">
                      {revision.markdown}
                    </pre>
                  </details>
                ))}
              </div>
            )}
          </section>
        ) : null}
      </CardContent>
    </Card>
  );
}

function BoundedField({
  id,
  label,
  value,
  maximumBytes,
  children,
}: {
  id: string;
  label: string;
  value: string;
  maximumBytes: number;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <Label htmlFor={id}>{label}</Label>
        <span className="text-xs tabular-nums text-muted-foreground">
          {TEXT.CONTEST.FORM_BYTES_USED(utf8Bytes(value), maximumBytes)}
        </span>
      </div>
      {children}
    </div>
  );
}

function AnnouncementActionDialog({
  contestID,
  target,
  onOpenChange,
  onCompleted,
}: {
  contestID: string;
  target: { item: ContestAnnouncement; action: AnnouncementAction } | null;
  onOpenChange: (open: boolean) => void;
  onCompleted: () => void;
}) {
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const valid =
    reason.trim().length > 0 &&
    utf8Bytes(reason.trim()) <= CONTEST_CONTENT_LIMITS.REASON_BYTES;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!target || saving || !valid) return;
    setSaving(true);
    try {
      if (target.action === "publish") {
        await contestContentService.publishAnnouncement(
          contestID,
          target.item.id,
          target.item.version,
          reason.trim(),
        );
        notify.success(TEXT.CONTEST.ANNOUNCEMENT_PUBLISH_SUCCESS);
      } else {
        await contestContentService.withdrawAnnouncement(
          contestID,
          target.item.id,
          target.item.version,
          reason.trim(),
        );
        notify.success(TEXT.CONTEST.ANNOUNCEMENT_WITHDRAW_SUCCESS);
      }
      setReason("");
      onCompleted();
    } catch (error) {
      notify.error(
        getErrorMessage(error, TEXT.CONTEST.ANNOUNCEMENT_ACTION_ERROR),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog
      open={target !== null}
      onOpenChange={(open) => {
        if (!open && !saving) {
          setReason("");
          onOpenChange(false);
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {TEXT.CONTEST.ANNOUNCEMENT_ACTION_REASON_TITLE}
          </DialogTitle>
          <DialogDescription>
            {TEXT.CONTEST.ANNOUNCEMENT_ACTION_REASON_DESCRIPTION}
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={submit}>
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="announcement-action-reason">
                {TEXT.CONTEST.ANNOUNCEMENT_REASON_LABEL}
              </Label>
              <span className="text-xs tabular-nums text-muted-foreground">
                {TEXT.CONTEST.FORM_BYTES_USED(
                  utf8Bytes(reason),
                  CONTEST_CONTENT_LIMITS.REASON_BYTES,
                )}
              </span>
            </div>
            <Textarea
              id="announcement-action-reason"
              value={reason}
              maxLength={CONTEST_CONTENT_LIMITS.REASON_BYTES}
              disabled={saving}
              className="min-h-28"
              placeholder={TEXT.CONTEST.ANNOUNCEMENT_REASON_PLACEHOLDER}
              onChange={(event) => setReason(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={saving}
              onClick={() => onOpenChange(false)}
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button
              type="submit"
              variant={
                target?.action === "withdraw" ? "destructive" : "default"
              }
              disabled={!valid || saving}
            >
              {saving ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {target?.action === "withdraw"
                ? TEXT.CONTEST.ANNOUNCEMENT_WITHDRAW_ACTION
                : TEXT.CONTEST.ANNOUNCEMENT_PUBLISH_ACTION}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function AnnouncementListSkeleton() {
  return (
    <div className="space-y-3" aria-hidden="true">
      {[0, 1, 2].map((item) => (
        <div
          key={item}
          className="h-28 animate-pulse rounded-xl border border-border bg-muted/60 motion-reduce:animate-none"
        />
      ))}
    </div>
  );
}
