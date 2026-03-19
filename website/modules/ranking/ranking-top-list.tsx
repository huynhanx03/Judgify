/**
 * Reusable top-10 ranking list card with featured top-3 podium.
 * Used for both Rating and Level/EXP rankings side by side.
 */

import { Crown, Flame } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import type { Cultivator } from "@/types/ranking";

interface RankingTopListProps {
  title: string;
  subtitle: string;
  icon: "crown" | "flame";
  accentColor: "amber" | "emerald";
  data: Cultivator[];
  mode: "rating" | "level";
}

/** Maps level to cultivation disciple title */
function getLevelTitle(level: number): string {
  if (level >= 90) return "Đại Trưởng Lão";
  if (level >= 80) return "Trưởng Lão";
  if (level >= 70) return "Đệ Tử Chân Truyền";
  if (level >= 60) return "Nội Môn Đệ Tử";
  if (level >= 50) return "Ngoại Môn Đệ Tử";
  return "Tạp Dịch Đệ Tử";
}

const ACCENT = {
  amber: {
    icon: "text-amber-400",
    border: "border-amber-500/20",
    headerBg: "bg-amber-500/5",
    headerBorder: "border-amber-500/20",
    top1Glow: "shadow-[0_0_30px_rgba(245,158,11,0.3)]",
    top1Border: "border-amber-400",
    top1Text: "text-amber-400",
    top1Bg: "bg-amber-500/15",
    top2Text: "text-slate-300",
    top2Border: "border-slate-400/50",
    top2Bg: "bg-slate-500/10",
    top3Text: "text-orange-400",
    top3Border: "border-orange-400/50",
    top3Bg: "bg-orange-500/10",
    hoverBg: "hover:bg-amber-500/[0.03]",
    valueBg: "bg-amber-500/10 text-amber-400",
    badgeBg: "bg-gradient-to-r from-amber-500 to-amber-600 text-black",
    realmBg: "bg-amber-500/5 text-amber-400/70 border-amber-500/10",
  },
  emerald: {
    icon: "text-emerald-400",
    border: "border-emerald-500/20",
    headerBg: "bg-emerald-500/5",
    headerBorder: "border-emerald-500/20",
    top1Glow: "shadow-[0_0_30px_rgba(52,211,153,0.3)]",
    top1Border: "border-emerald-400",
    top1Text: "text-emerald-400",
    top1Bg: "bg-emerald-500/15",
    top2Text: "text-slate-300",
    top2Border: "border-slate-400/50",
    top2Bg: "bg-slate-500/10",
    top3Text: "text-teal-400",
    top3Border: "border-teal-400/50",
    top3Bg: "bg-teal-500/10",
    hoverBg: "hover:bg-emerald-500/[0.03]",
    valueBg: "bg-emerald-500/10 text-emerald-400",
    badgeBg: "bg-gradient-to-r from-emerald-500 to-emerald-600 text-black",
    realmBg: "bg-emerald-500/5 text-emerald-400/70 border-emerald-500/10",
  },
} as const;

