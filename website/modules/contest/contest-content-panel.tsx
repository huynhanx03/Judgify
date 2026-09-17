"use client";

import { useEffect, useState, type FormEvent } from "react";
import {
  Bell,
  CircleAlert,
  CircleHelp,
  Loader2,
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
  DialogTrigger,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { CONTEST_CONTENT_LIMITS } from "@/constants/contest-content";
import {
  EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1,
  EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1,
  EVENT_CONTEST_CLARIFICATION_ANSWERED_V1,
  EVENT_CONTEST_CLARIFICATION_CLOSED_V1,
  EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1,
  EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1,
  contestParticipantsTopic,
  contestPrivateUserTopic,
  contestPublicTopic,
} from "@/constants/realtime";
import { TEXT } from "@/constants/text";
import { useRealtime } from "@/contexts/realtime-context";
import { useSession } from "@/contexts/session-context";
import {
  useCursorFeed,
  type CursorFeedStatus,
} from "@/hooks/use-cursor-feed";
import { formatDateTime } from "@/lib/format";
import {
  clarificationQuestionIsValid,
  contestContentReasonIsValid,
  utf8Bytes,
} from "@/lib/contest/content-input";
import { parseContestInvalidationV1Envelope } from "@/lib/realtime/contest-event-schema";
import {
  REALTIME_TOPIC_STATE,
  type RealtimeEnvelope,
} from "@/lib/realtime/websocket-client";
import { notify } from "@/lib/toast";
import { contestContentService } from "@/services/contest-content.service";
import type {
  ContestAnnouncement,
  ContestClarification,
} from "@/types/contest";

const ANNOUNCEMENT_EVENTS = new Set([
  EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1,
  EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1,
]);

const CLARIFICATION_EVENTS = new Set([
  EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1,
  EVENT_CONTEST_CLARIFICATION_ANSWERED_V1,
  EVENT_CONTEST_CLARIFICATION_CLOSED_V1,
  EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1,
]);

const CONTENT_EVENTS = [
  ...ANNOUNCEMENT_EVENTS,
  ...CLARIFICATION_EVENTS,
] as const;

function contentIdentity(
  item: ContestAnnouncement | ContestClarification,
): string {
  return item.id;
}

function sanitizedMarkup(html: string) {
  return { __html: html };
}

function clarificationStatusLabel(
  status: ContestClarification["status"],
): string {
  return {
    open: TEXT.CONTEST.STATUS_OPEN,
    answered: TEXT.CONTEST.STATUS_ANSWERED,
    closed: TEXT.CONTEST.STATUS_CLOSED,
    withdrawn: TEXT.CONTEST.STATUS_WITHDRAWN,
  }[status];
}

interface ContestContentPanelProps {
  contestID: string;
  isParticipant: boolean;
  canSubmit: boolean;
}

export function ContestContentPanel({
  contestID,
  isParticipant,
  canSubmit,
}: ContestContentPanelProps) {
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onTopicStateChange,
  } = useRealtime();
  const session = useSession();
  const [question, setQuestion] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const sessionUserID =
    session.state.status === "authenticated"
      ? session.state.session.user.id
      : null;

  const announcements = useCursorFeed({
    resetKey: `${contestID}:announcements:${isParticipant}`,
    load: (cursor, signal) =>
      contestContentService.announcements(
        contestID,
        isParticipant,
        cursor ? { cursor } : {},
        signal,
      ),
    keyOf: contentIdentity,
  });
  const sharedClarifications = useCursorFeed({
    resetKey: `${contestID}:shared-clarifications`,
    enabled: isParticipant,
    load: (cursor, signal) =>
      contestContentService.participantClarifications(
        contestID,
        cursor ? { cursor } : {},
        signal,
      ),
    keyOf: contentIdentity,
  });
  const ownClarifications = useCursorFeed({
    resetKey: `${contestID}:own-clarifications`,
    enabled: isParticipant,
    load: (cursor, signal) =>
      contestContentService.myClarifications(
        contestID,
        cursor ? { cursor } : {},
        signal,
      ),
    keyOf: contentIdentity,
  });
  const reloadAnnouncements = announcements.reload;
  const reloadOwnClarifications = ownClarifications.reload;
  const reloadSharedClarifications = sharedClarifications.reload;

  useEffect(() => {
    const publicTopic = contestPublicTopic(contestID);
    const participantTopic = isParticipant
      ? contestParticipantsTopic(contestID)
      : null;
    const privateTopic =
      isParticipant && sessionUserID
        ? contestPrivateUserTopic(contestID, sessionUserID)
        : null;
    const topics = [
      publicTopic,
      ...(participantTopic ? [participantTopic] : []),
      ...(privateTopic ? [privateTopic] : []),
    ];
    const allowedTopics = new Set(topics);
    const invalidate = (
      payload: unknown,
      envelope: RealtimeEnvelope<unknown>,
    ) => {
      const event = parseContestInvalidationV1Envelope(
        payload,
        envelope,
        contestID,
        allowedTopics,
      );
      if (event.resource === "announcement") reloadAnnouncements();
      if (event.resource === "clarification") {
        reloadSharedClarifications();
        reloadOwnClarifications();
      }
    };

    const removers = topics.map((topic) => subscribeToTopic(topic));
    removers.push(
      ...CONTENT_EVENTS.map((eventType) =>
        subscribe<unknown>(eventType, invalidate),
      ),
      onResyncRequired(() => {
        reloadAnnouncements();
        reloadSharedClarifications();
        reloadOwnClarifications();
      }),
      ...topics.map((topic) =>
        onTopicStateChange(topic, (state) => {
          if (state !== REALTIME_TOPIC_STATE.SUBSCRIBED) return;
          if (topic === publicTopic || topic === participantTopic) {
            reloadAnnouncements();
          }
          if (topic === participantTopic) reloadSharedClarifications();
          if (topic === privateTopic) reloadOwnClarifications();
        }),
      ),
    );

    return () => removers.forEach((remove) => remove());
  }, [
    contestID,
    isParticipant,
    onResyncRequired,
    onTopicStateChange,
    reloadAnnouncements,
    reloadOwnClarifications,
    reloadSharedClarifications,
    sessionUserID,
    subscribe,
    subscribeToTopic,
  ]);

  const normalizedQuestion = question.trim();
  const questionIsValid = clarificationQuestionIsValid(question);

  async function submitQuestion(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!questionIsValid || submitting) return;

    setSubmitting(true);
    try {
      await contestContentService.submitClarification(
        contestID,
        normalizedQuestion,
      );
      setQuestion("");
      ownClarifications.reload();
      notify.success(TEXT.CONTEST.QUESTION_SENT);
    } catch {
      notify.error(TEXT.CONTEST.QUESTION_SEND_ERROR);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <ContentSection
        icon={<Bell className="size-5" aria-hidden="true" />}
        title={TEXT.CONTEST.ANNOUNCEMENTS}
        status={announcements.status}
        hasItems={announcements.items.length > 0}
        onRefresh={announcements.reload}
      >
        {announcements.items.length === 0 &&
        announcements.status !== "loading" ? (
          <EmptyFeed>{TEXT.CONTEST.NO_ANNOUNCEMENTS}</EmptyFeed>
        ) : (
          <div className="space-y-3">
            {announcements.items.map((announcement) => (
              <AnnouncementCard
                key={announcement.id}
                announcement={announcement}
              />
            ))}
          </div>
        )}
        <FeedPagination
          status={announcements.status}
          hasItems={announcements.items.length > 0}
          hasMore={announcements.hasMore}
          onLoadMore={announcements.loadMore}
        />
      </ContentSection>

      {isParticipant ? (
        <>
          {canSubmit ? (
            <section
              aria-labelledby="contest-submit-question"
              className="rounded-2xl border border-primary/15 bg-gradient-to-br from-primary/[0.08] via-card to-card p-5 shadow-sm sm:p-6"
            >
              <div className="mb-4 flex items-start gap-3">
                <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <CircleHelp className="size-5" aria-hidden="true" />
                </span>
                <div>
                  <h2
                    id="contest-submit-question"
                    className="text-lg font-bold tracking-tight"
                  >
                    {TEXT.CONTEST.ASK_QUESTION}
                  </h2>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">
                    {TEXT.CONTEST.QUESTION_PLACEHOLDER}
                  </p>
                </div>
              </div>
              <form onSubmit={submitQuestion} className="space-y-3">
                <label
                  className="text-sm font-medium"
                  htmlFor="contest-question"
                >
                  {TEXT.CONTEST.QUESTION_LABEL}
                </label>
                <Textarea
                  id="contest-question"
                  value={question}
                  maxLength={
                    CONTEST_CONTENT_LIMITS.CLARIFICATION_QUESTION_BYTES
                  }
                  onChange={(event) => setQuestion(event.target.value)}
                  placeholder={TEXT.CONTEST.QUESTION_PLACEHOLDER}
                  className="min-h-32 resize-y bg-background/85"
                  aria-invalid={question.length > 0 && !questionIsValid}
                  required
                />
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="text-xs tabular-nums text-muted-foreground">
                    {TEXT.CONTEST.FORM_BYTES_USED(
                      utf8Bytes(normalizedQuestion),
                      CONTEST_CONTENT_LIMITS.CLARIFICATION_QUESTION_BYTES,
                    )}
                    {question.length > 0 && !questionIsValid ? (
                      <span className="ml-2 text-destructive">
                        {TEXT.CONTEST.QUESTION_INPUT_INVALID}
                      </span>
                    ) : null}
                  </div>
                  <Button
                    type="submit"
                    disabled={submitting || !questionIsValid}
                  >
                    {submitting ? (
                      <Loader2
                        className="size-4 animate-spin motion-reduce:animate-none"
                        aria-hidden="true"
                      />
                    ) : (
                      <Send className="size-4" aria-hidden="true" />
                    )}
                    {submitting
                      ? TEXT.CONTEST.SENDING_QUESTION
                      : TEXT.CONTEST.SEND_QUESTION}
                  </Button>
                </div>
              </form>
            </section>
          ) : null}

          <ClarificationFeed
            title={TEXT.CONTEST.MY_QUESTIONS}
            empty={TEXT.CONTEST.NO_OWN_CLARIFICATIONS}
            feed={ownClarifications}
            own
            contestID={contestID}
          />
          <ClarificationFeed
            title={TEXT.CONTEST.CLARIFICATIONS}
            empty={TEXT.CONTEST.NO_CLARIFICATIONS}
            feed={sharedClarifications}
            contestID={contestID}
          />
        </>
      ) : null}
    </div>
  );
}

