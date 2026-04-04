"use client";

/**
 * Permission matrix — flat grid showing resource × CRUD scopes for a role.
 * No grouping, clear descriptions for each resource.
 */

import { Check } from "lucide-react";
import type { Permission, Resource } from "@/types/admin";
import { cn } from "@/lib/utils";

/** Permission scope bitmask constants matching backend. */
const SCOPES = [
  { label: "Tạo", value: 1, title: "Create" },
  { label: "Xem", value: 2, title: "Read" },
  { label: "Sửa", value: 4, title: "Update" },
  { label: "Xóa", value: 8, title: "Delete" },
] as const;

interface PermissionMatrixProps {
  roleId: number;
  resources: Resource[];
  permissions: Permission[];
  onToggleScope: (resourceId: number, scope: number, currentlyEnabled: boolean) => void;
  readOnly?: boolean;
}

export function PermissionMatrix({
  roleId,
  resources,
  permissions,
  onToggleScope,
  readOnly = false,
}: PermissionMatrixProps) {
  // Build lookup: resourceId → scopes bitmask
  const permMap = new Map<number, number>();
  for (const p of permissions) {
    if (p.role_id === roleId) {
      permMap.set(p.resource_id, p.scopes);
    }
  }

  return (
    <div className="rounded-lg border border-border overflow-hidden">
      {/* Header */}
      <div className="grid grid-cols-[1fr_repeat(4,64px)] bg-muted/30 border-b border-border">
        <div className="px-4 py-2.5 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
          Tài Nguyên
        </div>
        {SCOPES.map((s) => (
          <div
            key={s.label}
            className="flex items-center justify-center py-2.5 text-xs font-bold text-muted-foreground"
            title={s.title}
          >
            {s.label}
          </div>
        ))}
      </div>

      {/* Flat resource rows */}
      {resources.map((resource) => {
        const currentScopes = permMap.get(resource.id) ?? 0;

        return (
          <div
            key={resource.id}
            className="grid grid-cols-[1fr_repeat(4,64px)] border-b border-border last:border-0 hover:bg-muted/10 transition-colors"
          >
            {/* Resource info: name on top, description below */}
            <div className="flex flex-col justify-center px-4 py-2.5">
              <span className="text-sm font-medium">{resource.key}</span>
              <span className="text-[11px] text-muted-foreground">{resource.description}</span>
            </div>

            {/* Scope toggles */}
            {SCOPES.map((scope) => {
              const enabled = (currentScopes & scope.value) !== 0;

              return (
                <div key={scope.label} className="flex items-center justify-center">
                  <button
                    onClick={() => {
                      if (!readOnly) onToggleScope(resource.id, scope.value, enabled);
                    }}
                    disabled={readOnly}
                    className={cn(
                      "flex h-7 w-7 items-center justify-center rounded-md transition-all duration-150",
                      readOnly ? "cursor-default" : "cursor-pointer",
                      enabled
                        ? "bg-primary text-primary-foreground shadow-sm"
                        : "bg-muted/40 text-transparent hover:bg-muted hover:text-muted-foreground"
                    )}
                    title={`${scope.title} ${resource.key}`}
                  >
                    <Check className="h-3.5 w-3.5" strokeWidth={3} />
                  </button>
                </div>
              );
            })}
          </div>
        );
      })}
    </div>
  );
}
