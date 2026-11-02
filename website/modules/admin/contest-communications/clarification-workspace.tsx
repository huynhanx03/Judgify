"use client";

import { FormEvent, useMemo, useState } from "react";
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  CircleHelp,
  Clock3,
  History,
  Inbox,
  Loader2,
  MessageSquareReply,
  RefreshCw,
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
import { CONTEST_CONTENT_LIMITS } from "@/constants/contest-content";
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  clarificationAnswerIsValid,
  utf8Bytes,
} from "@/lib/contest/content-input";
import { formatDateTimeMinute } from "@/lib/format";
import { getErrorMessage, notify } from "@/lib/toast";
import {
  contestContentService,
  type AnswerClarificationInput,
} from "@/services/contest-content.service";
import type {
  ContestClarification,
  ContestContentPage,
  ContestContentRevision,
} from "@/types/contest";

type ClarificationFilter = "all" | ContestClarification["status"];

const EMPTY_PAGE: ContestContentPage<ContestClarification> = { items: [] };
const EMPTY_REVISIONS: ContestContentRevision[] = [];

const STATUS_COPY: Record<ContestClarification["status"], string> = {
  open: TEXT.CONTEST.STATUS_OPEN,
  answered: TEXT.CONTEST.STATUS_ANSWERED,
  closed: TEXT.CONTEST.STATUS_CLOSED,
  withdrawn: TEXT.CONTEST.STATUS_WITHDRAWN,
};

const REVISION_KIND_COPY: Record<
  ContestContentRevision["content_kind"],
  string
> = {
  announcement_body: TEXT.CONTEST.JURY_HISTORY_KIND_ANNOUNCEMENT,
  question: TEXT.CONTEST.JURY_HISTORY_KIND_QUESTION,
  public_question_summary: TEXT.CONTEST.JURY_HISTORY_KIND_SUMMARY,
  answer: TEXT.CONTEST.JURY_HISTORY_KIND_ANSWER,
};

function statusVariant(
  status: ContestClarification["status"],
): "default" | "secondary" | "outline" {
  if (status === "open") return "default";
  if (status === "answered") return "secondary";
  return "outline";
}

