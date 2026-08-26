"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import {
  ArrowLeft,
  AtSign,
  CalendarDays,
  CircleAlert,
  Eye,
  EyeOff,
  RefreshCw,
  ShieldCheck,
  Sparkles,
} from "lucide-react"

import { LoadingSpinner } from "@/components/loading-spinner"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { APP_ROUTES } from "@/constants/routes"
import { formatProfileAttributeNumber } from "@/constants/profile-attributes"
import { TEXT } from "@/constants/text"
import { useRetryableResource } from "@/hooks/use-retryable-resource"
import { ApiError } from "@/lib/api/error"
import { isUsernameValid } from "@/lib/auth/credentials"
import {
  formatCalendarDate,
  formatDateTimeMinute,
} from "@/lib/format"
import { publicProfileService } from "@/services/public-profile.service"
import type {
  PublicProfile,
  PublicProfileAttribute,
} from "@/types/public-profile"

function publicAttributeValue(attribute: PublicProfileAttribute): string {
  if (attribute.data_type === "boolean") {
    return attribute.value
      ? TEXT.PUBLIC_PROFILE.BOOLEAN_TRUE
      : TEXT.PUBLIC_PROFILE.BOOLEAN_FALSE
  }
  if (attribute.data_type === "number") {
    return formatProfileAttributeNumber(attribute.value as number)
  }
  if (attribute.data_type === "date") {
    return formatCalendarDate(attribute.value as string)
  }
  return attribute.value as string
}

function profileInitials(username: string): string {
  return username.slice(0, 2).toUpperCase()
}

