"use client";

/**
 * Dialog for creating a new user.
 */

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import { TEXT } from "@/constants/text";
import type { Role } from "@/types/admin";

interface CreateUserForm {
  username: string;
  password: string;
  role_id: string;
  first_name: string;
  last_name: string;
  gender: string;
  birthday: string;
}

interface CreateUserDialogProps {
  open: boolean;
  roles: Role[];
  onSave: (form: {
    username: string;
    password: string;
    role_id: number;
    first_name: string;
    last_name: string;
    gender: number;
    birthday: string;
  }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

const INITIAL_FORM: CreateUserForm = {
  username: "", password: "", role_id: "",
  first_name: "", last_name: "",
  gender: "0", birthday: "",
};

export function CreateUserDialog({ open, roles, onSave, onClose, isSaving }: CreateUserDialogProps) {
  const [form, setForm] = useState<CreateUserForm>(INITIAL_FORM);

  function handleField(field: keyof CreateUserForm, value: string) {
    setForm((f) => ({ ...f, [field]: value }));
  }

  async function handleSubmit() {
    if (!form.username || !form.password || !form.role_id || !form.first_name || !form.last_name || !form.birthday) return;
    await onSave({
      username: form.username,
      password: form.password,
      role_id: Number(form.role_id),
      first_name: form.first_name,
      last_name: form.last_name,
      gender: Number(form.gender),
      birthday: form.birthday,
    });
    setForm(INITIAL_FORM);
  }

  const isValid = form.username && form.password && form.role_id && form.first_name && form.last_name && form.birthday;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{TEXT.ADMIN.USERS_CREATE_TITLE}</DialogTitle>
          <DialogDescription>{TEXT.ADMIN.USERS_CREATE_DESC}</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-3 py-2">
          <div className="space-y-1.5 col-span-2">
            <Label>{TEXT.ADMIN.USERS_FORM_USERNAME}</Label>
            <Input placeholder={TEXT.ADMIN.USERS_FORM_USERNAME_PLACEHOLDER} value={form.username} onChange={(e) => handleField("username", e.target.value)} />
          </div>
          <div className="space-y-1.5 col-span-2">
            <Label>{TEXT.ADMIN.USERS_FORM_PASSWORD}</Label>
            <Input type="password" placeholder={TEXT.ADMIN.USERS_FORM_PASSWORD_PLACEHOLDER} value={form.password} onChange={(e) => handleField("password", e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>{TEXT.ADMIN.USERS_FORM_LAST_NAME}</Label>
            <Input placeholder={TEXT.AUTH.LAST_NAME_PLACEHOLDER} value={form.last_name} onChange={(e) => handleField("last_name", e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>{TEXT.ADMIN.USERS_FORM_FIRST_NAME}</Label>
            <Input placeholder={TEXT.AUTH.FIRST_NAME_PLACEHOLDER} value={form.first_name} onChange={(e) => handleField("first_name", e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>{TEXT.ADMIN.USERS_FORM_BIRTHDAY}</Label>
            <Input type="date" value={form.birthday} onChange={(e) => handleField("birthday", e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>{TEXT.ADMIN.USERS_FORM_GENDER}</Label>
            <Select value={form.gender} onValueChange={(v) => handleField("gender", v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="0">{TEXT.ADMIN.USERS_FORM_GENDER_OTHER}</SelectItem>
                <SelectItem value="1">{TEXT.ADMIN.USERS_FORM_GENDER_MALE}</SelectItem>
                <SelectItem value="2">{TEXT.ADMIN.USERS_FORM_GENDER_FEMALE}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5 col-span-2">
            <Label>{TEXT.ADMIN.USERS_FORM_ROLE}</Label>
            <Select value={form.role_id} onValueChange={(v) => handleField("role_id", v)}>
              <SelectTrigger><SelectValue placeholder={TEXT.ADMIN.USERS_FORM_ROLE_PLACEHOLDER} /></SelectTrigger>
              <SelectContent>
                {roles.map((r) => (
                  <SelectItem key={r.id} value={String(r.id)}>{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSaving} className="cursor-pointer">{TEXT.COMMON.CANCEL}</Button>
          <Button onClick={handleSubmit} disabled={isSaving || !isValid} className="cursor-pointer">
            {isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
