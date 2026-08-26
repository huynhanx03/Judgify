"use client"

import { useState, useMemo } from "react"
import Link from "next/link"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Sparkles, CalendarDays, Cake, Settings, LogOut, Target, Tag, CircleAlert, RefreshCw, LogIn, Loader2 } from "lucide-react"
import { LoadingSpinner } from "@/components/loading-spinner"
import { Button, buttonVariants } from "@/components/ui/button"
import { userService } from "@/services/user.service"
import { difficultyService } from "@/services/difficulty.service"
import { rewardProfileService } from "@/services/reward-profile.service"
import { useAuth } from "@/contexts/auth-context"
import { TEXT } from "@/constants/text"
import type { UserProfile, DiffStat } from "@/types/user"
import type { DifficultyResponse } from "@/types/difficulty"
import type { RewardProfileResponse } from "@/types/cultivation"
import { getTierColors } from "@/constants/cultivation-presentation"
import { CultivationPanel } from "@/modules/profile/cultivation-panel"
import { StatsPanel } from "@/modules/profile/stats-panel"
import { TagsPanel } from "@/modules/profile/tags-panel"
import { RewardProfileSummary } from "@/modules/cultivation/reward-profile-summary"
import { ProfileEditDialog } from "@/modules/profile/profile-edit-dialog"
import { RecoveryContactCard } from "@/modules/profile/recovery-contact-card"
import { SessionSecurityCard } from "@/modules/profile/session-security-card"
import { ChangePasswordCard } from "@/modules/profile/change-password-card"
import { OAuthAccountsCard } from "@/modules/profile/oauth-accounts-card"
import { ProfilePrivacyCard } from "@/modules/profile/profile-privacy-card"
import { AUTH_BOOTSTRAP_STATUS } from "@/constants/authorization"
import { APP_ROUTES } from "@/constants/routes"
import { useRetryableResource } from "@/hooks/use-retryable-resource"
import { formatDateTimeMinute } from "@/lib/format"
import { useLogoutAction } from "@/modules/auth/use-logout-action"

function getInitials(profile: UserProfile): string {
  return (
    (profile.first_name?.[0] ?? "") +
    (profile.last_name?.[0] ?? "") ||
    profile.username[0] ||
    "?"
  ).toUpperCase()
}

/** Merge all available difficulties with solved stats (fill 0 for unsolved). */
function mergeDifficulties(all: DifficultyResponse[], solved: DiffStat[]): DiffStat[] {
  const solvedMap = new Map(solved.map((s) => [s.level, s.solved_count]))
  return [...all]
    .sort((a, b) => a.level - b.level)
    .map((d) => ({ name: d.name, level: d.level, solved_count: solvedMap.get(d.level) ?? 0 }))
}

function ProfileAccessFeedback({
  title,
  description,
  actionLabel,
  onAction,
}: {
  title: string
  description: string
  actionLabel: string
  onAction: () => void
}) {
  return (
    <Card className="mx-auto mt-12 max-w-lg border-destructive/20 bg-card/80 shadow-lg">
      <CardContent
        className="flex flex-col items-center px-6 py-12 text-center"
        role="alert"
      >
        <div className="mb-5 flex size-14 items-center justify-center rounded-2xl bg-destructive/10 text-destructive ring-8 ring-destructive/5">
          <CircleAlert className="size-6" aria-hidden="true" />
        </div>
        <h1 className="text-xl font-bold tracking-tight">{title}</h1>
        <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
          {description}
        </p>
        <Button
          type="button"
          onClick={onAction}
          className="mt-6 min-h-11 cursor-pointer px-5"
        >
          <RefreshCw className="size-4" aria-hidden="true" />
          {actionLabel}
        </Button>
      </CardContent>
    </Card>
  )
}

