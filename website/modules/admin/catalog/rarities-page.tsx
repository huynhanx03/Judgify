"use client";

/**
 * Admin rarities page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { RarityDialog } from "@/modules/admin/dialogs/rarity-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { rarityService } from "@/services/rarity.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { RarityResponse } from "@/types/cultivation";

export default function AdminRaritiesPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.RARITY;
  const service = useMemo(() => ({
    find: rarityService.find.bind(rarityService),
    create: rarityService.create.bind(rarityService),
    update: rarityService.update.bind(rarityService),
    delete: rarityService.delete.bind(rarityService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<RarityResponse>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.RARITIES.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "code", label: ADMIN_TEXT.RARITIES.COL_CODE, className: "w-28",
      render: (r) => <span className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{r.code}</span> },
    { key: "weight", label: ADMIN_TEXT.RARITIES.COL_WEIGHT, className: "w-24",
      render: (r) => <span className="text-sm font-mono">{r.weight}</span> },
    { key: "description", label: ADMIN_TEXT.RARITIES.COL_DESC,
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
      title={ADMIN_TEXT.RARITIES.TITLE}
      subtitle={ADMIN_TEXT.RARITIES.SUBTITLE}
      createLabel={ADMIN_TEXT.RARITIES.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.RARITIES.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={ADMIN_TEXT.RARITIES.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <RarityDialog
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
