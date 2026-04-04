"use client";

/**
 * Admin tags page — paginated list with search, element filter, create, edit, delete.
 */

import { useEffect, useMemo, useState } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { TagDialog } from "@/modules/admin/dialogs/tag-dialog";
import { adminService } from "@/services/admin.service";
import { TEXT } from "@/constants/text";
import type { Tag } from "@/types/tag";
import type { ElementResponse } from "@/types/cultivation";

export default function AdminTagsPage() {
  const [elements, setElements] = useState<ElementResponse[]>([]);

  // Load elements for the filter dropdown
  useEffect(() => {
    adminService.getAllElements().then(setElements).catch(() => {});
  }, []);

  const service = useMemo(() => ({
    find: adminService.findTags.bind(adminService),
    create: adminService.createTag.bind(adminService),
    update: adminService.updateTag.bind(adminService),
    delete: adminService.deleteTag.bind(adminService),
  }), []);

  const crud = usePaginatedCRUD<Tag>({ service });

  const columns: AdminColumn<Tag>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.TAGS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "elements", label: TEXT.ADMIN.TAGS.COL_ELEMENTS,
      render: (r) => (
        <div className="flex gap-1 flex-wrap">
          {r.elements?.length
            ? r.elements.map((e) => (
                <span key={e.id} className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                  {e.name}
                </span>
              ))
            : <span className="text-xs text-muted-foreground">—</span>}
        </div>
      ) },
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
      title={TEXT.ADMIN.TAGS.TITLE}
      subtitle={TEXT.ADMIN.TAGS.SUBTITLE}
      createLabel={TEXT.ADMIN.TAGS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.TAGS.SEARCH_PLACEHOLDER}
      extraFilters={
        <select
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
          onChange={(e) => crud.setFilter("element_id", e.target.value ? Number(e.target.value) : null, "exact")}
        >
          <option value="">{TEXT.ADMIN.TAGS.FILTER_ELEMENT_ALL}</option>
          {elements.map((el) => (
            <option key={el.id} value={el.id}>{el.name}</option>
          ))}
        </select>
      }
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.TAGS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <TagDialog
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
          elements={elements}
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
