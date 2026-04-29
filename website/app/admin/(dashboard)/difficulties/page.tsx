"use client";

/**
 * Admin difficulties page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { DifficultyDialog } from "@/modules/admin/dialogs/difficulty-dialog";
import { difficultyService } from "@/services/difficulty.service";
import { TEXT } from "@/constants/text";
import type { DifficultyResponse } from "@/types/difficulty";

export default function AdminDifficultiesPage() {
  const service = useMemo(() => ({
    find: difficultyService.find.bind(difficultyService),
    create: difficultyService.create.bind(difficultyService),
    update: difficultyService.update.bind(difficultyService),
    delete: difficultyService.delete.bind(difficultyService),
  }), []);

  const crud = usePaginatedCRUD<DifficultyResponse>({ service });

  const columns: AdminColumn<DifficultyResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.DIFFICULTIES.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "level", label: TEXT.ADMIN.DIFFICULTIES.COL_LEVEL, className: "w-24",
      render: (r) => <span className="text-sm font-mono">{r.level}</span> },
    { key: "exp_reward", label: TEXT.ADMIN.DIFFICULTIES.COL_EXP, className: "w-28",
      render: (r) => <span className="text-sm text-muted-foreground">{r.exp_reward ?? "—"}</span> },
    { key: "description", label: TEXT.ADMIN.DIFFICULTIES.COL_DESC,
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
      title={TEXT.ADMIN.DIFFICULTIES.TITLE}
      subtitle={TEXT.ADMIN.DIFFICULTIES.SUBTITLE}
      createLabel={TEXT.ADMIN.DIFFICULTIES.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.DIFFICULTIES.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.DIFFICULTIES.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <DifficultyDialog
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
