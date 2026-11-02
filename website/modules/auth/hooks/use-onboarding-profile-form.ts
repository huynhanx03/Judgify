"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  invalidProfileValueDefinitionID,
  isProfileSchemaConflict,
} from "@/lib/auth/onboarding-profile-schema";
import {
  onboardingProfileAttributes,
  planOnboardingProfileEvidence,
  reconcileOnboardingProfileDrafts,
  validateOnboardingProfileDraft,
} from "@/modules/auth/onboarding-profile-model";
import type {
  ProfileAttributeDraftValue,
  ProfileAttributeDrafts,
  ProfileAttributeFieldErrors,
} from "@/modules/profile/profile-attribute-model";
import { authService } from "@/services/auth.service";
import type {
  OnboardingProfileSchemaResponse,
  ProfileSchemaEvidence,
  ProfileValueSuggestion,
} from "@/types/auth";

const EMPTY_SUGGESTIONS: readonly ProfileValueSuggestion[] = [];

export type OnboardingProfileNotice =
  | "schema_reloaded"
  | "invalid_value"
  | null;

interface UseOnboardingProfileFormOptions {
  enabled?: boolean;
  suggestions?: readonly ProfileValueSuggestion[];
}

export function useOnboardingProfileForm({
  enabled = true,
  suggestions = EMPTY_SUGGESTIONS,
}: UseOnboardingProfileFormOptions = {}) {
  const previousSchemaRef = useRef<OnboardingProfileSchemaResponse | null>(null);
  const [hydratedState, setHydratedState] = useState<{
    identity: string;
    requestToken: object;
  } | null>(null);
  const [drafts, setDrafts] = useState<ProfileAttributeDrafts>({});
  const [errors, setErrors] = useState<ProfileAttributeFieldErrors>({});
  const [notice, setNotice] = useState<OnboardingProfileNotice>(null);
  const [validationAttempt, setValidationAttempt] = useState(0);
  const schemaResource = useRetryableResource<OnboardingProfileSchemaResponse | null>({
    resetKey: enabled ? "onboarding-profile-schema" : "onboarding-profile-schema-disabled",
    enabled,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => authService.getOnboardingProfileSchema(signal),
  });
  const schema = schemaResource.data;
  const attributes = useMemo(
    () => (schema ? onboardingProfileAttributes(schema) : []),
    [schema],
  );
  const suggestionFingerprint = useMemo(
    () =>
      JSON.stringify(
        [...suggestions]
          .sort((left, right) =>
            left.key < right.key ? -1 : left.key > right.key ? 1 : 0,
          )
          .map((suggestion) => [suggestion.key, suggestion.value]),
      ),
    [suggestions],
  );
  const hydrationIdentity = schema
    ? `${schema.revision}:${schema.checksum}:${suggestionFingerprint}`
    : "";

  useEffect(() => {
    if (!enabled) {
      previousSchemaRef.current = null;
      return;
    }
    if (!schema || schemaResource.status !== "ready") return;
    if (
      hydratedState?.identity === hydrationIdentity &&
      hydratedState.requestToken === schemaResource.requestToken
    ) {
      return;
    }
    setDrafts((current) =>
      reconcileOnboardingProfileDrafts(
        schema,
        previousSchemaRef.current,
        current,
        suggestions,
      ),
    );
    setErrors({});
    setNotice((current) => (current === null ? current : null));
    setValidationAttempt((current) => (current === 0 ? current : 0));
    previousSchemaRef.current = schema;
    setHydratedState({
      identity: hydrationIdentity,
      requestToken: schemaResource.requestToken,
    });
  }, [
    enabled,
    hydratedState,
    hydrationIdentity,
    schema,
    schemaResource.requestToken,
    schemaResource.status,
    suggestions,
  ]);

  const hydrationReady =
    enabled &&
    schemaResource.status === "ready" &&
    hydrationIdentity !== "" &&
    hydratedState?.identity === hydrationIdentity &&
    hydratedState.requestToken === schemaResource.requestToken;

  const plan = useMemo(
    () => (schema ? planOnboardingProfileEvidence(schema, drafts) : null),
    [drafts, schema],
  );

  const updateDraft = useCallback(
    (definitionID: string, value: ProfileAttributeDraftValue) => {
      setDrafts((current) => ({ ...current, [definitionID]: value }));
      setErrors((current) => {
        if (!(definitionID in current)) return current;
        const next = { ...current };
        delete next[definitionID];
        return next;
      });
      setNotice((current) => (current === "invalid_value" ? null : current));
    },
    [],
  );

  const validateField = useCallback(
    (definitionID: string) => {
      const attribute = attributes.find(
        (candidate) => candidate.definition_id === definitionID,
      );
      if (!attribute) return;
      const error = validateOnboardingProfileDraft(
        attribute,
        drafts[definitionID],
      );
      setErrors((current) => {
        if (error) return { ...current, [definitionID]: error };
        if (!(definitionID in current)) return current;
        const next = { ...current };
        delete next[definitionID];
        return next;
      });
    },
    [attributes, drafts],
  );

  const buildEvidence = useCallback((): ProfileSchemaEvidence | null => {
    if (
      !schema ||
      schemaResource.status !== "ready" ||
      hydratedState?.identity !== hydrationIdentity ||
      hydratedState.requestToken !== schemaResource.requestToken
    ) {
      return null;
    }
    const nextPlan = planOnboardingProfileEvidence(schema, drafts);
    if (!nextPlan.ok) {
      setErrors(nextPlan.errors);
      setValidationAttempt((current) => current + 1);
      return null;
    }
    setErrors({});
    return nextPlan.evidence;
  }, [
    drafts,
    hydratedState,
    hydrationIdentity,
    schema,
    schemaResource.requestToken,
    schemaResource.status,
  ]);

  const handleSubmissionError = useCallback(
    (error: unknown): boolean => {
      if (isProfileSchemaConflict(error)) {
        setNotice("schema_reloaded");
        setErrors({});
        schemaResource.retry();
        return true;
      }
      const definitionID = invalidProfileValueDefinitionID(error);
      if (definitionID) {
        setErrors((current) => ({ ...current, [definitionID]: "invalid" }));
        setNotice("invalid_value");
        setValidationAttempt((current) => current + 1);
        return true;
      }
      return false;
    },
    [schemaResource],
  );

  const retry = useCallback(() => {
    setNotice(null);
    schemaResource.retry();
  }, [schemaResource]);

  return {
    attributes,
    buildEvidence,
    canSubmit: hydrationReady && Boolean(plan?.ok),
    drafts: hydrationReady ? drafts : {},
    errors: hydrationReady ? errors : {},
    handleSubmissionError,
    notice: hydrationReady ? notice : null,
    retry,
    schema,
    status:
      schemaResource.status === "ready" && !hydrationReady
        ? "loading"
        : schemaResource.status,
    updateDraft,
    validateField,
    validationAttempt: hydrationReady ? validationAttempt : 0,
  };
}

export type OnboardingProfileFormController = ReturnType<
  typeof useOnboardingProfileForm
>;
