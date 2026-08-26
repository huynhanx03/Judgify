import type { OAuthPurpose } from "@/types/auth";

export type OAuthCallbackQuery = Record<
  string,
  string | string[] | undefined
>;

export type OAuthCallbackQueryResult =
  | { ok: true; purpose: OAuthPurpose }
  | {
      ok: false;
      reason: "provider_denied" | "invalid_callback" | "exchange_failed";
    };

function firstQueryValue(value: string | string[] | undefined): string {
  const candidate = Array.isArray(value) ? value[0] : value;
  return candidate?.trim() ?? "";
}

/** Parses only the non-secret status emitted by the backend callback. */
export function parseOAuthCallbackQuery(
  query: OAuthCallbackQuery,
): OAuthCallbackQueryResult {
  const status = firstQueryValue(query.status);
  if (status === "ready") {
    const purpose = firstQueryValue(query.purpose);
    if (purpose === "login" || purpose === "link") {
      return { ok: true, purpose };
    }
    return { ok: false, reason: "invalid_callback" };
  }
  if (status === "error") {
    const reason = firstQueryValue(query.reason);
    if (
      reason === "provider_denied" ||
      reason === "invalid_callback" ||
      reason === "exchange_failed"
    ) {
      return { ok: false, reason };
    }
  }
  return { ok: false, reason: "invalid_callback" };
}
