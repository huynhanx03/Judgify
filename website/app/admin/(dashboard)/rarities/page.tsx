"use client";

/**
 * Admin rarities page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { RarityDialog } from "@/modules/admin/dialogs/rarity-dialog";
import { rarityService } from "@/services/rarity.service";
import { TEXT } from "@/constants/text";
import type { RarityResponse } from "@/types/cultivation";

export default function AdminRaritiesPage() {
  const service = useMemo(() => ({
    find: rarityService.find.bind(rarityService),
    create: rarityService.create.bind(rarityService),
    update: rarityService.update.bind(rarityService),
    delete: rarityService.delete.bind(rarityService),
  }), []);

  const crud = usePaginatedCRUD<RarityResponse>({ service });

  const columns: AdminColumn<RarityResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.RARITIES.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "code", label: TEXT.ADMIN.RARITIES.COL_CODE, className: "w-28",
      render: (r) => <span className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{r.code}</span> },
    { key: "weight", label: TEXT.ADMIN.RARITIES.COL_WEIGHT, className: "w-24",
      render: (r) => <span className="text-sm font-mono">{r.weight}</span> },
    { key: "description", label: TEXT.ADMIN.RARITIES.COL_DESC,
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
      title={TEXT.ADMIN.RARITIES.TITLE}
      subtitle={TEXT.ADMIN.RARITIES.SUBTITLE}
      createLabel={TEXT.ADMIN.RARITIES.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.RARITIES.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.RARITIES.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <RarityDialog
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
