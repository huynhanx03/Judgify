"use client";

/**
 * Profile page — Xianxia-themed cultivation profile.
 * Components extracted to modules/profile/.
 */

import { useState, useEffect, useMemo } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { getProfile } from "@/services/user.service";
import { useAuth } from "@/hooks/use-auth";
import { TEXT } from "@/constants/text";
import type { UserProfile } from "@/types/user";
import {
  MOCK_PROFILE_STATS,
  MOCK_SPIRITUAL_ROOTS,
  MOCK_INNATE_TALENTS,
  MOCK_CONSTITUTION,
  generateMockActivity,
  getRealmByRating,
  getRankByExp,
} from "@/mock/profile-stats";
import { RatingChart } from "@/modules/profile/rating-chart";
import { CultivationPanel } from "@/modules/profile/cultivation-panel";
import { InnateTalentsCard } from "@/modules/profile/innate-talents-card";
import { ConstitutionCard } from "@/modules/profile/constitution-card";
import { ActivityHeatmap } from "@/modules/profile/activity-heatmap";
import { QuickStat, DifficultyCard } from "@/modules/profile/profile-sub-components";
import {
  Loader2,
  Swords,
  Target,
  CalendarDays,
  Flame,
  TrendingUp,
  Sparkles,
  Award,
  Shield,
  Activity,
  Settings,
  LogOut,
} from "lucide-react";

function getInitials(profile: UserProfile): string {
  return (profile.first_name[0] + profile.last_name[0]).toUpperCase();
}

