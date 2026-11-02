"use client";

import { useState, useEffect, useCallback, useRef, type FormEvent } from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/auth-context";
import { cultivationService } from "@/services/cultivation.service";
import { notify } from "@/lib/toast";
import { ApiError } from "@/lib/api/error";
import { useLocale } from "@/contexts/locale-context";
import { REGISTRATION_TRAIT_RULES } from "@/constants/registration";
import {
  parseRegistrationTraitOffer,
  type RegistrationTraitOffer,
} from "@/lib/auth/registration-trait-offer";
import {
  isPasswordValid,
  isUsernameValid,
  normalizeUsername,
} from "@/lib/auth/credentials";
import { PersonalInfoSection } from "@/modules/auth/sections/personal-info-section";
import { TraitSelectionSection } from "@/modules/auth/sections/trait-selection-section";
import type { TraitResponse } from "@/types/cultivation";
import type { EntityID } from "@/types/api";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { APP_ROUTES } from "@/constants/routes";
import { OnboardingProfileFields } from "@/modules/auth/onboarding-profile-fields";
import { useOnboardingProfileForm } from "@/modules/auth/hooks/use-onboarding-profile-form";
import { OnboardingStepper } from "@/modules/auth/onboarding-stepper";

/**
 * The codex modal is deferred: it mounts closed (`showCodex` starts false), so
 * it is never part of the first paint, and its Dialog import is what pulls the
 * @base-ui dialog chunks into /register. `next/dynamic` has no `.preload()` in
 * the App Router, so the loader is held here and called directly on mount to
 * warm webpack's chunk cache long before the codex button can be clicked.
 */
const importTraitCodexModal = () =>
  import("@/modules/cultivation/trait-codex-modal");

const TraitCodexModal = dynamic(
  () => importTraitCodexModal().then((m) => m.TraitCodexModal),
  { ssr: false },
);

const MAXIMUM_BROWSER_TIMEOUT_MILLISECONDS = 2_147_000_000;

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

