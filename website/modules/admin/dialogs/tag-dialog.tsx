"use client";

/**
 * Create / edit dialog for Tag entities.
 * Elements list is passed in from the parent page to avoid duplicate fetching.
 * Form fields: name (required), element_ids (optional multi-select).
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
import type { Tag } from "@/types/tag";
import type { ElementResponse } from "@/types/cultivation";

interface TagDialogProps {
  open: boolean;
  editing: Tag | null;
  onSave: (input: { name: string; element_ids?: string[] }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
  /** Elements list loaded by the parent page — avoids a duplicate network request */
  elements: ElementResponse[];
}

export function TagDialog({ open, editing, onSave, onClose, isSaving, elements }: TagDialogProps) {
  const [name, setName] = useState(editing?.name ?? "");
  const [selectedIds, setSelectedIds] = useState<string[]>(
    editing?.elements?.map((element) => element.id) ?? [],
  );

  function toggleElement(id: string) {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  }

  function handleSubmit() {
    if (!name.trim()) return;
    onSave({ name: name.trim(), element_ids: selectedIds.length ? selectedIds : undefined });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.TAGS.DIALOG_EDIT_TITLE : ADMIN_TEXT.TAGS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.TAGS.DIALOG_EDIT_DESC : ADMIN_TEXT.TAGS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="tag-name">{ADMIN_TEXT.FIELDS.NAME}</Label>
            <Input id="tag-name" placeholder={ADMIN_TEXT.EXAMPLES.TAG} value={name} onChange={(e) => setName(e.target.value)} />
          </div>

          {elements.length > 0 && (
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium text-foreground">
                {ADMIN_TEXT.FIELDS.ELEMENT}
              </legend>
              <div className="grid grid-cols-2 gap-2">
                {elements.map((el) => (
                  <label
                    key={el.id}
                    className="flex items-center gap-2 text-sm cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={selectedIds.includes(el.id)}
                      onChange={() => toggleElement(el.id)}
                      className="accent-primary"
                    />
                    {el.name}
                  </label>
                ))}
              </div>
            </fieldset>
          )}
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
