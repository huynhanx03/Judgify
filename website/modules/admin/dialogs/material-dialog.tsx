"use client";

/**
 * Create / edit dialog for Material articles.
 * Form fields: title, description, content (markdown), category, difficulty,
 * tags, status, visibility.
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
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import type { MaterialArticle, MaterialCategory } from "@/types/material";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Tag } from "@/types/tag";

interface MaterialDialogProps {
  open: boolean;
  editing: MaterialArticle | null;
  onSave: (input: Record<string, unknown>) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
  categories: MaterialCategory[];
  difficulties: DifficultyResponse[];
  tags: Tag[];
}

export function MaterialDialog({
  open, editing, onSave, onClose, isSaving,
  categories, difficulties, tags,
}: MaterialDialogProps) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [content, setContent] = useState("");
  const [categoryId, setCategoryId] = useState<number>(0);
  const [difficultyId, setDifficultyId] = useState<number>(0);
  const [selectedTagIds, setSelectedTagIds] = useState<number[]>([]);
  const [status, setStatus] = useState<string>("draft");
  const [visibility, setVisibility] = useState<string>("public");

  useEffect(() => {
    if (editing) {
      setTitle(editing.title);
      setDescription(editing.description ?? "");
      setContent(editing.content ?? "");
      setCategoryId(editing.category?.id ?? 0);
      setDifficultyId(editing.difficulty?.id ?? 0);
      setSelectedTagIds(editing.tags?.map((t) => t.id) ?? []);
      setStatus(editing.status);
      setVisibility(editing.visibility);
    } else {
      setTitle("");
      setDescription("");
      setContent("");
      setCategoryId(0);
      setDifficultyId(0);
      setSelectedTagIds([]);
      setStatus("draft");
      setVisibility("public");
    }
  }, [editing, open]);

  function toggleTag(id: number) {
    setSelectedTagIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  }

  function handleSubmit() {
    if (!title.trim() || !categoryId || !difficultyId) return;
    onSave({
      title: title.trim(),
      description: description.trim() || undefined,
      content: content || undefined,
      category_id: categoryId,
      difficulty_id: difficultyId,
      tag_ids: selectedTagIds.length ? selectedTagIds : undefined,
      status,
      visibility,
    });
  }

  const isValid = title.trim() && categoryId > 0 && difficultyId > 0;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.MATERIALS.DIALOG_EDIT_TITLE : TEXT.ADMIN.MATERIALS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.MATERIALS.DIALOG_EDIT_DESC : TEXT.ADMIN.MATERIALS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.MATERIALS.FORM_TITLE}</Label>
            <Input
              placeholder={TEXT.ADMIN.MATERIALS.FORM_TITLE_PLACEHOLDER}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label>{TEXT.ADMIN.MATERIALS.FORM_DESCRIPTION}</Label>
            <Input
              placeholder={TEXT.ADMIN.MATERIALS.FORM_DESCRIPTION_PLACEHOLDER}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.MATERIALS.FORM_CATEGORY}</Label>
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={categoryId}
                onChange={(e) => setCategoryId(Number(e.target.value))}
              >
                <option value={0}>— Chọn danh mục —</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.MATERIALS.FORM_DIFFICULTY}</Label>
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={difficultyId}
                onChange={(e) => setDifficultyId(Number(e.target.value))}
              >
                <option value={0}>— Chọn độ khó —</option>
                {difficulties.map((d) => (
                  <option key={d.id} value={d.id}>{d.name}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="space-y-2">
            <Label>{TEXT.ADMIN.MATERIALS.FORM_TAGS}</Label>
            <div className="flex flex-wrap gap-2">
              {tags.map((tag) => (
                <label key={tag.id} className="flex items-center gap-1.5 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    checked={selectedTagIds.includes(tag.id)}
                    onChange={() => toggleTag(tag.id)}
                    className="accent-primary"
                  />
                  {tag.name}
                </label>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.MATERIALS.FORM_STATUS}</Label>
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={status}
                onChange={(e) => setStatus(e.target.value)}
              >
                <option value="draft">{TEXT.ADMIN.MATERIALS.STATUS_DRAFT}</option>
                <option value="published">{TEXT.ADMIN.MATERIALS.STATUS_PUBLISHED}</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.MATERIALS.FORM_VISIBILITY}</Label>
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={visibility}
                onChange={(e) => setVisibility(e.target.value)}
              >
                <option value="public">{TEXT.ADMIN.MATERIALS.VISIBILITY_PUBLIC}</option>
                <option value="group">{TEXT.ADMIN.MATERIALS.VISIBILITY_GROUP}</option>
              </select>
            </div>
          </div>

          <div className="space-y-2">
            <Label>{TEXT.ADMIN.MATERIALS.FORM_CONTENT}</Label>
            <Textarea
              placeholder={TEXT.ADMIN.MATERIALS.FORM_CONTENT_PLACEHOLDER}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              rows={12}
              className="font-mono text-sm"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" className="cursor-pointer" onClick={onClose} disabled={isSaving}>
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button className="cursor-pointer" disabled={!isValid || isSaving} onClick={handleSubmit}>
            {isSaving && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
