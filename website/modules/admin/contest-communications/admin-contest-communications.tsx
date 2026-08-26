"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ArrowLeft, Inbox, Megaphone } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1,
  EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1,
  EVENT_CONTEST_CLARIFICATION_ANSWERED_V1,
  EVENT_CONTEST_CLARIFICATION_CLOSED_V1,
  EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1,
  EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1,
  contestStaffTopic,
} from "@/constants/realtime";
import { TEXT } from "@/constants/text";
import { APP_ROUTES } from "@/constants/routes";
import { useAuth } from "@/contexts/auth-context";
import { useRealtime } from "@/contexts/realtime-context";
import { parseContestInvalidationV1Envelope } from "@/lib/realtime/contest-event-schema";
import {
  REALTIME_TOPIC_STATE,
  type RealtimeEnvelope,
} from "@/lib/realtime/websocket-client";
import {
  AdminAccessLoading,
  AdminAccessState,
} from "@/modules/admin/admin-access-state";
import { AnnouncementManager } from "@/modules/admin/contest-communications/announcement-manager";
import { ClarificationWorkspace } from "@/modules/admin/contest-communications/clarification-workspace";

const CONTEST_COMMUNICATION_EVENTS = [
  EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1,
  EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1,
  EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1,
  EVENT_CONTEST_CLARIFICATION_ANSWERED_V1,
  EVENT_CONTEST_CLARIFICATION_CLOSED_V1,
  EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1,
] as const;

export function AdminContestCommunications({
  contestID,
}: {
  contestID: string;
}) {
  const {
    isAuthenticated,
    isLoading,
    bootstrapStatus,
    can,
    retryBootstrap,
  } = useAuth();
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onTopicStateChange,
  } = useRealtime();
  const [realtimeRevision, setRealtimeRevision] = useState(0);
  const canRead = can(
    AUTHORIZATION_RESOURCE.CONTEST,
    AUTHORIZATION_ACTION.READ,
  );
  const canUpdate = can(
    AUTHORIZATION_RESOURCE.CONTEST,
    AUTHORIZATION_ACTION.UPDATE,
  );

  useEffect(() => {
    if (!isAuthenticated || !canRead) return;
    const topic = contestStaffTopic(contestID);
    const allowedTopics = new Set([topic]);
    const invalidate = (
      payload: unknown,
      envelope: RealtimeEnvelope<unknown>,
    ) => {
      parseContestInvalidationV1Envelope(
        payload,
        envelope,
        contestID,
        allowedTopics,
      );
      setRealtimeRevision((current) => current + 1);
    };
    const removers = [
      subscribeToTopic(topic),
      ...CONTEST_COMMUNICATION_EVENTS.map((eventType) =>
        subscribe<unknown>(eventType, invalidate),
      ),
      onResyncRequired(() =>
        setRealtimeRevision((current) => current + 1),
      ),
      onTopicStateChange(topic, (state) => {
        if (state === REALTIME_TOPIC_STATE.SUBSCRIBED) {
          setRealtimeRevision((current) => current + 1);
        }
      }),
    ];
    return () => removers.forEach((remove) => remove());
  }, [
    canRead,
    contestID,
    isAuthenticated,
    onResyncRequired,
    onTopicStateChange,
    subscribe,
    subscribeToTopic,
  ]);

  if (isLoading) return <AdminAccessLoading />;
  if (bootstrapStatus === "error") {
    return (
      <AdminAccessState kind="unavailable" onRetry={retryBootstrap} />
    );
  }
  if (!isAuthenticated || !canRead) {
    return <AdminAccessState kind="forbidden" />;
  }

  return (
    <div className="mx-auto w-full max-w-[90rem] space-y-6">
      <header className="rounded-2xl border border-border bg-card px-5 py-5 shadow-sm sm:px-7">
        <Link
          href={APP_ROUTES.ADMIN_CONTESTS}
          className="inline-flex min-h-11 items-center gap-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:rounded-md focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          {TEXT.CONTEST.BACK_TO_LIST}
        </Link>
        <div className="mt-3 max-w-3xl">
          <h1 className="text-balance text-2xl font-bold tracking-tight sm:text-3xl">
            {TEXT.CONTEST.COMMUNICATIONS_TITLE}
          </h1>
          <p className="mt-2 text-pretty text-sm leading-6 text-muted-foreground">
            {TEXT.CONTEST.COMMUNICATIONS_SUBTITLE}
          </p>
        </div>
      </header>

      <Tabs defaultValue="announcements">
        <TabsList className="w-full justify-start overflow-x-auto sm:w-fit">
          <TabsTrigger value="announcements">
            <Megaphone className="size-4" aria-hidden="true" />
            {TEXT.CONTEST.COMMUNICATIONS_ANNOUNCEMENTS_TAB}
          </TabsTrigger>
          <TabsTrigger value="clarifications">
            <Inbox className="size-4" aria-hidden="true" />
            {TEXT.CONTEST.COMMUNICATIONS_CLARIFICATIONS_TAB}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="announcements" className="pt-3">
          <AnnouncementManager
            contestID={contestID}
            canUpdate={canUpdate}
            realtimeRevision={realtimeRevision}
          />
        </TabsContent>
        <TabsContent value="clarifications" className="pt-3">
          <ClarificationWorkspace
            contestID={contestID}
            canUpdate={canUpdate}
            realtimeRevision={realtimeRevision}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
