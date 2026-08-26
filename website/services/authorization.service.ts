import { AUTHORIZATION_API } from "@/constants/api/identity";
import { api } from "@/lib/api/client";
import { effectiveCapabilitiesSchema } from "@/lib/admin/identity-schema";
import type { EffectiveCapabilitiesResponse } from "@/types/auth";

export const authorizationService = {
  getMyCapabilities(signal?: AbortSignal): Promise<EffectiveCapabilitiesResponse> {
    return api<EffectiveCapabilitiesResponse, never>(AUTHORIZATION_API.ME, {
      method: "GET",
      signal,
      schema: effectiveCapabilitiesSchema,
    });
  },
};
