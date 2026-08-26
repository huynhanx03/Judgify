"use client"

import Link from "next/link"
import { useState } from "react"
import {
  AlertTriangle,
  CheckCircle2,
  ExternalLink,
  Eye,
  EyeOff,
  Globe2,
  Loader2,
  RefreshCw,
  ShieldCheck,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Switch } from "@/components/ui/switch"
import { APP_ROUTES } from "@/constants/routes"
import { TEXT } from "@/constants/text"
import { useRetryableResource } from "@/hooks/use-retryable-resource"
import { ApiError } from "@/lib/api/error"
import { formatDateTimeMinute } from "@/lib/format"
import { cn } from "@/lib/utils"
import { publicProfileService } from "@/services/public-profile.service"
import type { EntityID } from "@/types/api"
import type {
  ProfilePrivacy,
  ProfileVisibility,
} from "@/types/public-profile"

interface ProfilePrivacyCardProps {
  userID: EntityID
  username: string
}

interface LocalPrivacySnapshot {
  userID: EntityID
  state: ProfilePrivacy
}

interface PrivacyFeedback {
  userID: EntityID
  tone: "error" | "success"
  message: string
}

interface PendingPrivacyMutation {
  userID: EntityID
  visibility: ProfileVisibility
}

export function ProfilePrivacyCard({
  userID,
  username,
}: ProfilePrivacyCardProps) {
  const [localSnapshot, setLocalSnapshot] =
    useState<LocalPrivacySnapshot | null>(null)
  const [feedback, setFeedback] = useState<PrivacyFeedback | null>(null)
  const [pending, setPending] = useState<PendingPrivacyMutation | null>(null)
  const privacyResource = useRetryableResource<ProfilePrivacy | null>({
    resetKey: userID,
    enabled: Boolean(userID),
    initialData: null,
    load: (signal) => publicProfileService.privacy(signal),
  })
  const serverState = privacyResource.data
  const privacy =
    localSnapshot?.userID === userID &&
    (!serverState || localSnapshot.state.version >= serverState.version)
      ? localSnapshot.state
      : serverState
  const currentFeedback = feedback?.userID === userID ? feedback : null
  const currentPending = pending?.userID === userID ? pending : null
  const isPublic = privacy?.visibility === "public"

  async function updateVisibility(checked: boolean) {
    if (!privacy || currentPending) return
    const visibility: ProfileVisibility = checked ? "public" : "private"
    if (visibility === privacy.visibility) return

    setFeedback(null)
    setPending({ userID, visibility })
    try {
      const receipt = await publicProfileService.updatePrivacy(userID, {
        visibility,
        expected_version: privacy.version,
      })
      setLocalSnapshot({ userID, state: receipt.state })
      setFeedback({
        userID,
        tone: "success",
        message:
          visibility === "public"
            ? TEXT.PROFILE.PRIVACY.PUBLIC_SUCCESS
            : TEXT.PROFILE.PRIVACY.PRIVATE_SUCCESS,
      })
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setLocalSnapshot(null)
        privacyResource.retry()
        setFeedback({
          userID,
          tone: "error",
          message: TEXT.PROFILE.PRIVACY.CONFLICT,
        })
      } else {
        setFeedback({
          userID,
          tone: "error",
          message: TEXT.PROFILE.PRIVACY.UPDATE_ERROR,
        })
      }
    } finally {
      setPending((current) =>
        current?.userID === userID && current.visibility === visibility
          ? null
          : current,
      )
    }
  }

  return (
    <Card className="glass-card overflow-hidden border-border/40">
      <CardHeader className="gap-3 border-b border-border/60 bg-muted/15 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <ShieldCheck className="size-5" aria-hidden="true" />
          </div>
          <div>
            <CardTitle className="text-lg">
              {TEXT.PROFILE.PRIVACY.TITLE}
            </CardTitle>
            <CardDescription className="mt-1 leading-5">
              {TEXT.PROFILE.PRIVACY.DESCRIPTION}
            </CardDescription>
          </div>
        </div>
        {privacy ? (
          <Badge
            variant={isPublic ? "default" : "secondary"}
            className="mt-0.5"
          >
            {isPublic ? (
              <Eye className="size-3" aria-hidden="true" />
            ) : (
              <EyeOff className="size-3" aria-hidden="true" />
            )}
            {isPublic
              ? TEXT.PROFILE.PRIVACY.PUBLIC_LABEL
              : TEXT.PROFILE.PRIVACY.PRIVATE_LABEL}
          </Badge>
        ) : null}
      </CardHeader>

      <CardContent className="space-y-4 p-4 sm:p-5">
        {privacyResource.status === "loading" && !privacy ? (
          <div
            className="flex min-h-32 items-center justify-center gap-2 text-sm text-muted-foreground"
            role="status"
            aria-busy="true"
          >
            <Loader2
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden="true"
            />
            {TEXT.PROFILE.PRIVACY.LOADING}
          </div>
        ) : privacyResource.status === "error" && !privacy ? (
          <div
            className="flex min-h-36 flex-col items-center justify-center text-center"
            role="alert"
          >
            <AlertTriangle
              className="size-5 text-destructive"
              aria-hidden="true"
            />
            <p className="mt-3 font-semibold">
              {TEXT.PROFILE.PRIVACY.LOAD_ERROR_TITLE}
            </p>
            <p className="mt-1 max-w-md text-sm leading-5 text-muted-foreground">
              {TEXT.PROFILE.PRIVACY.LOAD_ERROR_DESCRIPTION}
            </p>
            <Button
              type="button"
              variant="outline"
              className="mt-4"
              onClick={privacyResource.retry}
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.PROFILE.PRIVACY.RETRY}
            </Button>
          </div>
        ) : privacy ? (
          <>
            {currentFeedback ? (
              <div
                className={cn(
                  "flex items-start gap-2 rounded-xl border p-3 text-sm",
                  currentFeedback.tone === "success"
                    ? "border-success/25 bg-success/10 text-success"
                    : "border-destructive/25 bg-destructive/10 text-destructive",
                )}
                role={currentFeedback.tone === "error" ? "alert" : "status"}
              >
                {currentFeedback.tone === "success" ? (
                  <CheckCircle2
                    className="mt-0.5 size-4 shrink-0"
                    aria-hidden="true"
                  />
                ) : (
                  <AlertTriangle
                    className="mt-0.5 size-4 shrink-0"
                    aria-hidden="true"
                  />
                )}
                <span>{currentFeedback.message}</span>
              </div>
            ) : null}

            <div className="flex items-start gap-4 rounded-2xl border border-border/70 bg-background/70 p-4 transition-colors duration-200 hover:border-primary/30 motion-reduce:transition-none">
              <div className="flex size-11 shrink-0 items-center justify-center rounded-xl border border-border/70 bg-card text-primary shadow-sm">
                <Globe2 className="size-5" aria-hidden="true" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="font-semibold">
                  {isPublic
                    ? TEXT.PROFILE.PRIVACY.PUBLIC_LABEL
                    : TEXT.PROFILE.PRIVACY.PRIVATE_LABEL}
                </p>
                <p className="mt-1 text-sm leading-5 text-muted-foreground">
                  {isPublic
                    ? TEXT.PROFILE.PRIVACY.PUBLIC_DESCRIPTION
                    : TEXT.PROFILE.PRIVACY.PRIVATE_DESCRIPTION}
                </p>
                <p className="mt-2 text-xs text-muted-foreground">
                  {TEXT.PROFILE.PRIVACY.UPDATED_AT(
                    formatDateTimeMinute(privacy.updated_at),
                  )}
                </p>
              </div>
              <div className="flex min-h-11 min-w-11 shrink-0 items-center justify-center">
                {currentPending ? (
                  <Loader2
                    className="size-4 animate-spin text-muted-foreground motion-reduce:animate-none"
                    aria-label={TEXT.PROFILE.PRIVACY.UPDATING}
                  />
                ) : (
                  <Switch
                    checked={isPublic}
                    onCheckedChange={(checked) => {
                      void updateVisibility(checked)
                    }}
                    aria-label={
                      isPublic
                        ? TEXT.PROFILE.PRIVACY.DISABLE
                        : TEXT.PROFILE.PRIVACY.ENABLE
                    }
                  />
                )}
              </div>
            </div>

            <div className="flex flex-col gap-3 rounded-xl bg-muted/30 p-3 text-xs leading-5 text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
              <p className="flex-1">{TEXT.PROFILE.PRIVACY.SAFETY_NOTE}</p>
              {isPublic ? (
                <Link
                  href={APP_ROUTES.PUBLIC_PROFILE(username)}
                  className={buttonVariants({
                    variant: "outline",
                    size: "sm",
                    className: "shrink-0",
                  })}
                >
                  <ExternalLink className="size-3.5" aria-hidden="true" />
                  {TEXT.PROFILE.PRIVACY.VIEW_PUBLIC}
                </Link>
              ) : null}
            </div>
          </>
        ) : null}
      </CardContent>
    </Card>
  )
}
