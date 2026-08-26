"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { formatDateTime } from "@/lib/format";
import type { ReviewedRoleMetadataCommand } from "@/types/admin";

interface RoleMetadataReviewDialogProps {
  reviewed: ReviewedRoleMetadataCommand | null;
  busy: boolean;
  requiresReauthentication: boolean;
  error: string | null;
  onClose: () => void;
  onConfirm: (password: string) => void;
}

export function RoleMetadataReviewDialog({
  reviewed,
  busy,
  requiresReauthentication,
  error,
  onClose,
  onConfirm,
}: RoleMetadataReviewDialogProps) {
  const [password, setPassword] = useState("");

  useEffect(() => {
    setPassword("");
  }, [reviewed?.command_id]);

  if (!reviewed) return null;

  const { preview } = reviewed.review;
  const title =
    preview.action === "archive"
      ? ADMIN_TEXT.ROLES_METADATA_REVIEW.ARCHIVE_TITLE
      : ADMIN_TEXT.ROLES_METADATA_REVIEW.UPDATE_TITLE;

  function close() {
    if (busy) return;
    setPassword("");
    onClose();
  }

  return (
    <Dialog open onOpenChange={(open) => !open && close()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.ROLES_METADATA_REVIEW.DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <dl className="space-y-3 rounded-xl border border-border bg-muted/30 p-4 text-sm">
          <div>
            <dt className="text-xs font-medium text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.KEY}
            </dt>
            <dd className="mt-1 font-mono">{preview.after.key}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.VERSION_LABEL}
            </dt>
            <dd className="mt-1">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.VERSION(
                preview.expected_version,
                preview.resulting_version,
              )}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.BEFORE}
            </dt>
            <dd className="mt-1">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.ROLE_PROJECTION(
                preview.before.name,
                preview.before.description ?? "",
                preview.before.status,
              )}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.AFTER}
            </dt>
            <dd className="mt-1 font-medium">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.ROLE_PROJECTION(
                preview.after.name,
                preview.after.description ?? "",
                preview.after.status,
              )}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.REASON}
            </dt>
            <dd className="mt-1 whitespace-pre-wrap break-words">
              {reviewed.input.reason}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.EXPIRY_LABEL}
            </dt>
            <dd className="mt-1">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.EXPIRES_AT(
                formatDateTime(reviewed.review.expires_at),
              )}
            </dd>
          </div>
        </dl>

        {preview.action === "archive" ? (
          <p className="rounded-xl border border-destructive/20 bg-destructive/5 p-3 text-sm leading-relaxed text-muted-foreground">
            {ADMIN_TEXT.ROLES_METADATA_REVIEW.ARCHIVE_EFFECT}
          </p>
        ) : null}

        {requiresReauthentication ? (
          <div className="space-y-2 rounded-xl border border-primary/20 bg-primary/5 p-4">
            <p className="text-sm font-semibold">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.REAUTH_TITLE}
            </p>
            <p className="text-sm leading-relaxed text-muted-foreground">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.REAUTH_DESCRIPTION}
            </p>
            <Label htmlFor="role-metadata-password">
              {ADMIN_TEXT.ROLES_METADATA_REVIEW.PASSWORD}
            </Label>
            <Input
              id="role-metadata-password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder={ADMIN_TEXT.ROLES_METADATA_REVIEW.PASSWORD_PLACEHOLDER}
              disabled={busy}
            />
          </div>
        ) : null}

        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={close}>
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            type="button"
            disabled={busy || (requiresReauthentication && !password)}
            onClick={() => onConfirm(password)}
          >
            {busy ? (
              <>
                <Loader2
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
                {ADMIN_TEXT.ROLES_METADATA_REVIEW.APPLYING}
              </>
            ) : (
              ADMIN_TEXT.ROLES_METADATA_REVIEW.APPLY
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
