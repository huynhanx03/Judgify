"use client";

/**
 * Admin elements page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { Loader2, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { ElementDialog } from "@/modules/admin/dialogs/element-dialog";
import { adminService } from "@/services/admin.service";
import { TEXT } from "@/constants/text";
import type { ElementResponse } from "@/types/cultivation";

export default function AdminElementsPage() {
  const service = useMemo(() => ({
    find: adminService.findElements.bind(adminService),
    create: adminService.createElement.bind(adminService),
    update: adminService.updateElement.bind(adminService),
    delete: adminService.deleteElement.bind(adminService),
  }), []);

  const crud = usePaginatedCRUD<ElementResponse>({ service });

  const columns: AdminColumn<ElementResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.ELEMENTS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "code", label: TEXT.ADMIN.ELEMENTS.COL_CODE, className: "w-28",
      render: (r) => <span className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{r.code}</span> },
    { key: "description", label: TEXT.ADMIN.ELEMENTS.COL_DESC,
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

  if (crud.isLoading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <Loader2 className="h-8 w-8 text-primary animate-spin" />
    </div>
  );

  return (
    <DataTableShell
      title={TEXT.ADMIN.ELEMENTS.TITLE}
      subtitle={TEXT.ADMIN.ELEMENTS.SUBTITLE}
      createLabel={TEXT.ADMIN.ELEMENTS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.ELEMENTS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.ELEMENTS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <ElementDialog
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
