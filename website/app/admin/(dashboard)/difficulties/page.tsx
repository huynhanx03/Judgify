"use client";

/**
 * Admin difficulties management — paginated list with search, create, edit, delete.
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
import type { DifficultyResponse } from "@/types/difficulty";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;

export default function AdminDifficultiesPage() {
  const [data, setData] = useState<DifficultyResponse[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [isLoading, setIsLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<DifficultyResponse | null>(null);
  const [formName, setFormName] = useState("");
  const [formLevel, setFormLevel] = useState(1);
  const [formExpReward, setFormExpReward] = useState(0);
  const [formDesc, setFormDesc] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const fetchData = useCallback(async (p: number, s: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE } };
      if (s.trim()) query.filters = [{ key: "name", value: s.trim(), type: "search" }];
      const res = await adminService.findDifficulties(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search); }, [page, search, fetchData]);

  function openCreate() {
    setEditing(null); setFormName(""); setFormLevel(1); setFormExpReward(0); setFormDesc("");
    setDialogOpen(true);
  }
  function openEdit(d: DifficultyResponse) {
    setEditing(d); setFormName(d.name); setFormLevel(d.level); setFormExpReward(d.exp_reward); setFormDesc(d.description ?? "");
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!formName.trim()) return;
    setSaving(true);
    try {
      const payload = { name: formName, level: formLevel, exp_reward: formExpReward, description: formDesc || undefined };
      if (editing) { await adminService.updateDifficulty(editing.id, payload); notify.success("Cập nhật thành công"); }
      else { await adminService.createDifficulty(payload); notify.success("Tạo thành công"); }
      setDialogOpen(false); fetchData(page, search);
    } catch (err) { notify.error(getErrorMessage(err, "Thao tác thất bại")); }
    finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try { await adminService.deleteDifficulty(deleteId); notify.success("Xoá thành công"); fetchData(page, search); }
    catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  const columns: AdminColumn<DifficultyResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (d) => <span className="text-xs tabular-nums text-muted-foreground">{d.id}</span> },
    { key: "name", label: "Tên", render: (d) => <span className="font-medium">{d.name}</span> },
    { key: "level", label: "Cấp Độ", className: "w-24",
      render: (d) => <span className="inline-flex items-center rounded-full border border-border bg-muted/40 px-2 py-0.5 text-xs font-semibold">{d.level}</span> },
    { key: "exp_reward", label: "EXP", className: "w-24",
      render: (d) => <span className="text-sm font-mono text-amber-400">{d.exp_reward.toLocaleString()}</span> },
    { key: "desc", label: "Mô Tả",
      render: (d) => <span className="text-sm text-muted-foreground">{d.description ?? "—"}</span> },
    { key: "actions", label: "", className: "w-20",
      render: (d) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => openEdit(d)}><Pencil className="h-3.5 w-3.5" /></Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => setDeleteId(d.id)}><Trash2 className="h-3.5 w-3.5" /></Button>
        </div>
      ) },
  ];

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center"><Loader2 className="h-8 w-8 text-primary animate-spin" /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Độ Khó</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý các cấp độ khó của bài tập</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={openCreate}><Plus className="h-4 w-4" />Tạo Độ Khó</Button>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input placeholder="Tìm theo tên..." className="pl-9" value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(d) => d.id}
        emptyMessage="Chưa có độ khó nào" pagination={pagination} onPageChange={setPage} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? "Chỉnh Sửa Độ Khó" : "Tạo Độ Khó Mới"}</DialogTitle>
            <DialogDescription>{editing ? "Cập nhật thông tin độ khó." : "Thêm độ khó mới vào hệ thống."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2"><Label>Tên</Label><Input placeholder="VD: Dễ" value={formName} onChange={(e) => setFormName(e.target.value)} /></div>
            <div className="space-y-2"><Label>Cấp độ</Label><Input type="number" placeholder="1" value={formLevel} onChange={(e) => setFormLevel(Number(e.target.value))} /></div>
            <div className="space-y-2"><Label>EXP thưởng</Label><Input type="number" placeholder="100" value={formExpReward} onChange={(e) => setFormExpReward(Number(e.target.value))} /></div>
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
