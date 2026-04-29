"use client";

/**
 * Admin problems page — paginated list with search, difficulty filter, delete.
 * Create/Edit navigate to dedicated pages — no dialog.
 */

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { problemService } from "@/services/problem.service";
import { difficultyService } from "@/services/difficulty.service";
import { TEXT } from "@/constants/text";
import type { Problem } from "@/types/problem";
import type { DifficultyResponse } from "@/types/difficulty";

export default function AdminProblemsPage() {
  const router = useRouter();
  const [difficulties, setDifficulties] = useState<DifficultyResponse[]>([]);

  // Load difficulties for the filter dropdown
  useEffect(() => {
    difficultyService.getAll().then(setDifficulties).catch(() => {});
  }, []);

  // Problems page only uses find + delete — no create/update dialog
  const service = useMemo(() => ({
    find: problemService.find.bind(problemService),
    delete: problemService.delete.bind(problemService),
  }), []);

  const crud = usePaginatedCRUD<Problem>({ service, searchKey: "title" });

  const columns: AdminColumn<Problem>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (p) => <span className="text-xs tabular-nums text-muted-foreground">{p.id}</span> },
    { key: "title", label: TEXT.ADMIN.PROBLEMS.COL_TITLE, className: "max-w-[300px]",
      render: (p) => <p className="font-medium truncate" title={p.title}>{p.title}</p> },
    { key: "tags", label: TEXT.ADMIN.PROBLEMS.COL_TAGS, className: "max-w-[200px]",
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
    { key: "difficulty", label: TEXT.ADMIN.PROBLEMS.COL_DIFFICULTY, className: "w-28",
      render: (p) => <Badge variant="outline" className="text-[11px] border">{p.difficulty?.name ?? "N/A"}</Badge> },
    { key: "published", label: TEXT.ADMIN.PROBLEMS.COL_STATUS, className: "w-28",
      render: (p) => <Badge variant={p.is_published ? "default" : "outline"} className="text-[11px]">{p.is_published ? "Published" : "Draft"}</Badge> },
    { key: "limits", label: TEXT.ADMIN.PROBLEMS.COL_LIMITS, className: "w-36",
      render: (p) => (
        <div className="flex gap-2 text-xs text-muted-foreground">
          <span>{p.time_limit_ms >= 1000 ? `${(p.time_limit_ms / 1000).toFixed(1)}s` : `${p.time_limit_ms}ms`}</span>
          <span className="text-muted-foreground/40">|</span>
          <span>{p.memory_limit_kb >= 1024 ? `${(p.memory_limit_kb / 1024).toFixed(0)}MB` : `${p.memory_limit_kb}KB`}</span>
        </div>
      ) },
    { key: "actions", label: "", className: "w-20",
      render: (p) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer"
            onClick={() => router.push(`/admin/problems/${p.id}/edit`)}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer"
            onClick={() => crud.confirmDelete(p.id)}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) },
  ];

  if (crud.isLoading) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={TEXT.ADMIN.PROBLEMS.TITLE}
      subtitle={TEXT.ADMIN.PROBLEMS.SUBTITLE}
      createLabel={TEXT.ADMIN.PROBLEMS.CREATE}
      onCreateClick={() => router.push("/admin/problems/create")}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.PROBLEMS.SEARCH_PLACEHOLDER}
      extraFilters={
        <select
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
          onChange={(e) => crud.setFilter("difficulty_id", e.target.value ? Number(e.target.value) : null, "exact")}
        >
          <option value="">{TEXT.ADMIN.PROBLEMS.FILTER_DIFFICULTY_ALL}</option>
          {difficulties.map((d) => (
            <option key={d.id} value={d.id}>{d.name}</option>
          ))}
        </select>
      }
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(p) => p.id}
          emptyMessage={TEXT.ADMIN.PROBLEMS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      confirmDialog={
        <ConfirmDialog
          open={crud.deleteId !== null}
          onOpenChange={(open) => { if (!open) crud.cancelDelete(); }}
          onConfirm={crud.handleDelete}
        />
      }
    />
  );
}
