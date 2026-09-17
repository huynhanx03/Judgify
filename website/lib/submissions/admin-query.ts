import type {
  AdminSubmissionFindRequest,
  Language,
  SubmissionStatus,
} from "@/types/submission";
import { tryEntityID } from "@/lib/api/contracts";
import type { EntityID } from "@/types/api";

export const ADMIN_SUBMISSION_FILTER_ALL = "__all__" as const;

export interface AdminSubmissionFilterDraft {
  status: SubmissionStatus | typeof ADMIN_SUBMISSION_FILTER_ALL;
  language: Language | typeof ADMIN_SUBMISSION_FILTER_ALL;
  problemId: string;
  userId: string;
  contestId: string;
}

export interface AdminSubmissionFilters {
  status?: SubmissionStatus;
  language?: Language;
  problemId?: EntityID;
  userId?: EntityID;
  contestId?: EntityID;
}

export type AdminSubmissionIdentifierField =
  | "problemId"
  | "userId"
  | "contestId";

export type AdminSubmissionFilterResult =
  | { ok: true; filters: AdminSubmissionFilters }
  | { ok: false; invalidFields: AdminSubmissionIdentifierField[] };

export function createEmptyAdminSubmissionFilters(): AdminSubmissionFilterDraft {
  return {
    status: ADMIN_SUBMISSION_FILTER_ALL,
    language: ADMIN_SUBMISSION_FILTER_ALL,
    problemId: "",
    userId: "",
    contestId: "",
  };
}

export function normalizeAdminSubmissionFilters(
  draft: AdminSubmissionFilterDraft,
): AdminSubmissionFilterResult {
  const identifiers: Array<
    readonly [AdminSubmissionIdentifierField, string]
  > = [
    ["problemId", draft.problemId.trim()],
    ["userId", draft.userId.trim()],
    ["contestId", draft.contestId.trim()],
  ];
  const parsedIdentifiers = identifiers.map(
    ([field, value]) => [field, value, value ? tryEntityID(value) : null] as const,
  );
  const invalidFields = parsedIdentifiers
    .filter(([, value, parsed]) => value !== "" && parsed === null)
    .map(([field]) => field);
  if (invalidFields.length > 0) {
    return { ok: false, invalidFields };
  }

  const filters: AdminSubmissionFilters = {};
  if (draft.status !== ADMIN_SUBMISSION_FILTER_ALL) {
    filters.status = draft.status;
  }
  const language = draft.language.trim();
  if (language !== ADMIN_SUBMISSION_FILTER_ALL && language !== "") {
    filters.language = language;
  }
  for (const [field, , parsed] of parsedIdentifiers) {
    if (parsed !== null) filters[field] = parsed;
  }
  return { ok: true, filters };
}

export function toAdminSubmissionFindRequest(
  filters: AdminSubmissionFilters,
  page: number,
  pageSize: number,
): AdminSubmissionFindRequest {
  return {
    pagination: { page, page_size: pageSize },
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.language ? { language: filters.language } : {}),
    ...(filters.problemId ? { problem_id: filters.problemId } : {}),
    ...(filters.userId ? { user_id: filters.userId } : {}),
    ...(filters.contestId ? { contest_id: filters.contestId } : {}),
  };
}
