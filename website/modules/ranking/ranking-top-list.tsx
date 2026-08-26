import {
  CircleAlert,
  Crown,
  Flame,
  RefreshCw,
  Trophy,
} from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";
import type { AsyncResourceStatus } from "@/hooks/use-retryable-resource";
import { cn } from "@/lib/utils";
import { formatNumber } from "@/lib/format";
import type { Cultivator } from "@/types/ranking";

interface RankingTopListProps {
  title: string;
  subtitle: string;
  icon: "crown" | "flame";
  accentColor: "amber" | "emerald";
  data: Cultivator[];
  mode: "rating" | "level";
  status: AsyncResourceStatus;
  onRetry: () => void;
	hasMore: boolean;
	onLoadMore: () => void;
}

const ACCENT = {
  amber: {
    icon: "text-amber-600 dark:text-amber-400",
    border: "border-amber-500/20",
    headerBg: "bg-amber-500/5",
    headerBorder: "border-amber-500/20",
    firstGlow: "shadow-brand-strong",
    firstBorder: "border-amber-500/60",
    firstText: "text-amber-700 dark:text-amber-300",
    firstBg: "bg-amber-500/10",
    secondText: "text-slate-600 dark:text-slate-300",
    secondBorder: "border-slate-400/40",
    secondBg: "bg-slate-500/10",
    thirdText: "text-orange-700 dark:text-orange-300",
    thirdBorder: "border-orange-500/40",
    thirdBg: "bg-orange-500/10",
    hoverBg: "hover:bg-amber-500/[0.04]",
    valueBg:
      "border-amber-500/15 bg-amber-500/10 text-amber-700 dark:text-amber-300",
    firstValue:
      "border-transparent bg-gradient-to-r from-amber-400 to-amber-500 text-amber-950",
  },
  emerald: {
    icon: "text-emerald-600 dark:text-emerald-400",
    border: "border-emerald-500/20",
    headerBg: "bg-emerald-500/5",
    headerBorder: "border-emerald-500/20",
    firstGlow: "shadow-success-soft",
    firstBorder: "border-emerald-500/60",
    firstText: "text-emerald-700 dark:text-emerald-300",
    firstBg: "bg-emerald-500/10",
    secondText: "text-slate-600 dark:text-slate-300",
    secondBorder: "border-slate-400/40",
    secondBg: "bg-slate-500/10",
    thirdText: "text-teal-700 dark:text-teal-300",
    thirdBorder: "border-teal-500/40",
    thirdBg: "bg-teal-500/10",
    hoverBg: "hover:bg-emerald-500/[0.04]",
    valueBg:
      "border-emerald-500/15 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
    firstValue:
      "border-transparent bg-gradient-to-r from-emerald-400 to-emerald-500 text-emerald-950",
  },
} as const;

type Accent = (typeof ACCENT)[keyof typeof ACCENT];

function podiumStyle(colors: Accent, index: number) {
  if (index === 0) {
    return {
      border: colors.firstBorder,
      background: colors.firstBg,
      text: colors.firstText,
      shadow: colors.firstGlow,
    };
  }
  if (index === 1) {
    return {
      border: colors.secondBorder,
      background: colors.secondBg,
      text: colors.secondText,
      shadow: "",
    };
  }
  return {
    border: colors.thirdBorder,
    background: colors.thirdBg,
    text: colors.thirdText,
    shadow: "",
  };
}

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
  const isRefreshing = status === "loading" && data.length > 0;
  const isStale = status === "error" && data.length > 0;

  return (
    <section
      className={cn(
        "flex min-h-[32rem] flex-col overflow-hidden rounded-2xl border bg-card/80 shadow-lg backdrop-blur-xl",
        colors.border,
      )}
      aria-busy={isRefreshing}
    >
      <header
        className={cn(
          "flex items-center gap-3 border-b px-6 py-4",
          colors.headerBg,
          colors.headerBorder,
        )}
      >
        <Icon className={cn("size-5", colors.icon)} aria-hidden="true" />
        <div>
          <h2 className="text-base font-black tracking-wider uppercase">
            {title}
          </h2>
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
          className="flex items-center gap-2 border-b border-amber-500/20 bg-amber-500/5 px-5 py-3 text-xs text-amber-800 dark:text-amber-200"
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
        <div
          className="flex flex-1 flex-col items-center justify-center px-6 py-12 text-center"
          role="alert"
        >
          <div className="flex size-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
            <CircleAlert className="size-5" aria-hidden="true" />
          </div>
          <h3 className="mt-4 font-semibold">
            {TEXT.RANKING.LOAD_ERROR_TITLE}
          </h3>
          <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
            {TEXT.RANKING.LOAD_ERROR_DESCRIPTION}
          </p>
          <Button
            type="button"
            variant="outline"
            className="mt-5 min-h-11"
            onClick={onRetry}
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : data.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center px-6 py-12 text-center">
          <div className="flex size-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
            <Trophy className="size-5" aria-hidden="true" />
          </div>
          <p className="mt-4 text-sm font-semibold text-foreground">
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
			const isPodium = user.rank <= 3;
			const isFirst = user.rank === 1;
			const podium = podiumStyle(colors, user.rank - 1);

            return (
              <div
                key={user.id}
                role="listitem"
                className={cn(
                  "flex items-center gap-3 border-b border-border/40 px-5 py-3 transition-colors last:border-b-0",
                  colors.hoverBg,
                  isPodium &&
                    "mx-4 mt-3 rounded-xl border px-4 first:mt-5",
                  isPodium && podium.border,
                  isPodium && podium.background,
                  isPodium && podium.shadow,
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
			  disabled={status === "loading"}
			  onClick={onLoadMore}
			>
			  {status === "loading" ? (
				<RefreshCw className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
			  ) : null}
			  {status === "loading" ? TEXT.RANKING.LOADING_MORE : TEXT.RANKING.LOAD_MORE}
			</Button>
		  ) : (
			<p className="text-center text-xs text-muted-foreground">{TEXT.RANKING.END_OF_LIST}</p>
		  )}
		</footer>
	  ) : null}
	</section>
  );
}
