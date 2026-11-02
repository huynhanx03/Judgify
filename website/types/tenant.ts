import type { EntityID, ISODateTime } from "@/types/api";

export interface Tenant {
  id: EntityID;
  name: string;
  slug: string;
  role: "owner" | "member";
  status: "active" | "suspended" | "archived";
  can_change_name: boolean;
  next_name_change_at: ISODateTime | null;
}

export interface TenantListResponse {
  tenants: Tenant[];
}

export interface TenantInvitation {
  id: EntityID;
  tenant_id: EntityID;
  tenant_name: string;
  username: string;
  expires_at: ISODateTime;
}

export interface TenantInvitationListResponse {
  invitations: TenantInvitation[];
}

export interface TenantMember {
  user_id: EntityID;
  username: string;
  role: "owner" | "member";
}

export interface TenantMemberListResponse {
  members: TenantMember[];
}
