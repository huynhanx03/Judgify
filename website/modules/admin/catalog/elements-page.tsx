"use client";

/**
 * Admin elements page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { ElementDialog } from "@/modules/admin/dialogs/element-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { elementService } from "@/services/element.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { ElementResponse } from "@/types/cultivation";

export default function AdminElementsPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.ELEMENT;
  const service = useMemo(() => ({
    find: elementService.find.bind(elementService),
    create: elementService.create.bind(elementService),
    update: elementService.update.bind(elementService),
    delete: elementService.delete.bind(elementService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<ElementResponse>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.ELEMENTS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "code", label: ADMIN_TEXT.ELEMENTS.COL_CODE, className: "w-28",
      render: (r) => <span className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{r.code}</span> },
    { key: "description", label: ADMIN_TEXT.ELEMENTS.COL_DESC,
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
      title={ADMIN_TEXT.ELEMENTS.TITLE}
      subtitle={ADMIN_TEXT.ELEMENTS.SUBTITLE}
      createLabel={ADMIN_TEXT.ELEMENTS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.ELEMENTS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={ADMIN_TEXT.ELEMENTS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <ElementDialog
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
