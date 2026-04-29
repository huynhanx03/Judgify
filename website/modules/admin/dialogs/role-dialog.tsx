"use client";

/**
 * Create / edit dialog for Role entities.
 * Form fields: name, level.
 */

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogDescription,
  DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TEXT } from "@/constants/text";
import type { Role } from "@/types/admin";

interface RoleDialogProps {
  open: boolean;
  editing: Role | null;
  onSave: (input: { name: string; level: number }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function RoleDialog({ open, editing, onSave, onClose, isSaving }: RoleDialogProps) {
  const [roleName, setRoleName] = useState("");
  const [roleLevel, setRoleLevel] = useState(0);

  useEffect(() => {
    if (editing) {
      setRoleName(editing.name);
      setRoleLevel(editing.level);
    } else {
      setRoleName("");
      setRoleLevel(0);
    }
  }, [editing, open]);

  function handleSubmit() {
    if (!roleName.trim()) return;
    onSave({ name: roleName.trim(), level: roleLevel });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.ROLES_EDIT_DIALOG_TITLE : TEXT.ADMIN.ROLES_CREATE_DIALOG_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.ROLES_EDIT_DIALOG_DESC : TEXT.ADMIN.ROLES_CREATE_DIALOG_DESC}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="role-name">{TEXT.ADMIN.ROLES_FORM_NAME}</Label>
            <Input
              id="role-name"
              placeholder={TEXT.ADMIN.ROLES_FORM_NAME_PLACEHOLDER}
              value={roleName}
              onChange={(e) => setRoleName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="role-level">{TEXT.ADMIN.ROLES_FORM_LEVEL}</Label>
            <Input
              id="role-level"
              type="number"
              min={0}
              max={10}
              value={roleLevel}
              onChange={(e) => setRoleLevel(Number(e.target.value))}
            />
            <p className="text-xs text-muted-foreground">{TEXT.ADMIN.ROLES_FORM_LEVEL_HINT}</p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSaving} className="cursor-pointer">
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button onClick={handleSubmit} disabled={!roleName.trim() || isSaving} className="cursor-pointer">
            {isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
