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
import { TEXT } from "@/constants/text";
import { REGISTRATION_TRAIT_RULES } from "@/constants/registration";
import {
  parseRegistrationTraitOffer,
  type RegistrationTraitOffer,
} from "@/lib/auth/registration-trait-offer";
import {
  isRecoveryEmailValid,
  normalizeRecoveryEmail,
} from "@/lib/auth/recovery-email";
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
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  // Basic Info
  const [form, setForm] = useState({
    username: "",
    password: "",
    recovery_email: "",
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
        notify.error(TEXT.AUTH.TRAIT_FETCH_ERROR);
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
        notify.warning(TEXT.AUTH.TRAIT_MAX_TALENTS);
        return current;
      }
      return [...current, id];
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (
      !isUsernameValid(form.username) ||
      !isPasswordValid(form.password) ||
      !isRecoveryEmailValid(form.recovery_email)
    ) {
      notify.warning(TEXT.AUTH.REGISTER_MISSING_INFO);
      return;
    }
    const profileEvidence = profileForm.buildEvidence();
    if (!profileEvidence) {
      notify.warning(TEXT.ONBOARDING_PROFILE.COMPLETE_REQUIRED);
      return;
    }
    if (!rolledRootBone) {
      notify.warning(TEXT.AUTH.TRAIT_ROOT_BONE_REQUIRED);
      return;
    }
    if (selectedTalents.length !== REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT) {
      notify.warning(TEXT.AUTH.TRAIT_TALENTS_REQUIRED);
      return;
    }
    if (!traitOffer || rollExpired) {
      notify.warning(TEXT.AUTH.TRAIT_ROLL_EXPIRED);
      return;
    }
    setIsLoading(true);
    try {
      await register({
        username: normalizeUsername(form.username),
        password: form.password,
        recovery_email: normalizeRecoveryEmail(form.recovery_email),
        ...profileEvidence,
        root_bone_id: rolledRootBone.id,
        talent_ids: selectedTalents,
        offer_id: traitOffer.id,
        offer_sequence: traitOffer.sequence,
        catalog_revision_id: traitOffer.catalogRevisionId,
        catalog_checksum: traitOffer.catalogChecksum,
        offer_checksum: traitOffer.offerChecksum,
      });
      notify.success(TEXT.AUTH.REGISTER_SUCCESS);
    } catch (error) {
      if (
        error instanceof ApiError &&
        !error.retryable &&
        error.status < 500
      ) {
        retryBootstrap();
      }
      if (profileForm.handleSubmissionError(error)) {
        notify.warning(TEXT.ONBOARDING_PROFILE.SUBMISSION_REVIEW_REQUIRED);
        return;
      }
      notify.error(TEXT.AUTH.REGISTER_ERROR);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <form
      className="relative w-full"
      onSubmit={handleSubmit}
      aria-busy={isLoading}
    >
      {/* Header */}
      <div className="text-center space-y-1 mb-6">
        <h1 className="text-2xl md:text-3xl font-black tracking-[0.15em] text-primary uppercase font-serif text-shadow-brand">
          {TEXT.AUTH.REGISTER_TITLE}
        </h1>
        <p className="text-[10px] text-muted-foreground uppercase tracking-widest font-medium">
          {TEXT.AUTH.REGISTER_SUBTITLE}
        </p>
      </div>

      <PersonalInfoSection
        form={form}
        updateField={updateField}
        showPassword={showPassword}
        onTogglePassword={() => setShowPassword((p) => !p)}
        disabled={isLoading}
      />

      <OnboardingProfileFields
        form={profileForm}
        disabled={isLoading}
        className="mt-4"
      />

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

      {/* Submit + Link to login */}
      <div className="mt-6 space-y-4">
        <Button
          type="submit"
          className="button-brand-elevation h-12 w-full bg-primary text-md font-bold uppercase tracking-[0.2em] text-primary-foreground transition-colors hover:bg-primary/90 motion-reduce:transition-none"
          disabled={
            isLoading ||
            profileForm.status !== "ready" ||
            isRolling
          }
        >
          {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
          {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.REGISTER_SUBMIT}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          {TEXT.AUTH.HAS_ACCOUNT}{" "}
          <Link href={APP_ROUTES.LOGIN} className="font-semibold text-primary hover:text-primary/80 transition-colors">
            {TEXT.AUTH.TAB_LOGIN}
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