export default function ProfilePage() {
  const [savedProfile, setSavedProfile] = useState<{
    userID: string
    profile: UserProfile
  } | null>(null)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const {
    bootstrapStatus,
    isAuthenticated,
    isLoading: isAuthLoading,
    retryBootstrap,
    user,
  } = useAuth()
  const { isLoggingOut, performLogout } = useLogoutAction()
  const userID = user?.id ?? null
  const resourceEnabled = isAuthenticated && userID !== null
  const profileResource = useRetryableResource<UserProfile | null>({
    resetKey: userID,
    enabled: resourceEnabled,
    initialData: null,
    load: (signal) => userService.getProfile(signal),
  })
  const activeProfile =
    savedProfile?.userID === userID
      ? savedProfile.profile
      : profileResource.data?.id === userID
        ? profileResource.data
        : null
  const gameProfileReady =
    resourceEnabled && activeProfile?.profile_state === "ready"
  const difficultyResource = useRetryableResource<DifficultyResponse[]>({
    resetKey: userID,
    enabled: gameProfileReady,
    initialData: [],
    keepPreviousData: true,
    load: (signal) => difficultyService.getAll(signal),
  })
  const rewardProfileResource = useRetryableResource<RewardProfileResponse | null>({
    resetKey: userID,
    enabled: gameProfileReady,
    initialData: null,
    load: (signal) => rewardProfileService.current(signal),
  })
  const difficulties = useMemo(() => {
    if (!activeProfile || activeProfile.profile_state !== "ready") return []
    if (difficultyResource.data.length === 0) {
      return activeProfile.problem_stats.by_difficulty
    }
    return mergeDifficulties(
      difficultyResource.data,
      activeProfile.problem_stats.by_difficulty,
    )
  }, [activeProfile, difficultyResource.data])

  function handleProfileSaved(updatedProfile: UserProfile) {
    if (!userID) return
    setSavedProfile({ userID, profile: updatedProfile })
  }

  if (isAuthLoading || bootstrapStatus === AUTH_BOOTSTRAP_STATUS.LOADING) {
    return <LoadingSpinner label={TEXT.PROFILE.AUTH_LOADING} />
  }

  if (bootstrapStatus === AUTH_BOOTSTRAP_STATUS.ERROR) {
    return (
      <ProfileAccessFeedback
        title={TEXT.PROFILE.AUTH_ERROR_TITLE}
        description={TEXT.PROFILE.AUTH_ERROR_DESCRIPTION}
        actionLabel={TEXT.PROFILE.AUTH_RETRY}
        onAction={retryBootstrap}
      />
    )
  }

  if (!isAuthenticated || !userID) {
    return (
      <Card className="mx-auto mt-12 max-w-lg border-border/60 bg-card/80 shadow-lg">
        <CardContent className="flex flex-col items-center px-6 py-12 text-center">
          <div className="mb-5 flex size-14 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/5">
            <LogIn className="size-6" aria-hidden="true" />
          </div>
          <h1 className="text-xl font-bold tracking-tight">
            {TEXT.PROFILE.LOGIN_REQUIRED_TITLE}
          </h1>
          <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
            {TEXT.PROFILE.LOGIN_REQUIRED_DESCRIPTION}
          </p>
          <Link
            href={APP_ROUTES.LOGIN}
            className={buttonVariants({ className: "mt-6 min-h-11 px-5" })}
          >
            <LogIn className="size-4" aria-hidden="true" />
            {TEXT.PROFILE.LOGIN_ACTION}
          </Link>
        </CardContent>
      </Card>
    )
  }

  if (
    profileResource.status === "loading" ||
    (!activeProfile && profileResource.status !== "error")
  ) {
    return <LoadingSpinner label={TEXT.PROFILE.LOADING} />
  }

  if (profileResource.status === "error" || !activeProfile) {
    return (
      <ProfileAccessFeedback
        title={TEXT.PROFILE.LOAD_ERROR_TITLE}
        description={TEXT.PROFILE.LOAD_ERROR_DESCRIPTION}
        actionLabel={TEXT.COMMON.RETRY}
        onAction={profileResource.retry}
      />
    )
  }

  if (activeProfile.profile_state === "unprovisioned") {
    const profile = activeProfile
    return (
      <div className="space-y-8 animate-in fade-in duration-500 motion-reduce:animate-none">
        <div>
          <h1 className="heading-gaming text-3xl font-bold tracking-tight text-cultivation">
            {TEXT.PROFILE.TITLE}
          </h1>
          <p className="mt-1 text-muted-foreground">
            {TEXT.PROFILE.UNPROVISIONED_DESCRIPTION}
          </p>
        </div>

        <div className="grid items-start gap-6 lg:grid-cols-3">
          <Card className="glass-card overflow-hidden border-border/40 lg:sticky lg:top-6">
            <div className="relative h-24 bg-gradient-to-br from-primary/80 via-cultivation/70 to-warning/70">
              <div className="absolute inset-0 surface-grid-tight" />
            </div>
            <CardContent className="relative -mt-10 px-6 pb-6">
              <div className="flex items-end justify-between gap-3">
                <Avatar className="size-20 border-4 border-background shadow-xl">
                  <AvatarFallback className="heading-gaming bg-gradient-to-br from-cultivation to-primary text-xl font-bold text-cultivation-foreground">
                    {getInitials(profile)}
                  </AvatarFallback>
                </Avatar>
                <div className="flex gap-1.5">
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => setIsEditOpen(true)}
                    aria-label={TEXT.PROFILE.EDIT_TOOLTIP}
                  >
                    <Settings className="size-4" aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => void performLogout()}
                    disabled={isLoggingOut}
                    aria-label={isLoggingOut ? TEXT.NAV.LOGGING_OUT : TEXT.PROFILE.LOGOUT_TOOLTIP}
                  >
                    {isLoggingOut ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <LogOut className="size-4" aria-hidden="true" />}
                  </Button>
                </div>
              </div>
              <div className="mt-4 space-y-1">
                <h2 className="text-xl font-bold">
                  {[profile.first_name, profile.last_name].filter(Boolean).join(" ") ||
                    profile.username}
                </h2>
                <p className="text-sm text-muted-foreground">@{profile.username}</p>
              </div>
              <Badge variant="secondary" className="mt-3">
                {TEXT.PROFILE.UNPROVISIONED_BADGE}
              </Badge>
              <Separator className="my-4" />
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <CalendarDays className="size-4" aria-hidden="true" />
                <time dateTime={profile.joined_at}>
                  {formatDateTimeMinute(profile.joined_at)}
                </time>
              </div>
            </CardContent>
          </Card>

          <div className="space-y-6 lg:col-span-2">
            <Card className="border-primary/20 bg-primary/[0.04]">
              <CardHeader>
                <CardTitle>{TEXT.PROFILE.UNPROVISIONED_TITLE}</CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-6 text-muted-foreground">
                {TEXT.PROFILE.UNPROVISIONED_NOTE}
              </CardContent>
            </Card>
            <ProfilePrivacyCard userID={profile.id} username={profile.username} />
            <RecoveryContactCard />
            <OAuthAccountsCard />
            <ChangePasswordCard />
            <SessionSecurityCard />
          </div>
        </div>

        {isEditOpen ? (
          <ProfileEditDialog
            open={isEditOpen}
            profile={profile}
            onOpenChange={setIsEditOpen}
            onSaved={handleProfileSaved}
          />
        ) : null}
      </div>
    )
  }

  const profile = activeProfile
  const { cultivation } = profile
  const rankColors = getTierColors(cultivation.rank.tier_index)
  const levelColors = getTierColors(cultivation.level.tier_index)

  return (
    <div className="space-y-8 animate-in fade-in duration-500 motion-reduce:animate-none">
      {/* Page header */}
      <div>
        <h1 className="heading-gaming text-3xl font-bold tracking-tight text-cultivation">
          {TEXT.PROFILE.TITLE}
        </h1>
        <p className="text-muted-foreground mt-1">{TEXT.PROFILE.SUBTITLE}</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
        {/* ====== LEFT COLUMN ====== */}
        <div className="lg:col-span-1 space-y-6">

          {/* Profile card */}
          <Card className="glass-card border-border/40 overflow-hidden">
            <div className="relative h-28 bg-gradient-to-br from-cultivation via-warning to-primary">
              <div className="absolute inset-0 surface-grid-tight" />
              <div className="absolute inset-0 bg-gradient-to-t from-card/80 to-transparent" />
              <Sparkles className="absolute right-6 top-4 h-5 w-5 text-cultivation-foreground/40" aria-hidden="true" />
            </div>
            <div className="px-6 pb-6 -mt-12 relative z-10">
              <div className="flex items-end justify-between">
                <Avatar className="h-24 w-24 border-4 border-background shadow-xl">
                  <AvatarFallback className="heading-gaming bg-gradient-to-br from-cultivation to-primary text-2xl font-bold text-cultivation-foreground">
                    {getInitials(profile)}
                  </AvatarFallback>
                </Avatar>
                <div className="flex gap-1.5">
                  <button
                    type="button"
                    onClick={() => setIsEditOpen(true)}
                    className="flex size-11 cursor-pointer items-center justify-center rounded-xl border border-border/60 bg-background/70 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    title={TEXT.PROFILE.EDIT_TOOLTIP}
                    aria-label={TEXT.PROFILE.EDIT_TOOLTIP}
                  >
                    <Settings className="h-4 w-4" aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    onClick={() => void performLogout()}
                    disabled={isLoggingOut}
                    className="flex size-11 cursor-pointer items-center justify-center rounded-xl border border-border/60 bg-background/70 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    title={isLoggingOut ? TEXT.NAV.LOGGING_OUT : TEXT.PROFILE.LOGOUT_TOOLTIP}
                    aria-label={isLoggingOut ? TEXT.NAV.LOGGING_OUT : TEXT.PROFILE.LOGOUT_TOOLTIP}
                  >
                    {isLoggingOut ? <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <LogOut className="h-4 w-4" aria-hidden="true" />}
                  </button>
                </div>
              </div>
              <div className="mt-3 space-y-1">
                <h2 className="text-xl font-bold">{profile.first_name} {profile.last_name}</h2>
                <p className="text-sm text-muted-foreground">@{profile.username}</p>
              </div>
              <div className="flex flex-wrap gap-2 mt-3">
                <Badge className={`border-0 font-bold text-xs shadow-md ${rankColors.badge}`}>
                  {cultivation.rank.name || TEXT.COMMON.NOT_AVAILABLE}
                </Badge>
                <Badge className={`border-0 font-bold text-xs shadow-md ${levelColors.badge}`}>
                  {cultivation.level.name || TEXT.COMMON.NOT_AVAILABLE}
                </Badge>
              </div>
              <Separator className="my-4 border-border/40" />
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <CalendarDays className="h-4 w-4 shrink-0" />
                  <span>{TEXT.PROFILE.JOINED_DATE}:</span>
                  <time
                    className="font-medium text-foreground"
                    dateTime={profile.joined_at}
                  >
                    {formatDateTimeMinute(profile.joined_at)}
                  </time>
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
              {difficultyResource.status === "error" ? (
                <div
                  className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-warning/25 bg-warning/5 px-3 py-2 text-xs text-muted-foreground"
                  role="status"
                >
                  <span>{TEXT.PROFILE.DIFFICULTIES_LOAD_ERROR}</span>
                  <Button
                    type="button"
                    variant="outline"
                    size="xs"
                    onClick={difficultyResource.retry}
                  >
                    <RefreshCw className="size-3" aria-hidden="true" />
                    {TEXT.PROFILE.DIFFICULTIES_RETRY}
                  </Button>
                </div>
              ) : difficultyResource.status === "loading" ? (
                <div
                  className="mb-4 flex items-center gap-2 text-xs text-muted-foreground"
                  role="status"
                >
                  <Loader2 className="size-3 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  {TEXT.PROFILE.DIFFICULTIES_LOADING}
                </div>
              ) : null}
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

          <ProfilePrivacyCard
            userID={profile.id}
            username={profile.username}
          />

          <RecoveryContactCard />

		  <OAuthAccountsCard />

          <ChangePasswordCard />

          <SessionSecurityCard />

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

          {/* Immutable effective-dated reward profile */}
          <Card className="glass-card border-border/40">
            <CardHeader className="pb-2">
              <CardTitle className="text-lg flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-primary" />
                {TEXT.PROFILE.REWARD_PROFILE.TITLE}
                {rewardProfileResource.data ? (
                  <Badge variant="outline" className="ml-auto">
                    {TEXT.PROFILE.REWARD_PROFILE.REVISION(
                      rewardProfileResource.data.revision,
                    )}
                  </Badge>
                ) : null}
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">
                {TEXT.PROFILE.REWARD_PROFILE.DESCRIPTION}
              </p>
            </CardHeader>
            <CardContent>
              {rewardProfileResource.status === "loading" ? (
                <LoadingSpinner
                  className="min-h-48"
                  label={TEXT.PROFILE.REWARD_PROFILE.LOADING}
                />
              ) : rewardProfileResource.status === "error" ||
                !rewardProfileResource.data ? (
                <div
                  className="flex min-h-48 flex-col items-center justify-center rounded-xl border border-dashed border-border px-6 text-center"
                  role="alert"
                >
                  <CircleAlert className="size-5 text-destructive" aria-hidden="true" />
                  <p className="mt-3 font-semibold">
                    {TEXT.PROFILE.REWARD_PROFILE.LOAD_ERROR}
                  </p>
                  <p className="mt-1 max-w-md text-sm text-muted-foreground">
                    {TEXT.PROFILE.REWARD_PROFILE.LOAD_ERROR_DESCRIPTION}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    className="mt-4 min-h-11"
                    onClick={rewardProfileResource.retry}
                  >
                    <RefreshCw className="size-4" aria-hidden="true" />
                    {TEXT.PROFILE.REWARD_PROFILE.RETRY}
                  </Button>
                </div>
              ) : (
                <RewardProfileSummary profile={rewardProfileResource.data} />
              )}
            </CardContent>
          </Card>

        </div>
      </div>
      {isEditOpen ? (
        <ProfileEditDialog
          open={isEditOpen}
          profile={profile}
          onOpenChange={setIsEditOpen}
          onSaved={handleProfileSaved}
        />
      ) : null}
    </div>
  )
}
