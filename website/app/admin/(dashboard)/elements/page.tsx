"use client";

/**
 * Admin elements management — paginated list with search, create, edit, delete.
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
import type { ElementResponse } from "@/types/cultivation";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;

export default function AdminElementsPage() {
  const [data, setData] = useState<ElementResponse[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [isLoading, setIsLoading] = useState(true);

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<ElementResponse | null>(null);
  const [formName, setFormName] = useState("");
  const [formCode, setFormCode] = useState("");
  const [formDesc, setFormDesc] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const fetchData = useCallback(async (p: number, s: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE } };
      if (s.trim()) query.filters = [{ key: "name", value: s.trim(), type: "search" }];
      const res = await adminService.findElements(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search); }, [page, search, fetchData]);

  function openCreate() {
    setEditing(null); setFormName(""); setFormCode(""); setFormDesc("");
    setDialogOpen(true);
  }
  function openEdit(e: ElementResponse) {
    setEditing(e); setFormName(e.name); setFormCode(e.code); setFormDesc(e.description ?? "");
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!formName.trim() || !formCode.trim()) return;
    setSaving(true);
    try {
      if (editing) {
        await adminService.updateElement(editing.id, { name: formName, code: formCode, description: formDesc || undefined });
        notify.success("Cập nhật thành công");
      } else {
        await adminService.createElement({ name: formName, code: formCode, description: formDesc || undefined });
        notify.success("Tạo thành công");
      }
      setDialogOpen(false);
      fetchData(page, search);
    } catch (err) { notify.error(getErrorMessage(err, "Thao tác thất bại")); }
    finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try {
      await adminService.deleteElement(deleteId);
      notify.success("Xoá thành công");
      fetchData(page, search);
    } catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  const columns: AdminColumn<ElementResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (e) => <span className="text-xs tabular-nums text-muted-foreground">{e.id}</span> },
    { key: "name", label: "Tên", render: (e) => <span className="font-medium">{e.name}</span> },
    { key: "code", label: "Code", className: "w-28",
      render: (e) => <span className="font-mono text-xs rounded bg-muted px-2 py-0.5 text-muted-foreground">{e.code}</span> },
    { key: "description", label: "Mô Tả",
      render: (e) => <span className="text-sm text-muted-foreground">{e.description ?? "—"}</span> },
    { key: "actions", label: "", className: "w-20",
      render: (e) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => openEdit(e)}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => setDeleteId(e.id)}>
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
          <h1 className="text-2xl font-bold tracking-tight">Nguyên Tố</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý các nguyên tố tu luyện</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={openCreate}><Plus className="h-4 w-4" />Tạo Nguyên Tố</Button>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input placeholder="Tìm theo tên..." className="pl-9" value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(e) => e.id}
        emptyMessage="Chưa có nguyên tố nào" pagination={pagination} onPageChange={setPage} />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? "Chỉnh Sửa Nguyên Tố" : "Tạo Nguyên Tố Mới"}</DialogTitle>
            <DialogDescription>{editing ? "Cập nhật thông tin nguyên tố." : "Thêm nguyên tố mới vào hệ thống."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>Tên</Label>
              <Input placeholder="VD: Hỏa" value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Code</Label>
              <Input placeholder="VD: fire" value={formCode} onChange={(e) => setFormCode(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Mô tả</Label>
              <Input placeholder="Mô tả nguyên tố..." value={formDesc} onChange={(e) => setFormDesc(e.target.value)} />
            </div>
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
