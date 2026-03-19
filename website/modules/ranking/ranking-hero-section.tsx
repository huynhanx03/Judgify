/**
 * Hero section for the Ranking page — celestial banner with title and sparkles.
 */

import { TEXT } from "@/constants/text";
import { Badge } from "@/components/ui/badge";
import { Sparkles } from "lucide-react";
import Image from "next/image";

export function RankingHeroSection() {
  return (
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
  );
}