export default function PublicProfilePage() {
  const params = useParams()
  const requestedUsername =
    typeof params.username === "string" ? params.username : ""
  const validUsername = isUsernameValid(requestedUsername)
  const profileResource = useRetryableResource<PublicProfile | null>({
    resetKey: requestedUsername || "invalid-public-profile",
    enabled: validUsername,
    initialData: null,
    load: (signal) => publicProfileService.get(requestedUsername, signal),
  })
  const notFound =
    !validUsername ||
    (profileResource.error instanceof ApiError &&
      profileResource.error.status === 404)

  if (validUsername && profileResource.status === "loading") {
    return <LoadingSpinner label={TEXT.PUBLIC_PROFILE.LOADING} />
  }

  if (notFound) {
    return <PublicProfileUnavailable />
  }

  if (profileResource.status === "error" || !profileResource.data) {
    return (
      <PublicProfileFeedback
        title={TEXT.PUBLIC_PROFILE.LOAD_ERROR_TITLE}
        description={TEXT.PUBLIC_PROFILE.LOAD_ERROR_DESCRIPTION}
        actionLabel={TEXT.PUBLIC_PROFILE.RETRY}
        onAction={profileResource.retry}
      />
    )
  }

  const profile = profileResource.data

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-1 py-4 animate-in fade-in slide-in-from-bottom-2 duration-500 motion-reduce:animate-none sm:px-0 sm:py-6">
      <Link
        href={APP_ROUTES.ARENA}
        className="inline-flex min-h-11 items-center gap-2 rounded-lg px-1 text-sm font-medium text-muted-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring motion-reduce:transition-none"
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {TEXT.PUBLIC_PROFILE.BACK_TO_ARENA}
      </Link>

      <Card className="glass-card relative overflow-hidden border-border/40 py-0">
        <div className="relative min-h-44 overflow-hidden bg-gradient-to-br from-primary/90 via-cultivation/80 to-warning/75">
          <div className="absolute inset-0 surface-grid-tight opacity-50" />
          <div className="absolute inset-0 bg-gradient-to-t from-card via-card/20 to-transparent" />
          <Sparkles
            className="absolute right-8 top-7 size-6 text-primary-foreground/55"
            aria-hidden="true"
          />
        </div>
        <CardContent className="relative -mt-16 grid gap-5 px-5 pb-6 sm:grid-cols-[auto_1fr] sm:items-end sm:px-8 sm:pb-8">
          <Avatar className="size-28 border-4 border-background shadow-xl">
            <AvatarFallback className="heading-gaming bg-gradient-to-br from-primary to-cultivation text-3xl font-bold text-primary-foreground">
              {profileInitials(profile.username)}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0 space-y-3 pb-1">
            <Badge
              variant="outline"
              className="border-primary/25 bg-background/80 text-foreground backdrop-blur-sm"
            >
              <Eye className="size-3" aria-hidden="true" />
              {TEXT.PUBLIC_PROFILE.EYEBROW}
            </Badge>
            <div>
              <h1 className="break-all text-3xl font-black tracking-tight sm:text-4xl">
                {profile.username}
              </h1>
              <p className="mt-1 flex items-center gap-1.5 text-sm text-muted-foreground">
                <AtSign className="size-4" aria-hidden="true" />
                {profile.username}
              </p>
            </div>
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <CalendarDays className="size-4 text-primary" aria-hidden="true" />
              <span>{TEXT.PUBLIC_PROFILE.JOINED}</span>
              <time
                className="font-semibold text-foreground"
                dateTime={profile.joined_at}
              >
                {formatDateTimeMinute(profile.joined_at)}
              </time>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="glass-card border-border/40">
        <CardHeader className="gap-3 border-b border-border/60 bg-muted/15 sm:flex-row sm:items-start">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <ShieldCheck className="size-5" aria-hidden="true" />
          </div>
          <div>
            <CardTitle className="text-lg">
              {TEXT.PUBLIC_PROFILE.ATTRIBUTES_TITLE}
            </CardTitle>
            <CardDescription className="mt-1 leading-5">
              {TEXT.PUBLIC_PROFILE.ATTRIBUTES_DESCRIPTION}
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent className="p-4 sm:p-6">
          {profile.attributes.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center rounded-2xl border border-dashed border-border/80 bg-muted/15 px-6 text-center">
              <Eye className="size-6 text-muted-foreground" aria-hidden="true" />
              <h2 className="mt-3 font-semibold">
                {TEXT.PUBLIC_PROFILE.EMPTY_TITLE}
              </h2>
              <p className="mt-1 max-w-md text-sm leading-5 text-muted-foreground">
                {TEXT.PUBLIC_PROFILE.EMPTY_DESCRIPTION}
              </p>
            </div>
          ) : (
            <dl className="grid gap-3 sm:grid-cols-2">
              {profile.attributes.map((attribute) => (
                <div
                  key={attribute.key}
                  className="rounded-2xl border border-border/70 bg-background/70 p-4 transition-colors duration-200 hover:border-primary/30 hover:bg-muted/20 motion-reduce:transition-none"
                >
                  <dt className="font-semibold text-foreground">
                    {attribute.label}
                  </dt>
                  <dd>
                    {attribute.description ? (
                      <p className="mt-1 text-xs leading-5 text-muted-foreground">
                        {attribute.description}
                      </p>
                    ) : null}
                    <p className="mt-3 break-words text-base font-semibold text-primary">
                      {publicAttributeValue(attribute)}
                    </p>
                  </dd>
                </div>
              ))}
            </dl>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function PublicProfileUnavailable() {
  return (
    <div className="mx-auto flex min-h-[60vh] max-w-xl items-center px-4">
      <Card className="w-full border-border/60 bg-card/85 shadow-lg">
        <CardContent className="flex flex-col items-center px-6 py-12 text-center">
          <div className="flex size-14 items-center justify-center rounded-2xl bg-muted text-muted-foreground ring-8 ring-muted/40">
            <EyeOff className="size-6" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-xl font-bold tracking-tight">
            {TEXT.PUBLIC_PROFILE.NOT_FOUND_TITLE}
          </h1>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {TEXT.PUBLIC_PROFILE.NOT_FOUND_DESCRIPTION}
          </p>
          <Link
            href={APP_ROUTES.ARENA}
            className={buttonVariants({ variant: "outline", className: "mt-6" })}
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {TEXT.PUBLIC_PROFILE.BACK_TO_ARENA}
          </Link>
        </CardContent>
      </Card>
    </div>
  )
}

function PublicProfileFeedback({
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
    <div className="mx-auto flex min-h-[60vh] max-w-xl items-center px-4">
      <Card className="w-full border-destructive/20 bg-card/85 shadow-lg">
        <CardContent
          className="flex flex-col items-center px-6 py-12 text-center"
          role="alert"
        >
          <div className="flex size-14 items-center justify-center rounded-2xl bg-destructive/10 text-destructive ring-8 ring-destructive/5">
            <CircleAlert className="size-6" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-xl font-bold tracking-tight">{title}</h1>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {description}
          </p>
          <Button type="button" variant="outline" className="mt-6" onClick={onAction}>
            <RefreshCw className="size-4" aria-hidden="true" />
            {actionLabel}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
