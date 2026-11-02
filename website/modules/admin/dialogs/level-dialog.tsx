"use client";

/**
 * Create / edit dialog for Level entities.
 * Form fields: name (required), min_exp (required), description (optional).
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
import type { LevelResponse } from "@/types/cultivation";

interface LevelDialogProps {
  open: boolean;
  editing: LevelResponse | null;
  onSave: (input: { name: string; min_exp: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function LevelDialog({ open, editing, onSave, onClose, isSaving }: LevelDialogProps) {
  const [name, setName] = useState(editing?.name ?? "");
  const [minExp, setMinExp] = useState(editing?.min_exp ?? 0);
  const [description, setDescription] = useState(editing?.description ?? "");

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({ name: name.trim(), min_exp: minExp, description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.LEVELS.DIALOG_EDIT_TITLE : ADMIN_TEXT.LEVELS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.LEVELS.DIALOG_EDIT_DESC : ADMIN_TEXT.LEVELS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="level-name">{ADMIN_TEXT.FIELDS.NAME}</Label>
            <Input
              id="level-name"
              placeholder={ADMIN_TEXT.EXAMPLES.LEVEL}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="level-min-exp">{ADMIN_TEXT.FIELDS.MIN_EXP}</Label>
            <Input
              id="level-min-exp"
              type="number"
              min={0}
              value={minExp}
              onChange={(e) => setMinExp(Number(e.target.value))}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="level-description">{ADMIN_TEXT.FIELDS.DESCRIPTION}</Label>
            <Input
              id="level-description"
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
            disabled={!name.trim() || isSaving}
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
