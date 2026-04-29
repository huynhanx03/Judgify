"use client"

import { useState, useEffect } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Sparkles, Shield, Award, CalendarDays, Cake, Settings, LogOut, Gem, Target, Tag } from "lucide-react"
import { LoadingSpinner } from "@/components/loading-spinner"
import { userService } from "@/services/user.service"
import { difficultyService } from "@/services/difficulty.service"
import { useAuth } from "@/contexts/auth-context"
import { TEXT } from "@/constants/text"
import type { UserProfile, TraitInfo, DiffStat } from "@/types/user"
import type { DifficultyResponse } from "@/types/difficulty"
import type { TraitResponse } from "@/types/cultivation"
import { getTierColors } from "@/types/cultivation"
import { CultivationPanel } from "@/modules/profile/cultivation-panel"
import { StatsPanel } from "@/modules/profile/stats-panel"
import { TagsPanel } from "@/modules/profile/tags-panel"
import { TraitCard } from "@/modules/cultivation/trait-card"

function getInitials(profile: UserProfile): string {
  return ((profile.first_name?.[0] ?? "") + (profile.last_name?.[0] ?? "")).toUpperCase() || "?"
}

function adaptTrait(trait: TraitInfo, type: "root_bone" | "talent"): TraitResponse {
  return {
    id: 0,
    type,
    name: trait.name,
    description: trait.description,
    metadata: trait.metadata,
    rarity: trait.rarity_code
      ? { id: 0, code: trait.rarity_code, name: trait.rarity_name ?? trait.rarity_code, weight: 0 }
      : undefined,
  }
}

/** Merge all available difficulties with solved stats (fill 0 for unsolved). */
function mergeDifficulties(all: DifficultyResponse[], solved: DiffStat[]): DiffStat[] {
  const solvedMap = new Map(solved.map((s) => [s.level, s.solved_count]))
  return all
    .sort((a, b) => a.level - b.level)
    .map((d) => ({ name: d.name, level: d.level, solved_count: solvedMap.get(d.level) ?? 0 }))
}

