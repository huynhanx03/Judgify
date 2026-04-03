"use client";

/**
 * Admin traits management — paginated list with search, create, edit, delete.
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
import { RARITY_DISPLAY, type TraitResponse, type RarityResponse } from "@/types/cultivation";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;
const TYPE_DISPLAY: Record<string, { label: string; className: string }> = {
  root_bone: { label: "Căn Cốt", className: "text-amber-400 bg-amber-400/10 border-amber-400/40" },
  talent: { label: "Thiên Phú", className: "text-purple-400 bg-purple-400/10 border-purple-400/40" },
};

export default function AdminTraitsPage() {
  const [data, setData] = useState<TraitResponse[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState("");
  const [filterRarity, setFilterRarity] = useState("");
  const [allRarities, setAllRarities] = useState<RarityResponse[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    adminService.getAllRarities()
      .then(setAllRarities)
      .catch(() => {});
  }, []);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<TraitResponse | null>(null);
  const [formType, setFormType] = useState("root_bone");
  const [formName, setFormName] = useState("");
  const [formRarity, setFormRarity] = useState(1);
  const [formDesc, setFormDesc] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const fetchData = useCallback(async (p: number, s: string, type: string, rarity: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE }, filters: [] };
      if (s.trim()) query.filters!.push({ key: "name", value: s.trim(), type: "search" });
      if (type) query.filters!.push({ key: "type", value: type, type: "exact" });
      if (rarity) query.filters!.push({ key: "rarity_id", value: Number(rarity), type: "exact" });
      const res = await adminService.findTraits(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search, filterType, filterRarity); }, [page, search, filterType, filterRarity, fetchData]);

  function openCreate() {
    setEditing(null); setFormType("root_bone"); setFormName(""); setFormRarity(1); setFormDesc("");
    setDialogOpen(true);
  }
  function openEdit(t: TraitResponse) {
    setEditing(t); setFormType(t.type); setFormName(t.name); setFormRarity(t.rarity?.id ?? 1); setFormDesc(t.description ?? "");
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!formName.trim()) return;
    setSaving(true);
    try {
      const payload = { type: formType, name: formName, rarity_id: formRarity, description: formDesc || undefined };
      if (editing) { await adminService.updateTrait(editing.id, payload); notify.success("Cập nhật thành công"); }
      else { await adminService.createTrait(payload); notify.success("Tạo thành công"); }
      setDialogOpen(false); fetchData(page, search, filterType, filterRarity);
    } catch (err) { notify.error(getErrorMessage(err, "Thao tác thất bại")); }
    finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try { await adminService.deleteTrait(deleteId); notify.success("Xoá thành công"); fetchData(page, search, filterType, filterRarity); }
    catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  const columns: AdminColumn<TraitResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (t) => <span className="text-xs tabular-nums text-muted-foreground">{t.id}</span> },
    { key: "name", label: "Tên", render: (t) => <span className="font-medium">{t.name}</span> },
    { key: "type", label: "Loại", className: "w-28",
      render: (t) => {
        const d = TYPE_DISPLAY[t.type];
        return <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold ${d?.className ?? ""}`}>{d?.label ?? t.type}</span>;
      } },
    { key: "rarity", label: "Độ Hiếm", className: "w-32",
      render: (t) => {
        const r = t.rarity ? RARITY_DISPLAY[t.rarity.code] : undefined;
        if (!r) return <span className="text-xs text-muted-foreground">—</span>;
        return <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold ${r.color} ${r.bgColor} ${r.borderColor}`}>{r.name}</span>;
      } },
    { key: "desc", label: "Mô Tả",
      render: (t) => <span className="text-sm text-muted-foreground">{t.description ?? "—"}</span> },
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
          <h1 className="text-2xl font-bold tracking-tight">Đặc Tính</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý căn cốt và thiên phú của tu sĩ</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={openCreate}><Plus className="h-4 w-4" />Tạo Đặc Tính</Button>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input placeholder="Tìm theo tên..." className="pl-9" value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
        </div>
        <select value={filterType} onChange={(e) => { setFilterType(e.target.value); setPage(1); }}
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm">
          <option value="">Tất cả loại</option>
          <option value="root_bone">Căn Cốt</option>
          <option value="talent">Thiên Phú</option>
        </select>
        <select value={filterRarity} onChange={(e) => { setFilterRarity(e.target.value); setPage(1); }}
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm">
          <option value="">Tất cả độ hiếm</option>
          {allRarities.map((r) => <option key={r.id} value={r.id}>{RARITY_DISPLAY[r.code]?.name ?? r.name}</option>)}
        </select>
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(t) => t.id}
        emptyMessage="Chưa có đặc tính nào" pagination={pagination} onPageChange={setPage} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? "Chỉnh Sửa Đặc Tính" : "Tạo Đặc Tính Mới"}</DialogTitle>
            <DialogDescription>{editing ? "Cập nhật thông tin đặc tính." : "Thêm đặc tính mới vào hệ thống."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>Loại</Label>
              <select value={formType} onChange={(e) => setFormType(e.target.value)}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm">
                <option value="root_bone">Căn Cốt</option><option value="talent">Thiên Phú</option>
              </select>
            </div>
            <div className="space-y-2"><Label>Tên</Label><Input placeholder="VD: Long Cốt" value={formName} onChange={(e) => setFormName(e.target.value)} /></div>
            <div className="space-y-2">
              <Label>Độ Hiếm</Label>
              <select value={formRarity} onChange={(e) => setFormRarity(Number(e.target.value))}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm">
                {allRarities.map((r) => <option key={r.id} value={r.id}>{RARITY_DISPLAY[r.code]?.name ?? r.name}</option>)}
              </select>
            </div>
            <div className="space-y-2"><Label>Mô tả</Label><Input placeholder="Mô tả..." value={formDesc} onChange={(e) => setFormDesc(e.target.value)} /></div>
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
