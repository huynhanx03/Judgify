"use client";

/**
 * Admin page for managing material articles (Tàng Kinh Các).
 */

import { useEffect, useMemo, useState } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/loading-spinner";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { MaterialDialog } from "@/modules/admin/dialogs/material-dialog";
import { materialService } from "@/services/material.service";
import { tagService } from "@/services/tag.service";
import { difficultyService } from "@/services/difficulty.service";
import { TEXT } from "@/constants/text";
import type { MaterialArticle, MaterialCategory } from "@/types/material";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Tag } from "@/types/tag";

export default function AdminMaterialsPage() {
  const [categories, setCategories] = useState<MaterialCategory[]>([]);
  const [difficulties, setDifficulties] = useState<DifficultyResponse[]>([]);
  const [allTags, setAllTags] = useState<Tag[]>([]);

  // Load filter data
  useEffect(() => {
    materialService.getAllCategories().then(setCategories).catch(() => {});
    difficultyService.getAll().then(setDifficulties).catch(() => {});
    tagService.getAll().then(setAllTags).catch(() => {});
  }, []);

  const service = useMemo(() => ({
    find: materialService.find.bind(materialService),
    create: materialService.create.bind(materialService),
    update: materialService.update.bind(materialService),
    delete: materialService.delete.bind(materialService),
  }), []);

  const crud = usePaginatedCRUD<MaterialArticle>({ service });

  const columns: AdminColumn<MaterialArticle>[] = [
    {
      key: "id",
      label: "ID",
      className: "w-16",
      render: (r) => <span className="text-muted-foreground">{r.id}</span>,
    },
    {
      key: "title",
      label: TEXT.ADMIN.MATERIALS.COL_TITLE,
      render: (r) => <span className="font-medium">{r.title}</span>,
    },
    {
      key: "category",
      label: TEXT.ADMIN.MATERIALS.COL_CATEGORY,
      className: "w-32",
      render: (r) => <span className="text-sm">{r.category?.name ?? "—"}</span>,
    },
    {
      key: "difficulty",
      label: TEXT.ADMIN.MATERIALS.COL_DIFFICULTY,
      className: "w-28",
      render: (r) => <span className="text-sm">{r.difficulty?.name ?? "—"}</span>,
    },
    {
      key: "status",
      label: TEXT.ADMIN.MATERIALS.COL_STATUS,
      className: "w-28",
      render: (r) => (
        <span className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${
          r.status === "published"
            ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
            : "bg-muted text-muted-foreground"
        }`}>
          {r.status === "published" ? TEXT.ADMIN.MATERIALS.STATUS_PUBLISHED : TEXT.ADMIN.MATERIALS.STATUS_DRAFT}
        </span>
      ),
    },
    {
      key: "view_count",
      label: TEXT.ADMIN.MATERIALS.COL_VIEWS,
      className: "w-20 text-center",
      render: (r) => <span className="tabular-nums text-sm">{r.view_count}</span>,
    },
    {
      key: "actions",
      label: "",
      className: "w-20",
      render: (r) => (
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 cursor-pointer" onClick={() => crud.openEdit(r)}>
            <Pencil className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive cursor-pointer" onClick={() => crud.confirmDelete(r.id)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <DataTableShell
      title={TEXT.ADMIN.MATERIALS.TITLE}
      subtitle={TEXT.ADMIN.MATERIALS.SUBTITLE}
      createLabel={TEXT.ADMIN.MATERIALS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.MATERIALS.SEARCH_PLACEHOLDER}
      extraFilters={
        <div className="flex items-center gap-2">
          <select
            className="rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            onChange={(e) => {
              const v = e.target.value;
              if (v) crud.setFilter("category_id", Number(v), "exact");
              else crud.clearFilter("category_id");
            }}
          >
            <option value="">{TEXT.ADMIN.MATERIALS.FILTER_CATEGORY_ALL}</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <select
            className="rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            onChange={(e) => {
              const v = e.target.value;
              if (v) crud.setFilter("difficulty_id", Number(v), "exact");
              else crud.clearFilter("difficulty_id");
            }}
          >
            <option value="">{TEXT.ADMIN.MATERIALS.FILTER_DIFFICULTY_ALL}</option>
            {difficulties.map((d) => (
              <option key={d.id} value={d.id}>{d.name}</option>
            ))}
          </select>
          <select
            className="rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            onChange={(e) => {
              const v = e.target.value;
              if (v) crud.setFilter("status", v, "exact");
              else crud.clearFilter("status");
            }}
          >
            <option value="">{TEXT.ADMIN.MATERIALS.FILTER_STATUS_ALL}</option>
            <option value="draft">{TEXT.ADMIN.MATERIALS.STATUS_DRAFT}</option>
            <option value="published">{TEXT.ADMIN.MATERIALS.STATUS_PUBLISHED}</option>
          </select>
        </div>
      }
      table={
        crud.isLoading ? (
          <LoadingSpinner />
        ) : (
          <AdminDataTable
            columns={columns}
            data={crud.data}
            keyExtractor={(r) => r.id}
            pagination={crud.pagination}
            onPageChange={crud.onPageChange}
            emptyMessage={TEXT.ADMIN.MATERIALS.EMPTY}
          />
        )
      }
      dialog={
        <MaterialDialog
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
          categories={categories}
          difficulties={difficulties}
          tags={allTags}
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
