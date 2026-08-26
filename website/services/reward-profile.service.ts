import {
  ADMIN_CULTIVATION_API,
  REWARD_PROFILE_API,
} from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { commandPurpose } from "@/lib/api/command-purpose";
import { entityIDSchema } from "@/lib/api/contracts";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import {
  rewardProfilePreviewSchema,
  rewardProfileSchema,
} from "@/lib/cultivation/reward-profile-schema";
import { sha256ChecksumSchema } from "@/lib/cultivation/trait-schema";
import type {
  ApplyRewardProfileInput,
  RewardProfileAssignmentPreview,
  RewardProfileResponse,
  RewardProfileSelection,
} from "@/types/cultivation";

function normalizeSelection(
  selection: RewardProfileSelection,
): RewardProfileSelection {
  const rootTraitID = entityIDSchema.parse(selection.root_trait_id);
  const talentTraitIDs = selection.talent_trait_ids.map((value) =>
    entityIDSchema.parse(value),
  );
  const identities = new Set([rootTraitID, ...talentTraitIDs]);
  if (talentTraitIDs.length !== 3 || identities.size !== 4) {
    throw new TypeError("reward profile selection is invalid");
  }
  return {
    root_trait_id: rootTraitID,
    talent_trait_ids: talentTraitIDs,
  };
}

export const rewardProfileService = {
  current(signal?: AbortSignal): Promise<RewardProfileResponse> {
    return api<RewardProfileResponse, never>(REWARD_PROFILE_API.CURRENT, {
      method: "GET",
      signal,
      schema: rewardProfileSchema,
    });
  },

  adminCurrent(
    userID: string,
    signal?: AbortSignal,
  ): Promise<RewardProfileResponse> {
    return api<RewardProfileResponse, never>(
      ADMIN_CULTIVATION_API.REWARD_PROFILE(entityIDSchema.parse(userID)),
      { method: "GET", signal, schema: rewardProfileSchema },
    );
  },

  preview(
    userID: string,
    selection: RewardProfileSelection,
  ): Promise<RewardProfileAssignmentPreview> {
    const parsedUserID = entityIDSchema.parse(userID);
    const body = normalizeSelection(selection);
    return api<RewardProfileAssignmentPreview, RewardProfileSelection>(
      ADMIN_CULTIVATION_API.REWARD_PROFILE_PREVIEW(parsedUserID),
      {
        method: "POST",
        body,
        schema: rewardProfilePreviewSchema,
      },
    );
  },

  async apply(input: ApplyRewardProfileInput): Promise<RewardProfileResponse> {
    const userID = entityIDSchema.parse(input.user_id);
    const selection = normalizeSelection(input);
    const reason = input.reason.trim();
    if (
      !Number.isSafeInteger(input.expected_revision) ||
      input.expected_revision < 1 ||
      reason.length < 3 ||
      reason.length > 1024
    ) {
      throw new TypeError("reward profile command is invalid");
    }
    const body = {
      expected_revision: input.expected_revision,
      ...selection,
      preview_checksum: sha256ChecksumSchema.parse(input.preview_checksum),
      reason,
    };
    const purpose = await commandPurpose("reward-profile-assignment", [
      userID,
      body,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api<RewardProfileResponse, typeof body>(
        ADMIN_CULTIVATION_API.REWARD_PROFILE(userID),
        {
          method: "POST",
          body,
          idempotencyKey,
          schema: rewardProfileSchema,
        },
      ),
    );
  },
};
