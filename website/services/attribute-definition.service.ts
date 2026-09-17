import { ATTRIBUTE_DEFINITION_API } from "@/constants/api/identity";
import { api } from "@/lib/api/client";
import { entityIDSchema, type Paginated, type QueryOptions } from "@/lib/api/contracts";
import { ApiError } from "@/lib/api/error";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { OPERATION_KIND } from "@/constants/operation";
import {
  attributeLifecycleCommandInputSchema,
  attributeLifecyclePreviewResponseSchema,
  attributeLifecycleReceiptSchema,
  attributeDefinitionPageSchema,
  attributeDefinitionRevisionPageSchema,
  attributeDefinitionSchema,
} from "@/lib/admin/auxiliary-schema";
import type {
  AttributeLifecycleAction,
  AttributeLifecycleCommandInput,
  AttributeLifecycleReceipt,
  AttributeDefinition,
  AttributeDefinitionRevisionPage,
  CreateAttributeDefinitionInput,
  ReviewedAttributeLifecycleCommand,
  UpdateAttributeDefinitionInput,
} from "@/types/admin-auxiliary";

export const attributeDefinitionService = {
  find(
    query: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<AttributeDefinition>> {
    return api<Paginated<AttributeDefinition>, QueryOptions>(
      ATTRIBUTE_DEFINITION_API.FIND,
      {
        method: "POST",
        body: query,
        signal,
        schema: attributeDefinitionPageSchema,
      },
    );
  },

  async listRevisions(
    id: string,
    cursor?: string,
    signal?: AbortSignal,
  ): Promise<AttributeDefinitionRevisionPage> {
    const definitionID = entityIDSchema.parse(id);
    const params = new URLSearchParams({ limit: "25" });
    if (cursor) params.set("cursor", cursor);
    const page = await api<AttributeDefinitionRevisionPage, never>(
      `${ATTRIBUTE_DEFINITION_API.REVISIONS(definitionID)}?${params}`,
      {
        method: "GET",
        signal,
        schema: attributeDefinitionRevisionPageSchema,
      },
    );
    if (page.records.some((revision) => revision.definition_id !== definitionID)) {
      throw new TypeError("invalid attribute revision ownership");
    }
    return page;
  },

  create(input: CreateAttributeDefinitionInput): Promise<AttributeDefinition> {
    const purpose = "attribute-definition-create";
    const attempt = commandAttemptStore.getOrCreate(purpose);
    return api<
      AttributeDefinition,
      CreateAttributeDefinitionInput & { command_id: string }
    >(
      ATTRIBUTE_DEFINITION_API.CREATE,
      {
        method: "POST",
        body: { ...input, command_id: attempt.attempt_id },
        idempotencyKey: attempt.attempt_id,
        schema: attributeDefinitionSchema,
      },
    ).then((definition) => {
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return definition;
    }).catch((error: unknown) => {
      resolveDefinitiveAttributeFailure(purpose, attempt.attempt_id, error);
      throw error;
    });
  },

  update(id: string, input: UpdateAttributeDefinitionInput): Promise<AttributeDefinition> {
    const definitionID = entityIDSchema.parse(id);
    const purpose = `attribute-definition-update-${definitionID}`;
    const attempt = commandAttemptStore.getOrCreate(purpose);
    return api<
      AttributeDefinition,
      UpdateAttributeDefinitionInput & { command_id: string }
    >(ATTRIBUTE_DEFINITION_API.UPDATE(definitionID), {
      method: "PUT",
      body: { ...input, command_id: attempt.attempt_id },
      idempotencyKey: attempt.attempt_id,
      schema: attributeDefinitionSchema,
    }).then((definition) => {
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return definition;
    }).catch((error: unknown) => {
      resolveDefinitiveAttributeFailure(purpose, attempt.attempt_id, error);
      throw error;
    });
  },

  async previewLifecycle(
    id: string,
    action: AttributeLifecycleAction,
    input: AttributeLifecycleCommandInput,
  ): Promise<ReviewedAttributeLifecycleCommand> {
    const definitionID = entityIDSchema.parse(id);
    const normalized = attributeLifecycleCommandInputSchema.parse({
      ...input,
      reason: input.reason.trim(),
    });
    if (
      (action === "activate") !==
      (normalized.candidate_revision_id !== undefined)
    ) {
      throw new TypeError("invalid attribute lifecycle candidate");
    }
    const purpose = attributeLifecyclePurpose(definitionID, action);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api(
        action === "activate"
          ? ATTRIBUTE_DEFINITION_API.ACTIVATION_PREVIEW(definitionID)
          : ATTRIBUTE_DEFINITION_API.ARCHIVE_PREVIEW(definitionID),
        {
          method: "POST",
          body: { ...normalized, command_id: attempt.attempt_id },
          idempotencyKey: attempt.attempt_id,
          schema: attributeLifecyclePreviewResponseSchema,
        },
      );
      if (!validAttributeLifecycleReview(review, definitionID, action, normalized)) {
        throw new TypeError("invalid attribute lifecycle review");
      }
      return {
        definition_id: definitionID,
        command_id: attempt.attempt_id,
        input: normalized,
        review,
      };
    } catch (error) {
      resolveDefinitiveAttributeFailure(purpose, attempt.attempt_id, error);
      throw error;
    }
  },

  async applyLifecycle(
    reviewed: ReviewedAttributeLifecycleCommand,
  ): Promise<AttributeLifecycleReceipt> {
    const definitionID = entityIDSchema.parse(reviewed.definition_id);
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const input = attributeLifecycleCommandInputSchema.parse(reviewed.input);
    const review = attributeLifecyclePreviewResponseSchema.parse(reviewed.review);
    if (
      input.reason !== input.reason.trim() ||
      !validAttributeLifecycleReview(review, definitionID, review.preview.action, input)
    ) {
      throw new TypeError("invalid reviewed attribute lifecycle command");
    }
    const normalizedReviewed: ReviewedAttributeLifecycleCommand = {
      definition_id: definitionID,
      command_id: commandID,
      input,
      review,
    };
    const action = review.preview.action;
    const purpose = attributeLifecyclePurpose(definitionID, action);
    const activeAttempt = commandAttemptStore.getOrCreate(purpose);
    if (activeAttempt.attempt_id !== commandID) {
      throw new TypeError("attribute command identity expired");
    }
    try {
      const receipt = await api(
        action === "activate"
          ? ATTRIBUTE_DEFINITION_API.ACTIVATION_APPLY(definitionID)
          : ATTRIBUTE_DEFINITION_API.ARCHIVE_APPLY(definitionID),
        {
          method: "POST",
          body: {
            ...input,
            command_id: commandID,
            confirmation_token: review.confirmation_token,
            review_hash: review.review_hash,
          },
          idempotencyKey: commandID,
          schema: attributeLifecycleReceiptSchema,
        },
      );
      if (!validAttributeLifecycleReceipt(receipt, normalizedReviewed)) {
        throw new TypeError("invalid attribute lifecycle receipt");
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveAttributeFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonLifecycle(reviewed: ReviewedAttributeLifecycleCommand): void {
    commandAttemptStore.resolve(
      attributeLifecyclePurpose(
        entityIDSchema.parse(reviewed.definition_id),
        reviewed.review.preview.action,
      ),
      entityIDSchema.parse(reviewed.command_id),
    );
  },
};

function attributeLifecyclePurpose(
  definitionID: string,
  action: AttributeLifecycleAction,
): string {
  return `attribute-definition-${action}-${definitionID}`;
}

function validAttributeLifecycleReview(
  review: ReviewedAttributeLifecycleCommand["review"],
  definitionID: string,
  action: AttributeLifecycleAction,
  input: AttributeLifecycleCommandInput,
): boolean {
  const preview = review.preview;
  const candidateRevision = preview.candidate_revision;
  return (
    preview.action === action &&
    preview.definition_id === definitionID &&
    preview.expected_version === input.expected_version &&
    preview.resulting_version === input.expected_version + 1 &&
    Date.parse(review.expires_at) > Date.now() &&
    (preview.current_revision === undefined ||
      preview.current_revision.definition_id === definitionID) &&
    (action === "activate"
      ? candidateRevision !== undefined &&
        candidateRevision.id === input.candidate_revision_id &&
        candidateRevision.definition_id === definitionID &&
        preview.requires_migration ===
          (preview.source_value_count > 0 || preview.target_value_count > 0)
      : candidateRevision === undefined &&
        input.candidate_revision_id === undefined &&
        preview.target_value_count === 0 &&
        !preview.requires_migration)
  );
}

function validAttributeLifecycleReceipt(
  receipt: AttributeLifecycleReceipt,
  reviewed: ReviewedAttributeLifecycleCommand,
): boolean {
  const action = reviewed.review.preview.action;
  const preview = reviewed.review.preview;
  const baseValid =
    receipt.action === action &&
    receipt.command_id === reviewed.command_id &&
    receipt.definition_id === reviewed.definition_id &&
    receipt.definition.id === reviewed.definition_id &&
    receipt.definition.version === receipt.version;
  if (!baseValid) return false;
  if (receipt.status === "migration_scheduled") {
    return (
      action === "activate" &&
      receipt.event_id === undefined &&
      receipt.version === reviewed.input.expected_version &&
      preview.requires_migration &&
      preview.source_value_count + preview.target_value_count > 0 &&
      receipt.operation.id === reviewed.command_id &&
      receipt.operation.kind === OPERATION_KIND.IDENTITY_ATTRIBUTE_VALUE_MIGRATION &&
      receipt.operation.total ===
        preview.source_value_count + preview.target_value_count &&
      preview.current_revision !== undefined &&
      receipt.active_revision !== undefined &&
      receipt.active_revision.id === preview.current_revision.id &&
      receipt.active_revision.definition_id === reviewed.definition_id &&
      receipt.definition.status === "active" &&
      receipt.definition.active_revision_id === receipt.active_revision.id &&
      receipt.active_revision.id !== reviewed.input.candidate_revision_id
    );
  }
  if (
    receipt.status !== "committed" ||
    receipt.event_id !== reviewed.command_id ||
    receipt.operation !== undefined ||
    receipt.version !== reviewed.input.expected_version + 1
  ) {
    return false;
  }
  if (receipt.action === "activate") {
    return (
      !preview.requires_migration &&
      receipt.active_revision.id === reviewed.input.candidate_revision_id &&
      receipt.active_revision.definition_id === reviewed.definition_id &&
      receipt.definition.status === "active" &&
      receipt.definition.active_revision_id === reviewed.input.candidate_revision_id
    );
  }
  return (
    receipt.active_revision === undefined &&
    receipt.definition.status === "archived"
  );
}

function resolveDefinitiveAttributeFailure(
  purpose: string,
  commandID: string,
  error: unknown,
): void {
  if (
    error instanceof ApiError &&
    !error.retryable &&
    error.status < 500 &&
    error.status !== 403 &&
    error.status !== 409
  ) {
    commandAttemptStore.resolve(purpose, commandID);
  }
}
