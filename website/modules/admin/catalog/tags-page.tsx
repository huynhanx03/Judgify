"use client";

/**
 * Admin tags page — paginated list with search, element filter, create, edit, delete.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { TagDialog } from "@/modules/admin/dialogs/tag-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { tagService } from "@/services/tag.service";
import { elementService } from "@/services/element.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type { Tag } from "@/types/tag";
import type { ElementResponse } from "@/types/cultivation";
import { useRetryableResource } from "@/hooks/use-retryable-resource";

export default function AdminTagsPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.TAG;
  const canReadElements = can(
    AUTHORIZATION_RESOURCE.ELEMENT,
    AUTHORIZATION_ACTION.READ,
  );
  const elementCatalog = useRetryableResource<ElementResponse[]>({
    resetKey: canReadElements,
    enabled: canReadElements,
    initialData: [],
    load: () => elementService.getAll(),
  });
  const elements = elementCatalog.data;

  const service = useMemo(() => ({
    find: tagService.find.bind(tagService),
    create: tagService.create.bind(tagService),
    update: tagService.update.bind(tagService),
    delete: tagService.delete.bind(tagService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource });

  const columns: AdminColumn<Tag>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.TAGS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "elements", label: ADMIN_TEXT.TAGS.COL_ELEMENTS,
      render: (r) => (
        <div className="flex gap-1 flex-wrap">
          {r.elements?.length
            ? r.elements.map((e) => (
                <span key={e.id} className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                  {e.name}
                </span>
              ))
            : <span className="text-xs text-muted-foreground">{TEXT.COMMON.NOT_AVAILABLE}</span>}
        </div>
      ) },
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
      title={ADMIN_TEXT.TAGS.TITLE}
      subtitle={ADMIN_TEXT.TAGS.SUBTITLE}
      createLabel={ADMIN_TEXT.TAGS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={
        crud.loadError ??
        (elementCatalog.status === "error"
          ? ADMIN_TEXT.RELATED_DATA_LOAD_ERROR
          : null)
      }
      onRetryLoad={() => {
        crud.refresh();
        elementCatalog.retry();
      }}
      searchPlaceholder={ADMIN_TEXT.TAGS.SEARCH_PLACEHOLDER}
      extraFilters={
        <select
          aria-label={ADMIN_TEXT.TAGS.FILTER_ELEMENT_ALL}
          className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
          onChange={(e) => crud.setFilter("element_id", e.target.value || null, "exact")}
          disabled={!canReadElements || elementCatalog.status === "loading"}
        >
          <option value="">{ADMIN_TEXT.TAGS.FILTER_ELEMENT_ALL}</option>
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
          emptyMessage={ADMIN_TEXT.TAGS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <TagDialog
          key={`${crud.dialogOpen}:${crud.editing?.id ?? "create"}`}
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
