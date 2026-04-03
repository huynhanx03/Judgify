"use client";

/**
 * Admin problems list — paginated table with search, difficulty filter, delete.
 * Create/Edit navigates to dedicated pages.
 */

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Plus, Pencil, Trash2, Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { adminService } from "@/services/admin.service";
import { notify, getErrorMessage } from "@/lib/toast";
import type { Problem } from "@/types/problem";
import type { DifficultyResponse } from "@/types/difficulty";
import type { PaginationMeta, QueryOptions } from "@/types/api";

const PAGE_SIZE = 10;

export default function AdminProblemsPage() {
  const router = useRouter();
  const [data, setData] = useState<Problem[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [filterDifficulty, setFilterDifficulty] = useState("");
  const [difficulties, setDifficulties] = useState<DifficultyResponse[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  useEffect(() => {
    adminService.getAllDifficulties()
      .then(setDifficulties)
      .catch(() => {});
  }, []);

  const fetchData = useCallback(async (p: number, s: string, diffId: string) => {
    try {
      const query: QueryOptions = { pagination: { page: p, page_size: PAGE_SIZE }, filters: [] };
      if (s.trim()) query.filters!.push({ key: "title", value: s.trim(), type: "search" });
      if (diffId) query.filters!.push({ key: "difficulty_id", value: Number(diffId), type: "exact" });
      const res = await adminService.findProblems(query);
      setData(res.records);
      setPagination(res.pagination);
    } catch { notify.error("Không thể tải dữ liệu"); }
    finally { setIsLoading(false); }
  }, []);

  useEffect(() => { fetchData(page, search, filterDifficulty); }, [page, search, filterDifficulty, fetchData]);

  async function handleDelete() {
    if (!deleteId) return;
    try { await adminService.deleteProblem(deleteId); notify.success("Xoá thành công"); fetchData(page, search, filterDifficulty); }
    catch (err) { notify.error(getErrorMessage(err, "Không thể xoá")); }
    finally { setDeleteId(null); }
  }

  const columns: AdminColumn<Problem>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (p) => <span className="text-xs tabular-nums text-muted-foreground">{p.id}</span> },
    { key: "title", label: "Tiêu Đề", className: "max-w-[300px]",
      render: (p) => <p className="font-medium truncate" title={p.title}>{p.title}</p> },
    { key: "tags", label: "Tags", className: "max-w-[200px]",
      render: (p) => {
        if (!p.tags?.length) return <span className="text-xs text-muted-foreground">—</span>;
        return (
          <div className="flex gap-1 flex-wrap">
            {p.tags.slice(0, 3).map((t) => (
              <span key={t.id} className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">{t.name}</span>
            ))}
            {p.tags.length > 3 && (
              <span className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">+{p.tags.length - 3}</span>
            )}
          </div>
        );
      } },
    { key: "difficulty", label: "Độ Khó", className: "w-28",
      render: (p) => <Badge variant="outline" className="text-[11px] border">{p.difficulty?.name ?? "N/A"}</Badge> },
    { key: "published", label: "Trạng Thái", className: "w-28",
      render: (p) => <Badge variant={p.is_published ? "default" : "outline"} className="text-[11px]">{p.is_published ? "Published" : "Draft"}</Badge> },
    { key: "limits", label: "Giới Hạn", className: "w-36",
      render: (p) => (
        <div className="flex gap-2 text-xs text-muted-foreground">
          <span title="Time limit">{p.time_limit_ms >= 1000 ? `${(p.time_limit_ms / 1000).toFixed(1)}s` : `${p.time_limit_ms}ms`}</span>
          <span className="text-muted-foreground/40">|</span>
          <span title="Memory limit">{p.memory_limit_kb >= 1024 ? `${(p.memory_limit_kb / 1024).toFixed(0)}MB` : `${p.memory_limit_kb}KB`}</span>
        </div>
      ) },
    { key: "actions", label: "", className: "w-20",
      render: (p) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer"
            onClick={() => router.push(`/admin/problems/${p.id}/edit`)}><Pencil className="h-3.5 w-3.5" /></Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer"
            onClick={() => setDeleteId(p.id)}><Trash2 className="h-3.5 w-3.5" /></Button>
        </div>
      ) },
  ];

  if (isLoading) return <div className="flex h-[50vh] items-center justify-center"><Loader2 className="h-8 w-8 text-primary animate-spin" /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Bài Tập</h1>
          <p className="text-sm text-muted-foreground mt-1">Quản lý bài tập và test cases</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={() => router.push("/admin/problems/create")}><Plus className="h-4 w-4" />Tạo Bài Tập</Button>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input placeholder="Tìm theo tiêu đề..." className="pl-9" value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
        </div>
        <select value={filterDifficulty} onChange={(e) => { setFilterDifficulty(e.target.value); setPage(1); }}
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm">
          <option value="">Tất cả độ khó</option>
          {difficulties.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </select>
      </div>

      <AdminDataTable columns={columns} data={data} keyExtractor={(p) => p.id}
        emptyMessage="Chưa có bài tập nào" pagination={pagination} onPageChange={setPage} />

      <ConfirmDialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }} onConfirm={handleDelete} />
    </div>
  );
}
