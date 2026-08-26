import {
  createProfileAttributeDrafts,
  planProfileAttributeMutations,
  validateProfileAttributeDraft,
  type ProfileAttributeDraftValue,
  type ProfileAttributeDrafts,
  type ProfileAttributeFieldError,
  type ProfileAttributeFieldErrors,
} from "@/modules/profile/profile-attribute-model";
import type {
  OnboardingProfileSchemaResponse,
  ProfileSchemaEvidence,
  ProfileValueSuggestion,
} from "@/types/auth";
import type { ProfileAttribute, ProfileAttributeValue } from "@/types/user";

export type OnboardingProfilePlan =
  | { ok: true; evidence: ProfileSchemaEvidence }
  | { ok: false; errors: ProfileAttributeFieldErrors };

const REQUIRED_ONBOARDING_VALUES = {
  requireOnboardingValues: true,
} as const;

/**
 * The shared field renderer consumes the richer self-profile projection. This
 * adapter supplies only render/planning state; none of these compatibility
 * fields are sent back to the account-creation API.
 */
export function onboardingProfileAttributes(
  schema: OnboardingProfileSchemaResponse,
): ProfileAttribute[] {
  return schema.fields.map((field) => ({
    ...field,
    kind: "custom",
    required_on_onboarding: true,
    user_editable: true,
    has_value: false,
    value_version: 0,
  }));
}

export function emptyOnboardingProfileDrafts(
  schema: OnboardingProfileSchemaResponse,
): ProfileAttributeDrafts {
  return createProfileAttributeDrafts(onboardingProfileAttributes(schema));
}

export function reconcileOnboardingProfileDrafts(
  schema: OnboardingProfileSchemaResponse,
  previousSchema: OnboardingProfileSchemaResponse | null,
  currentDrafts: ProfileAttributeDrafts,
  suggestions: readonly ProfileValueSuggestion[],
): ProfileAttributeDrafts {
  const attributes = onboardingProfileAttributes(schema);
  const previousByKey = new Map(
    previousSchema?.fields.map((field) => [field.key, field]) ?? [],
  );
  const suggestionsByKey = new Map(
    suggestions.map((suggestion) => [suggestion.key, suggestion.value]),
  );
  const next = emptyOnboardingProfileDrafts(schema);

  for (const attribute of attributes) {
    const previousField = previousByKey.get(attribute.key);
    const previousDraft = previousField
      ? currentDrafts[previousField.definition_id]
      : undefined;
    if (
      previousDraft !== undefined &&
      draftIsCompatible(attribute, previousDraft)
    ) {
      next[attribute.definition_id] = previousDraft;
      continue;
    }

    const suggestion = suggestionsByKey.get(attribute.key);
    if (suggestion === undefined) continue;
    const suggestionDraft = draftValue(suggestion);
    if (draftIsCompatible(attribute, suggestionDraft)) {
      next[attribute.definition_id] = suggestionDraft;
    }
  }
  return next;
}

export function planOnboardingProfileEvidence(
  schema: OnboardingProfileSchemaResponse,
  drafts: ProfileAttributeDrafts,
): OnboardingProfilePlan {
  const attributes = onboardingProfileAttributes(schema);
  const plan = planProfileAttributeMutations(
    attributes,
    drafts,
    REQUIRED_ONBOARDING_VALUES,
  );
  if (!plan.ok) return plan;

  const errors: ProfileAttributeFieldErrors = {};
  const valuesByDefinition = new Map(
    plan.mutations.flatMap((mutation) =>
      mutation.value === undefined
        ? []
        : [
            [
              mutation.definition_id,
              {
                definition_id: mutation.definition_id,
                definition_revision_id:
                  mutation.expected_definition_revision_id,
                value: mutation.value,
              },
            ] as const,
          ],
    ),
  );

  for (const attribute of attributes) {
    if (!valuesByDefinition.has(attribute.definition_id)) {
      errors[attribute.definition_id] = "required";
    }
  }
  if (Object.keys(errors).length > 0) return { ok: false, errors };

  return {
    ok: true,
    evidence: {
      profile_schema_revision: schema.revision,
      profile_schema_checksum: schema.checksum,
      profile_values: [...valuesByDefinition.values()].sort((left, right) =>
        left.definition_id.localeCompare(right.definition_id),
      ),
    },
  };
}

export function validateOnboardingProfileDraft(
  attribute: ProfileAttribute,
  draft: ProfileAttributeDraftValue,
): ProfileAttributeFieldError | null {
  return validateProfileAttributeDraft(
    attribute,
    draft,
    REQUIRED_ONBOARDING_VALUES,
  );
}

function draftIsCompatible(
  attribute: ProfileAttribute,
  draft: ProfileAttributeDraftValue,
): boolean {
  const plan = planProfileAttributeMutations(
    [attribute],
    { [attribute.definition_id]: draft },
    REQUIRED_ONBOARDING_VALUES,
  );
  return plan.ok && plan.mutations.length === 1;
}

function draftValue(value: ProfileAttributeValue): ProfileAttributeDraftValue {
  return typeof value === "boolean" ? value : String(value);
}
