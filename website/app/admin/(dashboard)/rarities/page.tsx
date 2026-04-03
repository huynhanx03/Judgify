"use client";

/**
 * Admin rarities management — paginated list with search, create, edit, delete.
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
import type { RarityResponse } from "@/types/cultivation";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;

export default function AdminRaritiesPage() {
  const [data, setData] = useState<RarityResponse[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [isLoading, setIsLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<RarityResponse | null>(null);
  const [formName, setFormName] = useState("");
  const [formCode, setFormCode] = useState("");
  const [formWeight, setFormWeight] = useState(100);
  const [formDesc, setFormDesc] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const fetchData = useCallback(async (p: number, s: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE } };
      if (s.trim()) query.filters = [{ key: "name", value: s.trim(), type: "search" }];
      const res = await adminService.findRarities(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search); }, [page, search, fetchData]);

  function openCreate() {
    setEditing(null); setFormName(""); setFormCode(""); setFormWeight(100); setFormDesc("");
    setDialogOpen(true);
  }
  function openEdit(r: RarityResponse) {
    setEditing(r); setFormName(r.name); setFormCode(r.code); setFormWeight(r.weight); setFormDesc(r.description ?? "");
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!formName.trim() || !formCode.trim()) return;
    setSaving(true);
    try {
      const payload = { name: formName, code: formCode, weight: formWeight, description: formDesc || undefined };
      if (editing) { await adminService.updateRarity(editing.id, payload); notify.success("Cập nhật thành công"); }
      else { await adminService.createRarity(payload); notify.success("Tạo thành công"); }
      setDialogOpen(false); fetchData(page, search);
    } catch (err) { notify.error(getErrorMessage(err, "Thao tác thất bại")); }
    finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try { await adminService.deleteRarity(deleteId); notify.success("Xoá thành công"); fetchData(page, search); }
    catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  const columns: AdminColumn<RarityResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: "Tên", render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "code", label: "Code", className: "w-32",
      render: (r) => <span className="font-mono text-xs rounded bg-muted px-2 py-0.5 text-muted-foreground">{r.code}</span> },
    { key: "weight", label: "Trọng Số", className: "w-24",
      render: (r) => <span className="text-sm tabular-nums">{r.weight}</span> },
    { key: "desc", label: "Mô Tả",
      render: (r) => <span className="text-sm text-muted-foreground">{r.description ?? "—"}</span> },
    { key: "actions", label: "", className: "w-20",
      render: (r) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => openEdit(r)}><Pencil className="h-3.5 w-3.5" /></Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => setDeleteId(r.id)}><Trash2 className="h-3.5 w-3.5" /></Button>
        </div>
      ) },
  ];

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center"><Loader2 className="h-8 w-8 text-primary animate-spin" /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Độ Hiếm</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý các cấp độ hiếm trong hệ thống</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={openCreate}><Plus className="h-4 w-4" />Tạo Độ Hiếm</Button>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input placeholder="Tìm theo tên..." className="pl-9" value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(r) => r.id}
        emptyMessage="Chưa có độ hiếm nào" pagination={pagination} onPageChange={setPage} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? "Chỉnh Sửa Độ Hiếm" : "Tạo Độ Hiếm Mới"}</DialogTitle>
            <DialogDescription>{editing ? "Cập nhật thông tin độ hiếm." : "Thêm độ hiếm mới vào hệ thống."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2"><Label>Tên</Label><Input placeholder="VD: Phàm Phẩm" value={formName} onChange={(e) => setFormName(e.target.value)} /></div>
            <div className="space-y-2"><Label>Code</Label><Input placeholder="VD: common" value={formCode} onChange={(e) => setFormCode(e.target.value)} /></div>
            <div className="space-y-2"><Label>Trọng số (weight)</Label><Input type="number" placeholder="100" value={formWeight} onChange={(e) => setFormWeight(Number(e.target.value))} /></div>
            <div className="space-y-2"><Label>Mô tả</Label><Input placeholder="Mô tả..." value={formDesc} onChange={(e) => setFormDesc(e.target.value)} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" className="cursor-pointer" onClick={() => setDialogOpen(false)}>Hủy</Button>
            <Button className="cursor-pointer" disabled={!formName.trim() || !formCode.trim() || saving} onClick={handleSave}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}{editing ? "Lưu" : "Tạo"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }} onConfirm={handleDelete} />
    </div>
  );
}
