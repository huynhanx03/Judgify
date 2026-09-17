"use client";

/**
 * Admin page for managing material categories (Tàng Kinh Các — Danh Mục).
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { MaterialCategoryDialog } from "@/modules/admin/dialogs/material-category-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { materialService } from "@/services/material.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { MaterialCategory } from "@/types/material";

export default function AdminMaterialCategoriesPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.MATERIAL_CATEGORY;
  const service = useMemo(() => ({
    find: materialService.findCategories.bind(materialService),
    create: materialService.createCategory.bind(materialService),
    update: materialService.updateCategory.bind(materialService),
    delete: materialService.deleteCategory.bind(materialService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<MaterialCategory>[] = [
    {
      key: "id",
      label: ADMIN_TEXT.COLUMN_ID,
      className: "w-16",
      render: (r) => <span className="text-muted-foreground">{r.id}</span>,
    },
    {
      key: "name",
      label: ADMIN_TEXT.MATERIAL_CATEGORIES.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span>,
    },
    {
      key: "description",
      label: ADMIN_TEXT.MATERIAL_CATEGORIES.COL_DESC,
      render: (r) => (
        <span className="text-muted-foreground text-sm line-clamp-1">
          {r.description || TEXT.COMMON.NOT_AVAILABLE}
        </span>
      ),
    },
    {
      key: "article_count",
      label: ADMIN_TEXT.MATERIAL_CATEGORIES.COL_ARTICLES,
      className: "w-24 text-center",
      render: (r) => <span className="tabular-nums">{r.article_count}</span>,
    },
    {
      key: "actions",
      label: "",
      className: "w-20",
      render: (r) => (
        <AdminResourceActions
          resource={resource}
          onEdit={() => crud.openEdit(r)}
          onDelete={() => crud.confirmDelete(r.id)}
        />
      ),
    },
  ];

  return (
    <DataTableShell
      title={ADMIN_TEXT.MATERIAL_CATEGORIES.TITLE}
      subtitle={ADMIN_TEXT.MATERIAL_CATEGORIES.SUBTITLE}
      createLabel={ADMIN_TEXT.MATERIAL_CATEGORIES.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.MATERIAL_CATEGORIES.SEARCH_PLACEHOLDER}
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
            emptyMessage={ADMIN_TEXT.MATERIAL_CATEGORIES.EMPTY}
          />
        )
      }
      dialog={
        <MaterialCategoryDialog
          key={`${crud.dialogOpen}:${crud.editing?.id ?? "create"}`}
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
