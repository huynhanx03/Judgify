/**
 * Single contest card with status, difficulty, participants, and CTA.
 */

import { Badge } from "@/components/ui/badge";
import { Trophy, Users, Clock, Swords, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";
import type { Contest } from "@/types/contest";

interface ContestCardProps {
  contest: Contest;
  index: number;
}

export function ContestCard({ contest, index }: ContestCardProps) {
  return (
    <div
      className={cn(
        "group relative overflow-hidden rounded-[2.5rem] border transition-all duration-700 hover:-translate-y-1 animate-in fade-in slide-in-from-bottom-10",
        contest.isActive
          ? "bg-gradient-to-r from-amber-600/15 via-amber-600/5 to-transparent border-amber-500/40 shadow-[0_20px_40px_-15px_rgba(245,158,11,0.1)]"
          : "bg-card/40 border-border/40"
      )}
      style={{ animationDelay: `${index * 150}ms` }}
    >
      <div className="flex flex-col lg:flex-row items-center p-10 gap-10 relative z-10">
        <div className={cn(
          "h-32 w-32 rounded-[2rem] flex items-center justify-center shrink-0 shadow-2xl relative transition-transform duration-500 group-hover:scale-110 group-hover:rotate-3",
          contest.isActive
            ? "bg-gradient-to-br from-amber-400 to-amber-600 text-black"
            : "bg-muted text-muted-foreground border border-border/40"
        )}>
          <Trophy className="h-14 w-14" />
          {contest.isActive && <Sparkles className="absolute -top-2 -right-2 h-8 w-8 text-amber-300 animate-pulse" />}
        </div>

        <div className="flex-1 text-center lg:text-left space-y-4">
          <div className="flex flex-wrap items-center justify-center lg:justify-start gap-4">
            <Badge variant={contest.isActive ? "default" : "secondary"} className={cn(
              "font-black uppercase tracking-widest px-4 py-1.5 rounded-lg text-xs shadow-lg",
              contest.isActive ? "bg-amber-500 animate-pulse border-amber-400" : ""
            )}>
              {contest.status}
            </Badge>
            <div className="flex items-center gap-2 text-sm text-muted-foreground font-bold bg-muted/50 px-3 py-1 rounded-full">
              <Users className="h-4 w-4" />
              {contest.participants.toLocaleString()} Đạo Hữu
            </div>
            <Badge className={cn("rounded-full font-bold px-3 py-1",
              contest.difficulty === "Insane" ? "bg-red-500 text-white" :
              contest.difficulty === "Hard" ? "bg-orange-500 text-white" : "bg-blue-500 text-white")}>
              {contest.difficulty}
            </Badge>
          </div>

          <h3 className="text-3xl font-black text-foreground group-hover:text-amber-500 transition-colors tracking-tighter">
            {contest.title}
          </h3>

          <div className="flex flex-wrap items-center justify-center lg:justify-start gap-8 text-base text-muted-foreground font-medium">
            <div className="flex items-center gap-3 bg-background/40 backdrop-blur-md px-4 py-2 rounded-2xl border border-border/20">
              <Clock className="h-5 w-5 text-amber-500" />
              <span className="font-mono">{contest.isActive ? `Còn lại: ${contest.timeLeft}` : `Bắt đầu: ${contest.startTime}`}</span>
            </div>
            <div className="flex items-center gap-3">
              <span className="text-zinc-500">Phần thưởng:</span>
              <span className="text-amber-500 font-black text-lg drop-shadow-[0_0_8px_rgba(245,158,11,0.3)]">{contest.prize}</span>
            </div>
          </div>
        </div>

        <div className="shrink-0 w-full lg:w-auto">
          <button className={cn(
            "w-full px-12 py-5 rounded-2xl font-black transition-all active:scale-95 shadow-2xl tracking-widest uppercase text-sm",
            contest.isActive
              ? "bg-amber-500 text-black hover:bg-amber-400 hover:shadow-amber-500/40"
              : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
          )}>
            {contest.isActive ? "THAM GIA NGAY" : "ĐĂNG KÝ TRƯỚC"}
          </button>
        </div>
      </div>

      {/* Decorative background elements */}
      {contest.isActive && (
        <>
          <div className="absolute top-0 right-0 p-6 opacity-[0.05] transition-transform duration-1000 group-hover:scale-150 group-hover:-rotate-45">
            <Swords className="h-32 w-32 text-amber-500" />
          </div>
          <div className="absolute -left-10 -bottom-10 h-40 w-40 bg-amber-500/10 blur-[60px] rounded-full group-hover:bg-amber-500/20 transition-all duration-700" />
        </>
      )}
    </div>
  );
}
