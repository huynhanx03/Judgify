import { CircleAlert, Crown, Flame, RefreshCw } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  getPodiumPresentation,
  type RankingAccent,
} from "@/constants/ranking-presentation";
import { TEXT } from "@/constants/text";
import type { CursorFeedStatus } from "@/hooks/use-cursor-feed";
import { cn } from "@/lib/utils";
import { formatNumber } from "@/lib/format";
import { ResourceCollectionFeedback } from "@/modules/shared/resource-feedback";
import type { Cultivator } from "@/types/ranking";

interface RankingTopListProps {
  title: string;
  subtitle: string;
  icon: "crown" | "flame";
  accentColor: RankingAccent;
  data: Cultivator[];
  mode: "rating" | "level";
  status: CursorFeedStatus;
  onRetry: () => void;
  hasMore: boolean;
  onLoadMore: () => void;
}

const ACCENT = {
  amber: {
    icon: "text-primary",
    border: "border-primary/20",
    headerBg: "bg-primary/5",
    headerBorder: "border-primary/20",
    firstGlow: "shadow-brand-strong",
    hoverBg: "hover:bg-primary/[0.04]",
    valueBg: "border-primary/15 bg-primary/10 text-primary",
    firstValue: "border-transparent bg-primary text-primary-foreground",
  },
  emerald: {
    icon: "text-success",
    border: "border-success/20",
    headerBg: "bg-success/5",
    headerBorder: "border-success/20",
    firstGlow: "shadow-success-soft",
    hoverBg: "hover:bg-success/[0.04]",
    valueBg: "border-success/15 bg-success/10 text-success",
    firstValue: "border-transparent bg-success text-success-foreground",
  },
} as const;

type Accent = (typeof ACCENT)[keyof typeof ACCENT];

function RankingValue({
  user,
  mode,
  featured,
  colors,
}: {
  user: Cultivator;
  mode: "rating" | "level";
  featured: boolean;
  colors: Accent;
}) {
  const detail =
    mode === "level"
      ? `${TEXT.RANKING.LEVEL_ABBREVIATION}${user.level ?? 0}`
      : `${formatNumber(user.points)} ${
          featured
            ? TEXT.RANKING.SPIRIT_POWER
            : TEXT.RANKING.POINTS_ABBREVIATION
        }`;

  return (
    <div className="shrink-0 text-right">
      <span
        className={cn(
          "inline-block rounded-lg border px-2 py-0.5 text-[10px] font-bold",
          featured && "px-3 py-1 text-xs",
          featured ? colors.firstValue : colors.valueBg,
        )}
      >
        {user.realm ?? TEXT.COMMON.NOT_AVAILABLE}
      </span>
      <p className="mt-1 text-[10px] font-semibold text-muted-foreground">
        {detail}
      </p>
    </div>
  );
}

