import { RANKING_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { commandPurpose } from "@/lib/api/command-purpose";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import {
  rankingLimitSchema,
  rankingPageSchema,
  rankingPrivacySchema,
  rankingVisibilitySchema,
} from "@/lib/cultivation/ranking-schema";
import type {
  Cultivator,
  RankingEntryPayload,
  RankingPage,
  RankingPagePayload,
  RankingPrivacy,
  RankingVisibility,
} from "@/types/ranking";

function mapEntry(entry: RankingEntryPayload, kind: "rating" | "experience"): Cultivator {
  return {
    id: entry.public_id,
    rank: entry.rank,
    name: entry.display_name ?? "",
    displayMode: entry.display_mode,
    realm: kind === "rating" ? entry.rank_title : entry.level_name,
    level: entry.level,
    exp: entry.total_exp,
    points: kind === "rating" ? entry.rating : entry.total_exp,
  };
}

function rankingQuery(limit: number, cursor?: string): string {
  const parameters = new URLSearchParams();
  parameters.set("limit", String(rankingLimitSchema.parse(limit)));
  if (cursor) {
    if (cursor.length > 1024) throw new TypeError("ranking cursor is invalid");
    parameters.set("cursor", cursor);
  }
  return parameters.toString();
}

async function page(
  kind: "rating" | "experience",
  limit: number,
  cursor?: string,
  signal?: AbortSignal,
): Promise<RankingPage> {
  const query = rankingQuery(limit, cursor);
  const payload = await api<RankingPagePayload, never>(
    kind === "rating" ? RANKING_API.RATING(query) : RANKING_API.EXP(query),
    { method: "GET", auth: "none", signal, schema: rankingPageSchema },
  );
  if (cursor && payload.next_cursor === cursor) {
    throw new TypeError("ranking cursor did not advance");
  }
  return {
    entries: payload.entries.map((entry) => mapEntry(entry, kind)),
    nextCursor: payload.next_cursor,
    projectionRevision: payload.projection_revision,
  };
}

export const rankingService = {
  pageByRating(limit = 20, cursor?: string, signal?: AbortSignal) {
    return page("rating", limit, cursor, signal);
  },

  pageByExperience(limit = 20, cursor?: string, signal?: AbortSignal) {
    return page("experience", limit, cursor, signal);
  },

  privacy(signal?: AbortSignal): Promise<RankingPrivacy> {
    return api<RankingPrivacy, never>(RANKING_API.PRIVACY, {
      method: "GET", signal, schema: rankingPrivacySchema,
    });
  },

  async updatePrivacy(input: {
    visibility: RankingVisibility;
    expected_version: number;
    reason: string;
  }): Promise<RankingPrivacy> {
    const body = {
      visibility: rankingVisibilitySchema.parse(input.visibility),
      expected_version: input.expected_version,
      reason: input.reason.trim(),
    };
    if (!Number.isSafeInteger(body.expected_version) || body.expected_version < 1 ||
      body.reason.length < 3 || body.reason.length > 512) {
      throw new TypeError("ranking privacy command is invalid");
    }
    const purpose = await commandPurpose("ranking-privacy", body);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api<RankingPrivacy, typeof body>(RANKING_API.PRIVACY, {
        method: "PUT", body, idempotencyKey, schema: rankingPrivacySchema,
      }),
    );
  },
};
