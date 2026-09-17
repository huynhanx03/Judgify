"use client"

import { useEffect, useState, type FormEvent } from "react"
import {
  CircleAlert,
  Loader2,
  RefreshCw,
  ShieldCheck,
  UserRoundPen,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { TEXT } from "@/constants/text"
import { useAuth } from "@/contexts/auth-context"
import { ApiError } from "@/lib/api/error"
import { getErrorMessage, notify } from "@/lib/toast"
import { userService } from "@/services/user.service"
import type {
  ProfileAttributesResponse,
  UserProfile,
} from "@/types/user"
import { ProfileAttributeField } from "./profile-attribute-field"
import {
  createProfileAttributeDrafts,
  planProfileAttributeMutations,
  type ProfileAttributeDraftValue,
  type ProfileAttributeDrafts,
  type ProfileAttributeFieldErrors,
} from "./profile-attribute-model"

interface ProfileEditDialogProps {
  open: boolean
  profile: UserProfile
  onOpenChange: (open: boolean) => void
  onSaved: (profile: UserProfile) => void
}

export function ProfileEditDialog({
  open,
  profile,
  onOpenChange,
  onSaved,
}: ProfileEditDialogProps) {
  const { setUser } = useAuth()
  const [collection, setCollection] =
    useState<ProfileAttributesResponse | null>(null)
  const [drafts, setDrafts] = useState<ProfileAttributeDrafts>({})
  const [fieldErrors, setFieldErrors] =
    useState<ProfileAttributeFieldErrors>({})
  const [loadError, setLoadError] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [needsReload, setNeedsReload] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [loadAttempt, setLoadAttempt] = useState(0)

  useEffect(() => {
    if (!open) return
    const controller = new AbortController()
    setIsLoading(true)
    setLoadError(null)
    setSubmitError(null)
    setNeedsReload(false)
    setFieldErrors({})
    void userService
      .getProfileAttributes(controller.signal)
      .then((response) => {
        if (controller.signal.aborted) return
        if (response.profile.id !== profile.id) {
          throw new TypeError(TEXT.PROFILE.ATTRIBUTES.IDENTITY_MISMATCH)
        }
        setCollection(response)
        setDrafts(createProfileAttributeDrafts(response.attributes))
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        setCollection(null)
        setLoadError(
          getErrorMessage(error, TEXT.PROFILE.ATTRIBUTES.LOAD_ERROR_DESCRIPTION),
        )
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoading(false)
      })
    return () => controller.abort()
  }, [loadAttempt, open, profile.id])

  // The response schema verifies the backend's canonical
  // display_order/key/definition_id ordering. Preserve it here so an admin's
  // authored profile layout is identical across onboarding and self-service.
  const attributes = collection?.attributes ?? []

  function updateDraft(
    definitionID: string,
    value: ProfileAttributeDraftValue,
  ) {
    setDrafts((current) => ({ ...current, [definitionID]: value }))
    setFieldErrors((current) => {
      if (!current[definitionID]) return current
      const next = { ...current }
      delete next[definitionID]
      return next
    })
    setSubmitError(null)
    setNeedsReload(false)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!collection || isLoading || isSaving) return
    const plan = planProfileAttributeMutations(collection.attributes, drafts, {
      requireOnboardingValues: true,
    })
    if (!plan.ok) {
      setFieldErrors(plan.errors)
      return
    }
    if (plan.mutations.length === 0) {
      notify.info(TEXT.PROFILE.ATTRIBUTES.NO_CHANGES)
      onOpenChange(false)
      return
    }

    setIsSaving(true)
    setSubmitError(null)
    setNeedsReload(false)
    try {
      const updated = await userService.updateProfileAttributes({
        mutations: plan.mutations,
      })
      const updatedProfile: UserProfile = {
        ...profile,
        ...updated.profile,
        // Optional compatibility projections are omitted by JSON when a
        // value is unset. Assign them explicitly so object spreading cannot
        // retain stale identity text in the current session.
        first_name: updated.profile.first_name,
        last_name: updated.profile.last_name,
        birthday: updated.profile.birthday,
      }
      onSaved(updatedProfile)
      setUser(updatedProfile)
      notify.success(TEXT.PROFILE.UPDATE_SUCCESS)
      onOpenChange(false)
    } catch (error) {
      const conflict = error instanceof ApiError && error.status === 409
      setNeedsReload(conflict)
      setSubmitError(
        conflict
          ? TEXT.PROFILE.ATTRIBUTES.CONFLICT
          : getErrorMessage(error, TEXT.PROFILE.UPDATE_ERROR),
      )
    } finally {
      setIsSaving(false)
    }
  }

  const busy = isLoading || isSaving

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!isSaving) onOpenChange(nextOpen)
      }}
    >
      <DialogContent className="max-h-[92vh] overflow-y-auto border-border/70 bg-background/95 shadow-2xl sm:max-w-2xl">
        <form onSubmit={handleSubmit} className="grid gap-5">
          <DialogHeader>
            <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
              <UserRoundPen className="size-5" aria-hidden="true" />
            </div>
            <DialogTitle className="text-xl font-bold tracking-tight">
              {TEXT.PROFILE.EDIT_PROFILE}
            </DialogTitle>
            <DialogDescription className="leading-6">
              {TEXT.PROFILE.ATTRIBUTES.DESCRIPTION}
            </DialogDescription>
          </DialogHeader>

          <div className="flex gap-3 rounded-2xl border border-primary/15 bg-primary/[0.06] px-4 py-3 text-sm text-muted-foreground">
            <ShieldCheck className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden="true" />
            <p className="leading-6">{TEXT.PROFILE.ATTRIBUTES.SCHEMA_NOTE}</p>
          </div>

          {isLoading ? (
            <div className="flex min-h-56 flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-border text-muted-foreground" role="status">
              <Loader2 className="size-6 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              <span className="text-sm">{TEXT.PROFILE.ATTRIBUTES.LOADING}</span>
            </div>
          ) : loadError ? (
            <div className="flex min-h-56 flex-col items-center justify-center rounded-2xl border border-destructive/25 bg-destructive/5 px-6 text-center" role="alert">
              <CircleAlert className="size-6 text-destructive" aria-hidden="true" />
              <h3 className="mt-3 font-semibold">{TEXT.PROFILE.ATTRIBUTES.LOAD_ERROR_TITLE}</h3>
              <p className="mt-1 max-w-md text-sm leading-6 text-muted-foreground">
                {loadError}
              </p>
              <Button
                type="button"
                variant="outline"
                className="mt-4"
                onClick={() => setLoadAttempt((attempt) => attempt + 1)}
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.COMMON.RETRY}
              </Button>
            </div>
          ) : attributes.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-border px-6 py-12 text-center text-sm text-muted-foreground">
              {TEXT.PROFILE.ATTRIBUTES.EMPTY}
            </div>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2">
              {attributes.map((attribute) => (
                <ProfileAttributeField
                  key={attribute.definition_id}
                  attribute={attribute}
                  value={drafts[attribute.definition_id]}
                  error={fieldErrors[attribute.definition_id]}
                  disabled={isSaving}
                  allowUnset={!attribute.required_on_onboarding}
                  onChange={(value) => updateDraft(attribute.definition_id, value)}
                />
              ))}
            </div>
          )}

          {submitError ? (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive" role="alert">
              <span className="min-w-0 flex-1">{submitError}</span>
              {needsReload ? (
                <Button
                  type="button"
                  variant="outline"
                  size="xs"
                  onClick={() => setLoadAttempt((attempt) => attempt + 1)}
                >
                  <RefreshCw className="size-3.5" aria-hidden="true" />
                  {TEXT.PROFILE.ATTRIBUTES.RELOAD_SCHEMA}
                </Button>
              ) : null}
            </div>
          ) : null}

          <DialogFooter className="mt-1">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSaving}
              className="min-h-11 px-4"
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button
              type="submit"
              disabled={busy || !collection || Boolean(loadError)}
              className="min-h-11 px-4"
            >
              {isSaving ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {isSaving ? TEXT.PROFILE.SAVING : TEXT.PROFILE.SAVE_CHANGES}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
