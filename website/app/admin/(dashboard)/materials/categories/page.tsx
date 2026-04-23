"use client";

/**
 * Admin page for managing material categories (Tàng Kinh Các — Danh Mục).
 */

import { useMemo } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/loading-spinner";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { MaterialCategoryDialog } from "@/modules/admin/dialogs/material-category-dialog";
import { adminService } from "@/services/admin.service";
import { TEXT } from "@/constants/text";
import type { MaterialCategory } from "@/types/material";

export default function AdminMaterialCategoriesPage() {
  const service = useMemo(() => ({
    find: adminService.findMaterialCategories.bind(adminService),
    create: adminService.createMaterialCategory.bind(adminService),
    update: adminService.updateMaterialCategory.bind(adminService),
    delete: adminService.deleteMaterialCategory.bind(adminService),
  }), []);

  const crud = usePaginatedCRUD<MaterialCategory>({ service });

  const columns: AdminColumn<MaterialCategory>[] = [
    {
      key: "id",
      label: "ID",
      className: "w-16",
      render: (r) => <span className="text-muted-foreground">{r.id}</span>,
    },
    {
      key: "name",
      label: TEXT.ADMIN.MATERIAL_CATEGORIES.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span>,
    },
    {
      key: "description",
      label: TEXT.ADMIN.MATERIAL_CATEGORIES.COL_DESC,
      render: (r) => (
        <span className="text-muted-foreground text-sm line-clamp-1">
          {r.description || "—"}
        </span>
      ),
    },
    {
      key: "article_count",
      label: TEXT.ADMIN.MATERIAL_CATEGORIES.COL_ARTICLES,
      className: "w-24 text-center",
      render: (r) => <span className="tabular-nums">{r.article_count}</span>,
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
      title={TEXT.ADMIN.MATERIAL_CATEGORIES.TITLE}
      subtitle={TEXT.ADMIN.MATERIAL_CATEGORIES.SUBTITLE}
      createLabel={TEXT.ADMIN.MATERIAL_CATEGORIES.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.MATERIAL_CATEGORIES.SEARCH_PLACEHOLDER}
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
            emptyMessage={TEXT.ADMIN.MATERIAL_CATEGORIES.EMPTY}
          />
        )
      }
      dialog={
        <MaterialCategoryDialog
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
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