export function RankingTopList({
  title,
  subtitle,
  icon,
  accentColor,
  data,
  mode,
  status,
  onRetry,
  hasMore,
  onLoadMore,
}: RankingTopListProps) {
  const colors = ACCENT[accentColor];
  const Icon = icon === "crown" ? Crown : Flame;
  const isInitialLoading = status === "loading" && data.length === 0;
  const isUnavailable = status === "error" && data.length === 0;
  const isRefreshing = status === "refreshing";
  const isLoadingMore = status === "loading_more";
  const isStale = status === "error" && data.length > 0;

  return (
    <section
      className={cn(
        "flex min-h-[32rem] flex-col overflow-hidden rounded-xl border bg-card",
        colors.border,
      )}
      aria-busy={isRefreshing || isLoadingMore}
    >
      <header
        className={cn(
          "flex items-center gap-3 border-b px-5 py-4",
          colors.headerBg,
          colors.headerBorder,
        )}
      >
        <Icon className={cn("size-5", colors.icon)} aria-hidden="true" />
        <div>
          <h2 className="text-base font-bold tracking-wide">{title}</h2>
          <p className="text-xs text-muted-foreground">{subtitle}</p>
        </div>
        {isRefreshing ? (
          <RefreshCw
            className="ml-auto size-4 animate-spin text-muted-foreground motion-reduce:animate-none"
            aria-label={TEXT.RANKING.REFRESHING}
          />
        ) : null}
      </header>

      {isStale ? (
        <div
          className="flex items-center gap-2 border-b border-warning/20 bg-warning/5 px-5 py-3 text-xs text-foreground"
          role="status"
        >
          <CircleAlert className="size-4 shrink-0" aria-hidden="true" />
          <span className="flex-1">{TEXT.RANKING.STALE_DATA}</span>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-8"
            onClick={onRetry}
          >
            <RefreshCw className="size-3.5" aria-hidden="true" />
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}

      {isInitialLoading ? (
        <div
          className="flex flex-1 flex-col gap-3 p-4"
          role="status"
          aria-label={TEXT.RANKING.LOADING_LABEL}
        >
          {[0, 1, 2, 3, 4, 5].map((row) => (
            <div
              key={row}
              className={cn(
                "h-16 animate-pulse rounded-xl bg-muted/45 motion-reduce:animate-none",
                row > 2 && "h-12",
              )}
            />
          ))}
        </div>
      ) : isUnavailable ? (
        <ResourceCollectionFeedback
          className="flex-1"
          title={TEXT.RANKING.LOAD_ERROR_TITLE}
          description={TEXT.RANKING.LOAD_ERROR_DESCRIPTION}
          retryLabel={TEXT.COMMON.RETRY}
          onRetry={onRetry}
        />
      ) : data.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center px-6 py-12 text-center">
          <p className="text-sm font-semibold text-foreground">
            {TEXT.RANKING.EMPTY_CULTIVATORS}
          </p>
          <p className="mt-1 max-w-sm text-xs leading-5 text-muted-foreground">
            {TEXT.RANKING.EMPTY_DESCRIPTION}
          </p>
        </div>
      ) : (
        <div className="flex-1" role="list">
          {data.map((user) => {
            const displayName = user.name || TEXT.RANKING.ANONYMOUS;
            const podium = getPodiumPresentation(user.rank, accentColor);
            const isPodium = podium !== undefined;
            const isFirst = user.rank === 1;

            return (
              <div
                key={user.id}
                role="listitem"
                className={cn(
                  "flex items-center gap-3 border-b border-border/40 px-5 py-3 transition-colors last:border-b-0",
                  colors.hoverBg,
                  isPodium && "mx-4 mt-3 rounded-xl border px-4 first:mt-5",
                  isPodium && podium.border,
                  isPodium && podium.background,
                  isFirst && colors.firstGlow,
                  isFirst && "gap-4 py-5",
                )}
              >
                <span
                  className={cn(
                    "w-7 text-center font-mono text-sm font-black text-muted-foreground",
                    isPodium && podium.text,
                    isFirst && "text-2xl",
                  )}
                >
                  {user.rank}
                </span>
                <Avatar
                  className={cn(
                    "size-8 shrink-0 border border-border",
                    isPodium && "size-9 border-2",
                    isFirst && "size-12",
                    isPodium && podium.border,
                  )}
                >
                  {user.avatar ? (
                    <AvatarImage src={user.avatar} alt={displayName} />
                  ) : null}
                  <AvatarFallback
                    className={cn(
                      "bg-muted/60 text-xs font-bold text-muted-foreground",
                      isPodium && podium.background,
                      isPodium && podium.text,
                      isFirst && "text-lg",
                    )}
                  >
                    {displayName.charAt(0)}
                  </AvatarFallback>
                </Avatar>
                <div className="min-w-0 flex-1">
                  <p
                    className={cn(
                      "truncate text-sm font-bold text-foreground",
                      isFirst && "text-base font-black",
                    )}
                  >
                    {displayName}
                  </p>
                  {user.realm ? (
                    <p className="truncate text-[11px] text-muted-foreground">
                      {user.realm}
                    </p>
                  ) : null}
                </div>
                <RankingValue
                  user={user}
                  mode={mode}
                  featured={isFirst}
                  colors={colors}
                />
              </div>
            );
          })}
        </div>
      )}

      {data.length > 0 ? (
        <footer className="border-t border-border/60 p-4">
          {hasMore ? (
            <Button
              type="button"
              variant="outline"
              className="min-h-11 w-full"
              disabled={isLoadingMore || isRefreshing}
              onClick={onLoadMore}
            >
              {isLoadingMore ? (
                <RefreshCw
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : null}
              {isLoadingMore
                ? TEXT.RANKING.LOADING_MORE
                : TEXT.RANKING.LOAD_MORE}
            </Button>
          ) : (
            <p className="text-center text-xs text-muted-foreground">
              {TEXT.RANKING.END_OF_LIST}
            </p>
          )}
        </footer>
      ) : null}
    </section>
  );
}
