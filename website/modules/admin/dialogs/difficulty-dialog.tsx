"use client";

/**
 * Create / edit dialog for Difficulty entities.
 * Form fields: name (required), level (required), exp_reward (optional), description (optional).
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
import type { DifficultyResponse } from "@/types/difficulty";

interface DifficultyDialogProps {
  open: boolean;
  editing: DifficultyResponse | null;
  onSave: (input: { name: string; level: number; exp_reward?: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function DifficultyDialog({ open, editing, onSave, onClose, isSaving }: DifficultyDialogProps) {
  const [name, setName] = useState(editing?.name ?? "");
  const [level, setLevel] = useState(editing?.level ?? 1);
  const [expReward, setExpReward] = useState(
    editing?.exp_reward != null ? String(editing.exp_reward) : "",
  );
  const [description, setDescription] = useState(editing?.description ?? "");

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({
      name: name.trim(),
      level,
      exp_reward: expReward !== "" ? Number(expReward) : undefined,
      description: description.trim() || undefined,
    });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.DIFFICULTIES.DIALOG_EDIT_TITLE : ADMIN_TEXT.DIFFICULTIES.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.DIFFICULTIES.DIALOG_EDIT_DESC : ADMIN_TEXT.DIFFICULTIES.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="difficulty-name">{ADMIN_TEXT.FIELDS.NAME}</Label>
            <Input id="difficulty-name" placeholder={ADMIN_TEXT.EXAMPLES.DIFFICULTY} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="difficulty-level">{ADMIN_TEXT.FIELDS.LEVEL_NUM}</Label>
            <Input id="difficulty-level" type="number" min={1} value={level} onChange={(e) => setLevel(Number(e.target.value))} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="difficulty-exp-reward">{ADMIN_TEXT.FIELDS.EXP_REWARD}</Label>
            <Input
              id="difficulty-exp-reward"
              type="number"
              min={0}
              placeholder={ADMIN_TEXT.OPTIONAL}
              value={expReward}
              onChange={(e) => setExpReward(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="difficulty-description">{ADMIN_TEXT.FIELDS.DESCRIPTION}</Label>
            <Input id="difficulty-description" placeholder={ADMIN_TEXT.FIELDS.DESCRIPTION_PLACEHOLDER} value={description} onChange={(e) => setDescription(e.target.value)} />
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
            {isSaving && <Loader2 className="h-4 w-4 animate-spin mr-2 motion-reduce:animate-none" aria-hidden="true" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