export default function ProfilePage() {
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const { logout } = useAuth();
  const stats = MOCK_PROFILE_STATS;
  const realm = getRealmByRating(stats.rating);
  const rank = getRankByExp(stats.exp);
  const activityData = useMemo(() => generateMockActivity(), []);

  useEffect(() => {
    getProfile().then((data) => {
      setProfile(data);
      setIsLoading(false);
    });
  }, []);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (!profile) return null;

  return (
    <div className="space-y-8 animate-in fade-in duration-500">
      {/* Page Header */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight heading-gaming text-transparent bg-clip-text bg-gradient-to-r from-amber-500 via-amber-400 to-amber-200">
          {TEXT.PROFILE.TITLE}
        </h1>
        <p className="text-muted-foreground mt-1">{TEXT.PROFILE.SUBTITLE}</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* ====== LEFT COLUMN ====== */}
        <div className="lg:col-span-1 space-y-6">
          {/* Hero Profile Card */}
          <Card className="glass-card border-border/40 overflow-hidden relative">
            <div className={`h-28 bg-gradient-to-br ${realm.current.gradient} relative`}>
              <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:12px_12px]" />
              <div className="absolute inset-0 bg-gradient-to-t from-card/80 to-transparent" />
              <Sparkles className="absolute top-4 right-6 h-5 w-5 text-white/40 animate-pulse" />
              <Sparkles className="absolute top-8 right-16 h-3 w-3 text-white/20 animate-pulse delay-500" />
            </div>

            <div className="px-6 pb-6 -mt-12 relative z-10">
              <div className="flex items-end justify-between">
                <div className="relative inline-block">
                  <div className="absolute inset-0 rounded-full blur-xl opacity-40 scale-125" style={{ backgroundColor: realm.current.color }} />
                  <Avatar className="h-24 w-24 border-4 border-background shadow-xl relative">
                    <AvatarFallback
                      className="text-2xl font-bold heading-gaming text-white"
                      style={{ background: `linear-gradient(135deg, ${realm.current.color}, ${realm.current.color}88)` }}
                    >
                      {getInitials(profile)}
                    </AvatarFallback>
                  </Avatar>
                </div>
                <div className="flex gap-1.5">
                  <button
                    className="h-8 w-8 flex items-center justify-center rounded-lg border border-border/40 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors cursor-pointer"
                    title="Chỉnh sửa hồ sơ"
                  >
                    <Settings className="h-4 w-4" />
                  </button>
                  <button
                    onClick={logout}
                    className="h-8 w-8 flex items-center justify-center rounded-lg border border-border/40 text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
                    title="Đăng xuất"
                  >
                    <LogOut className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="mt-3 space-y-1">
                <h2 className="text-xl font-bold">{profile.first_name} {profile.last_name}</h2>
                <p className="text-sm text-muted-foreground">@{profile.username}</p>
              </div>

              <div className="flex flex-wrap gap-2 mt-3">
                <Badge className="text-white border-0 font-bold text-xs shadow-md" style={{ backgroundColor: realm.current.color }}>
                  {realm.current.name}
                </Badge>
                <Badge variant="outline" className="border-primary/30 text-primary text-xs font-bold">
                  {rank.current.name}
                </Badge>
              </div>

              <Separator className="my-4 border-border/40" />

              <div className="space-y-3">
                <QuickStat icon={<Flame className="h-4 w-4 text-orange-500" />} label="Streak" value={<span className="font-bold text-orange-500">{stats.streak} ngày</span>} />
                <QuickStat icon={<CalendarDays className="h-4 w-4 text-muted-foreground" />} label={TEXT.PROFILE.JOINED_DATE} value={<span className="font-medium">{stats.joinedDate}</span>} />
                <QuickStat icon={<Swords className="h-4 w-4 text-muted-foreground" />} label={TEXT.PROFILE.CONTESTS_JOINED} value={<span className="font-medium">{stats.contestsJoined}</span>} />
              </div>

            </div>
          </Card>

          {/* Difficulty Breakdown */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-4">
              <CardTitle className="text-lg flex items-center gap-2">
                <Target className="h-5 w-5 text-primary" />
                {TEXT.PROFILE.STATISTICS}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="flex items-center gap-5">
                {/* Donut ring */}
                <div className="relative shrink-0">
                  <svg width="80" height="80" viewBox="0 0 80 80">
                    <circle cx="40" cy="40" r="32" fill="none" stroke="currentColor" strokeWidth="6" opacity="0.08" />
                    {/* Easy arc */}
                    <circle
                      cx="40" cy="40" r="32"
                      fill="none" stroke="#10b981" strokeWidth="6"
                      strokeDasharray={`${(stats.easy / stats.problemsSolved) * 201} 201`}
                      strokeDashoffset="0"
                      transform="rotate(-90 40 40)"
                      strokeLinecap="round"
                    />
                    {/* Medium arc */}
                    <circle
                      cx="40" cy="40" r="32"
                      fill="none" stroke="#f59e0b" strokeWidth="6"
                      strokeDasharray={`${(stats.medium / stats.problemsSolved) * 201} 201`}
                      strokeDashoffset={`${-(stats.easy / stats.problemsSolved) * 201}`}
                      transform="rotate(-90 40 40)"
                      strokeLinecap="round"
                    />
                    {/* Hard arc */}
                    <circle
                      cx="40" cy="40" r="32"
                      fill="none" stroke="#f43f5e" strokeWidth="6"
                      strokeDasharray={`${(stats.hard / stats.problemsSolved) * 201} 201`}
                      strokeDashoffset={`${-((stats.easy + stats.medium) / stats.problemsSolved) * 201}`}
                      transform="rotate(-90 40 40)"
                      strokeLinecap="round"
                    />
                  </svg>
                  <div className="absolute inset-0 flex flex-col items-center justify-center">
                    <span className="text-xl font-black text-primary leading-none">{stats.problemsSolved}</span>
                    <span className="text-[9px] text-muted-foreground">bài</span>
                  </div>
                </div>
                {/* Difficulty cards */}
                <div className="flex-1 grid grid-cols-3 gap-2">
                  <DifficultyCard label={TEXT.PROFILE.EASY_SOLVED} count={stats.easy} total={stats.problemsSolved} color="emerald" />
                  <DifficultyCard label={TEXT.PROFILE.MEDIUM_SOLVED} count={stats.medium} total={stats.problemsSolved} color="amber" />
                  <DifficultyCard label={TEXT.PROFILE.HARD_SOLVED} count={stats.hard} total={stats.problemsSolved} color="rose" />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="rounded-lg bg-muted/30 px-3 py-2 text-center">
                  <span className="text-lg font-bold text-foreground">{stats.totalSubmissions}</span>
                  <p className="text-[10px] text-muted-foreground">{TEXT.PROFILE.TOTAL_SUBMISSIONS}</p>
                </div>
                <div className="rounded-lg bg-muted/30 px-3 py-2 text-center">
                  <span className="text-lg font-bold text-secondary">{stats.acceptanceRate}%</span>
                  <p className="text-[10px] text-muted-foreground">{TEXT.PROFILE.ACCEPTANCE_RATE}</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* ====== RIGHT COLUMN ====== */}
        <div className="lg:col-span-2 space-y-6">
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-primary" />
                Tu Luyện Tiến Trình
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">Cảnh giới, cấp bậc và linh căn ngũ hành</p>
            </CardHeader>
            <CardContent>
              <CultivationPanel rating={stats.rating} exp={stats.exp} roots={MOCK_SPIRITUAL_ROOTS} />
            </CardContent>
          </Card>

          {/* Căn Cốt & Thiên Phú */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Shield className="h-5 w-5 text-primary" />
                Căn Cốt & Thiên Phú
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">Thể chất bẩm sinh và tài năng thiên bẩm</p>
            </CardHeader>
            <CardContent className="space-y-6">
              <ConstitutionCard constitution={MOCK_CONSTITUTION} />
              <Separator className="border-border/30" />
              <div>
                <div className="flex items-center gap-2 mb-4">
                  <Award className="h-4 w-4 text-primary" />
                  <span className="text-sm font-bold">Thiên Phú</span>
                </div>
                <InnateTalentsCard talents={MOCK_INNATE_TALENTS} />
              </div>
            </CardContent>
          </Card>

          {/* Activity Heatmap */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Activity className="h-5 w-5 text-primary" />
                Hoạt Động Tu Luyện
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">Chuỗi luyện công trong 20 tuần gần nhất</p>
            </CardHeader>
            <CardContent>
              <ActivityHeatmap data={activityData} />
            </CardContent>
          </Card>

          {/* Rating Chart */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <TrendingUp className="h-5 w-5 text-primary" />
                Biểu Đồ Rating
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="bg-muted/20 rounded-xl p-4 border border-border/20">
                <RatingChart data={stats.ratingHistory} currentRating={stats.rating} />
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
