"use client";

/**
 * Admin tags management — paginated list with search, element filter, CRUD with element association.
 * Element data for display comes from tag.elements (BE returns nested).
 * Element list for filter/form selector loaded once from findElements.
 */

import { useEffect, useState, useCallback } from "react";
import { Loader2, Plus, Pencil, Trash2, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { adminService } from "@/services/admin.service";
import { notify, getErrorMessage } from "@/lib/toast";
import { ELEMENT_DISPLAY } from "@/types/cultivation";
import type { ElementResponse } from "@/types/cultivation";
import type { Tag, TagElement } from "@/types/tag";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;

export default function AdminTagsPage() {
  const [data, setData] = useState<Tag[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [filterElement, setFilterElement] = useState("");
  // All elements for filter dropdown and form selector
  const [allElements, setAllElements] = useState<ElementResponse[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Tag | null>(null);
  const [formName, setFormName] = useState("");
  const [formElementIds, setFormElementIds] = useState<number[]>([]);
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  useEffect(() => {
    adminService.getAllElements()
      .then(setAllElements)
      .catch(() => {});
  }, []);

  const fetchData = useCallback(async (p: number, s: string, elemId: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE }, filters: [] };
      if (s.trim()) query.filters!.push({ key: "name", value: s.trim(), type: "search" });
      if (elemId) query.filters!.push({ key: "element_id", value: Number(elemId), type: "exact" });
      const res = await adminService.findTags(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search, filterElement); }, [page, search, filterElement, fetchData]);

  function openCreate() {
    setEditing(null); setFormName(""); setFormElementIds([]);
    setDialogOpen(true);
  }
  function openEdit(t: Tag) {
    setEditing(t); setFormName(t.name);
    setFormElementIds(t.elements?.map((e) => e.id) ?? []);
    setDialogOpen(true);
  }

  function toggleElement(id: number) {
    setFormElementIds((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);
  }

  async function handleSave() {
    if (!formName.trim()) return;
    setSaving(true);
    try {
      const payload = { name: formName, element_ids: formElementIds };
      if (editing) { await adminService.updateTag(editing.id, payload); notify.success("Cập nhật thành công"); }
      else { await adminService.createTag(payload); notify.success("Tạo thành công"); }
      setDialogOpen(false); fetchData(page, search, filterElement);
    } catch (err) { notify.error(getErrorMessage(err, "Thao tác thất bại")); }
    finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try { await adminService.deleteTag(deleteId); notify.success("Xoá thành công"); fetchData(page, search, filterElement); }
    catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  /** Render element badges from tag.elements (BE nested data, no extra API call). */
  function renderElementBadges(elements?: TagElement[]) {
    if (!elements?.length) return <span className="text-xs text-muted-foreground">—</span>;
    return (
      <div className="flex gap-1 flex-wrap">
        {elements.map((el) => {
          const display = ELEMENT_DISPLAY[el.code];
          return (
            <span key={el.id} className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold ${display?.color ?? ""} ${display?.bgColor ?? "bg-muted"} ${display?.borderColor ?? "border-border"}`}>
              {display?.icon} {display?.name ?? el.name}
            </span>
          );
        })}
      </div>
    );
  }

  const columns: AdminColumn<Tag>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (t) => <span className="text-xs tabular-nums text-muted-foreground">{t.id}</span> },
    { key: "name", label: "Tên Tag", render: (t) => <span className="font-medium">{t.name}</span> },
    { key: "elements", label: "Nguyên Tố",
      render: (t) => renderElementBadges(t.elements) },
    { key: "actions", label: "", className: "w-20",
      render: (t) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => openEdit(t)}><Pencil className="h-3.5 w-3.5" /></Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => setDeleteId(t.id)}><Trash2 className="h-3.5 w-3.5" /></Button>
        </div>
      ) },
  ];

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center"><Loader2 className="h-8 w-8 text-primary animate-spin" /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Tags</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý thẻ phân loại bài tập</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={openCreate}><Plus className="h-4 w-4" />Tạo Tag</Button>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input placeholder="Tìm theo tên..." className="pl-9" value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
        </div>
        <select value={filterElement} onChange={(e) => { setFilterElement(e.target.value); setPage(1); }}
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm">
          <option value="">Tất cả nguyên tố</option>
          {allElements.map((el) => <option key={el.id} value={el.id}>{ELEMENT_DISPLAY[el.code]?.icon} {el.name}</option>)}
        </select>
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(t) => t.id}
        emptyMessage="Chưa có tag nào" pagination={pagination} onPageChange={setPage} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? "Chỉnh Sửa Tag" : "Tạo Tag Mới"}</DialogTitle>
            <DialogDescription>{editing ? "Cập nhật thông tin tag." : "Thêm tag mới vào hệ thống."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2"><Label>Tên</Label><Input placeholder="VD: Array" value={formName} onChange={(e) => setFormName(e.target.value)} /></div>
            <div className="space-y-2">
              <Label>Nguyên Tố</Label>
              <div className="flex flex-wrap gap-2">
                {allElements.map((el) => {
                  const active = formElementIds.includes(el.id);
                  const display = ELEMENT_DISPLAY[el.code];
                  return (
                    <button key={el.id} type="button" onClick={() => toggleElement(el.id)}
                      className={`inline-flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-semibold transition-colors cursor-pointer ${active ? `${display?.color ?? ""} ${display?.bgColor ?? "bg-primary/10"} ${display?.borderColor ?? "border-primary"}` : "text-muted-foreground border-border hover:border-foreground/30"}`}>
                      {display?.icon} {display?.name ?? el.name}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" className="cursor-pointer" onClick={() => setDialogOpen(false)}>Hủy</Button>
            <Button className="cursor-pointer" disabled={!formName.trim() || saving} onClick={handleSave}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}{editing ? "Lưu" : "Tạo"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }} onConfirm={handleDelete} />
    </div>
  );
}
