"use client";

/**
 * Cultivation progress bars for Rating/Realm and EXP/Rank.
 * Shows current level, progress to next level, and visual indicators.
 */

import { CULTIVATION_REALMS, DISCIPLE_RANKS, getRealmByRating, getRankByExp } from "@/mock/profile-stats";
import { Flame, Crown, Sparkles, Star, Sprout, TreePine, Gem } from "lucide-react";

interface CultivationProgressProps {
  rating: number;
  exp: number;
}

const RANK_ICONS: Record<string, React.ReactNode> = {
  seedling: <TreePine className="h-5 w-5" />,
  sprout: <Sprout className="h-5 w-5" />,
  flame: <Flame className="h-5 w-5" />,
  star: <Star className="h-5 w-5" />,
  crown: <Crown className="h-5 w-5" />,
  gem: <Gem className="h-5 w-5" />,
};

export function CultivationProgress({ rating, exp }: CultivationProgressProps) {
  const realm = getRealmByRating(rating);
  const rank = getRankByExp(exp);

  /** Progress percentage toward next realm. */
  const realmProgress = realm.next
    ? ((rating - realm.current.minRating) / (realm.next.minRating - realm.current.minRating)) * 100
    : 100;

  /** Progress percentage toward next disciple rank. */
  const rankProgress = rank.next
    ? ((exp - rank.current.minExp) / (rank.next.minExp - rank.current.minExp)) * 100
    : 100;

  return (
    <div className="space-y-6">
      {/* Rating / Realm Section */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Sparkles className="h-4 w-4" style={{ color: realm.current.color }} />
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Cảnh Giới
            </span>
          </div>
          <span className="text-xs text-muted-foreground">
            Rating: <span className="font-bold text-foreground">{rating}</span>
          </span>
        </div>

        {/* Current realm badge */}
        <div className="flex items-center gap-3">
          <div
            className={`px-4 py-2 rounded-xl font-bold text-sm text-white bg-gradient-to-r ${realm.current.gradient} shadow-lg`}
          >
            {realm.current.name}
          </div>
          {realm.next && (
            <>
              <div className="flex-1 h-px bg-border/40" />
              <span className="text-xs text-muted-foreground">
                {realm.next.name}
              </span>
            </>
          )}
        </div>

        {/* Rating progress bar */}
        <div className="relative">
          <div className="h-3 rounded-full bg-muted/50 overflow-hidden">
            <div
              className={`h-full rounded-full bg-gradient-to-r ${realm.current.gradient} transition-all duration-1000 relative`}
              style={{ width: `${Math.min(realmProgress, 100)}%` }}
            >
              <div className="absolute inset-0 bg-white/20 animate-pulse rounded-full" />
            </div>
          </div>
          {/* Realm threshold markers */}
          <div className="relative h-4 mt-1">
            {CULTIVATION_REALMS.slice(0, 5).map((r, i) => {
              const maxVisible = (realm.next?.minRating ?? realm.current.minRating + 400);
              const pos = ((r.minRating - realm.current.minRating) / (maxVisible - realm.current.minRating)) * 100;
              if (pos <= 0 || pos > 100) return null;
              return (
                <div
                  key={r.name}
                  className="absolute top-0 -translate-x-1/2"
                  style={{ left: `${pos}%` }}
                >
                  <div className="w-px h-2 bg-border/60 mx-auto" />
                </div>
              );
            })}
          </div>
        </div>

        {realm.next && (
          <p className="text-xs text-muted-foreground">
            Cần thêm <span className="font-bold text-foreground">{realm.next.minRating - rating}</span> rating để đột phá {realm.next.name}
          </p>
        )}
      </div>

      {/* Divider */}
      <div className="h-px bg-border/30" />

      {/* EXP / Disciple Rank Section */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {RANK_ICONS[rank.current.icon] ?? <Star className="h-4 w-4" />}
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Cấp Bậc
            </span>
          </div>
          <span className="text-xs text-muted-foreground">
            EXP: <span className="font-bold text-foreground">{exp.toLocaleString()}</span>
          </span>
        </div>

        {/* Current rank badge */}
        <div className="flex items-center gap-3">
          <div className="px-4 py-2 rounded-xl font-bold text-sm bg-gradient-to-r from-primary/20 to-secondary/20 border border-primary/30 text-primary">
            Lv.{rank.level} — {rank.current.name}
          </div>
          {rank.next && (
            <>
              <div className="flex-1 h-px bg-border/40" />
              <span className="text-xs text-muted-foreground">
                {rank.next.name}
              </span>
            </>
          )}
        </div>

        {/* EXP progress bar */}
        <div className="h-3 rounded-full bg-muted/50 overflow-hidden">
          <div
            className="h-full rounded-full bg-gradient-to-r from-primary to-secondary transition-all duration-1000 relative"
            style={{ width: `${Math.min(rankProgress, 100)}%` }}
          >
            <div className="absolute inset-0 bg-white/20 animate-pulse rounded-full" />
          </div>
        </div>

        {rank.next && (
          <p className="text-xs text-muted-foreground">
            Cần thêm <span className="font-bold text-foreground">{(rank.next.minExp - exp).toLocaleString()}</span> EXP để thăng cấp {rank.next.name}
          </p>
        )}
      </div>
    </div>
  );
}
