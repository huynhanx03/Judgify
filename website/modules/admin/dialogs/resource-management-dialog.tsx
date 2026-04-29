"use client";

/**
 * Resource management dialog — add, edit, delete resources with search.
 */

import { useState } from "react";
import { Plus, Pencil, Save, Search, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { notify, getErrorMessage } from "@/lib/toast";
import { roleService } from "@/services/role.service";
import { TEXT } from "@/constants/text";
import type { Resource } from "@/types/admin";

interface ResourceManagementDialogProps {
  open: boolean;
  onClose: () => void;
  resources: Resource[];
  onResourcesChange: (resources: Resource[]) => void;
  onResourceDeleted?: (id: number) => void;
}

export function ResourceManagementDialog({
  open,
  onClose,
  resources,
  onResourcesChange,
  onResourceDeleted,
}: ResourceManagementDialogProps) {
  const [newKey, setNewKey] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [search, setSearch] = useState("");
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editKey, setEditKey] = useState("");
  const [editDesc, setEditDesc] = useState("");

  async function handleAdd() {
    if (!newKey.trim()) return;
    try {
      const created = await roleService.createResource({
        key: newKey.trim().toLowerCase().replace(/\s+/g, "_"),
        description: newDesc.trim() || undefined,
      });
      onResourcesChange([...resources, created]);
      setNewKey("");
      setNewDesc("");
      notify.success(TEXT.ADMIN.RESOURCES_CREATE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.RESOURCES_CREATE_ERROR));
    }
  }

  function startEdit(res: Resource) {
    setEditingId(res.id);
    setEditKey(res.key);
    setEditDesc(res.description ?? "");
  }

  async function saveEdit() {
    if (!editingId || !editKey.trim()) return;
    try {
      const updated = await roleService.updateResource(editingId, {
        key: editKey.trim(),
        description: editDesc.trim() || undefined,
      });
      onResourcesChange(resources.map((r) => (r.id === editingId ? updated : r)));
      setEditingId(null);
      notify.success(TEXT.ADMIN.RESOURCES_UPDATE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.RESOURCES_UPDATE_ERROR));
    }
  }

  async function handleRemove(id: number) {
    try {
      await roleService.deleteResource(id);
      onResourcesChange(resources.filter((r) => r.id !== id));
      onResourceDeleted?.(id);
      notify.success(TEXT.ADMIN.RESOURCES_DELETE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.RESOURCES_DELETE_ERROR));
    }
  }

  const filtered = search
    ? resources.filter(
        (r) =>
          r.key.includes(search.toLowerCase()) ||
          (r.description ?? "").toLowerCase().includes(search.toLowerCase())
      )
    : resources;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{TEXT.ADMIN.RESOURCES_DIALOG_TITLE}</DialogTitle>
          <DialogDescription>{TEXT.ADMIN.RESOURCES_DIALOG_DESC}</DialogDescription>
        </DialogHeader>

        {/* Add new resource */}
        <div className="flex items-end gap-2">
          <div className="flex-1 space-y-1.5">
            <Label className="text-xs text-muted-foreground">{TEXT.ADMIN.RESOURCES_KEY_LABEL}</Label>
            <Input
              placeholder={TEXT.ADMIN.RESOURCES_KEY_PLACEHOLDER}
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label className="text-xs text-muted-foreground">{TEXT.ADMIN.RESOURCES_DESC_LABEL}</Label>
            <Input
              placeholder={TEXT.ADMIN.RESOURCES_DESC_PLACEHOLDER}
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
            />
          </div>
          <Button
            onClick={handleAdd}
            disabled={!newKey.trim()}
            className="gap-2 cursor-pointer shrink-0"
          >
            <Plus className="h-4 w-4" />
            {TEXT.ADMIN.RESOURCES_ADD}
          </Button>
        </div>

        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={TEXT.ADMIN.RESOURCES_SEARCH_PLACEHOLDER}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        {/* Resource table */}
        <div className="rounded-lg border border-border overflow-hidden">
          <div className="grid grid-cols-[1fr_1.5fr_72px] bg-muted/30 border-b border-border px-4 py-2">
            <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ADMIN.RESOURCES_COL_KEY}</span>
            <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ADMIN.RESOURCES_COL_DESC}</span>
            <span />
          </div>

          <div className="max-h-[320px] overflow-y-auto divide-y divide-border">
            {filtered.map((res) => (
              <div key={res.id} className="grid grid-cols-[1fr_1.5fr_72px] items-center px-4 py-2.5 hover:bg-muted/30 transition-colors">
                {editingId === res.id ? (
                  <>
                    <Input value={editKey} onChange={(e) => setEditKey(e.target.value)} className="h-8 text-sm font-mono" />
                    <Input value={editDesc} onChange={(e) => setEditDesc(e.target.value)} className="h-8 text-sm" />
                    <div className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="icon" className="h-7 w-7 text-primary cursor-pointer" onClick={saveEdit}>
                        <Save className="h-3.5 w-3.5" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground cursor-pointer" onClick={() => setEditingId(null)}>
                        ✕
                      </Button>
                    </div>
                  </>
                ) : (
                  <>
                    <span className="text-sm font-medium font-mono">{res.key}</span>
                    <span className="text-sm text-muted-foreground">{res.description}</span>
                    <div className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => startEdit(res)}>
                        <Pencil className="h-3 w-3" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => handleRemove(res.id)}>
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  </>
                )}
              </div>
            ))}

            {filtered.length === 0 && (
              <div className="px-4 py-8 text-center text-sm text-muted-foreground">
                {TEXT.ADMIN.RESOURCES_EMPTY}
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
