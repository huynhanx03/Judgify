"use client";

/**
 * Create / edit dialog for Rank entities.
 * Form fields: name (required), min_rating (required), description (optional).
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
import type { RankResponse } from "@/types/cultivation";

interface RankDialogProps {
  open: boolean;
  editing: RankResponse | null;
  onSave: (input: { name: string; min_rating: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function RankDialog({ open, editing, onSave, onClose, isSaving }: RankDialogProps) {
  const [name, setName] = useState(editing?.name ?? "");
  const [minRating, setMinRating] = useState(editing?.min_rating ?? 0);
  const [description, setDescription] = useState(editing?.description ?? "");

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({ name: name.trim(), min_rating: minRating, description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.RANKS.DIALOG_EDIT_TITLE : ADMIN_TEXT.RANKS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.RANKS.DIALOG_EDIT_DESC : ADMIN_TEXT.RANKS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="rank-name">{ADMIN_TEXT.FIELDS.NAME}</Label>
            <Input
              id="rank-name"
              placeholder={ADMIN_TEXT.EXAMPLES.RANK}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rank-min-rating">{ADMIN_TEXT.FIELDS.MIN_RATING}</Label>
            <Input
              id="rank-min-rating"
              type="number"
              min={0}
              value={minRating}
              onChange={(e) => setMinRating(Number(e.target.value))}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rank-description">{ADMIN_TEXT.FIELDS.DESCRIPTION}</Label>
            <Input
              id="rank-description"
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