export default function RegisterFlow() {
  const { register, retryBootstrap } = useAuth();
  const { t } = useLocale();
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [activeStep, setActiveStep] = useState(0);

  // Basic Info
  const [form, setForm] = useState({
    username: "",
    password: "",
  });
  const profileForm = useOnboardingProfileForm();

  // Traits
  const [traitOffer, setTraitOffer] = useState<RegistrationTraitOffer | null>(null);
  const [selectedTalents, setSelectedTalents] = useState<EntityID[]>([]);
  const [rollExpired, setRollExpired] = useState(false);
  const [isRolling, setIsRolling] = useState(false);
  const [showCodex, setShowCodex] = useState(false);
  const rollRequestSequence = useRef(0);
  const rollAbortControllerRef = useRef<AbortController | null>(null);
  const rollInFlightRef = useRef(false);
  const traitStepRequested = useRef(false);
  const traitCatalog = useRetryableResource<TraitResponse[]>({
    resetKey: "registration-trait-catalog",
    enabled: showCodex,
    initialData: [],
    keepPreviousData: true,
    load: (signal) => cultivationService.getAllTraits(signal),
  });

  function updateField(field: string, value: string) {
    setForm((prev) => ({ ...prev, [field]: value }));
  }

  const rolledRootBone = traitOffer?.rootBone ?? null;
  const rolledTalents = traitOffer?.talents ?? [];

  // Request one durable offer — 1 root bone (auto-selected), 6 talents.
  const rollTraits = useCallback(async () => {
    if (rollInFlightRef.current) return;
    rollInFlightRef.current = true;
    const requestSequence = ++rollRequestSequence.current;
    rollAbortControllerRef.current?.abort(
      new DOMException("Registration roll superseded", "AbortError"),
    );
    const controller = new AbortController();
    rollAbortControllerRef.current = controller;
    setIsRolling(true);
    setSelectedTalents([]);
    setTraitOffer(null);
    setRollExpired(false);

    try {
      const requestStartedAt = Date.now();
      const result = await cultivationService.createTraitOffer(
        controller.signal,
      );
      const offer = parseRegistrationTraitOffer(result, requestStartedAt);
      if (!offer) {
        throw new Error("invalid onboarding trait offer response");
      }
      if (requestSequence !== rollRequestSequence.current) return;
      setTraitOffer(offer);
      setRollExpired(offer.expiresAtEpochMs <= Date.now());
    } catch (error) {
      if (
        requestSequence === rollRequestSequence.current &&
        !isAbortError(error)
      ) {
        notify.error(t("AUTH.TRAIT_FETCH_ERROR"));
      }
    } finally {
      if (requestSequence === rollRequestSequence.current) {
        rollAbortControllerRef.current = null;
        rollInFlightRef.current = false;
        setIsRolling(false);
      }
    }
  }, []);

  useEffect(
    () => () => {
      rollRequestSequence.current += 1;
      rollAbortControllerRef.current?.abort(
        new DOMException("Registration flow closed", "AbortError"),
      );
    },
    [],
  );

  useEffect(() => {
    void importTraitCodexModal();
  }, []);

  useEffect(() => {
    if (!traitOffer) return;
    const remainingMs = traitOffer.expiresAtEpochMs - Date.now();
    const timeout = window.setTimeout(
      () => setRollExpired(true),
      Math.max(
        0,
        Math.min(
          remainingMs,
          MAXIMUM_BROWSER_TIMEOUT_MILLISECONDS,
        ),
      ),
    );
    return () => window.clearTimeout(timeout);
  }, [traitOffer]);

  const toggleTalent = (id: EntityID) => {
    if (!rolledTalents.some((talent) => talent.id === id)) return;
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

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (activeStep === 0) {
      if (!isUsernameValid(form.username) || !isPasswordValid(form.password)) {
        notify.warning(t("AUTH.REGISTER_MISSING_INFO"));
        return;
      }
      setActiveStep(1);
      return;
    }
    const profileEvidence = profileForm.buildEvidence();
    if (activeStep === 1 && profileEvidence) {
      setActiveStep(2);
      return;
    }
    if (activeStep !== 2 || !profileEvidence) {
      notify.warning(t("ONBOARDING_PROFILE.COMPLETE_REQUIRED"));
      return;
    }
    if (!rolledRootBone) {
      notify.warning(t("AUTH.TRAIT_ROOT_BONE_REQUIRED"));
      return;
    }
    if (selectedTalents.length !== REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT) {
      notify.warning(t("AUTH.TRAIT_TALENTS_REQUIRED"));
      return;
    }
    if (!traitOffer || rollExpired) {
      notify.warning(t("AUTH.TRAIT_ROLL_EXPIRED"));
      return;
    }
    setIsLoading(true);
    try {
      await register({
        username: normalizeUsername(form.username),
        password: form.password,
        ...profileEvidence,
        root_bone_id: rolledRootBone.id,
        talent_ids: selectedTalents,
        offer_id: traitOffer.id,
        offer_sequence: traitOffer.sequence,
        catalog_revision_id: traitOffer.catalogRevisionId,
        catalog_checksum: traitOffer.catalogChecksum,
        offer_checksum: traitOffer.offerChecksum,
      });
      notify.success(t("AUTH.REGISTER_SUCCESS"));
    } catch (error) {
      if (
        error instanceof ApiError &&
        !error.retryable &&
        error.status < 500
      ) {
        retryBootstrap();
      }
      if (profileForm.handleSubmissionError(error)) {
        notify.warning(t("ONBOARDING_PROFILE.SUBMISSION_REVIEW_REQUIRED"));
        return;
      }
      notify.error(t("AUTH.REGISTER_ERROR"));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (activeStep !== 2) {
      traitStepRequested.current = false;
      return;
    }
    if (!traitOffer && !traitStepRequested.current) {
      traitStepRequested.current = true;
      void rollTraits();
    }
  }, [activeStep, rollTraits, traitOffer]);

  return (
    <form
      className="relative w-full"
      onSubmit={handleSubmit}
      aria-busy={isLoading}
    >
      {/* Header */}
      <div className="text-center space-y-1 mb-6">
        <h1 className="font-heading text-2xl font-black tracking-[0.12em] text-primary uppercase text-shadow-brand md:text-3xl">
          {t("AUTH.REGISTER_TITLE")}
        </h1>
        <p className="text-[10px] text-muted-foreground uppercase tracking-widest font-medium">
          {t("AUTH.REGISTER_SUBTITLE")}
        </p>
      </div>

      <OnboardingStepper
        steps={[
          { id: "account", label: t("AUTH.ONBOARDING_ACCOUNT") },
          { id: "profile", label: t("AUTH.ONBOARDING_PROFILE") },
          { id: "character", label: t("AUTH.ONBOARDING_CHARACTER") },
        ]}
        activeStep={activeStep}
        title={[
          t("AUTH.ONBOARDING_ACCOUNT_TITLE"),
          t("AUTH.ONBOARDING_PROFILE_TITLE"),
          t("AUTH.ONBOARDING_CHARACTER_TITLE"),
        ][activeStep]}
      >
        {activeStep === 0 ? (
          <PersonalInfoSection
            form={form}
            updateField={updateField}
            showPassword={showPassword}
            onTogglePassword={() => setShowPassword((p) => !p)}
            disabled={isLoading}
          />
        ) : null}
        {activeStep === 1 ? (
          <OnboardingProfileFields form={profileForm} disabled={isLoading} />
        ) : null}
        {activeStep === 2 ? (
          <TraitSelectionSection
            rolledRootBone={rolledRootBone}
            rolledTalents={rolledTalents}
            selectedTalents={selectedTalents}
            rollTicketExpiresAt={traitOffer?.expiresAt ?? ""}
            remainingOffers={traitOffer?.remainingOffers}
            rollExpired={rollExpired}
            isRolling={isRolling}
            onRoll={rollTraits}
            onToggleTalent={toggleTalent}
            onOpenCodex={() => setShowCodex(true)}
            disabled={isLoading}
          />
        ) : null}
      </OnboardingStepper>

      {/* Submit + Link to login */}
      <div className="mt-6 space-y-4">
        <Button
          type={activeStep === 2 ? "submit" : "button"}
          size="lg"
          className="button-brand-elevation w-full text-base font-bold uppercase tracking-[0.16em] motion-reduce:transition-none"
          disabled={
            isLoading ||
            isRolling ||
            (activeStep === 0 && (!isUsernameValid(form.username) || !isPasswordValid(form.password))) ||
            (activeStep === 1 && profileForm.status !== "ready") ||
            (activeStep === 2 && (!traitOffer || rollExpired || selectedTalents.length !== REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT))
          }
          onClick={activeStep < 2 ? () => {
            if (activeStep === 1 && !profileForm.buildEvidence()) {
              notify.warning(t("ONBOARDING_PROFILE.COMPLETE_REQUIRED"));
              return;
            }
            setActiveStep((step) => Math.min(2, step + 1));
          } : undefined}
        >
          {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
          {isLoading ? t("COMMON.LOADING") : activeStep === 2 ? t("AUTH.REGISTER_SUBMIT") : t("AUTH.ONBOARDING_CONTINUE")}
        </Button>

        {activeStep > 0 ? (
          <Button type="button" variant="outline" className="w-full" disabled={isLoading} onClick={() => setActiveStep((step) => Math.max(0, step - 1))}>
            {t("AUTH.ONBOARDING_BACK")}
          </Button>
        ) : null}

        <p className="text-center text-sm text-muted-foreground">
          {t("AUTH.HAS_ACCOUNT")}{" "}
          <Link href={APP_ROUTES.LOGIN} className="font-semibold text-primary hover:text-primary/80 transition-colors">
            {t("AUTH.TAB_LOGIN")}
          </Link>
        </p>
      </div>

      {/* Codex Modal */}
      <TraitCodexModal
        open={showCodex}
        onClose={() => setShowCodex(false)}
        traits={traitCatalog.data}
        status={traitCatalog.status}
        onRetry={traitCatalog.retry}
      />
    </form>
  );
}
