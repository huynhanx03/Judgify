"use client";

import { useTenant } from "@/contexts/tenant-context";
import { text } from "@/i18n/text";

export function TenantRowLabel({ tenantID }: { tenantID?: string }) {
  const { tenants, isAllTenants } = useTenant();
  if (!isAllTenants || !tenantID) return null;
  const tenant = tenants.find((item) => item.id === tenantID);
  return (
    <span className="mt-1 block truncate text-[10px] text-muted-foreground" title={tenant?.name ?? tenantID}>
      {text("TENANT.ROW_LABEL", { name: tenant?.name ?? tenantID })}
    </span>
  );
}
