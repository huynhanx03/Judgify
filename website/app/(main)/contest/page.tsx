"use client";

import { useEffect, useState } from "react";
import { TEXT } from "@/constants/text";
import { Trophy, Users, Clock, Swords, Flame, Loader2, Sparkles } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { contestService } from "@/services/contest.service";
import type { Contest } from "@/types/contest";
import { cn } from "@/lib/utils";
import Image from "next/image";

export default function ContestPage() {
  const [contests, setContests] = useState<Contest[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchContests = async () => {
      try {
        const data = await contestService.getContests();
        setContests(data);
      } catch (error) {
        console.error("Failed to fetch contests:", error);
      } finally {
        setIsLoading(false);
      }
    };
    fetchContests();
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Loader2 className="h-10 w-10 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-12 pb-20">
      {/* Hero Section */}
      <div className="relative h-[450px] w-full overflow-hidden rounded-[3.5rem] border border-amber-500/20 shadow-[0_0_50px_rgba(245,158,11,0.15)] group">
        <Image 
          src="/images/contest-bg.png" 
          alt="Grand Arena" 
          fill 
          className="object-cover object-center brightness-[0.5] transition-transform duration-[2000ms] group-hover:scale-110"
          priority
        />
        <div className="absolute inset-0 bg-gradient-to-t from-background via-transparent to-black/60" />
        
        {/* Animated ambient light */}
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full h-full bg-amber-500/10 mix-blend-overlay blur-[120px] animate-pulse" />

        <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-6 space-y-6">
          <div className="inline-flex items-center gap-2 px-5 py-2 rounded-full bg-amber-500/20 border border-amber-500/40 text-amber-500 text-sm font-black tracking-widest uppercase animate-in fade-in zoom-in duration-700">
            <Flame className="h-4 w-4 fill-current animate-pulse" />
            ĐẠI HỘI QUẦN HÙNG
          </div>
          
          <div className="space-y-2 max-w-4xl">
            <h1 className="text-6xl md:text-8xl font-playfair italic font-bold tracking-tight text-white drop-shadow-[0_0_30px_rgba(245,158,11,0.5)] animate-in slide-in-from-bottom-8 duration-700">
              {TEXT.NAV.CONTEST}
            </h1>
            <p className="text-amber-100/80 text-xl md:text-2xl max-w-3xl mx-auto leading-relaxed animate-in slide-in-from-bottom-10 duration-700 delay-100">
              Nơi vạn tông hội tụ, thiên tài tranh phong. Khai mở bí cảnh, đoạt lấy thiên địa linh bảo và ghi danh trên Vạn Cổ Tiên Bảng.
            </p>
          </div>

          <div className="flex gap-4 pt-6 animate-in slide-in-from-bottom-12 duration-700 delay-200">
            <button className="px-8 py-4 bg-amber-500 text-black font-black rounded-2xl shadow-[0_0_20px_rgba(245,158,11,0.4)] hover:shadow-[0_0_30px_rgba(245,158,11,0.6)] hover:scale-105 active:scale-95 transition-all">
              ĐĂNG KÝ NGAY
            </button>
            <button className="px-8 py-4 bg-white/10 backdrop-blur-xl border border-white/20 text-white font-black rounded-2xl hover:bg-white/20 transition-all">
              XEM LỊCH THI ĐẤU
            </button>
          </div>
        </div>
      </div>

      <div className="max-w-6xl mx-auto px-4 space-y-10">
        <div className="flex items-center justify-between border-b border-border/40 pb-6">
          <div className="flex items-center gap-3">
            <Trophy className="h-8 w-8 text-amber-500" />
            <h2 className="text-3xl font-black tracking-tight">{TEXT.ARENA.FILTER_TITLE}</h2>
          </div>
          <div className="flex gap-2">
            <Badge variant="secondary" className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-0 px-4 py-1.5 rounded-full font-bold">Tất Cả</Badge>
            <Badge variant="outline" className="px-4 py-1.5 rounded-full font-bold border-border/40 hover:bg-muted transition-colors">Đang Diễn Ra</Badge>
            <Badge variant="outline" className="px-4 py-1.5 rounded-full font-bold border-border/40 hover:bg-muted transition-colors">Sắp Tới</Badge>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-8">
          {contests.map((contest, index) => (
            <div 
              key={contest.id}
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
          ))}
        </div>
      </div>
    </div>
  );
}


