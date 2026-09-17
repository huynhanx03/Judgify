import {
  Calendar,
  Clock,
  Sparkles,
  Swords,
  Trophy,
  Users,
} from "lucide-react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { formatDateTimeMinute } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Contest, ContestStatus } from "@/types/contest";

interface ContestCardProps {
  contest: Contest;
}

const STATUS_MAP: Record<
  ContestStatus,
  { label: string; color: string }
> = {
  running: {
    label: TEXT.CONTEST.STATUS_RUNNING,
    color: "border-success/25 bg-success/10 text-success",
  },
  upcoming: {
    label: TEXT.CONTEST.STATUS_UPCOMING,
    color: "border-info/25 bg-info/10 text-info",
  },
  ended: {
    label: TEXT.CONTEST.STATUS_ENDED,
    color: "border-border bg-muted text-muted-foreground",
  },
  draft: {
    label: TEXT.CONTEST.STATUS_DRAFT,
    color: "border-border bg-muted text-muted-foreground",
  },
  cancelled: {
    label: TEXT.CONTEST.STATUS_CANCELLED,
    color:
      "border-destructive/25 bg-destructive/10 text-destructive",
  },
};

export function ContestCard({ contest }: ContestCardProps) {
  const isActive = contest.status === "running";
  const isUpcoming = contest.status === "upcoming";
  const statusInfo = STATUS_MAP[contest.status];

  return (
    <Link
      href={APP_ROUTES.CONTEST_DETAIL(contest.id)}
      className={cn(
        "group relative block overflow-hidden rounded-3xl border bg-card/70 outline-none transition-[border-color,box-shadow] duration-200 hover:shadow-lg focus-visible:ring-3 focus-visible:ring-ring/50 motion-reduce:transition-none",
        isActive
          ? "surface-brand-feature border-primary/40 shadow-brand-soft"
          : "border-border/70",
      )}
    >
      <article className="relative z-10 flex flex-col items-center gap-6 p-6 sm:p-8 lg:flex-row lg:gap-8">
        <div
          className={cn(
            "relative flex size-24 shrink-0 items-center justify-center rounded-3xl border shadow-lg lg:size-28",
            isActive
              ? "border-cultivation/30 bg-cultivation text-cultivation-foreground"
              : "border-border bg-muted text-muted-foreground",
          )}
        >
          <Trophy className="size-11" aria-hidden="true" />
          {isActive ? (
            <Sparkles
              className="absolute -top-2 -right-2 size-7 text-cultivation"
              aria-hidden="true"
            />
          ) : null}
        </div>

        <div className="min-w-0 flex-1 space-y-4 text-center lg:text-left">
          <div className="flex flex-wrap items-center justify-center gap-3 lg:justify-start">
            <Badge
              variant="outline"
              className={cn(
                "rounded-lg px-3 py-1 text-xs font-black tracking-wider uppercase",
                statusInfo.color,
              )}
            >
              {statusInfo.label}
            </Badge>
            <div className="flex items-center gap-2 rounded-full bg-muted/60 px-3 py-1 text-sm font-semibold text-muted-foreground">
              <Users className="size-4" aria-hidden="true" />
              <span>
                {contest.participant_count}
                {contest.max_participants !== undefined
                  ? `/${contest.max_participants}`
                  : ""}{" "}
                {TEXT.CONTEST.PARTICIPANTS}
              </span>
            </div>
            {contest.is_registered ? (
              <Badge
                variant="outline"
                className="rounded-full border-success/30 bg-success/10 px-3 py-1 font-bold text-success"
              >
                {TEXT.CONTEST.REGISTERED}
              </Badge>
            ) : null}
          </div>

          <h3 className="text-2xl font-black tracking-tight text-foreground transition-colors group-hover:text-primary sm:text-3xl">
            {contest.title}
          </h3>

          <div className="flex flex-wrap items-center justify-center gap-4 text-sm font-medium text-muted-foreground lg:justify-start">
            <div className="flex flex-wrap items-center justify-center gap-2 rounded-2xl border border-border/60 bg-background/50 px-4 py-2">
              <Calendar
                className="size-4 text-cultivation"
                aria-hidden="true"
              />
              <time dateTime={contest.start_time}>
                {formatDateTimeMinute(contest.start_time)}
              </time>
              <span aria-hidden="true">→</span>
              <time dateTime={contest.end_time}>
                {formatDateTimeMinute(contest.end_time)}
              </time>
            </div>
            <div className="flex items-center gap-2">
              <Clock
                className="size-4 text-cultivation"
                aria-hidden="true"
              />
              <span>
                {TEXT.CONTEST.PROBLEMS}: {contest.problem_count}
              </span>
            </div>
          </div>
        </div>

        <span
          className={cn(
            "flex min-h-11 w-full shrink-0 items-center justify-center rounded-xl px-6 py-3 text-sm font-black tracking-wider uppercase transition-colors lg:w-auto",
            isActive || isUpcoming
              ? "bg-primary text-primary-foreground group-hover:bg-primary/90"
              : "bg-secondary text-secondary-foreground group-hover:bg-secondary/80",
          )}
        >
          {contest.status === "ended"
            ? TEXT.CONTEST.VIEW_RESULTS
            : TEXT.CONTEST.VIEW_DETAILS}
        </span>
      </article>

      {isActive ? (
        <>
          <div
            className="pointer-events-none absolute right-0 top-0 p-6 opacity-[0.05]"
            aria-hidden="true"
          >
            <Swords className="size-32 text-cultivation" />
          </div>
          <div
            className="pointer-events-none absolute -bottom-10 -left-10 size-40 rounded-full bg-cultivation/10 blur-[60px] transition-colors group-hover:bg-cultivation/20 motion-reduce:transition-none"
            aria-hidden="true"
          />
        </>
      ) : null}
    </Link>
  );
}
