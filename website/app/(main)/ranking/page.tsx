"use client";

import { useEffect, useState } from "react";
import { TEXT } from "@/constants/text";
import { Crown, Medal, User, Loader2, Sparkles, TrendingUp } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { rankingService } from "@/services/ranking.service";
import type { Cultivator, Leaderboard } from "@/types/ranking";
import Image from "next/image";

export default function RankingPage() {
  const [data, setData] = useState<Leaderboard | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const result = await rankingService.getLeaderboard();
        setData(result);
      } catch (error) {
        console.error("Failed to fetch ranking data:", error);
      } finally {
        setIsLoading(false);
      }
    };
    fetchData();
  }, []);

  if (isLoading || !data) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Loader2 className="h-10 w-10 text-primary animate-spin" />
      </div>
    );
  }

  const { topThree, others } = data;

  return (
    <div className="space-y-16 pb-24">
      {/* Hero Section */}
      <div className="relative h-[500px] w-full overflow-hidden rounded-[4rem] border border-white/10 shadow-2xl">
        <Image 
          src="/images/ranking-bg.png" 
          alt="Celestial Heaven" 
          fill 
          className="object-cover object-center brightness-[0.4] transition-transform duration-[3000ms] group-hover:scale-110"
          priority
        />
        <div className="absolute inset-0 bg-gradient-to-t from-background via-black/20 to-transparent" />
        
        {/* Divine light effect */}
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-full h-[300px] bg-gradient-to-b from-white/10 to-transparent blur-3xl opacity-50" />

        <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-8 space-y-6">
          <Badge className="bg-white/10 backdrop-blur-xl text-white border-white/20 px-6 py-2 text-sm font-black tracking-widest uppercase animate-in fade-in zoom-in duration-1000">
            VẠN CỔ TIÊN BẢNG
          </Badge>
          <div className="space-y-2">
            <h1 className="text-6xl md:text-8xl font-playfair italic font-bold tracking-tight text-white drop-shadow-[0_0_40px_rgba(255,255,255,0.4)] animate-in slide-in-from-bottom-10 duration-700">
              {TEXT.NAV.RANKING}
            </h1>
            <p className="text-zinc-300 text-xl md:text-2xl max-w-3xl mx-auto leading-relaxed animate-in slide-in-from-bottom-12 duration-700 delay-150">
              Nơi vinh danh những bậc đại năng có căn cốt phi phàm, khắc tên vào sử sách của giới tu chân.
            </p>
          </div>
          <div className="flex gap-1 animate-in fade-in duration-1000 delay-300">
            {[1, 2, 3, 4, 5].map((i) => (
              <Sparkles key={i} className="h-5 w-5 text-amber-400 animate-pulse" style={{ animationDelay: `${i * 200}ms` }} />
            ))}
          </div>
        </div>
      </div>

      {/* Top 3 Podium - Enhanced 3D Presence */}
      <div className="flex flex-col md:flex-row items-end justify-center gap-8 px-4 max-w-6xl mx-auto relative mt-[-100px] z-10">
        {/* Rank 2 */}
        <div className="w-full md:w-64 order-2 md:order-1 animate-in fade-in slide-in-from-bottom-12 duration-1000 delay-200">
          <div className="relative group text-center mb-6">
             <div className="absolute inset-0 bg-slate-500/20 blur-2xl rounded-full scale-150 opacity-0 group-hover:opacity-100 transition-opacity" />
             <Avatar className="h-28 w-28 mx-auto border-4 border-slate-400/40 shadow-[0_20px_40px_rgba(0,0,0,0.5)] ring-4 ring-slate-400/10 group-hover:scale-110 transition-all duration-500 relative z-10">
                <AvatarFallback className="bg-slate-500/20 text-slate-400 text-3xl font-black">2</AvatarFallback>
             </Avatar>
             <div className="absolute -bottom-4 left-1/2 -translate-x-1/2 z-20">
               <div className="bg-slate-400 text-black px-4 py-1 rounded-lg font-black text-sm shadow-lg">HẠNG 2</div>
             </div>
          </div>
          <div className="bg-gradient-to-b from-slate-500/10 to-card border border-slate-400/20 rounded-t-[2.5rem] p-8 h-48 flex flex-col items-center justify-end shadow-2xl backdrop-blur-xl relative overflow-hidden group">
             <div className="absolute inset-0 bg-slate-500/5 translate-y-full group-hover:translate-y-0 transition-transform duration-700" />
             <p className="font-black text-xl truncate w-full relative z-10">{topThree[1].name}</p>
             <p className="text-primary text-sm font-bold mb-3 relative z-10 uppercase tracking-tighter">{topThree[1].realm}</p>
             <div className="px-5 py-1.5 rounded-full bg-slate-500/20 text-slate-400 font-black text-sm relative z-10 border border-slate-400/10">
                {topThree[1].points.toLocaleString()} Pts
             </div>
          </div>
        </div>

        {/* Rank 1 - Supreme Godhood */}
        <div className="w-full md:w-80 order-1 md:order-2 animate-in fade-in slide-in-from-bottom-20 duration-1000">
          <div className="relative group text-center mb-10">
             <div className="absolute inset-0 bg-amber-500/30 blur-[60px] rounded-full scale-150 animate-pulse" />
             <Avatar className="h-40 w-40 mx-auto border-[6px] border-amber-400 shadow-[0_30px_60px_rgba(245,158,11,0.4)] ring-[12px] ring-amber-400/10 group-hover:scale-110 transition-all duration-700 relative z-10">
                <AvatarImage src={topThree[0].avatar} className="object-cover" />
                <AvatarFallback className="bg-amber-500/20 text-amber-500 text-5xl font-black italic">1</AvatarFallback>
             </Avatar>
             <Crown className="absolute -top-16 left-1/2 -translate-x-1/2 h-24 w-24 text-amber-400 fill-current drop-shadow-[0_0_30px_rgba(245,158,11,0.8)] animate-bounce" />
             <div className="absolute -bottom-6 left-1/2 -translate-x-1/2 z-20">
               <div className="bg-amber-400 text-black px-8 py-2 rounded-xl font-black text-lg shadow-[0_10px_30px_rgba(245,158,11,0.5)] rotate-[-2deg]">THIÊN TÔN</div>
             </div>
          </div>
          <div className="bg-gradient-to-b from-amber-500/20 via-card to-card border border-amber-500/40 rounded-t-[3.5rem] p-10 h-72 flex flex-col items-center justify-end shadow-[0_40px_80px_rgba(0,0,0,0.6)] backdrop-blur-2xl relative overflow-hidden">
             <div className="absolute top-0 inset-x-0 h-1 bg-gradient-to-r from-transparent via-amber-400 to-transparent" />
             <Sparkles className="absolute top-8 left-8 h-8 w-8 text-amber-500/30 animate-pulse" />
             <Sparkles className="absolute bottom-12 right-12 h-6 w-6 text-amber-500/20 animate-pulse delay-500" />
             
             <p className="font-black text-3xl truncate w-full mb-1 tracking-tighter text-transparent bg-clip-text bg-gradient-to-r from-white via-amber-200 to-white">{topThree[0].name}</p>
             <p className="text-amber-500 text-lg font-black mb-5 tracking-widest uppercase italic drop-shadow-sm">{topThree[0].realm}</p>
             <div className="px-10 py-3 rounded-2xl bg-gradient-to-r from-amber-500 to-amber-600 text-black font-black text-xl shadow-[0_0_30px_rgba(245,158,11,0.5)] hover:scale-105 transition-transform cursor-default">
                {topThree[0].points.toLocaleString()} Pts
             </div>
          </div>
        </div>

        {/* Rank 3 */}
        <div className="w-full md:w-64 order-3 animate-in fade-in slide-in-from-bottom-12 duration-1000 delay-400">
           <div className="relative group text-center mb-6">
             <div className="absolute inset-0 bg-orange-500/20 blur-2xl rounded-full scale-150 opacity-0 group-hover:opacity-100 transition-opacity" />
             <Avatar className="h-28 w-28 mx-auto border-4 border-orange-400/40 shadow-[0_20px_40px_rgba(0,0,0,0.5)] ring-4 ring-orange-400/10 group-hover:scale-110 transition-all duration-500 relative z-10">
                <AvatarFallback className="bg-orange-500/20 text-orange-400 text-3xl font-black">3</AvatarFallback>
             </Avatar>
             <div className="absolute -bottom-4 left-1/2 -translate-x-1/2 z-20">
               <div className="bg-orange-500 text-black px-4 py-1 rounded-lg font-black text-sm shadow-lg">HẠNG 3</div>
             </div>
          </div>
          <div className="bg-gradient-to-b from-orange-500/10 to-card border border-orange-400/20 rounded-t-[2.5rem] p-8 h-40 flex flex-col items-center justify-end shadow-2xl backdrop-blur-xl relative overflow-hidden group">
             <p className="font-black text-lg truncate w-full">{topThree[2].name}</p>
             <p className="text-primary text-sm font-bold mb-3 uppercase tracking-tighter">{topThree[2].realm}</p>
             <div className="px-5 py-1.5 rounded-full bg-orange-500/20 text-orange-500 font-black text-sm border border-orange-500/10">
                {topThree[2].points.toLocaleString()} Pts
             </div>
          </div>
        </div>
      </div>

      {/* Others Table - Clean & High Spec */}
      <div className="max-w-5xl mx-auto px-4 w-full">
         <div className="flex items-center gap-4 mb-8">
            <TrendingUp className="h-8 w-8 text-primary" />
            <h2 className="text-3xl font-black tracking-tighter uppercase italic">Quần Anh Hội Tụ</h2>
            <div className="flex-1 h-px bg-gradient-to-r from-border/60 to-transparent" />
         </div>
         
         <div className="rounded-[2.5rem] border border-border/40 bg-card/40 backdrop-blur-2xl overflow-hidden shadow-2xl animate-in fade-in slide-in-from-bottom-8 duration-1000">
            <table className="w-full text-left">
              <thead>
                <tr className="border-b border-border/40 bg-muted/40 backdrop-blur-md">
                  <th className="px-8 py-5 font-black text-xs uppercase tracking-[0.2em] text-muted-foreground w-28">Thứ Hạng</th>
                  <th className="px-8 py-5 font-black text-xs uppercase tracking-[0.2em] text-muted-foreground">Vị Đại Năng</th>
                  <th className="px-8 py-5 font-black text-xs uppercase tracking-[0.2em] text-muted-foreground text-center">Cảnh Giới</th>
                  <th className="px-8 py-5 font-black text-xs uppercase tracking-[0.2em] text-muted-foreground text-right w-40">Linh Lực</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/20">
                {others.map((user) => (
                  <tr key={user.rank} className="group hover:bg-primary/[0.03] transition-colors">
                    <td className="px-8 py-6">
                       <span className="font-mono text-xl font-black text-zinc-500 group-hover:text-primary transition-colors">
                         #{user.rank}
                       </span>
                    </td>
                    <td className="px-8 py-6">
                       <div className="flex items-center gap-4">
                         <div className="h-12 w-12 rounded-2xl bg-muted/50 border border-border/40 flex items-center justify-center text-muted-foreground group-hover:border-primary/40 group-hover:bg-primary/10 transition-all">
                            <User className="h-6 w-6 group-hover:text-primary" />
                         </div>
                         <div>
                            <p className="font-black text-lg tracking-tight group-hover:text-primary transition-colors">{user.name}</p>
                            <p className="text-xs text-muted-foreground font-bold uppercase tracking-widest">{user.sect}</p>
                         </div>
                       </div>
                    </td>
                    <td className="px-8 py-6 text-center">
                       <Badge variant="outline" className="border-primary/30 bg-primary/5 text-primary text-xs font-black uppercase rounded-full px-4 py-1 tracking-widest">
                         {user.realm}
                       </Badge>
                    </td>
                    <td className="px-8 py-6 text-right">
                       <div className="flex flex-col items-end">
                         <span className="font-black text-xl tracking-tighter group-hover:text-amber-500 transition-colors">
                           {user.points.toLocaleString()}
                         </span>
                         <span className="text-[10px] text-muted-foreground font-black uppercase tracking-widest">Linh Lực</span>
                       </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
         </div>
      </div>
    </div>
  );
}


