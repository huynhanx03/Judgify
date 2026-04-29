"use client";

/**
 * Dialog for changing a user's role.
 */

import { useState, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import { TEXT } from "@/constants/text";
import type { AdminUser, Role } from "@/types/admin";

interface EditRoleDialogProps {
  user: AdminUser | null;
  roles: Role[];
  onSave: (userId: number, roleId: number) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function EditRoleDialog({ user, roles, onSave, onClose, isSaving }: EditRoleDialogProps) {
  const [roleId, setRoleId] = useState("");

  useEffect(() => {
    if (user) setRoleId(String(user.role_id));
  }, [user]);

  async function handleSubmit() {
    if (!user || !roleId) return;
    await onSave(user.id, Number(roleId));
  }

  return (
    <Dialog open={!!user} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{TEXT.ADMIN.USERS_EDIT_ROLE_TITLE}</DialogTitle>
          <DialogDescription>{TEXT.ADMIN.USERS_EDIT_ROLE_DESC(user?.username ?? "")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2 py-2">
          <Label>{TEXT.ADMIN.USERS_FORM_ROLE}</Label>
          <Select value={roleId} onValueChange={setRoleId}>
            <SelectTrigger><SelectValue placeholder={TEXT.ADMIN.USERS_ROLE_PLACEHOLDER} /></SelectTrigger>
            <SelectContent>
              {roles.map((r) => (
                <SelectItem key={r.id} value={String(r.id)}>{r.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSaving} className="cursor-pointer">{TEXT.COMMON.CANCEL}</Button>
          <Button onClick={handleSubmit} disabled={isSaving || !roleId} className="cursor-pointer">
            {isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : TEXT.COMMON.SAVE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
