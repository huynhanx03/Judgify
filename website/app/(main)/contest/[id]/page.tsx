"use client";

/**
 * Contest detail page — info + realtime ICPC standings via SSE + rating changes.
 * Tabs: Thong Tin / Bang Xep Hang (SSE) / Bien Dong Rating (ended only).
 */

import { useState, useEffect, use, useCallback } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/loading-spinner";
import {
  getContestById,
  registerContest,
  unregisterContest,
  getContestRatingChanges,
} from "@/services/contest.service";
import { useContestSSE } from "@/hooks/use-contest-sse";
import { notify, getErrorMessage } from "@/lib/toast";
import { TEXT } from "@/constants/text";
import type { Contest, RatingChange } from "@/types/contest";
import {
  ArrowLeft,
  Calendar,
  Clock,
  Trophy,
  Users,
  FileCode2,
  Swords,
  Info,
  UserX,
  CheckCircle2,
  Wifi,
  WifiOff,
  TrendingUp,
} from "lucide-react";

const STATUS_STYLES: Record<string, string> = {
  draft: "bg-zinc-500/10 text-zinc-500 border-zinc-500/20",
  upcoming: "bg-blue-500/10 text-blue-500 border-blue-500/20",
  running: "bg-green-500/10 text-green-500 border-green-500/20",
  ended: "bg-zinc-500/10 text-zinc-500 border-zinc-500/20",
};

const STATUS_LABELS: Record<string, string> = {
  draft: TEXT.CONTEST.STATUS_DRAFT,
  upcoming: TEXT.CONTEST.STATUS_UPCOMING,
  running: TEXT.CONTEST.STATUS_RUNNING,
  ended: TEXT.CONTEST.STATUS_ENDED,
};

type Tab = "info" | "standings" | "rating";