interface ContentSectionProps {
  icon: React.ReactNode;
  title: string;
  status: CursorFeedStatus;
  hasItems: boolean;
  onRefresh: () => void;
  children: React.ReactNode;
}

function ContentSection({
  icon,
  title,
  status,
  hasItems,
  onRefresh,
  children,
}: ContentSectionProps) {
  const firstLoad = status === "loading" && !hasItems;
  const refreshing = status === "refreshing";

  return (
    <section aria-labelledby={`content-${title}`}>
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-primary">
          {icon}
          <h2
            id={`content-${title}`}
            className="text-lg font-bold tracking-tight text-foreground"
          >
            {title}
          </h2>
        </div>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onRefresh}
          disabled={firstLoad || refreshing}
        >
          <RefreshCw
            className={refreshing ? "size-4 animate-spin motion-reduce:animate-none" : "size-4"}
            aria-hidden="true"
          />
          {refreshing
            ? TEXT.CONTEST.CONTENT_REFRESHING
            : TEXT.CONTEST.CONTENT_REFRESH}
        </Button>
      </div>

      {firstLoad ? <FeedLoading /> : null}
      {status === "error" && !hasItems ? (
        <FeedError onRetry={onRefresh} />
      ) : null}
      {status === "error" && hasItems ? <StaleFeedNotice /> : null}
      {!firstLoad && (status !== "error" || hasItems) ? children : null}
    </section>
  );
}

