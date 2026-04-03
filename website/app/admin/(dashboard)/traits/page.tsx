"use client";

/**
 * Admin traits page — paginated list with search, type filter, rarity filter.
 */

import { useEffect, useMemo, useState } from "react";
import { Loader2, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { TraitDialog } from "@/modules/admin/dialogs/trait-dialog";
import { adminService } from "@/services/admin.service";
import { TEXT } from "@/constants/text";
import type { TraitResponse, RarityResponse } from "@/types/cultivation";

export default function AdminTraitsPage() {
  const [rarities, setRarities] = useState<RarityResponse[]>([]);

  // Load rarities for the filter dropdown
  useEffect(() => {
    adminService.getAllRarities().then(setRarities).catch(() => {});
  }, []);

  const service = useMemo(() => ({
    find: adminService.findTraits.bind(adminService),
    create: adminService.createTrait.bind(adminService),
    update: adminService.updateTrait.bind(adminService),
    delete: adminService.deleteTrait.bind(adminService),
  }), []);

  const crud = usePaginatedCRUD<TraitResponse>({ service });

  const columns: AdminColumn<TraitResponse>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: TEXT.ADMIN.TRAITS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "type", label: TEXT.ADMIN.TRAITS.COL_TYPE, className: "w-28",
      render: (r) => (
        <span className="text-xs bg-muted px-1.5 py-0.5 rounded">
          {r.type === "root_bone" ? TEXT.ADMIN.FIELDS.TYPE_ROOT_BONE : TEXT.ADMIN.FIELDS.TYPE_TALENT}
        </span>
      ) },
    { key: "rarity", label: TEXT.ADMIN.TRAITS.COL_RARITY, className: "w-28",
      render: (r) => <span className="text-sm text-muted-foreground">{r.rarity?.name ?? "—"}</span> },
    { key: "description", label: TEXT.ADMIN.TRAITS.COL_DESC,
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
      title={TEXT.ADMIN.TRAITS.TITLE}
      subtitle={TEXT.ADMIN.TRAITS.SUBTITLE}
      createLabel={TEXT.ADMIN.TRAITS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.TRAITS.SEARCH_PLACEHOLDER}
      extraFilters={
        <>
          {/* Type filter */}
          <select
            className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
            onChange={(e) => crud.setFilter("type", e.target.value || null, "exact")}
          >
            <option value="">{TEXT.ADMIN.TRAITS.FILTER_TYPE_ALL}</option>
            <option value="root_bone">{TEXT.ADMIN.FIELDS.TYPE_ROOT_BONE}</option>
            <option value="talent">{TEXT.ADMIN.FIELDS.TYPE_TALENT}</option>
          </select>
          {/* Rarity filter */}
          <select
            className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
            onChange={(e) => crud.setFilter("rarity_id", e.target.value ? Number(e.target.value) : null, "exact")}
          >
            <option value="">{TEXT.ADMIN.TRAITS.FILTER_RARITY_ALL}</option>
            {rarities.map((r) => (
              <option key={r.id} value={r.id}>{r.name}</option>
            ))}
          </select>
        </>
      }
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(r) => r.id}
          emptyMessage={TEXT.ADMIN.TRAITS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <TraitDialog
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
