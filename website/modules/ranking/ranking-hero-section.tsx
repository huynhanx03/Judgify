/**
 * Hero section for the Ranking page — celestial banner with title and sparkles.
 */

import { TEXT } from "@/constants/text";
import { Badge } from "@/components/ui/badge";
import { Sparkles } from "lucide-react";
import Image from "next/image";

export function RankingHeroSection() {
  return (
    <div className="relative h-[420px] w-full overflow-hidden rounded-[2rem] border border-white/10 shadow-2xl sm:h-[500px] sm:rounded-[4rem]">
      <Image
        src="/images/ranking-bg.png"
        alt={TEXT.RANKING.HERO_IMAGE_ALT}
        fill
        className="object-cover object-center brightness-[0.4]"
        priority
      />
      <div className="absolute inset-0 bg-gradient-to-t from-background via-black/20 to-transparent" />

      {/* Divine light effect */}
      <div className="absolute top-0 left-1/2 -translate-x-1/2 w-full h-[300px] bg-gradient-to-b from-white/10 to-transparent blur-3xl opacity-50" />

      <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-8 space-y-6">
        <Badge className="animate-in fade-in zoom-in border-white/20 bg-white/10 px-6 py-2 text-sm font-black uppercase tracking-widest text-white backdrop-blur-xl duration-300 motion-reduce:animate-none">
          {TEXT.RANKING.HERO_BADGE}
        </Badge>
        <div className="space-y-2">
          <h1 className="animate-in slide-in-from-bottom-10 font-playfair text-4xl font-bold italic tracking-tight text-media-foreground text-shadow-media duration-300 motion-reduce:animate-none sm:text-6xl md:text-8xl">
            {TEXT.NAV.RANKING}
          </h1>
          <p className="mx-auto max-w-3xl animate-in slide-in-from-bottom-12 text-base leading-relaxed text-zinc-300 duration-300 motion-reduce:animate-none sm:text-xl md:text-2xl">
            {TEXT.RANKING.HERO_DESC}
          </p>
        </div>
        <div className="flex gap-1 animate-in fade-in duration-300 motion-reduce:animate-none">
          {[1, 2, 3, 4, 5].map((i) => (
            <Sparkles key={i} className="h-5 w-5 text-amber-400" aria-hidden="true" />
          ))}
        </div>
      </div>
    </div>
  );
}
