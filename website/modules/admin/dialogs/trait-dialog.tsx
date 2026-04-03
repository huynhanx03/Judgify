"use client";

/**
 * Create / edit dialog for Trait entities.
 * Loads rarities on mount for the rarity select.
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
import {
  Select, SelectContent, SelectItem,
  SelectTrigger, SelectValue,
} from "@/components/ui/select";
import { TEXT } from "@/constants/text";
import { adminService } from "@/services/admin.service";
import type { TraitResponse, RarityResponse } from "@/types/cultivation";

interface TraitDialogProps {
  open: boolean;
  editing: TraitResponse | null;
  onSave: (input: { type: string; name: string; rarity_id: number; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function TraitDialog({ open, editing, onSave, onClose, isSaving }: TraitDialogProps) {
  const [type, setType] = useState("root_bone");
  const [name, setName] = useState("");
  const [rarityId, setRarityId] = useState("");
  const [description, setDescription] = useState("");
  const [rarities, setRarities] = useState<RarityResponse[]>([]);

  // Load rarities once on mount
  useEffect(() => {
    adminService.getAllRarities().then(setRarities).catch(() => {});
  }, []);

  // Pre-fill when editing
  useEffect(() => {
    if (editing) {
      setType(editing.type);
      setName(editing.name);
      setRarityId(String(editing.rarity_id));
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
            <Select value={type} onValueChange={setType}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="root_bone">{TEXT.ADMIN.FIELDS.TYPE_ROOT_BONE}</SelectItem>
                <SelectItem value="talent">{TEXT.ADMIN.FIELDS.TYPE_TALENT}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input placeholder="VD: Kim Cương Căn" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.RARITY}</Label>
            <Select value={rarityId} onValueChange={setRarityId}>
              <SelectTrigger>
                <SelectValue placeholder={TEXT.ADMIN.TRAITS.FILTER_RARITY_ALL} />
              </SelectTrigger>
              <SelectContent>
                {rarities.map((r) => (
                  <SelectItem key={r.id} value={String(r.id)}>{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
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
