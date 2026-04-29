"use client";

/**
 * Admin ranks page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { RankDialog } from "@/modules/admin/dialogs/rank-dialog";
import { rankService } from "@/services/rank.service";
import { TEXT } from "@/constants/text";
import type { RankResponse } from "@/types/cultivation";

export default function AdminRanksPage() {
  const service = useMemo(() => ({
    find: rankService.find.bind(rankService),
    create: rankService.create.bind(rankService),
    update: rankService.update.bind(rankService),
    delete: rankService.delete.bind(rankService),
  }), []);

  const crud = usePaginatedCRUD<RankResponse>({ service });

  const columns: AdminColumn<RankResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.RANKS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "min_rating", label: TEXT.ADMIN.RANKS.COL_MIN_RATING, className: "w-36",
      render: (r) => <span className="text-sm font-mono text-blue-400">{r.min_rating.toLocaleString()}</span> },
    { key: "description", label: TEXT.ADMIN.RANKS.COL_DESC,
      render: (r) => <span className="text-sm text-muted-foreground">{r.description ?? "—"}</span> },
    { key: "actions", label: "", className: "w-20",
      render: (r) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => crud.openEdit(r)}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => crud.confirmDelete(r.id)}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) },
  ];

  if (crud.isLoading) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={TEXT.ADMIN.RANKS.TITLE}
      subtitle={TEXT.ADMIN.RANKS.SUBTITLE}
      createLabel={TEXT.ADMIN.RANKS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.RANKS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.RANKS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <RankDialog
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
