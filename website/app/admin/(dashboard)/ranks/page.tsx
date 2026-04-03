"use client";

/**
 * Admin ranks management — paginated list with search, create, edit, delete.
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
import type { RankResponse } from "@/types/cultivation";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;

export default function AdminRanksPage() {
  const [data, setData] = useState<RankResponse[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [isLoading, setIsLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<RankResponse | null>(null);
  const [formName, setFormName] = useState("");
  const [formMinRating, setFormMinRating] = useState(0);
  const [formDesc, setFormDesc] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const fetchData = useCallback(async (p: number, s: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE } };
      if (s.trim()) query.filters = [{ key: "name", value: s.trim(), type: "search" }];
      const res = await adminService.findRanks(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search); }, [page, search, fetchData]);

  function openCreate() {
    setEditing(null); setFormName(""); setFormMinRating(0); setFormDesc("");
    setDialogOpen(true);
  }
  function openEdit(r: RankResponse) {
    setEditing(r); setFormName(r.name); setFormMinRating(r.min_rating); setFormDesc(r.description ?? "");
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!formName.trim()) return;
    setSaving(true);
    try {
      if (editing) {
        await adminService.updateRank(editing.id, { name: formName, min_rating: formMinRating, description: formDesc || undefined });
        notify.success("Cập nhật thành công");
      } else {
        await adminService.createRank({ name: formName, min_rating: formMinRating, description: formDesc || undefined });
        notify.success("Tạo thành công");
      }
      setDialogOpen(false); fetchData(page, search);
    } catch (err) { notify.error(getErrorMessage(err, "Thao tác thất bại")); }
    finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try { await adminService.deleteRank(deleteId); notify.success("Xoá thành công"); fetchData(page, search); }
    catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  const columns: AdminColumn<RankResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: "Tên", render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "min_rating", label: "Rating Tối Thiểu", className: "w-36",
      render: (r) => <span className="text-sm font-mono text-blue-400">{r.min_rating.toLocaleString()}</span> },
    { key: "description", label: "Mô Tả",
      render: (r) => <span className="text-sm text-muted-foreground">{r.description ?? "—"}</span> },
    { key: "actions", label: "", className: "w-20",
      render: (r) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => openEdit(r)}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => setDeleteId(r.id)}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) },
  ];

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center"><Loader2 className="h-8 w-8 text-primary animate-spin" /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Danh Hiệu</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý các danh hiệu xếp hạng</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={openCreate}><Plus className="h-4 w-4" />Tạo Danh Hiệu</Button>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input placeholder="Tìm theo tên..." className="pl-9" value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(r) => r.id}
        emptyMessage="Chưa có danh hiệu nào" pagination={pagination} onPageChange={setPage} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? "Chỉnh Sửa Danh Hiệu" : "Tạo Danh Hiệu Mới"}</DialogTitle>
            <DialogDescription>{editing ? "Cập nhật thông tin danh hiệu." : "Thêm danh hiệu mới vào hệ thống."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2"><Label>Tên</Label><Input placeholder="VD: Bạch Kim" value={formName} onChange={(e) => setFormName(e.target.value)} /></div>
            <div className="space-y-2">
              <Label>Rating Tối Thiểu</Label>
              <Input type="number" min={0} value={formMinRating} onChange={(e) => setFormMinRating(Number(e.target.value))} />
            </div>
            <div className="space-y-2"><Label>Mô tả</Label><Input placeholder="Mô tả danh hiệu..." value={formDesc} onChange={(e) => setFormDesc(e.target.value)} /></div>
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
