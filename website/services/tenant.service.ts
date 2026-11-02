import { api } from "@/lib/api/client";
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import { recordAt, type Schema } from "@/lib/api/schema";
import type { Tenant, TenantInvitation, TenantInvitationListResponse, TenantListResponse, TenantMember, TenantMemberListResponse } from "@/types/tenant";

const tenantSchema: Schema<Tenant> = {
  parse(value, path = "$") {
    const record = recordAt(value, path);
    if (
      typeof record.name !== "string" ||
      typeof record.slug !== "string" ||
      !["owner", "member"].includes(String(record.role)) ||
      !["active", "suspended", "archived"].includes(String(record.status)) ||
      typeof record.can_change_name !== "boolean"
    ) {
      throw new TypeError(`${path}: invalid tenant response`);
    }
    return {
      id: entityIDSchema.parse(record.id, `${path}.id`),
      name: record.name,
      slug: record.slug,
      role: record.role as Tenant["role"],
      status: record.status as Tenant["status"],
      can_change_name: record.can_change_name,
      next_name_change_at:
        record.next_name_change_at == null
          ? null
          : isoDateTimeSchema.parse(
              record.next_name_change_at,
              `${path}.next_name_change_at`,
            ),
    };
  },
};

const tenantListSchema: Schema<TenantListResponse> = {
  parse(value, path = "$") {
    const record = recordAt(value, path);
    if (!Array.isArray(record.tenants)) {
      throw new TypeError(`${path}.tenants: expected array`);
    }
    return {
      tenants: record.tenants.map((tenant, index) =>
        tenantSchema.parse(tenant, `${path}.tenants[${index}]`),
      ),
    };
  },
};

const invitationSchema: Schema<TenantInvitation> = {
  parse(value, path = "$ ") {
    const record = recordAt(value, path);
    if (typeof record.tenant_name !== "string" || typeof record.username !== "string") {
      throw new TypeError(`${path}: invalid tenant invitation response`);
    }
    return {
      id: entityIDSchema.parse(record.id, `${path}.id`),
      tenant_id: entityIDSchema.parse(record.tenant_id, `${path}.tenant_id`),
      tenant_name: record.tenant_name,
      username: record.username,
      expires_at: isoDateTimeSchema.parse(record.expires_at, `${path}.expires_at`),
    };
  },
};

const invitationListSchema: Schema<TenantInvitationListResponse> = {
  parse(value, path = "$") {
    const record = recordAt(value, path);
    if (!Array.isArray(record.invitations)) throw new TypeError(`${path}.invitations: expected array`);
    return { invitations: record.invitations.map((item, index) => invitationSchema.parse(item, `${path}.invitations[${index}]`)) };
  },
};

const memberListSchema: Schema<TenantMemberListResponse> = {
  parse(value, path = "$") {
    const record = recordAt(value, path);
    if (!Array.isArray(record.members)) throw new TypeError(`${path}.members: expected array`);
    return { members: record.members.map((value, index) => {
      const memberPath = `${path}.members[${index}]`;
      const member = recordAt(value, memberPath);
      if (typeof member.username !== "string" || !["owner", "member"].includes(String(member.role))) {
        throw new TypeError(`${memberPath}: invalid tenant member`);
      }
      return {
        user_id: entityIDSchema.parse(member.user_id, `${memberPath}.user_id`),
        username: member.username,
        role: member.role as TenantMember["role"],
      };
    }) };
  },
};

export const tenantService = {
  findForUser(signal?: AbortSignal): Promise<TenantListResponse> {
    return api("/tenants", { method: "GET", signal, schema: tenantListSchema });
  },
  create(name: string): Promise<Tenant> {
    return api<Tenant, { name: string }>("/tenants", {
      method: "POST",
      body: { name },
      expectedStatus: 201,
      schema: tenantSchema,
    });
  },
  rename(tenantID: string, name: string): Promise<Tenant> {
    const id = entityIDSchema.parse(tenantID);
    return api<Tenant, { name: string }>(`/tenants/${encodeURIComponent(id)}/name`, {
      method: "PUT",
      body: { name },
      schema: tenantSchema,
    });
  },
  invite(tenantID: string, username: string): Promise<TenantInvitation> {
    const id = entityIDSchema.parse(tenantID);
    return api<TenantInvitation, { username: string }>(`/tenants/${encodeURIComponent(id)}/invitations`, {
      method: "POST", body: { username }, expectedStatus: 201, schema: invitationSchema,
    });
  },
  findPendingInvitationsForUser(signal?: AbortSignal): Promise<TenantInvitationListResponse> {
    return api("/tenant-invitations", { method: "GET", signal, schema: invitationListSchema });
  },
  acceptInvitation(invitationID: string): Promise<Tenant> {
    const id = entityIDSchema.parse(invitationID);
    return api(`/tenant-invitations/${encodeURIComponent(id)}/accept`, { method: "POST", schema: tenantSchema });
  },
  leave(tenantID: string): Promise<{ left: boolean }> {
    const id = entityIDSchema.parse(tenantID);
    return api<{ left: boolean }>(`/tenants/${encodeURIComponent(id)}/leave`, { method: "POST" });
  },
  findMembersForOwner(tenantID: string, signal?: AbortSignal): Promise<TenantMemberListResponse> {
    const id = entityIDSchema.parse(tenantID);
    return api(`/tenants/${encodeURIComponent(id)}/members`, { method: "GET", signal, schema: memberListSchema });
  },
  removeMember(tenantID: string, userID: string): Promise<{ removed: boolean }> {
    const tenant = entityIDSchema.parse(tenantID);
    const user = entityIDSchema.parse(userID);
    return api<{ removed: boolean }>(`/tenants/${encodeURIComponent(tenant)}/members/${encodeURIComponent(user)}`, { method: "DELETE" });
  },
  transferOwnership(tenantID: string, nextOwnerUserID: string): Promise<{ transferred: boolean }> {
    const tenant = entityIDSchema.parse(tenantID);
    const user = entityIDSchema.parse(nextOwnerUserID);
    return api<{ transferred: boolean }, { next_owner_user_id: string }>(`/tenants/${encodeURIComponent(tenant)}/transfer`, {
      method: "POST", body: { next_owner_user_id: user },
    });
  },
};