function AnnouncementCard({
  announcement,
}: {
  announcement: ContestAnnouncement;
}) {
  const timestamp = announcement.published_at ?? announcement.updated_at;

  return (
    <Card className="gap-3 border-border/70 py-0 shadow-sm transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-primary/25 hover:shadow-md">
      <CardHeader className="gap-2 px-5 pt-5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Badge variant="secondary">{TEXT.CONTEST.ANNOUNCEMENTS}</Badge>
          <time
            dateTime={timestamp}
            className="text-xs tabular-nums text-muted-foreground"
          >
            {TEXT.CONTEST.ANNOUNCEMENT_PUBLISHED_AT(
              formatDateTime(timestamp),
            )}
          </time>
        </div>
        <CardTitle className="text-lg font-bold tracking-tight">
          {announcement.title}
        </CardTitle>
      </CardHeader>
      <CardContent className="px-5 pb-5">
        <div
          className="prose prose-sm max-w-none text-pretty dark:prose-invert"
          dangerouslySetInnerHTML={sanitizedMarkup(
            announcement.sanitized_html,
          )}
        />
      </CardContent>
    </Card>
  );
}

interface ClarificationFeedProps {
  title: string;
  empty: string;
  feed: ReturnType<typeof useCursorFeed<ContestClarification>>;
  contestID: string;
  own?: boolean;
}

