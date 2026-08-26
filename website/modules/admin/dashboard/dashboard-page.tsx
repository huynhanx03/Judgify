"use client";

/** Admin overview that only requests and renders capability-permitted stats. */

import { useMemo } from "react";
import { AlertCircle, FileCode2, Loader2, Shield, Tags, Trophy, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/contexts/auth-context";
import { filterAuthorizedItems, type CapabilityBoundItem } from "@/lib/auth/admin-policy";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { contestService } from "@/services/contest.service";
import { problemService } from "@/services/problem.service";
import { roleService } from "@/services/role.service";
import { tagService } from "@/services/tag.service";
import { userService } from "@/services/user.service";
import { AdminRecentAudit } from "@/modules/admin/audit/admin-recent-audit";
import { useRetryableResource } from "@/hooks/use-retryable-resource";

type StatKey = "totalContests" | "totalProblems" | "totalTags" | "totalRoles" | "totalUsers";
type DashboardStats = Partial<Record<StatKey, number>>;

interface DashboardProjection {
  stats: DashboardStats;
  failedStats: ReadonlySet<StatKey>;
}

interface StatCardDefinition extends CapabilityBoundItem {
  key: StatKey;
  label: string;
  icon: typeof Users;
  color: string;
  load: (signal: AbortSignal) => Promise<number>;
}

const countQuery = { pagination: { page: 1, page_size: 1 } };
const EMPTY_DASHBOARD: DashboardProjection = {
  stats: {},
  failedStats: new Set(),
};

const STAT_CARDS: readonly StatCardDefinition[] = [
  {
    key: "totalUsers",
    label: ADMIN_TEXT.STAT_USERS,
    icon: Users,
    color: "text-primary",
    requirements: [
      { resource: AUTHORIZATION_RESOURCE.USER, action: AUTHORIZATION_ACTION.READ },
    ],
    load: async (signal) =>
      (await userService.find(countQuery, signal)).pagination.total_items,
  },
  {
    key: "totalProblems",
    label: ADMIN_TEXT.STAT_PROBLEMS,
    icon: FileCode2,
    color: "text-success",
    requirements: [
      { resource: AUTHORIZATION_RESOURCE.PROBLEM, action: AUTHORIZATION_ACTION.READ },
    ],
    load: async (signal) =>
      (await problemService.findAdmin(countQuery, signal)).pagination.total_items,
  },
  {
    key: "totalContests",
    label: ADMIN_TEXT.STAT_CONTESTS,
    icon: Trophy,
    color: "text-info",
    requirements: [
      { resource: AUTHORIZATION_RESOURCE.CONTEST, action: AUTHORIZATION_ACTION.READ },
    ],
    load: async (signal) =>
      (await contestService.findAdmin(countQuery, signal)).pagination.total_items,
  },
  {
    key: "totalTags",
    label: ADMIN_TEXT.STAT_TAGS,
    icon: Tags,
    color: "text-cultivation",
    requirements: [
      { resource: AUTHORIZATION_RESOURCE.TAG, action: AUTHORIZATION_ACTION.READ },
    ],
    load: async (signal) =>
      (await tagService.find(countQuery, signal)).pagination.total_items,
  },
  {
    key: "totalRoles",
    label: ADMIN_TEXT.STAT_ROLES,
    icon: Shield,
    color: "text-primary",
    requirements: [
      { resource: AUTHORIZATION_RESOURCE.ROLE, action: AUTHORIZATION_ACTION.READ },
      {
        resource: AUTHORIZATION_RESOURCE.AUTHORIZATION,
        action: AUTHORIZATION_ACTION.MANAGE,
      },
    ],
    load: async (signal) => (await roleService.getAll(signal)).length,
  },
];

export default function AdminDashboardPage() {
  const { capabilityIndex, can } = useAuth();
  const canReadAudit = can(
    AUTHORIZATION_RESOURCE.AUDIT,
    AUTHORIZATION_ACTION.READ,
  );
  const authorizedCards = useMemo(
    () => filterAuthorizedItems(STAT_CARDS, capabilityIndex),
    [capabilityIndex],
  );
  const dashboardKey = useMemo(
    () => authorizedCards.map((card) => card.key).join(":"),
    [authorizedCards],
  );
  const dashboardResource = useRetryableResource<DashboardProjection>({
    resetKey: dashboardKey,
    initialData: EMPTY_DASHBOARD,
    load: async (signal) => {
      const results = await Promise.allSettled(
        authorizedCards.map(
          async (card) => [card.key, await card.load(signal)] as const,
        ),
      );
      if (signal.aborted) {
        throw signal.reason ??
          new DOMException("Dashboard request superseded", "AbortError");
      }
      const nextStats: DashboardStats = {};
      const nextFailures = new Set<StatKey>();
      results.forEach((result, index) => {
        if (result.status === "fulfilled") {
          const [key, value] = result.value;
          nextStats[key] = value;
        } else {
          nextFailures.add(authorizedCards[index].key);
        }
      });
      return { stats: nextStats, failedStats: nextFailures };
    },
  });
  const { stats, failedStats } = dashboardResource.data;
  const isLoading = dashboardResource.status === "loading";

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {ADMIN_TEXT.DASHBOARD_TITLE}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {ADMIN_TEXT.DASHBOARD_SUBTITLE}
        </p>
      </div>

      {isLoading ? (
        <div className="flex min-h-40 items-center justify-center rounded-xl border border-border bg-card/50">
          <Loader2
            className="size-6 animate-spin text-primary motion-reduce:animate-none"
            aria-hidden="true"
          />
          <span className="sr-only">{TEXT.COMMON.LOADING}</span>
        </div>
      ) : authorizedCards.length ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
          {authorizedCards.map(({ key, label, icon: Icon, color }) => (
            <Card key={key} className="border-border">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {label}
                </CardTitle>
                <Icon className={`size-4 ${color}`} aria-hidden="true" />
              </CardHeader>
              <CardContent>
                {failedStats.has(key) ? (
                  <span className="text-sm text-destructive">
                    {ADMIN_TEXT.STAT_UNAVAILABLE}
                  </span>
                ) : (
                  <div className="text-3xl font-bold tabular-nums">
                    {stats[key] ?? 0}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <Card className="border-border">
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {ADMIN_TEXT.STATS_EMPTY}
          </CardContent>
        </Card>
      )}

      {failedStats.size ? (
        <div className="flex flex-col gap-3 rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm sm:flex-row sm:items-center">
          <AlertCircle
            className="size-4 shrink-0 text-destructive"
            aria-hidden="true"
          />
          <span className="flex-1 text-muted-foreground">
            {ADMIN_TEXT.STATS_LOAD_ERROR}
          </span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={dashboardResource.retry}
          >
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}

      {canReadAudit ? <AdminRecentAudit /> : null}
    </div>
  );
}
