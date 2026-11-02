"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { CircleAlert, Loader2, ShieldCheck } from "lucide-react";
import { CardContent } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { useLocale } from "@/contexts/locale-context";
import { APP_ROUTES } from "@/constants/routes";
import { REGISTRATION_TRAIT_RULES } from "@/constants/registration";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import type { OAuthCallbackQueryResult } from "@/lib/auth/oauth-callback";
import {
  parseRegistrationTraitOffer,
  type RegistrationTraitOffer,
} from "@/lib/auth/registration-trait-offer";
import { cn } from "@/lib/utils";
import { isoDateTimeSchema } from "@/lib/api/contracts";
import { notify } from "@/lib/toast";
import { TraitCodexModal } from "@/modules/cultivation/trait-codex-modal";
import { AuthSurface } from "@/modules/auth/auth-surface";
import { TraitSelectionSection } from "@/modules/auth/sections/trait-selection-section";
import { OnboardingProfileFields } from "@/modules/auth/onboarding-profile-fields";
import { useOnboardingProfileForm } from "@/modules/auth/hooks/use-onboarding-profile-form";
import { OnboardingStepper } from "@/modules/auth/onboarding-stepper";
import { authService } from "@/services/auth.service";
import { cultivationService } from "@/services/cultivation.service";
import type { EntityID } from "@/types/api";
import type { TraitResponse } from "@/types/cultivation";
import type {
  ProfileValueSuggestion,
  TraitOfferSelectionRequest,
} from "@/types/auth";

const MAXIMUM_BROWSER_TIMEOUT_MILLISECONDS = 2_147_000_000;

function callbackError(result: OAuthCallbackQueryResult, t: ReturnType<typeof useLocale>["t"]): string {
  if (!result.ok && result.reason === "provider_denied") {
    return t("AUTH.OAUTH_PROVIDER_DENIED");
  }
  if (!result.ok && result.reason === "invalid_callback") {
    return t("AUTH.OAUTH_CALLBACK_INVALID");
  }
  return t("AUTH.OAUTH_CALLBACK_ERROR");
}

