"use client";

/**
 * Contest page — list of contests with hero banner.
 * Components extracted to modules/contest/.
 */

import { useEffect, useState } from "react";
import { TEXT } from "@/constants/text";
import { Trophy, Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { contestService } from "@/services/contest.service";
import type { Contest } from "@/types/contest";
import { ContestHeroSection } from "@/modules/contest/contest-hero-section";
import { ContestCard } from "@/modules/contest/contest-card";

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
      <ContestHeroSection />

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
            <ContestCard key={contest.id} contest={contest} index={index} />
          ))}
        </div>
      </div>
    </div>
  );
}
