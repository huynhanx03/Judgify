"use client";

/**
 * Create / edit dialog for Tag entities.
 * Loads elements on mount for the multi-select checkboxes.
 * Form fields: name (required), element_ids (optional multi-select).
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
import { adminService } from "@/services/admin.service";
import type { Tag } from "@/types/tag";
import type { ElementResponse } from "@/types/cultivation";

interface TagDialogProps {
  open: boolean;
  editing: Tag | null;
  onSave: (input: { name: string; element_ids?: number[] }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function TagDialog({ open, editing, onSave, onClose, isSaving }: TagDialogProps) {
  const [name, setName] = useState("");
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [elements, setElements] = useState<ElementResponse[]>([]);

  // Load elements once on mount
  useEffect(() => {
    adminService.getAllElements().then(setElements).catch(() => {});
  }, []);

  // Pre-fill when editing
  useEffect(() => {
    if (editing) {
      setName(editing.name);
      setSelectedIds(editing.elements?.map((e) => e.id) ?? []);
    } else {
      setName("");
      setSelectedIds([]);
    }
  }, [editing, open]);

  function toggleElement(id: number) {
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
            {editing ? TEXT.ADMIN.TAGS.DIALOG_EDIT_TITLE : TEXT.ADMIN.TAGS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.TAGS.DIALOG_EDIT_DESC : TEXT.ADMIN.TAGS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.FIELDS.NAME}</Label>
            <Input placeholder="VD: Quy Hoạch Động" value={name} onChange={(e) => setName(e.target.value)} />
          </div>

          {elements.length > 0 && (
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.FIELDS.ELEMENT}</Label>
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
            </div>
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
            {isSaving && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