export function RankingTopList({ title, subtitle, icon, accentColor, data, mode }: RankingTopListProps) {
  const colors = ACCENT[accentColor];
  const Icon = icon === "crown" ? Crown : Flame;
  const top3 = data.slice(0, 3);
  const rest = data.slice(3);

  return (
    <div className={`rounded-2xl border ${colors.border} bg-card/60 backdrop-blur-xl overflow-hidden shadow-lg flex flex-col`}>
      {/* Header */}
      <div className={`px-6 py-4 ${colors.headerBg} border-b ${colors.headerBorder} flex items-center gap-3`}>
        <Icon className={`h-5 w-5 ${colors.icon}`} />
        <div>
          <h3 className="font-black text-base uppercase tracking-wider">{title}</h3>
          <p className="text-xs text-muted-foreground">{subtitle}</p>
        </div>
      </div>

      {/* Top 3 featured */}
      <div className="px-4 py-5 space-y-3">
        {top3.map((user, i) => {
          const isFirst = i === 0;
          const podiumColors = i === 0
            ? { glow: colors.top1Glow, border: colors.top1Border, text: colors.top1Text, bg: colors.top1Bg }
            : i === 1
              ? { glow: "", border: colors.top2Border, text: colors.top2Text, bg: colors.top2Bg }
              : { glow: "", border: colors.top3Border, text: colors.top3Text, bg: colors.top3Bg };

          return (
            <div
              key={user.rank}
              className={`relative flex items-center gap-4 px-4 ${isFirst ? "py-5" : "py-3"} rounded-xl border ${podiumColors.border} ${podiumColors.bg} ${podiumColors.glow} transition-all`}
            >
              <span className={`font-black ${isFirst ? "text-2xl" : "text-lg"} ${podiumColors.text} w-6 text-center`}>
                {user.rank}
              </span>

              <Avatar className={`${isFirst ? "h-12 w-12" : "h-9 w-9"} border-2 ${podiumColors.border} shrink-0`}>
                {user.avatar && <AvatarImage src={user.avatar} />}
                <AvatarFallback className={`${podiumColors.bg} ${podiumColors.text} font-black ${isFirst ? "text-lg" : "text-sm"}`}>
                  {user.name.charAt(0)}
                </AvatarFallback>
              </Avatar>

              <div className="flex-1 min-w-0">
                <p className={`font-black ${isFirst ? "text-base" : "text-sm"} truncate`}>{user.name}</p>
                <p className="text-[10px] text-muted-foreground truncate">{user.sect}</p>
              </div>

              {/* Value badge */}
              <div className="text-right shrink-0">
                {mode === "level" ? (
                  <div>
                    <span className={`inline-block font-black ${isFirst ? "text-sm px-3 py-1" : "text-xs px-2 py-0.5"} rounded-lg ${isFirst ? colors.badgeBg : colors.valueBg}`}>
                      {getLevelTitle(user.level ?? 0)}
                    </span>
                    <p className="text-[9px] text-muted-foreground font-bold mt-1">Lv.{user.level}</p>
                  </div>
                ) : (
                  <div>
                    <span className={`inline-block font-black ${isFirst ? "text-sm px-3 py-1" : "text-xs px-2 py-0.5"} rounded-lg ${isFirst ? colors.badgeBg : colors.valueBg}`}>
                      {user.realm}
                    </span>
                    <p className="text-[9px] text-muted-foreground font-bold mt-1">
                      {(user.points ?? 0).toLocaleString()} Linh Lực
                    </p>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Divider */}
      <div className="mx-5 h-px bg-gradient-to-r from-transparent via-border/40 to-transparent" />

      {/* Rest (4-10) with realm shown */}
      <div className="divide-y divide-border/10 flex-1">
        {rest.map((user) => (
          <div key={user.rank} className={`flex items-center gap-3 px-5 py-3 ${colors.hoverBg} transition-colors`}>
            <span className="font-mono text-sm font-black w-7 text-center text-zinc-600">{user.rank}</span>
            <Avatar className="h-8 w-8 shrink-0 border border-border/30">
              <AvatarFallback className="bg-muted/50 text-muted-foreground text-xs font-bold">
                {user.name.charAt(0)}
              </AvatarFallback>
            </Avatar>
            <div className="flex-1 min-w-0">
              <p className="font-bold text-sm truncate">{user.name}</p>
              <p className="text-[10px] text-muted-foreground truncate">{user.sect}</p>
            </div>
            <div className="text-right shrink-0">
              {mode === "level" ? (
                <div>
                  <span className={`text-[10px] font-bold px-2 py-0.5 rounded-md border ${colors.realmBg}`}>{getLevelTitle(user.level ?? 0)}</span>
                  <p className="text-[9px] text-muted-foreground font-bold mt-0.5">Lv.{user.level}</p>
                </div>
              ) : (
                <div>
                  <span className={`text-[10px] font-bold px-2 py-0.5 rounded-md border ${colors.realmBg}`}>{user.realm}</span>
                  <p className="text-[9px] text-muted-foreground font-bold mt-0.5">{(user.points ?? 0).toLocaleString()} Pts</p>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
