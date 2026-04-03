"use client";

/**
 * Create / edit dialog for Trait entities.
 * Rarities list is passed in from the parent page to avoid duplicate fetching.
 * Form fields: type (select), name (required), rarity_id (required), description (optional).
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
import type { TraitResponse, RarityResponse } from "@/types/cultivation";

interface TraitDialogProps {
  open: boolean;
  editing: TraitResponse | null;
  onSave: (input: { type: string; name: string; rarity_id: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
  /** Rarities list loaded by the parent page — avoids a duplicate network request */
  rarities: RarityResponse[];
}

export function TraitDialog({ open, editing, onSave, onClose, isSaving, rarities }: TraitDialogProps) {
  const [type, setType] = useState("root_bone");
  const [name, setName] = useState("");
  const [rarityId, setRarityId] = useState("");
  const [description, setDescription] = useState("");

  // Pre-fill when editing
  useEffect(() => {
    if (editing) {
      setType(editing.type);
      setName(editing.name);
      setRarityId(editing.rarity ? String(editing.rarity.id) : "");
      setDescription(editing.description ?? "");
    } else {
      setType("root_bone");
      setName("");
      setRarityId("");
      setDescription("");
    }
  }, [editing, open]);

  function handleSubmit() {
    if (!name.trim() || !rarityId) return;
    onSave({ type, name: name.trim(), rarity_id: Number(rarityId), description: description.trim() || undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.TRAITS.DIALOG_EDIT_TITLE : TEXT.ADMIN.TRAITS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.TRAITS.DIALOG_EDIT_DESC : TEXT.ADMIN.TRAITS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.TYPE}</Label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value)}
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="root_bone">{TEXT.ADMIN.FIELDS.TYPE_ROOT_BONE}</option>
              <option value="talent">{TEXT.ADMIN.FIELDS.TYPE_TALENT}</option>
            </select>
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input placeholder="VD: Kim Cương Căn" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.RARITY}</Label>
            <select
              value={rarityId}
              onChange={(e) => setRarityId(e.target.value)}
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="">{TEXT.ADMIN.FIELDS.SELECT_RARITY_PLACEHOLDER}</option>
              {rarities.map((r) => (
                <option key={r.id} value={String(r.id)}>{r.name}</option>
              ))}
            </select>
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
            disabled={!name.trim() || !rarityId || isSaving}
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
