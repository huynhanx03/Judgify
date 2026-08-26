import { AUDIT_API } from "@/constants/api/audit";
import { api } from "@/lib/api/client";
import { toAuditSearchParams } from "@/lib/audit/query";
import type { AuditListQuery, AuditLogPage } from "@/types/audit";
import { auditLogPageSchema } from "@/lib/audit/audit-schema";

export const auditService = {
  async list(query: AuditListQuery, signal?: AbortSignal): Promise<AuditLogPage> {
    const params = toAuditSearchParams(query);
    return api<AuditLogPage, never>(AUDIT_API.LIST(params.toString()), {
      method: "GET",
      signal,
      schema: auditLogPageSchema,
    });
  },
};