function ClarificationFeed({
  title,
  empty,
  feed,
  contestID,
  own = false,
}: ClarificationFeedProps) {
  return (
    <ContentSection
      icon={<CircleHelp className="size-5" aria-hidden="true" />}
      title={title}
      status={feed.status}
      hasItems={feed.items.length > 0}
      onRefresh={feed.reload}
    >
      {feed.items.length === 0 && feed.status !== "loading" ? (
        <EmptyFeed>{empty}</EmptyFeed>
      ) : (
        <div className="space-y-3">
          {feed.items.map((clarification) => (
            <ClarificationCard
              key={clarification.id}
              clarification={clarification}
              contestID={contestID}
              own={own}
              onWithdrawn={feed.reload}
            />
          ))}
        </div>
      )}
      <FeedPagination
        status={feed.status}
        hasItems={feed.items.length > 0}
        hasMore={feed.hasMore}
        onLoadMore={feed.loadMore}
      />
    </ContentSection>
  );
}

interface ClarificationCardProps {
  clarification: ContestClarification;
  contestID: string;
  own: boolean;
  onWithdrawn: () => void;
}

function ClarificationCard({
  clarification,
  contestID,
  own,
  onWithdrawn,
}: ClarificationCardProps) {
  return (
    <Card className="gap-3 border-border/70 py-0">
      <CardHeader className="gap-3 px-5 pt-5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Badge
            variant={
              clarification.status === "open" ? "secondary" : "outline"
            }
          >
            {clarificationStatusLabel(clarification.status)}
          </Badge>
          <time
            dateTime={clarification.created_at}
            className="text-xs tabular-nums text-muted-foreground"
          >
            {TEXT.CONTEST.QUESTION_CREATED_AT(
              formatDateTime(clarification.created_at),
            )}
          </time>
        </div>
        <CardDescription>
          {clarification.audience === "participants"
            ? TEXT.CONTEST.SHARED_WITH_PARTICIPANTS
            : TEXT.CONTEST.PRIVATE}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 px-5 pb-5">
        {clarification.question_html ? (
          <div
            className="prose prose-sm max-w-none text-pretty dark:prose-invert"
            dangerouslySetInnerHTML={sanitizedMarkup(
              clarification.question_html,
            )}
          />
        ) : null}
        {clarification.public_summary_html ? (
          <ClarificationBlock
            title={TEXT.CONTEST.SUMMARY}
            html={clarification.public_summary_html}
          />
        ) : null}
        {clarification.answer_html ? (
          <ClarificationBlock
            title={TEXT.CONTEST.ANSWER}
            html={clarification.answer_html}
            emphasized
          />
        ) : null}
        {own && clarification.status === "open" ? (
          <div className="flex justify-end border-t border-border/60 pt-4">
            <WithdrawClarificationDialog
              contestID={contestID}
              clarification={clarification}
              onWithdrawn={onWithdrawn}
            />
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

function ClarificationBlock({
  title,
  html,
  emphasized = false,
}: {
  title: string;
  html: string;
  emphasized?: boolean;
}) {
  return (
    <div
      className={
        emphasized
          ? "rounded-xl border border-primary/15 bg-primary/[0.06] p-4"
          : "rounded-xl border border-border/60 bg-muted/35 p-4"
      }
    >
      <h3 className="mb-2 text-sm font-semibold">{title}</h3>
      <div
        className="prose prose-sm max-w-none text-pretty dark:prose-invert"
        dangerouslySetInnerHTML={sanitizedMarkup(html)}
      />
    </div>
  );
}

function WithdrawClarificationDialog({
  contestID,
  clarification,
  onWithdrawn,
}: {
  contestID: string;
  clarification: ContestClarification;
  onWithdrawn: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const reasonIsValid = contestContentReasonIsValid(reason);

  async function withdraw() {
    if (!reasonIsValid || submitting) return;
    setSubmitting(true);
    try {
      await contestContentService.withdrawClarification(
        contestID,
        clarification.id,
        clarification.version,
        reason.trim(),
      );
      setOpen(false);
      setReason("");
      onWithdrawn();
      notify.success(TEXT.CONTEST.QUESTION_WITHDRAW_SUCCESS);
    } catch {
      notify.error(TEXT.CONTEST.QUESTION_WITHDRAW_ERROR);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (submitting) return;
        setOpen(nextOpen);
        if (!nextOpen) setReason("");
      }}
    >
      <DialogTrigger render={<Button type="button" size="sm" variant="ghost" />}>
        <Undo2 className="size-4" aria-hidden="true" />
        {TEXT.CONTEST.QUESTION_WITHDRAW}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{TEXT.CONTEST.QUESTION_WITHDRAW_TITLE}</DialogTitle>
          <DialogDescription>
            {TEXT.CONTEST.QUESTION_WITHDRAW_DESCRIPTION}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <label
            className="text-sm font-medium"
            htmlFor={`withdraw-reason-${clarification.id}`}
          >
            {TEXT.CONTEST.QUESTION_WITHDRAW_REASON_LABEL}
          </label>
          <Textarea
            id={`withdraw-reason-${clarification.id}`}
            value={reason}
            maxLength={CONTEST_CONTENT_LIMITS.REASON_BYTES}
            onChange={(event) => setReason(event.target.value)}
            placeholder={TEXT.CONTEST.QUESTION_WITHDRAW_REASON_PLACEHOLDER}
            className="min-h-24"
            aria-invalid={reason.length > 0 && !reasonIsValid}
          />
          <p className="text-xs tabular-nums text-muted-foreground">
            {TEXT.CONTEST.FORM_BYTES_USED(
              utf8Bytes(reason.trim()),
              CONTEST_CONTENT_LIMITS.REASON_BYTES,
            )}
          </p>
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="destructive"
            disabled={!reasonIsValid || submitting}
            onClick={withdraw}
          >
            {submitting ? (
              <Loader2
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : (
              <Undo2 className="size-4" aria-hidden="true" />
            )}
            {submitting
              ? TEXT.CONTEST.QUESTION_WITHDRAWING
              : TEXT.CONTEST.QUESTION_WITHDRAW}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FeedPagination({
  status,
  hasItems,
  hasMore,
  onLoadMore,
}: {
  status: CursorFeedStatus;
  hasItems: boolean;
  hasMore: boolean;
  onLoadMore: () => void | Promise<void>;
}) {
  if (!hasItems) return null;
  const loadingMore = status === "loading_more";

  return (
    <div className="mt-4 flex justify-center">
      {hasMore ? (
        <Button
          type="button"
          variant="outline"
          onClick={() => void onLoadMore()}
          disabled={loadingMore}
        >
          {loadingMore ? (
            <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
          ) : null}
          {loadingMore
            ? TEXT.CONTEST.CONTENT_LOADING_MORE
            : TEXT.CONTEST.CONTENT_LOAD_MORE}
        </Button>
      ) : (
        <p className="text-xs text-muted-foreground">
          {TEXT.CONTEST.CONTENT_END}
        </p>
      )}
    </div>
  );
}

function FeedLoading() {
  return (
    <div
      className="flex min-h-28 items-center justify-center rounded-xl border border-dashed border-border bg-muted/20 text-sm text-muted-foreground"
      role="status"
    >
      <Loader2 className="mr-2 size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
      {TEXT.CONTEST.CONTENT_LOADING}
    </div>
  );
}

function FeedError({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex flex-col items-center gap-3 rounded-xl border border-destructive/20 bg-destructive/[0.06] p-6 text-center"
      role="alert"
    >
      <CircleAlert className="size-5 text-destructive" aria-hidden="true" />
      <p className="text-sm text-muted-foreground">
        {TEXT.CONTEST.CONTENT_ERROR}
      </p>
      <Button type="button" size="sm" variant="outline" onClick={onRetry}>
        <RefreshCw className="size-4" aria-hidden="true" />
        {TEXT.CONTEST.RETRY}
      </Button>
    </div>
  );
}

function StaleFeedNotice() {
  return (
    <div
      className="mb-3 flex items-start gap-2 rounded-lg border border-warning/20 bg-warning/[0.08] px-3 py-2 text-xs text-muted-foreground"
      role="status"
    >
      <CircleAlert
        className="mt-0.5 size-4 shrink-0 text-warning"
        aria-hidden="true"
      />
      {TEXT.CONTEST.CONTENT_REFRESH_ERROR}
    </div>
  );
}

function EmptyFeed({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-border bg-muted/20 px-5 py-10 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