export function OAuthCompletionClient({
  result,
}: {
  result: OAuthCallbackQueryResult;
}) {
  const { t } = useLocale();
  const auth = useAuth();
  const router = useRouter();
  const startedRef = useRef(false);
  const offerRequestRef = useRef(0);
  const onboardingTraitStepRequested = useRef(false);
  const offerAbortRef = useRef<AbortController | null>(null);
  const completionExpiresAtRef = useRef<number | null>(null);
  const existingLoginFinalizeRef = useRef(false);
  const onboardingFinalizeRef = useRef(false);
  const [attempt, setAttempt] = useState(0);
  const [failed, setFailed] = useState(!result.ok);
  const [failureMessage, setFailureMessage] = useState(callbackError(result, t));
  const [onboardingRequired, setOnboardingRequired] = useState(false);
  const [onboardingStep, setOnboardingStep] = useState(0);
  const [profileSuggestions, setProfileSuggestions] = useState<
    ProfileValueSuggestion[]
  >([]);
  const [traitOffer, setTraitOffer] = useState<RegistrationTraitOffer | null>(null);
  const [selectedTalents, setSelectedTalents] = useState<EntityID[]>([]);
  const [rollExpired, setRollExpired] = useState(false);
  const [isRolling, setIsRolling] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [operationError, setOperationError] = useState<string | null>(null);
  const [showCodex, setShowCodex] = useState(false);
  const profileForm = useOnboardingProfileForm({
    enabled: onboardingRequired && !failed,
    suggestions: profileSuggestions,
  });
  const traitCatalog = useRetryableResource<TraitResponse[]>({
    resetKey: "oauth-onboarding-trait-catalog",
    enabled: showCodex,
    initialData: [],
    keepPreviousData: true,
    load: (signal) => cultivationService.getAllTraits(signal),
  });

  const rollTraits = useCallback(async () => {
    const requestID = ++offerRequestRef.current;
    offerAbortRef.current?.abort(
      new DOMException("OAuth onboarding offer superseded", "AbortError"),
    );
    const controller = new AbortController();
    offerAbortRef.current = controller;
    setIsRolling(true);
    setOperationError(null);
    // Issuing a new offer supersedes the previous durable aggregate. Remove
    // the old projection immediately so a lost reroll response can never be
    // submitted as though it were still authoritative.
    setTraitOffer(null);
    setSelectedTalents([]);
    setRollExpired(false);
    try {
      const requestStartedAt = Date.now();
      const response = await cultivationService.createTraitOffer(controller.signal);
      const offer = parseRegistrationTraitOffer(response, requestStartedAt);
      if (!offer) throw new Error("invalid onboarding trait offer response");
      if (requestID !== offerRequestRef.current) return;
      const completionExpiresAt = completionExpiresAtRef.current;
      if (completionExpiresAt === null || !Number.isFinite(completionExpiresAt)) {
        throw new Error("invalid OAuth completion deadline");
      }
      const effectiveExpiresAtEpochMs = Math.min(
        offer.expiresAtEpochMs,
        completionExpiresAt,
      );
      const effectiveOffer =
        effectiveExpiresAtEpochMs === offer.expiresAtEpochMs
          ? offer
          : {
              ...offer,
              expiresAt: isoDateTimeSchema.parse(
                new Date(effectiveExpiresAtEpochMs).toISOString(),
              ),
              expiresAtEpochMs: effectiveExpiresAtEpochMs,
            };
      setTraitOffer(effectiveOffer);
      setSelectedTalents([]);
      setRollExpired(effectiveExpiresAtEpochMs <= Date.now());
    } catch (error) {
      if (
        requestID === offerRequestRef.current &&
        !(error instanceof DOMException && error.name === "AbortError")
      ) {
        setOperationError(t("AUTH.TRAIT_FETCH_ERROR"));
      }
    } finally {
      if (requestID === offerRequestRef.current) {
        offerAbortRef.current = null;
        setIsRolling(false);
      }
    }
  }, [t]);

  useEffect(() => {
    if (!result.ok || startedRef.current) return;
    // The protected link callback can arrive before the canonical session
    // projection finishes bootstrapping. Wait for that read instead of
    // presenting a false authentication failure that requires a manual retry.
    if (result.purpose === "link" && auth.isLoading) return;
    startedRef.current = true;
    setFailed(false);
    setFailureMessage(t("AUTH.OAUTH_CALLBACK_ERROR"));
    const complete = async () => {
      if (result.purpose === "link") {
        if (!auth.isAuthenticated) throw new Error("link session unavailable");
        await authService.finalizeOAuthLink();
        notify.success(t("AUTH.OAUTH_LINK_SUCCESS"));
        router.replace(APP_ROUTES.PROFILE);
        return;
      }
      if (existingLoginFinalizeRef.current) {
        // The finalize transaction may have committed even when its response
        // or the following session projection was lost. Resolve that unknown
        // outcome from canonical session state first; otherwise replay the
        // exact finalize command with SessionProvider's retained attempt key.
        if (await auth.refreshCapabilities()) {
          router.replace(APP_ROUTES.ARENA);
          return;
        }
        await auth.completeOAuth({}, APP_ROUTES.ARENA);
        return;
      }
      const inspectionStartedAt = Date.now();
      const inspection = await authService.inspectOAuthLogin();
      setProfileSuggestions(inspection.profile_suggestions ?? []);
      const serverTime = Date.parse(inspection.server_time);
      const serverExpiresAt = Date.parse(inspection.expires_at);
      completionExpiresAtRef.current =
        inspectionStartedAt + Math.max(0, serverExpiresAt - serverTime);
      if (!inspection.onboarding_required) {
        existingLoginFinalizeRef.current = true;
        await auth.completeOAuth({}, APP_ROUTES.ARENA);
        return;
      }
      setOnboardingRequired(true);
    };
    void complete().catch(() => setFailed(true));
  }, [attempt, auth, result, rollTraits, router, t]);

  useEffect(() => {
    if (onboardingRequired && onboardingStep === 1 && !traitOffer && !onboardingTraitStepRequested.current) {
      onboardingTraitStepRequested.current = true;
      void rollTraits();
    }
    if (onboardingStep !== 1) onboardingTraitStepRequested.current = false;
  }, [onboardingRequired, onboardingStep, rollTraits, traitOffer]);

  useEffect(
    () => () => {
      offerRequestRef.current += 1;
      offerAbortRef.current?.abort(
        new DOMException("OAuth onboarding closed", "AbortError"),
      );
    },
    [],
  );

  useEffect(() => {
    if (!traitOffer) return;
    const remainingMilliseconds = traitOffer.expiresAtEpochMs - Date.now();
    const timeout = window.setTimeout(
      () => setRollExpired(true),
      Math.max(
        0,
        Math.min(remainingMilliseconds, MAXIMUM_BROWSER_TIMEOUT_MILLISECONDS),
      ),
    );
    return () => window.clearTimeout(timeout);
  }, [traitOffer]);

  const retryCompletion = () => {
    onboardingTraitStepRequested.current = false;
    startedRef.current = false;
    setFailed(false);
    setFailureMessage(t("AUTH.OAUTH_CALLBACK_ERROR"));
    setProfileSuggestions([]);
    setAttempt((value) => value + 1);
  };

  const toggleTalent = (id: EntityID) => {
    if (!traitOffer?.talents.some((talent) => talent.id === id)) return;
    setSelectedTalents((current) => {
      if (current.includes(id)) {
        return current.filter((talentID) => talentID !== id);
      }
      if (current.length >= REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT) {
        notify.warning(t("AUTH.TRAIT_MAX_TALENTS"));
        return current;
      }
      return [...current, id];
    });
  };

  const finishOnboarding = async () => {
    if (
      !traitOffer ||
      rollExpired ||
      selectedTalents.length !== REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT
    ) {
      setOperationError(
        rollExpired
          ? t("AUTH.TRAIT_ROLL_EXPIRED")
          : t("AUTH.TRAIT_TALENTS_REQUIRED"),
      );
      return;
    }
    const profileEvidence = profileForm.buildEvidence();
    if (!profileEvidence) {
      setOperationError(t("ONBOARDING_PROFILE.COMPLETE_REQUIRED"));
      return;
    }
    if (onboardingFinalizeRef.current) {
      // Resolve a previously unknown outcome from canonical session state
      // before replaying the exact command. A successful finalize response
      // clears the scoped completion ticket, so blindly resubmitting after a
      // later /session/me failure would otherwise strand an authenticated
      // user on this page.
      try {
        if (await auth.refreshCapabilities()) {
          router.replace(APP_ROUTES.ARENA);
          return;
        }
      } catch {
        // A failed canonical read does not prove that finalize committed.
        // Continue with SessionProvider's retained idempotency key so the
        // server can either recover the receipt or execute the first attempt.
      }
    }
    const offerSelection: TraitOfferSelectionRequest = {
      offer_id: traitOffer.id,
      offer_sequence: traitOffer.sequence,
      catalog_revision_id: traitOffer.catalogRevisionId,
      catalog_checksum: traitOffer.catalogChecksum,
      offer_checksum: traitOffer.offerChecksum,
      root_bone_id: traitOffer.rootBone.id,
      talent_ids: [...selectedTalents],
    };
    setIsSubmitting(true);
    setOperationError(null);
    onboardingFinalizeRef.current = true;
    try {
      await auth.completeOAuth(
        { onboarding: { ...offerSelection, ...profileEvidence } },
        APP_ROUTES.ARENA,
      );
    } catch (error) {
      try {
        if (await auth.refreshCapabilities()) {
          router.replace(APP_ROUTES.ARENA);
          return;
        }
      } catch {
        // Preserve the original finalize failure and allow an exact retry.
      }
      if (profileForm.handleSubmissionError(error)) {
        setOperationError(t("ONBOARDING_PROFILE.SUBMISSION_REVIEW_REQUIRED"));
        return;
      }
      setOperationError(t("AUTH.OAUTH_ONBOARDING_COMPLETE_ERROR"));
    } finally {
      setIsSubmitting(false);
    }
  };

  const isLink = result.ok && result.purpose === "link";
  const returnPath = isLink ? APP_ROUTES.PROFILE : APP_ROUTES.LOGIN;
  const returnLabel = isLink
    ? t("AUTH.OAUTH_BACK_TO_PROFILE")
    : t("AUTH.BACK_TO_LOGIN");

  if (onboardingRequired && !failed) {
    return (
      <>
        <AuthSurface className="mx-auto max-w-2xl">
          <CardContent className="px-5 py-7 sm:px-8">
            <div className="text-center">
              <h1 className="text-xl font-bold tracking-tight">
                {t("AUTH.OAUTH_ONBOARDING_TITLE")}
              </h1>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("AUTH.OAUTH_ONBOARDING_DESCRIPTION")}
              </p>
            </div>

            {isRolling && !traitOffer ? (
              <div className="mt-6 flex items-center justify-center gap-2 text-sm text-muted-foreground" role="status" aria-live="polite">
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                {t("AUTH.OAUTH_ONBOARDING_LOADING")}
              </div>
            ) : null}

            {operationError ? (
              <div className="mt-5 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">
                <p>{operationError}</p>
                {!traitOffer ? (
                  <Button type="button" variant="outline" className="mt-3" onClick={() => void rollTraits()}>
                    {t("AUTH.OAUTH_ONBOARDING_RETRY")}
                  </Button>
                ) : null}
              </div>
            ) : null}

            <OnboardingStepper
              steps={[
                { id: "profile", label: t("AUTH.ONBOARDING_PROFILE") },
                { id: "character", label: t("AUTH.ONBOARDING_CHARACTER") },
              ]}
              activeStep={onboardingStep}
              title={onboardingStep === 0 ? t("AUTH.ONBOARDING_PROFILE_TITLE") : t("AUTH.ONBOARDING_CHARACTER_TITLE")}
            >
              {onboardingStep === 0 ? (
                <OnboardingProfileFields
                  form={profileForm}
                  disabled={isSubmitting}
                  className="mt-2"
                />
              ) : (
                <TraitSelectionSection
                  rolledRootBone={traitOffer?.rootBone ?? null}
                  rolledTalents={traitOffer?.talents ?? []}
                  selectedTalents={selectedTalents}
                  rollTicketExpiresAt={traitOffer?.expiresAt ?? ""}
                  remainingOffers={traitOffer?.remainingOffers}
                  rollExpired={rollExpired}
                  isRolling={isRolling}
                  onRoll={() => void rollTraits()}
                  onToggleTalent={toggleTalent}
                  onOpenCodex={() => setShowCodex(true)}
                  disabled={isSubmitting}
                />
              )}
            </OnboardingStepper>

            <div className="mt-6 flex flex-col gap-3 sm:flex-row-reverse">
              {onboardingStep === 0 ? (
                <Button
                  type="button"
                  className="min-h-11 flex-1"
                  disabled={isSubmitting || profileForm.status !== "ready"}
                  onClick={() => {
                    if (!profileForm.buildEvidence()) {
                      setOperationError(t("ONBOARDING_PROFILE.COMPLETE_REQUIRED"));
                      return;
                    }
                    setOperationError(null);
                    setOnboardingStep(1);
                  }}
                >
                  {t("AUTH.ONBOARDING_CONTINUE")}
                </Button>
              ) : (
                <Button
                  type="button"
                  className="min-h-11 flex-1"
                  disabled={
                    isSubmitting || isRolling || !traitOffer || rollExpired ||
                    selectedTalents.length !== REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT
                  }
                  onClick={() => void finishOnboarding()}
                >
                  {isSubmitting ? <Loader2 className="mr-2 size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
                  {isSubmitting ? t("AUTH.OAUTH_ONBOARDING_SUBMITTING") : t("AUTH.OAUTH_ONBOARDING_COMPLETE")}
                </Button>
              )}
              {onboardingStep > 0 ? (
                <Button type="button" variant="outline" className="min-h-11 flex-1" disabled={isSubmitting} onClick={() => setOnboardingStep(0)}>
                  {t("AUTH.ONBOARDING_BACK")}
                </Button>
              ) : null}
            </div>
          </CardContent>
        </AuthSurface>
        <TraitCodexModal
          open={showCodex}
          onClose={() => setShowCodex(false)}
          traits={traitCatalog.data}
          status={traitCatalog.status}
          onRetry={traitCatalog.retry}
        />
      </>
    );
  }

  const description = isLink
    ? t("AUTH.OAUTH_LINK_DESCRIPTION")
    : t("AUTH.OAUTH_CALLBACK_DESCRIPTION");

  return (
    <AuthSurface className="mx-auto max-w-md overflow-hidden">
      <CardContent className="flex min-h-80 flex-col items-center justify-center px-6 py-10 text-center sm:px-10">
        {!failed ? (
          <div className="flex flex-col items-center" role="status" aria-live="polite">
            <div className="relative mb-6 flex size-16 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary">
              <ShieldCheck className="size-7" aria-hidden="true" />
              <Loader2 className="absolute -right-1 -top-1 size-5 animate-spin rounded-full bg-background motion-reduce:animate-none" aria-hidden="true" />
            </div>
            <h1 className="text-xl font-bold tracking-tight">
              {t("AUTH.OAUTH_CALLBACK_TITLE")}
            </h1>
            <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
              {description}
            </p>
          </div>
        ) : (
          <div className="flex flex-col items-center" role="alert">
            <div className="mb-6 flex size-16 items-center justify-center rounded-2xl border border-destructive/20 bg-destructive/10 text-destructive">
              <CircleAlert className="size-7" aria-hidden="true" />
            </div>
            <h1 className="text-xl font-bold tracking-tight">
              {t("AUTH.OAUTH_CALLBACK_FAILED_TITLE")}
            </h1>
            <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
              {failureMessage}
            </p>
            {result.ok ? (
              <Button type="button" size="lg" className="mt-7" onClick={retryCompletion}>
                {t("AUTH.OAUTH_ONBOARDING_RETRY")}
              </Button>
            ) : (
              <Link
                href={returnPath}
                className={cn(buttonVariants({ size: "lg" }), "mt-7 min-h-11 cursor-pointer px-5")}
              >
                {returnLabel}
              </Link>
            )}
          </div>
        )}
      </CardContent>
    </AuthSurface>
  );
}
