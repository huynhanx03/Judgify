import { tryEntityID } from "@/lib/api/contracts";
import type { EntityID } from "@/types/api";

export type SearchParamValue = string | string[] | undefined;

/** Builds the canonical problem route while retaining contest admission context. */
export function contestProblemHref(
  contestId: EntityID,
  problemId: EntityID,
  contestProblemId?: EntityID,
): string {
  const params = new URLSearchParams({ contest: contestId });
  if (contestProblemId) params.set("contest_problem", contestProblemId);
  return `/arena/${encodeURIComponent(problemId)}?${params.toString()}`;
}

/** Accepts one valid UUID from Next.js search params and rejects forged context. */
export function readContestContext(value: SearchParamValue): EntityID | null {
  const candidate = Array.isArray(value) ? value[0] : value;
  return tryEntityID(candidate);
}
