import { JUDGE_API } from "@/constants/api/judge";
import { api } from "@/lib/api/client";
import { judgeOperationsOverviewSchema } from "@/lib/judge/operations-schema";
import type { JudgeOperationsOverview } from "@/types/judge-operations";

export const judgeOperationsService = {
  getOverview(signal?: AbortSignal): Promise<JudgeOperationsOverview> {
    return api<JudgeOperationsOverview, never>(JUDGE_API.ADMIN_OVERVIEW, {
      method: "GET",
      signal,
      schema: judgeOperationsOverviewSchema,
    });
  },
};
