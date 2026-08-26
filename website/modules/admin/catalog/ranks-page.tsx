"use client";

/**
 * Admin ranks page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { RankDialog } from "@/modules/admin/dialogs/rank-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { rankService } from "@/services/rank.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { formatNumber } from "@/lib/format";
import type { RankResponse } from "@/types/cultivation";

export default function AdminRanksPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.RANK;
  const service = useMemo(() => ({
    find: rankService.find.bind(rankService),
    create: rankService.create.bind(rankService),
    update: rankService.update.bind(rankService),
    delete: rankService.delete.bind(rankService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<RankResponse>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.RANKS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "min_rating", label: ADMIN_TEXT.RANKS.COL_MIN_RATING, className: "w-36",
      render: (r) => <span className="text-sm font-mono text-blue-400">{formatNumber(r.min_rating)}</span> },
    { key: "description", label: ADMIN_TEXT.RANKS.COL_DESC,
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
      title={ADMIN_TEXT.RANKS.TITLE}
      subtitle={ADMIN_TEXT.RANKS.SUBTITLE}
      createLabel={ADMIN_TEXT.RANKS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.RANKS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={ADMIN_TEXT.RANKS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <RankDialog
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
