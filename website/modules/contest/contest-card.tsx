/**
 * Single contest card with status, participants, time, and CTA.
 * Updated to use real API data (Contest type from backend).
 */

import { Badge } from "@/components/ui/badge";
import { Trophy, Users, Clock, Swords, Sparkles, Calendar } from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { TEXT } from "@/constants/text";
import type { Contest } from "@/types/contest";

interface ContestCardProps {
  contest: Contest;
  index: number;
  onRegister?: (id: number) => void;
  onUnregister?: (id: number) => void;
}

const STATUS_MAP: Record<string, { label: string; color: string }> = {
  running: { label: TEXT.CONTEST.STATUS_RUNNING, color: "bg-green-500 text-white" },
  upcoming: { label: TEXT.CONTEST.STATUS_UPCOMING, color: "bg-blue-500 text-white" },
  ended: { label: TEXT.CONTEST.STATUS_ENDED, color: "bg-zinc-500 text-white" },
  draft: { label: TEXT.CONTEST.STATUS_DRAFT, color: "bg-zinc-400 text-white" },
};

function formatTime(iso: string) {
  return new Date(iso).toLocaleDateString("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function ContestCard({ contest, index, onRegister, onUnregister }: ContestCardProps) {
  const isActive = contest.status === "running";
  const isUpcoming = contest.status === "upcoming";
  const isEnded = contest.status === "ended";
  const statusInfo = STATUS_MAP[contest.status] ?? { label: contest.status, color: "bg-zinc-400" };

  return (
    <Link href={`/contest/${contest.id}`} className="block">
    <div
      className={cn(
        "group relative overflow-hidden rounded-[2.5rem] border transition-all duration-700 hover:-translate-y-1 animate-in fade-in slide-in-from-bottom-10",
        isActive
          ? "bg-gradient-to-r from-amber-600/15 via-amber-600/5 to-transparent border-amber-500/40 shadow-[0_20px_40px_-15px_rgba(245,158,11,0.1)]"
          : "bg-card/40 border-border/40"
      )}
      style={{ animationDelay: `${index * 150}ms` }}
    >
      <div className="flex flex-col lg:flex-row items-center p-10 gap-10 relative z-10">
        <div className={cn(
          "h-32 w-32 rounded-[2rem] flex items-center justify-center shrink-0 shadow-2xl relative transition-transform duration-500 group-hover:scale-110 group-hover:rotate-3",
          isActive
            ? "bg-gradient-to-br from-amber-400 to-amber-600 text-black"
            : "bg-muted text-muted-foreground border border-border/40"
        )}>
          <Trophy className="h-14 w-14" />
          {isActive && <Sparkles className="absolute -top-2 -right-2 h-8 w-8 text-amber-300 animate-pulse" />}
        </div>

        <div className="flex-1 text-center lg:text-left space-y-4">
          <div className="flex flex-wrap items-center justify-center lg:justify-start gap-4">
            <Badge className={cn("font-black uppercase tracking-widest px-4 py-1.5 rounded-lg text-xs shadow-lg", statusInfo.color)}>
              {statusInfo.label}
            </Badge>
            <div className="flex items-center gap-2 text-sm text-muted-foreground font-bold bg-muted/50 px-3 py-1 rounded-full">
              <Users className="h-4 w-4" />
              {contest.participant_count}{contest.max_participants > 0 ? `/${contest.max_participants}` : ""} {TEXT.CONTEST.PARTICIPANTS}
            </div>
            {contest.is_registered && (
              <Badge variant="outline" className="bg-green-500/10 text-green-600 border-green-500/30 px-3 py-1 rounded-full font-bold">
                {TEXT.CONTEST.REGISTERED}
              </Badge>
            )}
          </div>

          <h3 className="text-3xl font-black text-foreground group-hover:text-amber-500 transition-colors tracking-tighter">
            {contest.title}
          </h3>

          <div className="flex flex-wrap items-center justify-center lg:justify-start gap-8 text-base text-muted-foreground font-medium">
            <div className="flex items-center gap-3 bg-background/40 backdrop-blur-md px-4 py-2 rounded-2xl border border-border/20">
              <Calendar className="h-5 w-5 text-amber-500" />
              <span className="font-mono text-sm">{formatTime(contest.start_time)}</span>
              <span className="text-zinc-500">→</span>
              <span className="font-mono text-sm">{formatTime(contest.end_time)}</span>
            </div>
            <div className="flex items-center gap-3">
              <Clock className="h-5 w-5 text-amber-500" />
              <span className="font-mono text-sm">{TEXT.CONTEST.PROBLEMS}: {contest.problem_ids?.length ?? 0}</span>
            </div>
          </div>
        </div>

        <div className="shrink-0 w-full lg:w-auto">
          {isEnded ? (
            <button className="w-full px-12 py-5 rounded-2xl font-black transition-all shadow-2xl tracking-widest uppercase text-sm bg-zinc-800 text-zinc-400">
              XEM KẾT QUẢ
            </button>
          ) : contest.is_registered ? (
            <button
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); onUnregister?.(contest.id); }}
              className="w-full px-12 py-5 rounded-2xl font-black transition-all active:scale-95 shadow-2xl tracking-widest uppercase text-sm bg-red-500/10 text-red-500 border border-red-500/30 hover:bg-red-500/20"
            >
              {TEXT.CONTEST.UNREGISTER}
            </button>
          ) : (
            <button
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); onRegister?.(contest.id); }}
              className={cn(
                "w-full px-12 py-5 rounded-2xl font-black transition-all active:scale-95 shadow-2xl tracking-widest uppercase text-sm",
                isActive || isUpcoming
                  ? "bg-amber-500 text-black hover:bg-amber-400 hover:shadow-amber-500/40"
                  : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
              )}
            >
              {isActive ? "THAM GIA NGAY" : "ĐĂNG KÝ TRƯỚC"}
            </button>
          )}
        </div>
      </div>

      {/* Decorative background elements */}
      {isActive && (
        <>
          <div className="absolute top-0 right-0 p-6 opacity-[0.05] transition-transform duration-1000 group-hover:scale-150 group-hover:-rotate-45">
            <Swords className="h-32 w-32 text-amber-500" />
          </div>
          <div className="absolute -left-10 -bottom-10 h-40 w-40 bg-amber-500/10 blur-[60px] rounded-full group-hover:bg-amber-500/20 transition-all duration-700" />
        </>
      )}
    </div>
    </Link>
  );
}
