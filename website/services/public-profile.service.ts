import { PUBLIC_PROFILE_API } from "@/constants/api/public-profile"
import { api } from "@/lib/api/client"
import { commandPurpose } from "@/lib/api/command-purpose"
import { entityIDSchema } from "@/lib/api/contracts"
import { runIdempotentCommand } from "@/lib/api/idempotent-command"
import {
  profilePrivacyMutationReceiptSchema,
  profilePrivacySchema,
  profileVisibilitySchema,
  publicProfileSchema,
  publicUsernameSchema,
} from "@/lib/profile/public-profile-schema"
import type {
  ProfilePrivacy,
  ProfilePrivacyMutationReceipt,
  PublicProfile,
  UpdateProfilePrivacyInput,
} from "@/types/public-profile"
import type { EntityID } from "@/types/api"

export const publicProfileService = {
  get(username: string, signal?: AbortSignal): Promise<PublicProfile> {
    const canonicalUsername = publicUsernameSchema.parse(username)
    return api<PublicProfile, never>(PUBLIC_PROFILE_API.GET(canonicalUsername), {
      method: "GET",
      auth: "none",
      signal,
      schema: publicProfileSchema,
    })
  },

  privacy(signal?: AbortSignal): Promise<ProfilePrivacy> {
    return api<ProfilePrivacy, never>(PUBLIC_PROFILE_API.PRIVACY, {
      method: "GET",
      signal,
      schema: profilePrivacySchema,
    })
  },

  async updatePrivacy(
    userID: EntityID,
    input: UpdateProfilePrivacyInput,
  ): Promise<ProfilePrivacyMutationReceipt> {
    const canonicalUserID = entityIDSchema.parse(userID)
    const body: UpdateProfilePrivacyInput = {
      visibility: profileVisibilitySchema.parse(input.visibility),
      expected_version: input.expected_version,
    }
    if (!Number.isSafeInteger(body.expected_version) || body.expected_version < 1) {
      throw new TypeError("profile privacy command is invalid")
    }
    const purpose = await commandPurpose("profile-privacy", {
      user_id: canonicalUserID,
      ...body,
    })
    return runIdempotentCommand(purpose, async (idempotencyKey) => {
      const receipt = await api<
        ProfilePrivacyMutationReceipt,
        UpdateProfilePrivacyInput
      >(PUBLIC_PROFILE_API.PRIVACY, {
        method: "PUT",
        body,
        idempotencyKey,
        schema: profilePrivacyMutationReceiptSchema,
      })
      if (
        receipt.command_id !== idempotencyKey ||
        receipt.event_id !== idempotencyKey ||
        receipt.state.visibility !== body.visibility ||
        receipt.state.version !== body.expected_version + 1
      ) {
        throw new TypeError("profile privacy receipt does not match command")
      }
      return receipt
    })
  },
}
