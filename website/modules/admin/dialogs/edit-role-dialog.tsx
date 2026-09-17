"use client";

import { useState } from "react";
import { Check, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { POLICY_REASON_LIMITS } from "@/constants/authorization";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { cn } from "@/lib/utils";
import { roleDisplayName } from "@/lib/auth/role-presentation";
import type { AdminUser, Role, UserRoleCommandInput } from "@/types/admin";

interface EditRoleDialogProps {
  user: AdminUser | null;
  roles: Role[];
  initialRequest?: UserRoleCommandInput | null;
  onReview: (userId: string, request: UserRoleCommandInput) => Promise<void>;
  onClose: () => void;
  isReviewing: boolean;
  operationError?: string | null;
}

export function EditRoleDialog({
  user,
  roles,
  initialRequest,
  onReview,
  onClose,
  isReviewing,
  operationError,
}: EditRoleDialogProps) {
  const [selectedRoles, setSelectedRoles] = useState<string[]>(
    () => initialRequest?.roles ?? user?.roles ?? [],
  );
  const [reason, setReason] = useState(() => initialRequest?.reason ?? "");

  function toggleRole(roleKey: string) {
    setSelectedRoles((current) =>
      current.includes(roleKey)
        ? current.filter((key) => key !== roleKey)
        : [...current, roleKey],
    );
  }

  async function handleSubmit() {
    if (!user || selectedRoles.length === 0 || !isReasonValid || !isDirty) return;
    await onReview(user.id, {
      expected_revision: user.authorization_revision,
      reason: trimmedReason,
      roles: selectedRoles,
    });
  }

  const trimmedReason = reason.trim();
  const isReasonValid =
    trimmedReason.length >= POLICY_REASON_LIMITS.MIN_LENGTH &&
    trimmedReason.length <= POLICY_REASON_LIMITS.MAX_LENGTH;
  const isDirty =
    [...selectedRoles].sort().join("\u0000") !==
    [...(user?.roles ?? [])].sort().join("\u0000");

  return (
    <Dialog
      open={!!user}
      onOpenChange={(open) => !open && !isReviewing && onClose()}
    >
      <DialogContent className="sm:max-w-md" showCloseButton={!isReviewing}>
        <DialogHeader>
          <DialogTitle>{ADMIN_TEXT.USERS_EDIT_ROLE_TITLE}</DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.USERS_EDIT_ROLE_DESC(user?.username ?? "")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          {operationError ? (
            <div
              className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
              role="alert"
              aria-live="assertive"
            >
              {operationError}
            </div>
          ) : null}
          <p id="user-role-options-label" className="text-sm font-medium">
            {ADMIN_TEXT.USERS_FORM_ROLES}
          </p>
          <div
            className="grid max-h-72 gap-2 overflow-y-auto rounded-xl border border-border p-2"
            role="group"
            aria-labelledby="user-role-options-label"
          >
            {roles.map((role) => {
              const selected = selectedRoles.includes(role.key);
              return (
                <button
                  key={role.id}
                  type="button"
                  aria-pressed={selected}
                  disabled={isReviewing}
                  onClick={() => toggleRole(role.key)}
                  className={cn(
                    "flex min-h-11 cursor-pointer items-center gap-3 rounded-lg border px-3 py-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-60",
                    selected
                      ? "border-primary/30 bg-primary/10 text-primary"
                      : "border-transparent bg-muted/30 text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  <span
                    className={cn(
                      "flex size-5 shrink-0 items-center justify-center rounded-md border",
                      selected
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-border bg-background",
                    )}
                  >
                    {selected ? <Check className="size-3.5" strokeWidth={3} /> : null}
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-semibold">
                      {roleDisplayName(role)}
                    </span>
                    <code className="block truncate text-[11px] opacity-70">{role.key}</code>
                  </span>
                </button>
              );
            })}
          </div>
          {selectedRoles.length === 0 ? (
            <p className="text-xs text-destructive">{ADMIN_TEXT.USERS_ROLE_REQUIRED}</p>
          ) : null}

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="user-role-change-reason">
                {ADMIN_TEXT.USERS_ROLE_REASON_LABEL}
              </Label>
              <span className="text-xs tabular-nums text-muted-foreground">
                {ADMIN_TEXT.USERS_ROLE_REASON_COUNT(
                  reason.length,
                  POLICY_REASON_LIMITS.MAX_LENGTH,
                )}
              </span>
            </div>
            <Textarea
              id="user-role-change-reason"
              className="min-h-20 resize-y"
              value={reason}
              maxLength={POLICY_REASON_LIMITS.MAX_LENGTH}
              placeholder={ADMIN_TEXT.USERS_ROLE_REASON_PLACEHOLDER}
              aria-describedby="user-role-change-reason-hint"
              aria-invalid={!isReasonValid}
              disabled={isReviewing}
              onChange={(event) => setReason(event.target.value)}
            />
            <p
              id="user-role-change-reason-hint"
              className={cn(
                "text-xs",
                isReasonValid ? "text-muted-foreground" : "text-destructive",
              )}
            >
              {isReasonValid
                ? ADMIN_TEXT.USERS_ROLE_REASON_HINT(
                    POLICY_REASON_LIMITS.MIN_LENGTH,
                    POLICY_REASON_LIMITS.MAX_LENGTH,
                  )
                : ADMIN_TEXT.USERS_ROLE_REASON_REQUIRED}
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={isReviewing}
            className="cursor-pointer"
          >
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={
              isReviewing ||
              selectedRoles.length === 0 ||
              !isReasonValid ||
              !isDirty
            }
            className="cursor-pointer"
          >
            {isReviewing ? (
              <Loader2
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : null}
            {isReviewing
              ? ADMIN_TEXT.USERS_ROLE_REVIEW.REVIEWING
              : ADMIN_TEXT.USERS_ROLE_REVIEW.REVIEW}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
