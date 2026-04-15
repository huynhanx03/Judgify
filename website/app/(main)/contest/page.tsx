"use client";

/**
 * Contest page — list of contests with hero banner.
 * Uses real API instead of mock data.
 */

import { useEffect, useState } from "react";
import { TEXT } from "@/constants/text";
import { Trophy, Search } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { getContests, registerContest, unregisterContest } from "@/services/contest.service";
import { notify, getErrorMessage } from "@/lib/toast";
import type { Contest, ContestStatus } from "@/types/contest";
import { ContestHeroSection } from "@/modules/contest/contest-hero-section";
import { ContestCard } from "@/modules/contest/contest-card";

type StatusFilter = ContestStatus | "all";

const STATUS_FILTERS: { value: StatusFilter; label: string }[] = [
  { value: "all", label: "Tất Cả" },
  { value: "running", label: "Đang Diễn Ra" },
  { value: "upcoming", label: "Sắp Tới" },
  { value: "ended", label: "Đã Kết Thúc" },
];

export default function ContestPage() {
  const [contests, setContests] = useState<Contest[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");

  useEffect(() => {
    fetchContests();
  }, []);

  async function fetchContests() {
    try {
      const res = await getContests({
        pagination: { page: 1, page_size: 50 },
      });
      setContests(res.records ?? []);
    } catch (error) {
      console.error("Failed to fetch contests:", error);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleRegister(id: number) {
    try {
      await registerContest(id);
      notify.success(TEXT.CONTEST.REGISTER_SUCCESS);
      fetchContests();
    } catch (error) {
      notify.error(getErrorMessage(error, "Đăng ký thất bại"));
    }
  }

  async function handleUnregister(id: number) {
    try {
      await unregisterContest(id);
      notify.success(TEXT.CONTEST.UNREGISTER_SUCCESS);
      fetchContests();
    } catch (error) {
      notify.error(getErrorMessage(error, "Hủy đăng ký thất bại"));
    }
  }

  const filtered = contests.filter((c) => {
    if (statusFilter !== "all" && c.status !== statusFilter) return false;
    if (search && !c.title.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  if (isLoading) return <LoadingSpinner />;

  return (
    <div className="space-y-12 pb-20">
      <ContestHeroSection />

      <div className="max-w-6xl mx-auto px-4 space-y-10">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between border-b border-border/40 pb-6 gap-4">
          <div className="flex items-center gap-3">
            <Trophy className="h-8 w-8 text-amber-500" />
            <h2 className="text-3xl font-black tracking-tight">{TEXT.CONTEST.TITLE}</h2>
          </div>
          <div className="flex flex-wrap gap-2">
            {STATUS_FILTERS.map((f) => (
              <Badge
                key={f.value}
                variant={statusFilter === f.value ? "default" : "outline"}
                className={
                  statusFilter === f.value
                    ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-0 px-4 py-1.5 rounded-full font-bold cursor-pointer"
                    : "px-4 py-1.5 rounded-full font-bold border-border/40 hover:bg-muted transition-colors cursor-pointer"
                }
                onClick={() => setStatusFilter(f.value)}
              >
                {f.label}
              </Badge>
            ))}
          </div>
        </div>

        <div className="relative max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={TEXT.CONTEST.SEARCH_PLACEHOLDER}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-10 rounded-full"
          />
        </div>

        <div className="grid grid-cols-1 gap-8">
          {filtered.map((contest, index) => (
            <ContestCard
              key={contest.id}
              contest={contest}
              index={index}
              onRegister={handleRegister}
              onUnregister={handleUnregister}
            />
          ))}
        </div>

        {filtered.length === 0 && (
          <div className="text-center py-20 text-muted-foreground">
            <p className="text-xl font-bold">{TEXT.CONTEST.NO_CONTESTS}</p>
            <p className="text-sm mt-2">{TEXT.CONTEST.NO_CONTESTS_DESC}</p>
          </div>
        )}
      </div>
    </div>
  );
}