export default function ContestDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const contestId = Number(id);

  const [contest, setContest] = useState<Contest | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<Tab>("info");
  const [isRegistering, setIsRegistering] = useState(false);
  const [ratingChanges, setRatingChanges] = useState<RatingChange[]>([]);

  // SSE: only connect when standings tab is active
  const { standings, isConnected } = useContestSSE(
    contestId,
    activeTab === "standings"
  );

  useEffect(() => {
    async function loadContest() {
      try {
        const data = await getContestById(contestId);
        setContest(data);
      } catch {
        // contest stays null
      } finally {
        setIsLoading(false);
      }
    }
    loadContest();
  }, [contestId]);

  // Load rating changes when tab is active and contest is ended
  const loadRatingChanges = useCallback(async () => {
    if (activeTab !== "rating" || !contest || contest.status !== "ended") return;
    try {
      const data = await getContestRatingChanges(contestId);
      setRatingChanges(data);
    } catch {
      // rating changes stay empty
    }
  }, [activeTab, contest, contestId]);

  useEffect(() => {
    loadRatingChanges();
  }, [loadRatingChanges]);

  async function handleRegister() {
    setIsRegistering(true);
    try {
      await registerContest(contestId);
      notify.success(TEXT.CONTEST.REGISTER_SUCCESS);
      const data = await getContestById(contestId);
      setContest(data);
    } catch (err) {
      notify.error(getErrorMessage(err, "Đăng ký thất bại"));
    } finally {
      setIsRegistering(false);
    }
  }

  async function handleUnregister() {
    setIsRegistering(true);
    try {
      await unregisterContest(contestId);
      notify.success(TEXT.CONTEST.UNREGISTER_SUCCESS);
      const data = await getContestById(contestId);
      setContest(data);
    } catch (err) {
      notify.error(getErrorMessage(err, "Hủy đăng ký thất bại"));
    } finally {
      setIsRegistering(false);
    }
  }

  function formatTime(iso: string) {
    return new Date(iso).toLocaleDateString("vi-VN", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function getDuration(start: string, end: string): string {
    const ms = new Date(end).getTime() - new Date(start).getTime();
    const h = Math.floor(ms / 3600000);
    const m = Math.floor((ms % 3600000) / 60000);
    return `${h}h${m > 0 ? ` ${m}m` : ""}`;
  }

  if (isLoading) return <LoadingSpinner />;

  if (!contest) {
    return (
      <div className="text-center py-20 space-y-4">
        <p className="text-lg text-muted-foreground">Không tìm thấy cuộc thi</p>
        <Link href="/contest" className="text-primary hover:underline">
          {TEXT.CONTEST.BACK_TO_LIST}
        </Link>
      </div>
    );
  }

  const isActive = contest.status === "running";
  const isUpcoming = contest.status === "upcoming";
  const isEnded = contest.status === "ended";

  return (
    <div className="space-y-4 animate-in fade-in duration-300">
      <Card className="glass-card border-border/40 overflow-hidden flex flex-col min-h-[calc(100vh-180px)]">
        {/* Top bar */}
        <div className="px-6 py-4 border-b border-border/40 bg-muted/20">
          <div className="flex items-center justify-between gap-4 flex-wrap">
            <div className="flex items-center gap-3">
              <Link
                href="/contest"
                className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <ArrowLeft className="h-4 w-4" />
                {TEXT.CONTEST.BACK_TO_LIST}
              </Link>
            </div>
            <div className="flex items-center gap-3">
              <Badge variant="outline" className={STATUS_STYLES[contest.status] ?? ""}>
                {STATUS_LABELS[contest.status] ?? contest.status}
              </Badge>
              {contest.is_registered ? (
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="bg-green-500/10 text-green-600 border-green-500/30">
                    <CheckCircle2 className="h-3 w-3 mr-1" />
                    {TEXT.CONTEST.REGISTERED}
                  </Badge>
                  {(isUpcoming || isActive) && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-red-500 border-red-500/30 hover:bg-red-500/10"
                      onClick={handleUnregister}
                      disabled={isRegistering}
                    >
                      <UserX className="h-3.5 w-3.5 mr-1" />
                      {TEXT.CONTEST.UNREGISTER}
                    </Button>
                  )}
                </div>
              ) : (
                (isUpcoming || isActive) && (
                  <Button size="sm" onClick={handleRegister} disabled={isRegistering}>
                    {TEXT.CONTEST.REGISTER}
                  </Button>
                )
              )}
            </div>
          </div>
        </div>

        {/* Header */}
        <div className="px-6 py-5 border-b border-border/40 space-y-3">
          <h1 className="text-2xl font-bold">{contest.title}</h1>
          <div className="flex flex-wrap items-center gap-5 text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Calendar className="h-4 w-4 text-amber-500" />
              <span>{formatTime(contest.start_time)} → {formatTime(contest.end_time)}</span>
            </div>
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-amber-500" />
              <span>{getDuration(contest.start_time, contest.end_time)}</span>
            </div>
            <div className="flex items-center gap-2">
              <Users className="h-4 w-4 text-amber-500" />
              <span>{contest.participant_count}{contest.max_participants > 0 ? `/${contest.max_participants}` : ""} {TEXT.CONTEST.PARTICIPANTS}</span>
            </div>
            <div className="flex items-center gap-2">
              <FileCode2 className="h-4 w-4 text-amber-500" />
              <span>{TEXT.CONTEST.PROBLEMS}: {contest.problem_ids?.length ?? 0}</span>
            </div>
          </div>
        </div>

        {/* Tab bar */}
        <div className="flex border-b border-border/40 bg-muted/10 px-2">
          <button
            onClick={() => setActiveTab("info")}
            className={`flex items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer ${
              activeTab === "info"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Info className="h-4 w-4" />
            {TEXT.CONTEST.DETAIL_INFO}
          </button>
          <button
            onClick={() => setActiveTab("standings")}
            className={`flex items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer ${
              activeTab === "standings"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Trophy className="h-4 w-4" />
            {TEXT.CONTEST.STANDINGS}
            {activeTab === "standings" && (
              isConnected ? (
                <Wifi className="h-3.5 w-3.5 text-green-500" />
              ) : (
                <WifiOff className="h-3.5 w-3.5 text-muted-foreground animate-pulse" />
              )
            )}
          </button>
          {isEnded && (
            <button
              onClick={() => setActiveTab("rating")}
              className={`flex items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer ${
                activeTab === "rating"
                  ? "text-primary border-b-2 border-primary"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <TrendingUp className="h-4 w-4" />
              {TEXT.CONTEST.RATING_CHANGES}
            </button>
          )}
        </div>

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto p-6">
          {activeTab === "info" && (
            <div className="space-y-6 max-w-3xl">
              {contest.description ? (
                <div>
                  <h3 className="text-lg font-bold mb-3">{TEXT.CONTEST.DETAIL_DESCRIPTION}</h3>
                  <p className="text-muted-foreground whitespace-pre-wrap leading-relaxed">
                    {contest.description}
                  </p>
                </div>
              ) : (
                <p className="text-muted-foreground italic">Chưa có mô tả.</p>
              )}

              <div className="h-px bg-border/40" />

              {/* Problem list */}
              <div>
                <h3 className="text-lg font-bold mb-3">{TEXT.CONTEST.PROBLEMS}</h3>
                {contest.problem_ids && contest.problem_ids.length > 0 ? (
                  <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                    {contest.problem_ids.map((pid, idx) => (
                      <Link
                        key={pid}
                        href={`/arena/${pid}`}
                        className="flex items-center gap-3 p-4 rounded-xl border border-border/40 bg-muted/20 hover:bg-muted/40 transition-colors"
                      >
                        <div className="h-10 w-10 rounded-lg bg-amber-500/10 text-amber-500 flex items-center justify-center font-bold text-sm">
                          {String.fromCharCode(65 + idx)}
                        </div>
                        <div>
                          <p className="font-medium text-sm">Bài #{pid}</p>
                          <p className="text-xs text-muted-foreground">Problem ID: {pid}</p>
                        </div>
                      </Link>
                    ))}
                  </div>
                ) : (
                  <p className="text-muted-foreground italic">Chưa có bài tập.</p>
                )}
              </div>
            </div>
          )}

          {activeTab === "standings" && (
            standings.length === 0 ? (
              <div className="text-center py-16 text-muted-foreground">
                <Swords className="h-12 w-12 mx-auto mb-3 opacity-30" />
                <p className="text-lg font-medium">Chưa có dữ liệu xếp hạng</p>
                <p className="text-sm mt-1">Bảng xếp hạng sẽ hiển thị khi cuộc thi bắt đầu.</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border/40">
                      <th className="text-left py-3 px-4 font-bold text-muted-foreground w-16">{TEXT.CONTEST.RANK}</th>
                      <th className="text-left py-3 px-4 font-bold text-muted-foreground">{TEXT.CONTEST.USER}</th>
                      <th className="text-center py-3 px-4 font-bold text-muted-foreground w-20">{TEXT.CONTEST.SOLVED}</th>
                      <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.PENALTY}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {standings.map((s) => (
                      <tr key={s.user_id} className="border-b border-border/20 hover:bg-muted/20 transition-colors">
                        <td className="py-3 px-4">
                          <span className={`font-bold ${s.rank <= 3 ? "text-amber-500" : ""}`}>
                            {s.rank <= 3 ? ["🥇", "🥈", "🥉"][s.rank - 1] : s.rank}
                          </span>
                        </td>
                        <td className="py-3 px-4">
                          <span className="font-medium">{s.username}</span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="font-bold text-green-500">{s.solved_count}</span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="font-mono text-muted-foreground">{s.penalty}</span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          )}

          {activeTab === "rating" && isEnded && (
            ratingChanges.length === 0 ? (
              <div className="text-center py-16 text-muted-foreground">
                <TrendingUp className="h-12 w-12 mx-auto mb-3 opacity-30" />
                <p className="text-lg font-medium">{TEXT.CONTEST.NO_RATING_CHANGES}</p>
                <p className="text-sm mt-1">{TEXT.CONTEST.NO_RATING_CHANGES_DESC}</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border/40">
                      <th className="text-left py-3 px-4 font-bold text-muted-foreground w-16">{TEXT.CONTEST.RANK}</th>
                      <th className="text-left py-3 px-4 font-bold text-muted-foreground">{TEXT.CONTEST.USER}</th>
                      <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.RATING_OLD}</th>
                      <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.RATING_NEW}</th>
                      <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.RATING_DELTA}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {ratingChanges.map((rc) => (
                      <tr key={rc.user_id} className="border-b border-border/20 hover:bg-muted/20 transition-colors">
                        <td className="py-3 px-4">
                          <span className={`font-bold ${rc.rank <= 3 ? "text-amber-500" : ""}`}>
                            {rc.rank <= 3 ? ["🥇", "🥈", "🥉"][rc.rank - 1] : rc.rank}
                          </span>
                        </td>
                        <td className="py-3 px-4">
                          <span className="font-medium">{rc.username}</span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="font-mono text-muted-foreground">{rc.old_rating}</span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="font-mono font-bold">{rc.new_rating}</span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className={`font-mono font-bold ${rc.delta > 0 ? "text-green-500" : rc.delta < 0 ? "text-red-500" : "text-muted-foreground"}`}>
                            {rc.delta > 0 ? `+${rc.delta}` : rc.delta}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          )}
        </div>
      </Card>
    </div>
  );
}
