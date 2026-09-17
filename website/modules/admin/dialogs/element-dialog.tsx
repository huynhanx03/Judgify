"use client";

/**
 * Create / edit dialog for Element entities.
 * Form fields: name (required), code (required), description (optional).
 */

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogDescription,
  DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { ElementResponse } from "@/types/cultivation";

interface ElementDialogProps {
  open: boolean;
  editing: ElementResponse | null;
  onSave: (input: { name: string; code: string; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function ElementDialog({ open, editing, onSave, onClose, isSaving }: ElementDialogProps) {
  const [name, setName] = useState(editing?.name ?? "");
  const [code, setCode] = useState(editing?.code ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");

  function handleSubmit() {
    if (!name.trim() || !code.trim()) return;
    onSave({ name: name.trim(), code: code.trim().toUpperCase(), description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.ELEMENTS.DIALOG_EDIT_TITLE : ADMIN_TEXT.ELEMENTS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.ELEMENTS.DIALOG_EDIT_DESC : ADMIN_TEXT.ELEMENTS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="element-name">{ADMIN_TEXT.FIELDS.NAME}</Label>
            <Input
              id="element-name"
              placeholder={ADMIN_TEXT.EXAMPLES.ELEMENT}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="element-code">{ADMIN_TEXT.FIELDS.CODE}</Label>
            <Input
              id="element-code"
              placeholder={ADMIN_TEXT.FIELDS.CODE_PLACEHOLDER}
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="element-description">{ADMIN_TEXT.FIELDS.DESCRIPTION}</Label>
            <Input
              id="element-description"
              placeholder={ADMIN_TEXT.FIELDS.DESCRIPTION_PLACEHOLDER}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" className="cursor-pointer" onClick={onClose} disabled={isSaving}>
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            type="button"
            className="cursor-pointer"
            disabled={!name.trim() || !code.trim() || isSaving}
            onClick={handleSubmit}
          >
            {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
