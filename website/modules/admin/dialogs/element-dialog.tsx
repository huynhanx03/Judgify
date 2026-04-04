"use client";

/**
 * Create / edit dialog for Element entities.
 * Form fields: name (required), code (required), description (optional).
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
import type { ElementResponse } from "@/types/cultivation";

interface ElementDialogProps {
  open: boolean;
  editing: ElementResponse | null;
  onSave: (input: { name: string; code: string; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function ElementDialog({ open, editing, onSave, onClose, isSaving }: ElementDialogProps) {
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setCode(editing.code);
      setDescription(editing.description ?? "");
    } else {
      setName("");
      setCode("");
      setDescription("");
    }
  }, [editing, open]);

  function handleSubmit() {
    if (!name.trim() || !code.trim()) return;
    onSave({ name: name.trim(), code: code.trim().toUpperCase(), description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.ELEMENTS.DIALOG_EDIT_TITLE : TEXT.ADMIN.ELEMENTS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.ELEMENTS.DIALOG_EDIT_DESC : TEXT.ADMIN.ELEMENTS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input
              placeholder="VD: Hoả"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.CODE}</Label>
            <Input
              placeholder={TEXT.ADMIN.FIELDS.CODE_PLACEHOLDER}
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.DESCRIPTION}</Label>
            <Input
              placeholder={TEXT.ADMIN.FIELDS.DESCRIPTION_PLACEHOLDER}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" className="cursor-pointer" onClick={onClose} disabled={isSaving}>
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            className="cursor-pointer"
            disabled={!name.trim() || !code.trim() || isSaving}
            onClick={handleSubmit}
          >
            {isSaving && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
