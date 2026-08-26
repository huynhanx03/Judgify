"use client";

import { useState } from "react";
import {
  CircleAlert,
  Loader2,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  UserRoundSearch,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { notify } from "@/lib/toast";
import { APP_LANGUAGE } from "@/i18n/locale";
import {
  RewardProfileSummary,
  RewardProfileTraitGrid,
} from "@/modules/cultivation/reward-profile-summary";
import { TraitCard } from "@/modules/cultivation/trait-card";
import { AdminUserLookup } from "@/modules/admin/fields/admin-related-entity-fields";
import { cultivationService } from "@/services/cultivation.service";
import { rewardProfileService } from "@/services/reward-profile.service";
import type { EntityID } from "@/types/api";
import type {
  RewardProfileAssignmentPreview,
  RewardProfileResponse,
  TraitResponse,
} from "@/types/cultivation";

interface AssignmentDraft {
  userID: EntityID;
  rootTraitID: EntityID | null;
  talentTraitIDs: EntityID[];
}

function draftFromProfile(profile: RewardProfileResponse): AssignmentDraft {
  const root = profile.traits.find((trait) => trait.slot === "root_bone");
  const talents = profile.traits
    .filter((trait) => trait.slot === "talent")
    .sort((left, right) => left.slot_order - right.slot_order);
  return {
    userID: profile.user_id,
    rootTraitID: root?.trait_id ?? null,
    talentTraitIDs: talents.map((trait) => trait.trait_id),
  };
}

function sortedTraits(traits: TraitResponse[]): TraitResponse[] {
  return [...traits].sort(
    (left, right) =>
      left.display_order - right.display_order ||
      left.name.localeCompare(right.name, APP_LANGUAGE),
  );
}

export function AdminRewardProfile() {
  const { can } = useAuth();
  const canReadUsers = can(
    AUTHORIZATION_RESOURCE.USER,
    AUTHORIZATION_ACTION.READ,
  );
  const canRead = can(
    AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE,
    AUTHORIZATION_ACTION.READ,
  );
  const canPublish = can(
    AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE,
    AUTHORIZATION_ACTION.PUBLISH,
  );
  const [userID, setUserID] = useState<EntityID | null>(null);
  const [draft, setDraft] = useState<AssignmentDraft | null>(null);
  const [preview, setPreview] =
    useState<RewardProfileAssignmentPreview | null>(null);
  const [appliedProfile, setAppliedProfile] =
    useState<RewardProfileResponse | null>(null);
  const [reason, setReason] = useState("");
  const [isPreviewing, setIsPreviewing] = useState(false);
  const [isApplying, setIsApplying] = useState(false);

  const catalog = useRetryableResource<TraitResponse[]>({
    resetKey: canRead,
    enabled: canRead,
    initialData: [],
    keepPreviousData: true,
    load: (signal) => cultivationService.getAllTraits(signal),
  });
  const profileResource = useRetryableResource<RewardProfileResponse | null>({
    resetKey: userID,
    enabled: canRead && userID !== null,
    initialData: null,
    load: (signal) => rewardProfileService.adminCurrent(userID!, signal),
    onSuccess: (profile) => {
      if (!profile) return;
      setDraft(draftFromProfile(profile));
      setPreview(null);
      setAppliedProfile(null);
      setReason("");
    },
  });

  const currentProfile =
    appliedProfile?.user_id === userID
      ? appliedProfile
      : profileResource.dataKey === userID
        ? profileResource.data
        : null;
  const currentDraft = draft?.userID === userID ? draft : null;
  const usableTraits = catalog.data.filter(
    (trait) => trait.active && trait.effect_revision !== undefined,
  );
  const roots = sortedTraits(
    usableTraits.filter((trait) => trait.type === "root_bone"),
  );
  const talents = sortedTraits(
    usableTraits.filter((trait) => trait.type === "talent"),
  );
  const selectionReady = Boolean(
    userID &&
      currentDraft?.rootTraitID &&
      currentDraft.talentTraitIDs.length === 3,
  );
  const reasonValid = reason.trim().length >= 3 && reason.trim().length <= 1024;

  function selectUser(nextUserID: EntityID | null) {
    setUserID(nextUserID);
    setDraft(null);
    setPreview(null);
    setAppliedProfile(null);
    setReason("");
  }

  function updateDraft(update: (value: AssignmentDraft) => AssignmentDraft) {
    if (!currentDraft) return;
    setDraft(update(currentDraft));
    setPreview(null);
  }

  function chooseRoot(traitID: EntityID) {
    if (!canPublish) return;
    updateDraft((value) => ({ ...value, rootTraitID: traitID }));
  }

  function toggleTalent(traitID: EntityID) {
    if (!canPublish) return;
    updateDraft((value) => {
      const selected = value.talentTraitIDs.includes(traitID);
      if (selected) {
        return {
          ...value,
          talentTraitIDs: value.talentTraitIDs.filter((id) => id !== traitID),
        };
      }
      if (value.talentTraitIDs.length >= 3) return value;
      return { ...value, talentTraitIDs: [...value.talentTraitIDs, traitID] };
    });
  }

  async function createPreview() {
    if (!userID || !currentDraft || !selectionReady || isPreviewing) return;
    setIsPreviewing(true);
    try {
      const nextPreview = await rewardProfileService.preview(userID, {
        root_trait_id: currentDraft.rootTraitID!,
        talent_trait_ids: currentDraft.talentTraitIDs,
      });
      setPreview(nextPreview);
    } catch {
      setPreview(null);
      notify.error(ADMIN_TEXT.USER_TRAITS.APPLY_ERROR);
    } finally {
      setIsPreviewing(false);
    }
  }

  async function applyPreview() {
    if (
      !userID ||
      !currentDraft ||
      !preview ||
      !reasonValid ||
      isApplying
    ) return;
    setIsApplying(true);
    try {
      const profile = await rewardProfileService.apply({
        user_id: userID,
        expected_revision: preview.current_revision,
        root_trait_id: currentDraft.rootTraitID!,
        talent_trait_ids: currentDraft.talentTraitIDs,
        preview_checksum: preview.preview_checksum,
        reason,
      });
      setAppliedProfile(profile);
      setDraft(draftFromProfile(profile));
      setPreview(null);
      setReason("");
      notify.success(ADMIN_TEXT.USER_TRAITS.APPLY_SUCCESS);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setAppliedProfile(null);
        setPreview(null);
        profileResource.retry();
        catalog.retry();
        notify.warning(ADMIN_TEXT.USER_TRAITS.CONFLICT);
      } else {
        notify.error(ADMIN_TEXT.USER_TRAITS.APPLY_ERROR);
      }
    } finally {
      setIsApplying(false);
    }
  }

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex size-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <ShieldCheck className="size-5" aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              {ADMIN_TEXT.USER_TRAITS.TITLE}
            </h1>
            <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
              {ADMIN_TEXT.USER_TRAITS.SUBTITLE}
            </p>
          </div>
        </div>
      </header>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <UserRoundSearch className="size-4 text-primary" aria-hidden="true" />
            {ADMIN_TEXT.USER_TRAITS.STEP_USER}
          </CardTitle>
          <CardDescription>
            {ADMIN_TEXT.USER_TRAITS.STEP_USER_DESCRIPTION}
          </CardDescription>
        </CardHeader>
        <CardContent className="max-w-2xl">
          <AdminUserLookup
            id="reward-profile-user"
            value={userID}
            onChange={selectUser}
            enabled={canReadUsers}
            disabled={isApplying}
          />
        </CardContent>
      </Card>

      <Card aria-live="polite">
        <CardHeader>
          <CardTitle className="flex flex-wrap items-center gap-2">
            {ADMIN_TEXT.USER_TRAITS.CURRENT_TITLE}
            {currentProfile ? (
              <Badge variant="outline">
                {TEXT.PROFILE.REWARD_PROFILE.REVISION(currentProfile.revision)}
              </Badge>
            ) : null}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {!userID ? (
            <p className="rounded-xl border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
              {ADMIN_TEXT.USER_TRAITS.CURRENT_EMPTY}
            </p>
          ) : profileResource.status === "loading" && !currentProfile ? (
            <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground" role="status">
              <Loader2 className="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              {ADMIN_TEXT.USER_TRAITS.CURRENT_LOADING}
            </div>
          ) : profileResource.status === "error" && !currentProfile ? (
            <div className="flex min-h-40 flex-col items-center justify-center text-center" role="alert">
              <CircleAlert className="size-5 text-destructive" aria-hidden="true" />
              <p className="mt-3 text-sm font-medium">
                {ADMIN_TEXT.USER_TRAITS.CURRENT_LOAD_ERROR}
              </p>
              <Button type="button" variant="outline" className="mt-4" onClick={profileResource.retry}>
                <RefreshCw className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.USER_TRAITS.RETRY}
              </Button>
            </div>
          ) : currentProfile ? (
            <RewardProfileSummary profile={currentProfile} />
          ) : null}
        </CardContent>
      </Card>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.45fr)_minmax(22rem,0.55fr)]">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Sparkles className="size-4 text-primary" aria-hidden="true" />
              {ADMIN_TEXT.USER_TRAITS.STEP_SELECTION}
            </CardTitle>
            <CardDescription>
              {ADMIN_TEXT.USER_TRAITS.STEP_SELECTION_DESCRIPTION}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {!canPublish ? (
              <p className="mb-4 rounded-xl border border-warning/30 bg-warning/5 p-3 text-sm text-muted-foreground" role="status">
                {ADMIN_TEXT.USER_TRAITS.READ_ONLY}
              </p>
            ) : null}
            {catalog.status === "loading" ? (
              <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-muted-foreground" role="status">
                <Loader2 className="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                {ADMIN_TEXT.USER_TRAITS.CATALOG_LOADING}
              </div>
            ) : catalog.status === "error" ? (
              <div className="flex min-h-48 flex-col items-center justify-center text-center" role="alert">
                <CircleAlert className="size-5 text-destructive" aria-hidden="true" />
                <p className="mt-3 text-sm font-medium">
                  {ADMIN_TEXT.USER_TRAITS.CATALOG_LOAD_ERROR}
                </p>
                <Button type="button" variant="outline" className="mt-4" onClick={catalog.retry}>
                  <RefreshCw className="size-4" aria-hidden="true" />
                  {ADMIN_TEXT.USER_TRAITS.RETRY}
                </Button>
              </div>
            ) : (
              <div className="space-y-7">
                <fieldset disabled={!canPublish || !currentDraft || isApplying || isPreviewing}>
                  <legend className="mb-3 text-sm font-bold">
                    {ADMIN_TEXT.USER_TRAITS.ROOT_LABEL}
                  </legend>
                  <div className="grid max-h-[28rem] gap-2 overflow-y-auto pr-1 md:grid-cols-2">
                    {roots.map((trait) => (
                      <TraitCard
                        key={trait.id}
                        trait={trait}
                        variant="compact"
                        selected={currentDraft?.rootTraitID === trait.id}
                        onClick={canPublish && currentDraft ? () => chooseRoot(trait.id) : undefined}
                      />
                    ))}
                  </div>
                </fieldset>
                <fieldset disabled={!canPublish || !currentDraft || isApplying || isPreviewing}>
                  <legend className="mb-1 text-sm font-bold">
                    {ADMIN_TEXT.USER_TRAITS.TALENT_LABEL}
                  </legend>
                  <p className="mb-3 text-xs text-muted-foreground" aria-live="polite">
                    {ADMIN_TEXT.USER_TRAITS.TALENT_PROGRESS(
                      currentDraft?.talentTraitIDs.length ?? 0,
                    )}
                  </p>
                  <div className="grid max-h-[36rem] gap-2 overflow-y-auto pr-1 md:grid-cols-2">
                    {talents.map((trait) => (
                      <TraitCard
                        key={trait.id}
                        trait={trait}
                        variant="compact"
                        selected={currentDraft?.talentTraitIDs.includes(trait.id)}
                        onClick={canPublish && currentDraft ? () => toggleTalent(trait.id) : undefined}
                      />
                    ))}
                  </div>
                </fieldset>
                <Button
                  type="button"
                  className="min-h-11 w-full sm:w-auto"
                  onClick={() => void createPreview()}
                  disabled={!canPublish || !selectionReady || isPreviewing || isApplying}
                >
                  {isPreviewing ? (
                    <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  ) : (
                    <ShieldCheck className="size-4" aria-hidden="true" />
                  )}
                  {isPreviewing
                    ? ADMIN_TEXT.USER_TRAITS.PREVIEWING
                    : ADMIN_TEXT.USER_TRAITS.PREVIEW_ACTION}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>

        <Card className="xl:sticky xl:top-24">
          <CardHeader>
            <CardTitle>{ADMIN_TEXT.USER_TRAITS.STEP_PREVIEW}</CardTitle>
            <CardDescription>
              {ADMIN_TEXT.USER_TRAITS.STEP_PREVIEW_DESCRIPTION}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            {!preview ? (
              <p className="rounded-xl border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                {ADMIN_TEXT.USER_TRAITS.PREVIEW_EMPTY}
              </p>
            ) : (
              <div className="space-y-5" aria-live="polite">
                <div className="rounded-xl border border-primary/25 bg-primary/5 p-4">
                  <p className="font-semibold">
                    {ADMIN_TEXT.USER_TRAITS.PREVIEW_TITLE}
                  </p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {ADMIN_TEXT.USER_TRAITS.PREVIEW_REVISION(
                      preview.current_revision,
                      preview.next_revision,
                    )}
                  </p>
                  <p className="mt-3 text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.USER_TRAITS.PREVIEW_CHECKSUM}
                  </p>
                  <p className="mt-1 break-all font-mono text-xs" title={preview.preview_checksum}>
                    {preview.preview_checksum}
                  </p>
                </div>
                <RewardProfileTraitGrid traits={preview.traits} idPrefix="admin-preview" />
                <div className="space-y-2">
                  <Label htmlFor="reward-profile-reason">
                    {ADMIN_TEXT.USER_TRAITS.REASON_LABEL}
                  </Label>
                  <Textarea
                    id="reward-profile-reason"
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    placeholder={ADMIN_TEXT.USER_TRAITS.REASON_PLACEHOLDER}
                    maxLength={1024}
                    disabled={isApplying}
                    aria-describedby="reward-profile-reason-help"
                    aria-invalid={reason.length > 0 && !reasonValid}
                  />
                  <p id="reward-profile-reason-help" className="text-xs text-muted-foreground">
                    {ADMIN_TEXT.USER_TRAITS.REASON_HELP}
                  </p>
                </div>
                <Button
                  type="button"
                  className="min-h-11 w-full"
                  onClick={() => void applyPreview()}
                  disabled={!canPublish || !reasonValid || isApplying}
                >
                  {isApplying ? (
                    <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  ) : (
                    <ShieldCheck className="size-4" aria-hidden="true" />
                  )}
                  {isApplying
                    ? ADMIN_TEXT.USER_TRAITS.APPLYING
                    : ADMIN_TEXT.USER_TRAITS.APPLY_ACTION}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
