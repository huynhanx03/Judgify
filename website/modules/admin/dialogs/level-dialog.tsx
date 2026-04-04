"use client";

/**
 * Create / edit dialog for Level entities.
 * Form fields: name (required), min_exp (required), description (optional).
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
import type { LevelResponse } from "@/types/cultivation";

interface LevelDialogProps {
  open: boolean;
  editing: LevelResponse | null;
  onSave: (input: { name: string; min_exp: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function LevelDialog({ open, editing, onSave, onClose, isSaving }: LevelDialogProps) {
  const [name, setName] = useState("");
  const [minExp, setMinExp] = useState(0);
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setMinExp(editing.min_exp);
      setDescription(editing.description ?? "");
    } else {
      setName("");
      setMinExp(0);
      setDescription("");
    }
  }, [editing, open]);

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({ name: name.trim(), min_exp: minExp, description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.LEVELS.DIALOG_EDIT_TITLE : TEXT.ADMIN.LEVELS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.LEVELS.DIALOG_EDIT_DESC : TEXT.ADMIN.LEVELS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input
              placeholder="VD: Luyện Khí"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.MIN_EXP}</Label>
            <Input
              type="number"
              min={0}
              value={minExp}
              onChange={(e) => setMinExp(Number(e.target.value))}
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
            disabled={!name.trim() || isSaving}
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
