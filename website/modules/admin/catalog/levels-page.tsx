"use client";

/**
 * Admin levels page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { LevelDialog } from "@/modules/admin/dialogs/level-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { levelService } from "@/services/level.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { formatNumber } from "@/lib/format";
import type { LevelResponse } from "@/types/cultivation";

export default function AdminLevelsPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.LEVEL;
  const service = useMemo(() => ({
    find: levelService.find.bind(levelService),
    create: levelService.create.bind(levelService),
    update: levelService.update.bind(levelService),
    delete: levelService.delete.bind(levelService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<LevelResponse>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.LEVELS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "min_exp", label: ADMIN_TEXT.LEVELS.COL_MIN_EXP, className: "w-36",
      render: (r) => <span className="font-mono text-sm text-info">{formatNumber(r.min_exp)}</span> },
    { key: "description", label: ADMIN_TEXT.LEVELS.COL_DESC,
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
      title={ADMIN_TEXT.LEVELS.TITLE}
      subtitle={ADMIN_TEXT.LEVELS.SUBTITLE}
      createLabel={ADMIN_TEXT.LEVELS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.LEVELS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={ADMIN_TEXT.LEVELS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <LevelDialog
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
