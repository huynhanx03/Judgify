/**
 * Hero banner for the Contest page — grand arena with call-to-action buttons.
 */

import { TEXT } from "@/constants/text";
import { Flame } from "lucide-react";
import Image from "next/image";

interface ContestHeroSectionProps {
  onExplore: () => void;
  onShowUpcoming: () => void;
}

export function ContestHeroSection({
  onExplore,
  onShowUpcoming,
}: ContestHeroSectionProps) {
  return (
    <section className="group relative h-[420px] w-full overflow-hidden rounded-[2rem] border border-primary/20 shadow-brand-hero sm:h-[450px] sm:rounded-[3.5rem]">
      <Image
        src="/images/contest-bg.png"
        alt={TEXT.CONTEST.HERO_IMAGE_ALT}
        fill
        className="object-cover object-center brightness-[0.5]"
        priority
      />
      <div className="absolute inset-0 bg-gradient-to-t from-background via-transparent to-foreground/60" />

      <div className="absolute top-1/2 left-1/2 h-full w-full -translate-x-1/2 -translate-y-1/2 bg-cultivation/10 mix-blend-overlay blur-[120px]" />

      <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-6 space-y-6">
        <div className="inline-flex items-center gap-2 rounded-full border border-cultivation/40 bg-cultivation/20 px-5 py-2 text-sm font-black tracking-widest text-cultivation uppercase animate-in fade-in zoom-in duration-300 motion-reduce:animate-none">
          <Flame className="h-4 w-4 fill-current" aria-hidden="true" />
          {TEXT.CONTEST.HERO_BADGE}
        </div>

        <div className="space-y-2 max-w-4xl">
          <h1 className="font-playfair text-5xl font-bold tracking-tight text-media-foreground italic text-shadow-brand-strong animate-in slide-in-from-bottom-8 duration-300 motion-reduce:animate-none sm:text-6xl md:text-8xl">
            {TEXT.NAV.CONTEST}
          </h1>
          <p className="mx-auto max-w-3xl text-lg leading-relaxed text-media-foreground/80 animate-in slide-in-from-bottom-10 duration-300 delay-100 motion-reduce:animate-none md:text-2xl">
            {TEXT.CONTEST.HERO_DESCRIPTION}
          </p>
        </div>

        <div className="flex w-full max-w-md flex-col gap-3 pt-6 animate-in slide-in-from-bottom-12 duration-300 delay-200 motion-reduce:animate-none sm:flex-row sm:justify-center">
          <button
            type="button"
            className="button-brand-elevation min-h-11 rounded-2xl bg-primary px-8 py-3 font-black text-primary-foreground outline-none transition-colors hover:bg-primary/90 focus-visible:ring-3 focus-visible:ring-media-foreground/70 motion-reduce:transition-none"
            onClick={onExplore}
          >
            {TEXT.CONTEST.HERO_REGISTER}
          </button>
          <button
            type="button"
            className="min-h-11 rounded-2xl border border-media-foreground/20 bg-media-foreground/10 px-8 py-3 font-black text-media-foreground outline-none backdrop-blur-xl transition-colors hover:bg-media-foreground/20 focus-visible:ring-3 focus-visible:ring-media-foreground/70 motion-reduce:transition-none"
            onClick={onShowUpcoming}
          >
            {TEXT.CONTEST.HERO_SCHEDULE}
          </button>
        </div>
      </div>
    </section>
  );
}
