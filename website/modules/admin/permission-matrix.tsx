"use client";

import { Check, Minus } from "lucide-react";
import { ADMIN_TEXT } from "@/constants/admin-text";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import type { AuthorizationResource, PolicyRule } from "@/types/admin";

interface PermissionMatrixProps {
  resources: AuthorizationResource[];
  policies: PolicyRule[];
  onToggle: (resource: string, action: string, enabled: boolean) => void;
  readOnly?: boolean;
}

function policyKey(resource: string, action: string) {
  return `${resource}\u0000${action}`;
}

/**
 * Dynamic resource/action matrix backed only by the server capability catalog.
 * No action or permission value is invented by the frontend.
 */
export function PermissionMatrix({
  resources,
  policies,
  onToggle,
  readOnly = false,
}: PermissionMatrixProps) {
  const enabledPolicies = new Set(
    policies.map((policy) => policyKey(policy.resource, policy.action)),
  );

  if (resources.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-border bg-card/40 px-6 py-12 text-center">
        <p className="font-medium text-foreground">
          {ADMIN_TEXT.PERMISSIONS.EMPTY_TITLE}
        </p>
        <p className="mt-1 text-sm text-muted-foreground">
          {ADMIN_TEXT.PERMISSIONS.EMPTY_DESCRIPTION}
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-border bg-card/70 shadow-sm">
        <Table
          containerLabel={ADMIN_TEXT.PERMISSIONS.TABLE_LABEL}
          className="min-w-[720px] border-collapse"
        >
          <TableCaption className="sr-only">
            {ADMIN_TEXT.PERMISSIONS.TABLE_LABEL}
          </TableCaption>
          <TableHeader className="sticky top-0 z-10 bg-muted/80 backdrop-blur-xl">
            <TableRow className="border-border">
              <TableHead className="min-w-64 px-5 py-3 text-left uppercase">
                {ADMIN_TEXT.PERMISSIONS.RESOURCE}
              </TableHead>
              <TableHead className="px-5 py-3 text-left uppercase">
                {ADMIN_TEXT.PERMISSIONS.ACTIONS}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="divide-y divide-border/70">
            {resources.map((resource) => (
              <TableRow key={resource.key} className="hover:bg-muted/30">
                <th scope="row" className="px-5 py-4 text-left align-top">
                  <span className="block font-semibold text-foreground">
                    {ADMIN_TEXT.PERMISSIONS.RESOURCE_LABEL(resource.key)}
                  </span>
                  <code className="mt-1 block text-xs font-normal text-primary">
                    {resource.key}
                  </code>
                  {ADMIN_TEXT.PERMISSIONS.RESOURCE_DESCRIPTION(resource.key) ? (
                    <span className="mt-1.5 block max-w-md text-xs font-normal leading-relaxed text-muted-foreground">
                      {ADMIN_TEXT.PERMISSIONS.RESOURCE_DESCRIPTION(resource.key)}
                    </span>
                  ) : null}
                </th>
                <TableCell className="whitespace-normal px-5 py-4 align-top">
                  <div className="flex flex-wrap gap-2">
                    {resource.actions.length > 0 ? (
                      resource.actions.map((action) => {
                        const enabled = enabledPolicies.has(
                          policyKey(resource.key, action.key),
                        );
                        const resourceLabel =
                          ADMIN_TEXT.PERMISSIONS.RESOURCE_LABEL(resource.key);
                        const actionLabel =
                          ADMIN_TEXT.PERMISSIONS.ACTION_LABEL(action.key);
                        const actionDescription =
                          ADMIN_TEXT.PERMISSIONS.ACTION_DESCRIPTION(action.key);
                        const accessibleLabel = enabled
                          ? ADMIN_TEXT.PERMISSIONS.REVOKE(
                              actionLabel,
                              resourceLabel,
                            )
                          : ADMIN_TEXT.PERMISSIONS.GRANT(
                              actionLabel,
                              resourceLabel,
                            );

                        return (
                          <button
                            key={action.key}
                            type="button"
                            aria-pressed={enabled}
                            aria-label={accessibleLabel}
                            title={actionDescription || accessibleLabel}
                            disabled={readOnly}
                            onClick={() => onToggle(resource.key, action.key, enabled)}
                            className={cn(
                              "inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-xl border px-3 py-2 text-xs font-semibold transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60",
                              enabled
                                ? "border-primary/40 bg-primary/10 text-primary shadow-sm"
                                : "border-border bg-background/70 text-muted-foreground hover:border-primary/30 hover:bg-muted hover:text-foreground",
                            )}
                          >
                            <span
                              aria-hidden="true"
                              className={cn(
                                "flex size-5 items-center justify-center rounded-md transition-colors",
                                enabled
                                  ? "bg-primary text-primary-foreground"
                                  : "bg-muted text-muted-foreground",
                              )}
                            >
                              {enabled ? (
                                <Check className="size-3.5" strokeWidth={3} />
                              ) : (
                                <Minus className="size-3.5" />
                              )}
                            </span>
                            {actionLabel}
                          </button>
                        );
                      })
                    ) : (
                      <span className="text-xs text-muted-foreground">
                        {ADMIN_TEXT.PERMISSIONS.NO_ACTIONS}
                      </span>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
    </div>
  );
}
