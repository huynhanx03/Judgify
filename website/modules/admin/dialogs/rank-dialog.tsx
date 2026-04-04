"use client";

/**
 * Create / edit dialog for Rank entities.
 * Form fields: name (required), min_rating (required), description (optional).
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
import type { RankResponse } from "@/types/cultivation";

interface RankDialogProps {
  open: boolean;
  editing: RankResponse | null;
  onSave: (input: { name: string; min_rating: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function RankDialog({ open, editing, onSave, onClose, isSaving }: RankDialogProps) {
  const [name, setName] = useState("");
  const [minRating, setMinRating] = useState(0);
  const [description, setDescription] = useState("");

  // Pre-fill form when editing
  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setMinRating(editing.min_rating);
      setDescription(editing.description ?? "");
    } else {
      setName("");
      setMinRating(0);
      setDescription("");
    }
  }, [editing, open]);

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({ name: name.trim(), min_rating: minRating, description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.RANKS.DIALOG_EDIT_TITLE : TEXT.ADMIN.RANKS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.RANKS.DIALOG_EDIT_DESC : TEXT.ADMIN.RANKS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input
              placeholder="VD: Bạch Kim"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.MIN_RATING}</Label>
            <Input
              type="number"
              min={0}
              value={minRating}
              onChange={(e) => setMinRating(Number(e.target.value))}
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
