"use client";

/**
 * Create / edit dialog for Material Category entities.
 * Form fields: name (required), description (optional).
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
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { MaterialCategory } from "@/types/material";

interface MaterialCategoryDialogProps {
  open: boolean;
  editing: MaterialCategory | null;
  onSave: (input: { name: string; description?: string }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function MaterialCategoryDialog({ open, editing, onSave, onClose, isSaving }: MaterialCategoryDialogProps) {
  const [name, setName] = useState(editing?.name ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({
      name: name.trim(),
      description: description.trim() || undefined,
    });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.MATERIAL_CATEGORIES.DIALOG_EDIT_TITLE : ADMIN_TEXT.MATERIAL_CATEGORIES.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.MATERIAL_CATEGORIES.DIALOG_EDIT_DESC : ADMIN_TEXT.MATERIAL_CATEGORIES.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="material-category-name">{ADMIN_TEXT.MATERIAL_CATEGORIES.FORM_NAME}</Label>
            <Input
              id="material-category-name"
              placeholder={ADMIN_TEXT.MATERIAL_CATEGORIES.FORM_NAME_PLACEHOLDER}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="material-category-description">{ADMIN_TEXT.MATERIAL_CATEGORIES.FORM_DESCRIPTION}</Label>
            <Textarea
              id="material-category-description"
              placeholder={ADMIN_TEXT.MATERIAL_CATEGORIES.FORM_DESCRIPTION_PLACEHOLDER}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
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
            {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
