"use client";

/**
 * Admin levels page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { LevelDialog } from "@/modules/admin/dialogs/level-dialog";
import { levelService } from "@/services/level.service";
import { TEXT } from "@/constants/text";
import type { LevelResponse } from "@/types/cultivation";

export default function AdminLevelsPage() {
  const service = useMemo(() => ({
    find: levelService.find.bind(levelService),
    create: levelService.create.bind(levelService),
    update: levelService.update.bind(levelService),
    delete: levelService.delete.bind(levelService),
  }), []);

  const crud = usePaginatedCRUD<LevelResponse>({ service });

  const columns: AdminColumn<LevelResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.LEVELS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "min_exp", label: TEXT.ADMIN.LEVELS.COL_MIN_EXP, className: "w-36",
      render: (r) => <span className="text-sm font-mono text-blue-400">{r.min_exp.toLocaleString()}</span> },
    { key: "description", label: TEXT.ADMIN.LEVELS.COL_DESC,
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
      title={TEXT.ADMIN.LEVELS.TITLE}
      subtitle={TEXT.ADMIN.LEVELS.SUBTITLE}
      createLabel={TEXT.ADMIN.LEVELS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.LEVELS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.LEVELS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <LevelDialog
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
