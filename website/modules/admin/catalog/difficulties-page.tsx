"use client";

/**
 * Admin difficulties page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { DifficultyDialog } from "@/modules/admin/dialogs/difficulty-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { difficultyService } from "@/services/difficulty.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { DifficultyResponse } from "@/types/difficulty";

export default function AdminDifficultiesPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.DIFFICULTY;
  const service = useMemo(() => ({
    find: difficultyService.find.bind(difficultyService),
    create: difficultyService.create.bind(difficultyService),
    update: difficultyService.update.bind(difficultyService),
    delete: difficultyService.delete.bind(difficultyService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<DifficultyResponse>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.DIFFICULTIES.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "level", label: ADMIN_TEXT.DIFFICULTIES.COL_LEVEL, className: "w-24",
      render: (r) => <span className="text-sm font-mono">{r.level}</span> },
    { key: "exp_reward", label: ADMIN_TEXT.DIFFICULTIES.COL_EXP, className: "w-28",
      render: (r) => <span className="text-sm text-muted-foreground">{r.exp_reward ?? TEXT.COMMON.NOT_AVAILABLE}</span> },
    { key: "description", label: ADMIN_TEXT.DIFFICULTIES.COL_DESC,
      render: (r) => <span className="text-sm text-muted-foreground">{r.description ?? TEXT.COMMON.NOT_AVAILABLE}</span> },
    { key: "actions", label: "", className: "w-20",
      render: (r) => (
        <AdminResourceActions
          resource={resource}
          onEdit={() => crud.openEdit(r)}
          onDelete={() => crud.confirmDelete(r.id)}
        />
      ) },
  ];

  if (crud.isLoading && !crud.hasSettled) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={ADMIN_TEXT.DIFFICULTIES.TITLE}
      subtitle={ADMIN_TEXT.DIFFICULTIES.SUBTITLE}
      createLabel={ADMIN_TEXT.DIFFICULTIES.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.DIFFICULTIES.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={ADMIN_TEXT.DIFFICULTIES.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <DifficultyDialog
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
