"use client";

/**
 * Create / edit dialog for Material articles.
 * Form fields: title, description, content (markdown), category, difficulty,
 * tags, status, visibility.
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
import { tryEntityID, type EntityID } from "@/lib/api/contracts";
import type {
  CreateMaterialRequest,
  MaterialArticle,
  MaterialCategory,
  UpdateMaterialRequest,
} from "@/types/material";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Tag } from "@/types/tag";

interface MaterialDialogProps {
  open: boolean;
  editing: MaterialArticle | null;
  onSave: (
    input: CreateMaterialRequest | UpdateMaterialRequest,
  ) => Promise<void>;
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
  const [title, setTitle] = useState(editing?.title ?? "");
  const [slug, setSlug] = useState(editing?.slug ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");
  const [content, setContent] = useState(editing?.content ?? "");
  const [categoryId, setCategoryId] = useState<EntityID | "">(
    editing?.category?.id ?? "",
  );
  const [difficultyId, setDifficultyId] = useState<EntityID | "">(
    editing?.difficulty?.id ?? "",
  );
  const [selectedTagIds, setSelectedTagIds] = useState<EntityID[]>(
    editing?.tags?.map((tag) => tag.id) ?? [],
  );
  const [changeSummary, setChangeSummary] = useState("");
  const [visibility, setVisibility] = useState<MaterialArticle["visibility"]>(
    editing?.visibility ?? "public",
  );

  function toggleTag(id: EntityID) {
    setSelectedTagIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  }

  function handleSubmit() {
    if (
      !title.trim() ||
      !slug.trim() ||
      !description.trim() ||
      !content.trim() ||
      !categoryId ||
      !difficultyId ||
      changeSummary.trim().length < 3
    ) return;
    const common = {
      title: title.trim(),
      description: description.trim(),
      content,
      category_id: categoryId,
      difficulty_id: difficultyId,
      tag_ids: selectedTagIds,
      visibility,
      change_summary: changeSummary.trim(),
    };
    onSave(
      editing
        ? { ...common, expected_version: editing.version }
        : { ...common, slug: slug.trim() },
    );
  }

  const isValid = Boolean(
    title.trim() &&
      slug.trim() &&
      description.trim() &&
      content.trim() &&
      categoryId &&
      difficultyId &&
      changeSummary.trim().length >= 3,
  );

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {editing ? ADMIN_TEXT.MATERIALS.DIALOG_EDIT_TITLE : ADMIN_TEXT.MATERIALS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? ADMIN_TEXT.MATERIALS.DIALOG_EDIT_DESC : ADMIN_TEXT.MATERIALS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="material-title">{ADMIN_TEXT.MATERIALS.FORM_TITLE}</Label>
            <Input
              id="material-title"
              placeholder={ADMIN_TEXT.MATERIALS.FORM_TITLE_PLACEHOLDER}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="material-slug">{ADMIN_TEXT.MATERIALS.FORM_SLUG}</Label>
            <Input
              id="material-slug"
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              placeholder={ADMIN_TEXT.MATERIALS.FORM_SLUG_PLACEHOLDER}
              disabled={Boolean(editing)}
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="material-description">{ADMIN_TEXT.MATERIALS.FORM_DESCRIPTION}</Label>
            <Input
              id="material-description"
              placeholder={ADMIN_TEXT.MATERIALS.FORM_DESCRIPTION_PLACEHOLDER}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="material-category">{ADMIN_TEXT.MATERIALS.FORM_CATEGORY}</Label>
              <select
                id="material-category"
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={categoryId}
                onChange={(event) =>
                  setCategoryId(tryEntityID(event.target.value) ?? "")
                }
              >
                <option value="">{ADMIN_TEXT.SELECT_CATEGORY}</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="material-difficulty">{ADMIN_TEXT.MATERIALS.FORM_DIFFICULTY}</Label>
              <select
                id="material-difficulty"
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={difficultyId}
                onChange={(event) =>
                  setDifficultyId(tryEntityID(event.target.value) ?? "")
                }
              >
                <option value="">{ADMIN_TEXT.SELECT_DIFFICULTY}</option>
                {difficulties.map((d) => (
                  <option key={d.id} value={d.id}>{d.name}</option>
                ))}
              </select>
            </div>
          </div>

          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-foreground">
              {ADMIN_TEXT.MATERIALS.FORM_TAGS}
            </legend>
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
          </fieldset>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="material-visibility">{ADMIN_TEXT.MATERIALS.FORM_VISIBILITY}</Label>
              <select
                id="material-visibility"
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={visibility}
                onChange={(event) =>
                  setVisibility(event.target.value as MaterialArticle["visibility"])
                }
              >
                <option value="public">{ADMIN_TEXT.MATERIALS.VISIBILITY_PUBLIC}</option>
                <option value="authenticated">{ADMIN_TEXT.MATERIALS.VISIBILITY_AUTHENTICATED}</option>
              </select>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="material-content">{ADMIN_TEXT.MATERIALS.FORM_CONTENT}</Label>
            <Textarea
              id="material-content"
              placeholder={ADMIN_TEXT.MATERIALS.FORM_CONTENT_PLACEHOLDER}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              rows={12}
              className="font-mono text-sm"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="material-change-summary">
              {ADMIN_TEXT.MATERIALS.FORM_CHANGE_SUMMARY}
            </Label>
            <Textarea
              id="material-change-summary"
              value={changeSummary}
              onChange={(event) => setChangeSummary(event.target.value)}
              placeholder={ADMIN_TEXT.MATERIALS.FORM_CHANGE_SUMMARY_PLACEHOLDER}
              maxLength={500}
              aria-describedby="material-change-summary-hint"
              required
            />
            <p id="material-change-summary-hint" className="text-xs text-muted-foreground">
              {ADMIN_TEXT.MATERIALS.FORM_CHANGE_SUMMARY_HINT}
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" className="cursor-pointer" onClick={onClose} disabled={isSaving}>
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button className="cursor-pointer" disabled={!isValid || isSaving} onClick={handleSubmit}>
            {isSaving && <Loader2 className="h-4 w-4 animate-spin mr-2 motion-reduce:animate-none" aria-hidden="true" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
