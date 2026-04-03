"use client";

/**
 * Create / edit dialog for Difficulty entities.
 * Form fields: name (required), level (required), exp_reward (optional), description (optional).
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
import type { DifficultyResponse } from "@/types/difficulty";

interface DifficultyDialogProps {
  open: boolean;
  editing: DifficultyResponse | null;
  onSave: (input: { name: string; level: number; exp_reward?: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function DifficultyDialog({ open, editing, onSave, onClose, isSaving }: DifficultyDialogProps) {
  const [name, setName] = useState("");
  const [level, setLevel] = useState(1);
  const [expReward, setExpReward] = useState("");
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setLevel(editing.level);
      setExpReward(editing.exp_reward != null ? String(editing.exp_reward) : "");
      setDescription(editing.description ?? "");
    } else {
      setName("");
      setLevel(1);
      setExpReward("");
      setDescription("");
    }
  }, [editing, open]);

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
            {editing ? TEXT.ADMIN.DIFFICULTIES.DIALOG_EDIT_TITLE : TEXT.ADMIN.DIFFICULTIES.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.DIFFICULTIES.DIALOG_EDIT_DESC : TEXT.ADMIN.DIFFICULTIES.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input placeholder="VD: Dễ" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.LEVEL_NUM}</Label>
            <Input type="number" min={1} value={level} onChange={(e) => setLevel(Number(e.target.value))} />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.EXP_REWARD}</Label>
            <Input
              type="number"
              min={0}
              placeholder="Tuỳ chọn"
              value={expReward}
              onChange={(e) => setExpReward(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.DESCRIPTION}</Label>
            <Input placeholder={TEXT.ADMIN.FIELDS.DESCRIPTION_PLACEHOLDER} value={description} onChange={(e) => setDescription(e.target.value)} />
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
