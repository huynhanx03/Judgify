"use client";

import { useState } from "react";
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
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { CreateRoleRequest, Role } from "@/types/admin";

interface RoleDialogProps {
  open: boolean;
  editing: Role | null;
  onSave: (input: CreateRoleRequest) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

function normalizeRoleKey(value: string) {
  return value
    .trim()
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

export function RoleDialog({
  open,
  editing,
  onSave,
  onClose,
  isSaving,
}: RoleDialogProps) {
  const [roleKey, setRoleKey] = useState(editing?.key ?? "");
  const [roleName, setRoleName] = useState(editing?.name ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");
  const [reason, setReason] = useState("");

  const normalizedKey = normalizeRoleKey(roleKey);
  const normalizedName = roleName.trim();
  const normalizedDescription = description.trim();
  const normalizedReason = reason.trim();
  const metadataChanged =
    !editing ||
    normalizedName !== editing.name ||
    normalizedDescription !== (editing.description ?? "");
  const isValid =
    /^[a-z][a-z0-9_-]{1,49}$/.test(normalizedKey) &&
    normalizedName.length >= 2 &&
    normalizedName.length <= 50 &&
    normalizedReason.length >= 3 &&
    normalizedReason.length <= 1024 &&
    metadataChanged;

  function handleSubmit() {
    if (!isValid) return;
    void onSave({
      key: normalizedKey,
      name: normalizedName,
      description: normalizedDescription || undefined,
      reason: normalizedReason,
    });
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {editing
              ? ADMIN_TEXT.ROLES_EDIT_DIALOG_TITLE
              : ADMIN_TEXT.ROLES_CREATE_DIALOG_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing
              ? ADMIN_TEXT.ROLES_EDIT_DIALOG_DESC
              : ADMIN_TEXT.ROLES_CREATE_DIALOG_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="role-key">{ADMIN_TEXT.ROLES_FORM_KEY}</Label>
            <Input
              id="role-key"
              autoComplete="off"
              placeholder={ADMIN_TEXT.ROLES_FORM_KEY_PLACEHOLDER}
              value={roleKey}
              maxLength={50}
              onChange={(event) => setRoleKey(event.target.value)}
              disabled={editing !== null || isSaving}
            />
            <p className="text-xs leading-relaxed text-muted-foreground">
              {editing
                ? ADMIN_TEXT.ROLES_IMMUTABLE_KEY_HINT
                : ADMIN_TEXT.ROLES_FORM_KEY_HINT}
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="role-name">{ADMIN_TEXT.ROLES_FORM_NAME}</Label>
            <Input
              id="role-name"
              placeholder={ADMIN_TEXT.ROLES_FORM_NAME_PLACEHOLDER}
              value={roleName}
              maxLength={50}
              onChange={(event) => setRoleName(event.target.value)}
              disabled={isSaving}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="role-description">
              {ADMIN_TEXT.ROLES_FORM_DESCRIPTION}
            </Label>
            <Textarea
              id="role-description"
              placeholder={ADMIN_TEXT.ROLES_FORM_DESCRIPTION_PLACEHOLDER}
              value={description}
              maxLength={255}
              onChange={(event) => setDescription(event.target.value)}
              disabled={isSaving}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="role-change-reason">
              {ADMIN_TEXT.ROLES_CHANGE_REASON_LABEL}
            </Label>
            <Textarea
              id="role-change-reason"
              value={reason}
              maxLength={1024}
              placeholder={ADMIN_TEXT.ROLES_CHANGE_REASON_PLACEHOLDER}
              onChange={(event) => setReason(event.target.value)}
              disabled={isSaving}
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={isSaving}
            className="cursor-pointer"
          >
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={!isValid || isSaving}
            className="cursor-pointer"
          >
            {isSaving ? (
              <Loader2
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : editing ? (
              TEXT.COMMON.SAVE
            ) : (
              TEXT.COMMON.CREATE
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