export function ClarificationWorkspace({
  contestID,
  canUpdate,
  realtimeRevision,
}: {
  contestID: string;
  canUpdate: boolean;
  realtimeRevision: number;
}) {
  const [filter, setFilter] = useState<ClarificationFilter>("open");
  const [cursorStack, setCursorStack] = useState<(string | undefined)[]>([
    undefined,
  ]);
  const [pageIndex, setPageIndex] = useState(0);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const cursor = cursorStack[pageIndex];

  const inbox = useRetryableResource({
    resetKey:
      `${contestID}:${filter}:${pageIndex}:${cursor ?? "first"}:` +
      realtimeRevision,
    initialData: EMPTY_PAGE,
    keepPreviousData: true,
    load: (signal) =>
      contestContentService.adminClarifications(
        contestID,
        {
          ...(filter === "all" ? {} : { status: filter }),
          ...(cursor ? { cursor } : {}),
        },
        signal,
      ),
  });

  function changeFilter(value: ClarificationFilter) {
    setFilter(value);
    setCursorStack([undefined]);
    setPageIndex(0);
    setSelectedID(null);
  }

  function nextPage() {
    if (!inbox.data.next_cursor) return;
    setCursorStack((current) => {
      const next = current.slice(0, pageIndex + 1);
      next.push(inbox.data.next_cursor);
      return next;
    });
    setPageIndex((current) => current + 1);
    setSelectedID(null);
  }

  return (
    <div className="grid min-h-[36rem] gap-5 lg:grid-cols-[22rem_minmax(0,1fr)]">
      <Card className="h-fit lg:sticky lg:top-24">
        <CardHeader className="border-b">
          <div className="flex items-center gap-3">
            <div className="rounded-xl bg-primary/10 p-2.5 text-primary">
              <Inbox className="size-5" aria-hidden="true" />
            </div>
            <div>
              <CardTitle>{TEXT.CONTEST.JURY_INBOX}</CardTitle>
              <CardDescription>
                {TEXT.CONTEST.COMMUNICATIONS_CLARIFICATIONS_TAB}
              </CardDescription>
            </div>
          </div>
          <Select
            value={filter}
            onValueChange={(value) =>
              changeFilter(value as ClarificationFilter)
            }
          >
            <SelectTrigger aria-label={TEXT.CONTEST.JURY_FILTER_ALL}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {TEXT.CONTEST.JURY_FILTER_ALL}
              </SelectItem>
              <SelectItem value="open">
                {TEXT.CONTEST.JURY_FILTER_OPEN}
              </SelectItem>
              <SelectItem value="answered">
                {TEXT.CONTEST.STATUS_ANSWERED}
              </SelectItem>
              <SelectItem value="closed">
                {TEXT.CONTEST.STATUS_CLOSED}
              </SelectItem>
              <SelectItem value="withdrawn">
                {TEXT.CONTEST.STATUS_WITHDRAWN}
              </SelectItem>
            </SelectContent>
          </Select>
        </CardHeader>
        <CardContent className="space-y-3">
          {inbox.status === "loading" && inbox.data.items.length === 0 ? (
            <InboxSkeleton />
          ) : null}
          {inbox.status === "error" && inbox.data.items.length === 0 ? (
            <div
              className="flex min-h-48 flex-col items-center justify-center gap-3 text-center"
              role="alert"
            >
              <AlertTriangle
                className="size-6 text-destructive"
                aria-hidden="true"
              />
              <p className="text-sm text-muted-foreground">
                {TEXT.CONTEST.JURY_LOAD_ERROR}
              </p>
              <Button type="button" variant="outline" onClick={inbox.retry}>
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.CONTEST.RETRY}
              </Button>
            </div>
          ) : null}
          {inbox.status === "ready" && inbox.data.items.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-center">
              <CircleHelp
                className="size-7 text-muted-foreground"
                aria-hidden="true"
              />
              <p className="text-sm text-muted-foreground">
                {TEXT.CONTEST.JURY_INBOX_EMPTY}
              </p>
            </div>
          ) : null}
          <div className="space-y-2" aria-busy={inbox.status === "loading"}>
            {inbox.data.items.map((item) => (
              <button
                key={item.id}
                type="button"
                aria-pressed={selectedID === item.id}
                onClick={() => setSelectedID(item.id)}
                className="min-h-20 w-full rounded-xl border border-border bg-background p-3 text-left outline-none transition-colors hover:border-primary/30 hover:bg-muted/50 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-pressed:border-primary aria-pressed:bg-primary/5"
              >
                <span className="flex items-center justify-between gap-2">
                  <Badge variant={statusVariant(item.status)}>
                    {STATUS_COPY[item.status]}
                  </Badge>
                  <span className="text-xs tabular-nums text-muted-foreground">
                    {formatDateTimeMinute(item.created_at)}
                  </span>
                </span>
                <span className="mt-2 block truncate text-xs text-muted-foreground">
                  {item.contest_problem_id ?? item.id}
                </span>
              </button>
            ))}
          </div>
          {(pageIndex > 0 || Boolean(inbox.data.next_cursor)) && (
            <nav
              className="flex items-center justify-between border-t pt-3"
              aria-label={TEXT.COMMON.PAGINATION}
            >
              <Button
                type="button"
                size="icon"
                variant="outline"
                aria-label={TEXT.CONTEST.JURY_LOAD_PREVIOUS}
                disabled={pageIndex === 0 || inbox.status === "loading"}
                onClick={() => {
                  setPageIndex((current) => Math.max(0, current - 1));
                  setSelectedID(null);
                }}
              >
                <ChevronLeft className="size-4" aria-hidden="true" />
              </Button>
              <span className="text-sm tabular-nums text-muted-foreground">
                {TEXT.CONTEST.JURY_PAGE(pageIndex + 1)}
              </span>
              <Button
                type="button"
                size="icon"
                variant="outline"
                aria-label={TEXT.CONTEST.JURY_LOAD_NEXT}
                disabled={
                  !inbox.data.next_cursor || inbox.status === "loading"
                }
                onClick={nextPage}
              >
                <ChevronRight className="size-4" aria-hidden="true" />
              </Button>
            </nav>
          )}
        </CardContent>
      </Card>

      {selectedID ? (
        <ClarificationDetail
          key={selectedID}
          contestID={contestID}
          clarificationID={selectedID}
          canUpdate={canUpdate}
          realtimeRevision={realtimeRevision}
          onMutated={inbox.retry}
        />
      ) : (
        <Card className="flex min-h-80 items-center justify-center">
          <CardContent className="flex max-w-md flex-col items-center gap-4 text-center">
            <div className="rounded-2xl bg-muted p-4 text-muted-foreground">
              <MessageSquareReply className="size-7" aria-hidden="true" />
            </div>
            <div>
              <h2 className="font-semibold">{TEXT.CONTEST.JURY_DETAIL}</h2>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {TEXT.CONTEST.JURY_SELECT_PROMPT}
              </p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function ClarificationDetail({
  contestID,
  clarificationID,
  canUpdate,
  realtimeRevision,
  onMutated,
}: {
  contestID: string;
  clarificationID: string;
  canUpdate: boolean;
  realtimeRevision: number;
  onMutated: () => void;
}) {
  const detail = useRetryableResource({
    resetKey: `${clarificationID}:${realtimeRevision}`,
    initialData: null as ContestClarification | null,
    keepPreviousData: true,
    load: (signal) =>
      contestContentService.adminClarification(
        contestID,
        clarificationID,
        signal,
      ),
  });
  const history = useRetryableResource({
    resetKey: `history:${clarificationID}`,
    initialData: EMPTY_REVISIONS,
    keepPreviousData: true,
    load: (signal) =>
      contestContentService.revisions(contestID, clarificationID, signal),
  });

  function refreshAfterMutation() {
    detail.retry();
    history.retry();
    onMutated();
  }

  if (detail.status === "loading" && !detail.data) {
    return <DetailSkeleton />;
  }
  if (detail.status === "error" || !detail.data) {
    return (
      <Card className="flex min-h-80 items-center justify-center">
        <CardContent className="flex flex-col items-center gap-4 text-center">
          <AlertTriangle
            className="size-7 text-destructive"
            aria-hidden="true"
          />
          <p className="text-sm text-muted-foreground">
            {TEXT.CONTEST.JURY_LOAD_ERROR}
          </p>
          <Button type="button" variant="outline" onClick={detail.retry}>
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.CONTEST.RETRY}
          </Button>
        </CardContent>
      </Card>
    );
  }

  const clarification = detail.data;
  const canRespond =
    canUpdate &&
    (clarification.status === "open" ||
      clarification.status === "answered");

  return (
    <div className="min-w-0 space-y-5">
      <Card>
        <CardHeader className="border-b">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <CardTitle>{TEXT.CONTEST.JURY_QUESTION_LABEL}</CardTitle>
              <CardDescription className="mt-1 flex items-center gap-1.5">
                <Clock3 className="size-3.5" aria-hidden="true" />
                {TEXT.CONTEST.JURY_QUESTION_CREATED_AT(
                  formatDateTimeMinute(clarification.created_at),
                )}
              </CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="sr-only">
                {TEXT.CONTEST.JURY_STATUS_LABEL}
              </span>
              <Badge variant={statusVariant(clarification.status)}>
                {STATUS_COPY[clarification.status]}
              </Badge>
              <Badge variant="outline">
                {clarification.audience === "participants"
                  ? TEXT.CONTEST.JURY_AUDIENCE_PARTICIPANTS
                  : TEXT.CONTEST.JURY_AUDIENCE_REQUESTER}
              </Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {clarification.question_html ? (
            <div
              className="prose prose-sm max-w-none dark:prose-invert"
              dangerouslySetInnerHTML={{
                __html: clarification.question_html,
              }}
            />
          ) : (
            <p className="whitespace-pre-wrap text-sm leading-6">
              {clarification.question_markdown}
            </p>
          )}
        </CardContent>
      </Card>

      {canRespond ? (
        <ClarificationAnswerForm
          key={`${clarification.id}:${clarification.version}`}
          contestID={contestID}
          clarification={clarification}
          onCompleted={refreshAfterMutation}
        />
      ) : null}

      <Card>
        <CardHeader className="border-b">
          <CardTitle className="flex items-center gap-2">
            <History className="size-4 text-primary" aria-hidden="true" />
            {TEXT.CONTEST.JURY_HISTORY}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {history.status === "loading" && history.data.length === 0 ? (
            <div className="space-y-3" aria-hidden="true">
              <div className="h-20 animate-pulse rounded-lg bg-muted motion-reduce:animate-none" />
              <div className="h-20 animate-pulse rounded-lg bg-muted motion-reduce:animate-none" />
            </div>
          ) : history.status === "error" ? (
            <div className="flex items-center justify-between gap-4" role="alert">
              <p className="text-sm text-muted-foreground">
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
          ) : history.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {TEXT.CONTEST.JURY_NO_HISTORY}
            </p>
          ) : (
            <ol className="space-y-3">
              {history.data.map((revision) => (
                <li
                  key={revision.id}
                  className="rounded-xl border border-border bg-muted/20 p-4"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <Badge variant="outline">
                      {REVISION_KIND_COPY[revision.content_kind]}
                    </Badge>
                    <span className="text-xs tabular-nums text-muted-foreground">
                      {TEXT.CONTEST.ANNOUNCEMENT_REVISION(
                        revision.revision_number,
                      )}
                    </span>
                  </div>
                  <pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-background p-3 font-mono text-xs leading-5 text-muted-foreground">
                    {revision.markdown}
                  </pre>
                </li>
              ))}
            </ol>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function ClarificationAnswerForm({
  contestID,
  clarification,
  onCompleted,
}: {
  contestID: string;
  clarification: ContestClarification;
  onCompleted: () => void;
}) {
  const [audience, setAudience] =
    useState<AnswerClarificationInput["audience"]>(
      clarification.audience ?? "requester",
    );
  const [answer, setAnswer] = useState("");
  const [summary, setSummary] = useState("");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState<"answer" | "close" | null>(null);
  const normalized = useMemo<AnswerClarificationInput>(
    () => ({
      audience,
      answer_markdown: answer.trim(),
      ...(audience === "participants"
        ? { public_summary_markdown: summary.trim() }
        : {}),
      reason: reason.trim(),
    }),
    [answer, audience, reason, summary],
  );
  const answerValid = clarificationAnswerIsValid(normalized);
  const closeValid =
    reason.trim().length > 0 &&
    utf8Bytes(reason.trim()) <= CONTEST_CONTENT_LIMITS.REASON_BYTES;

  async function answerQuestion(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving || !answerValid) return;
    setSaving("answer");
    try {
      await contestContentService.answerClarification(
        contestID,
        clarification.id,
        clarification.version,
        normalized,
      );
      notify.success(TEXT.CONTEST.JURY_SAVE_SUCCESS);
      onCompleted();
    } catch (error) {
      notify.error(getErrorMessage(error, TEXT.CONTEST.JURY_SAVE_ERROR));
      onCompleted();
    } finally {
      setSaving(null);
    }
  }

  async function closeQuestion() {
    if (saving || !closeValid) return;
    setSaving("close");
    try {
      await contestContentService.closeClarification(
        contestID,
        clarification.id,
        clarification.version,
        reason.trim(),
      );
      notify.success(TEXT.CONTEST.JURY_CLOSE_SUCCESS);
      onCompleted();
    } catch (error) {
      notify.error(getErrorMessage(error, TEXT.CONTEST.JURY_CLOSE_ERROR));
      onCompleted();
    } finally {
      setSaving(null);
    }
  }

  return (
    <Card>
      <CardHeader className="border-b">
        <CardTitle>{TEXT.CONTEST.JURY_ANSWER}</CardTitle>
        <CardDescription>
          {TEXT.CONTEST.ANNOUNCEMENT_ACTION_REASON_DESCRIPTION}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={answerQuestion}>
          <div className="space-y-2">
            <Label htmlFor="clarification-audience">
              {TEXT.CONTEST.JURY_AUDIENCE_LABEL}
            </Label>
            <Select
              value={audience}
              disabled={saving !== null}
              onValueChange={(value) =>
                setAudience(value as AnswerClarificationInput["audience"])
              }
            >
              <SelectTrigger id="clarification-audience">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="requester">
                  {TEXT.CONTEST.JURY_AUDIENCE_REQUESTER}
                </SelectItem>
                <SelectItem value="participants">
                  {TEXT.CONTEST.JURY_AUDIENCE_PARTICIPANTS}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          {audience === "participants" ? (
            <ResponseField
              id="clarification-summary"
              label={TEXT.CONTEST.JURY_SUMMARY_LABEL}
              value={summary}
              maximumBytes={
                CONTEST_CONTENT_LIMITS.CLARIFICATION_SUMMARY_BYTES
              }
              disabled={saving !== null}
              onChange={setSummary}
            />
          ) : null}

          <ResponseField
            id="clarification-answer"
            label={TEXT.CONTEST.JURY_ANSWER_LABEL}
            value={answer}
            maximumBytes={CONTEST_CONTENT_LIMITS.CLARIFICATION_ANSWER_BYTES}
            disabled={saving !== null}
            onChange={setAnswer}
          />

          <ResponseField
            id="clarification-reason"
            label={TEXT.CONTEST.JURY_REASON_LABEL}
            value={reason}
            maximumBytes={CONTEST_CONTENT_LIMITS.REASON_BYTES}
            disabled={saving !== null}
            onChange={setReason}
          />

          <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <Button
              type="button"
              variant="outline"
              disabled={!closeValid || saving !== null}
              onClick={closeQuestion}
            >
              {saving === "close" ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {TEXT.CONTEST.JURY_CLOSE}
            </Button>
            <Button
              type="submit"
              disabled={!answerValid || saving !== null}
            >
              {saving === "answer" ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {TEXT.CONTEST.JURY_ANSWER}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function ResponseField({
  id,
  label,
  value,
  maximumBytes,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  maximumBytes: number;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <Label htmlFor={id}>{label}</Label>
        <span className="text-xs tabular-nums text-muted-foreground">
          {TEXT.CONTEST.FORM_BYTES_USED(utf8Bytes(value), maximumBytes)}
        </span>
      </div>
      <Textarea
        id={id}
        value={value}
        maxLength={maximumBytes}
        disabled={disabled}
        className="min-h-28 resize-y"
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function InboxSkeleton() {
  return (
    <div className="space-y-2" aria-hidden="true">
      {[0, 1, 2, 3].map((item) => (
        <div
          key={item}
          className="h-20 animate-pulse rounded-xl border border-border bg-muted motion-reduce:animate-none"
        />
      ))}
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="space-y-5" aria-hidden="true">
      <div className="h-52 animate-pulse rounded-xl border border-border bg-muted motion-reduce:animate-none" />
      <div className="h-80 animate-pulse rounded-xl border border-border bg-muted motion-reduce:animate-none" />
    </div>
  );
}