export default function ProfilePage() {
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [difficulties, setDifficulties] = useState<DiffStat[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const { logout } = useAuth()

  useEffect(() => {
    Promise.all([userService.getProfile(), difficultyService.getAll()])
      .then(([p, diffs]) => {
        setProfile(p)
        setDifficulties(mergeDifficulties(diffs, p.problem_stats.by_difficulty))
      })
      .finally(() => setIsLoading(false))
  }, [])

  if (isLoading) return <LoadingSpinner />;

  if (!profile) return null

  const { cultivation } = profile
  const rankColors = getTierColors(cultivation.rank.tier_index)
  const levelColors = getTierColors(cultivation.level.tier_index)

  return (
    <div className="space-y-8 animate-in fade-in duration-500">
      {/* Page header */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight heading-gaming text-transparent bg-clip-text bg-gradient-to-r from-amber-500 via-amber-400 to-amber-200">
          {TEXT.PROFILE.TITLE}
        </h1>
        <p className="text-muted-foreground mt-1">{TEXT.PROFILE.SUBTITLE}</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
        {/* ====== LEFT COLUMN ====== */}
        <div className="lg:col-span-1 space-y-6">

          {/* Profile card */}
          <Card className="glass-card border-border/40 overflow-hidden">
            <div className="h-28 bg-gradient-to-br from-amber-600 via-amber-500 to-amber-400 relative">
              <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:12px_12px]" />
              <div className="absolute inset-0 bg-gradient-to-t from-card/80 to-transparent" />
              <Sparkles className="absolute top-4 right-6 h-5 w-5 text-white/40 animate-pulse" />
            </div>
            <div className="px-6 pb-6 -mt-12 relative z-10">
              <div className="flex items-end justify-between">
                <Avatar className="h-24 w-24 border-4 border-background shadow-xl">
                  <AvatarFallback className="text-2xl font-bold heading-gaming text-white bg-gradient-to-br from-amber-500 to-amber-700">
                    {getInitials(profile)}
                  </AvatarFallback>
                </Avatar>
                <div className="flex gap-1.5">
                  <button
                    className="h-8 w-8 flex items-center justify-center rounded-lg border border-border/40 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
                    title={TEXT.PROFILE.EDIT_TOOLTIP}
                  >
                    <Settings className="h-4 w-4" />
                  </button>
                  <button
                    onClick={logout}
                    className="h-8 w-8 flex items-center justify-center rounded-lg border border-border/40 text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                    title={TEXT.PROFILE.LOGOUT_TOOLTIP}
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
                <Badge className={`border-0 font-bold text-xs shadow-md ${rankColors.badge}`}>
                  {cultivation.rank.name || "—"}
                </Badge>
                <Badge className={`border-0 font-bold text-xs shadow-md ${levelColors.badge}`}>
                  {cultivation.level.name || "—"}
                </Badge>
              </div>
              <Separator className="my-4 border-border/40" />
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <CalendarDays className="h-4 w-4 shrink-0" />
                  <span>{TEXT.PROFILE.JOINED_DATE}:</span>
                  <span className="font-medium text-foreground">{profile.joined_at}</span>
                </div>
                {profile.birthday && (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Cake className="h-4 w-4 shrink-0" />
                    <span>{TEXT.PROFILE.BIRTHDAY}:</span>
                    <span className="font-medium text-foreground">{profile.birthday}</span>
                  </div>
                )}
              </div>
            </div>
          </Card>

          {/* Stats card */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Target className="h-5 w-5 text-primary" />
                {TEXT.STATS.TITLE}
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">{TEXT.STATS.SUBTITLE}</p>
            </CardHeader>
            <CardContent>
              <StatsPanel stats={profile.problem_stats} difficulties={difficulties} />
            </CardContent>
          </Card>

          {/* Tags card */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Tag className="h-5 w-5 text-primary" />
                {TEXT.STATS.TAGS_TITLE}
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">{TEXT.STATS.TAGS_SUBTITLE}</p>
            </CardHeader>
            <CardContent>
              <TagsPanel tags={profile.problem_stats.by_tag} />
            </CardContent>
          </Card>

        </div>

        {/* ====== RIGHT COLUMN ====== */}
        <div className="lg:col-span-2 space-y-6">

          {/* Cultivation progress */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-primary" />
                {TEXT.PROFILE.CULTIVATION_PROGRESS}
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">{TEXT.PROFILE.CULTIVATION_SUBTITLE}</p>
            </CardHeader>
            <CardContent>
              <CultivationPanel cultivation={cultivation} />
            </CardContent>
          </Card>

          {/* Traits */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Shield className="h-5 w-5 text-primary" />
                {TEXT.PROFILE.TRAITS_TITLE}
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">{TEXT.PROFILE.TRAITS_SUBTITLE}</p>
            </CardHeader>
            <CardContent className="space-y-6">
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <Gem className="h-4 w-4 text-amber-400" />
                  <span className="text-sm font-bold">{TEXT.AUTH.TRAIT_ROOT_BONE}</span>
                </div>
                {cultivation.root_bone ? (
                  <TraitCard trait={adaptTrait(cultivation.root_bone, "root_bone")} variant="compact" />
                ) : (
                  <p className="text-sm text-muted-foreground italic">{TEXT.PROFILE.NO_ROOT_BONE}</p>
                )}
              </div>
              <Separator className="border-border/20" />
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <Award className="h-4 w-4 text-primary" />
                  <span className="text-sm font-bold">{TEXT.PROFILE.TALENTS_LABEL}</span>
                </div>
                {cultivation.talents.length > 0 ? (
                  <div className="space-y-2">
                    {cultivation.talents.map((talent, i) => (
                      <TraitCard key={i} trait={adaptTrait(talent, "talent")} variant="compact" />
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground italic">{TEXT.PROFILE.NO_TALENTS}</p>
                )}
              </div>
            </CardContent>
          </Card>

        </div>
      </div>
    </div>
  )
}
