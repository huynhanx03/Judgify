"use client";

/**
 * Admin page for managing material articles (Tàng Kinh Các).
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { MaterialDialog } from "@/modules/admin/dialogs/material-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { MaterialLifecycleActions } from "@/modules/admin/material-lifecycle-actions";
import { MaterialCursorPagination } from "@/modules/admin/material-cursor-pagination";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { materialService } from "@/services/material.service";
import { tagService } from "@/services/tag.service";
import { difficultyService } from "@/services/difficulty.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type {
  CreateMaterialRequest,
  MaterialArticle,
  MaterialCategory,
  UpdateMaterialRequest,
} from "@/types/material";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Tag } from "@/types/tag";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { notify } from "@/lib/toast";

export default function AdminMaterialsPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.MATERIAL;
  const canReadCategories = can(
    AUTHORIZATION_RESOURCE.MATERIAL_CATEGORY,
    AUTHORIZATION_ACTION.READ,
  );
  const canReadDifficulties = can(
    AUTHORIZATION_RESOURCE.DIFFICULTY,
    AUTHORIZATION_ACTION.READ,
  );
  const canReadTags = can(AUTHORIZATION_RESOURCE.TAG, AUTHORIZATION_ACTION.READ);
  const categoryCatalog = useRetryableResource<MaterialCategory[]>({
    resetKey: canReadCategories,
    enabled: canReadCategories,
    initialData: [],
    load: (signal) => materialService.getAllCategories(signal),
  });
  const difficultyCatalog = useRetryableResource<DifficultyResponse[]>({
    resetKey: canReadDifficulties,
    enabled: canReadDifficulties,
    initialData: [],
    load: (signal) => difficultyService.getAll(signal),
  });
  const tagCatalog = useRetryableResource<Tag[]>({
    resetKey: canReadTags,
    enabled: canReadTags,
    initialData: [],
    load: (signal) => tagService.getAll(signal),
  });
  const categories = categoryCatalog.data;
  const difficulties = difficultyCatalog.data;
  const allTags = tagCatalog.data;
  const catalogLoadFailed = [
    categoryCatalog.status,
    difficultyCatalog.status,
    tagCatalog.status,
  ].includes("error");

  const service = useMemo(() => ({
    find: materialService.findAdmin.bind(materialService),
    create: materialService.create.bind(materialService),
    update: materialService.update.bind(materialService),
  }), []);

  const crud = usePaginatedCRUD<
    MaterialArticle,
    CreateMaterialRequest,
    UpdateMaterialRequest
  >({ service, resource });

  async function openEditor(material: MaterialArticle) {
    try {
      crud.openEdit(await materialService.getAdmin(material.id));
    } catch {
      notify.error(ADMIN_TEXT.MATERIALS.EDITOR_LOAD_ERROR);
    }
  }

  const columns: AdminColumn<MaterialArticle>[] = [
    {
      key: "id",
      label: ADMIN_TEXT.COLUMN_ID,
      className: "w-16",
      render: (r) => <span className="text-muted-foreground">{r.id}</span>,
    },
    {
      key: "title",
      label: ADMIN_TEXT.MATERIALS.COL_TITLE,
      render: (r) => <span className="font-medium">{r.title}</span>,
    },
    {
      key: "category",
      label: ADMIN_TEXT.MATERIALS.COL_CATEGORY,
      className: "w-32",
      render: (r) => <span className="text-sm">{r.category?.name ?? TEXT.COMMON.NOT_AVAILABLE}</span>,
    },
    {
      key: "difficulty",
      label: ADMIN_TEXT.MATERIALS.COL_DIFFICULTY,
      className: "w-28",
      render: (r) => <span className="text-sm">{r.difficulty?.name ?? TEXT.COMMON.NOT_AVAILABLE}</span>,
    },
    {
      key: "status",
      label: ADMIN_TEXT.MATERIALS.COL_STATUS,
      className: "w-28",
      render: (r) => (
        <span className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${
          r.status === "published"
            ? "bg-success/10 text-success"
            : "bg-muted text-muted-foreground"
        }`}>
          {materialStatusLabel(r.status)}
        </span>
      ),
    },
    {
      key: "view_count",
      label: ADMIN_TEXT.MATERIALS.COL_VIEWS,
      className: "w-20 text-center",
      render: (r) => <span className="tabular-nums text-sm">{r.view_count}</span>,
    },
    {
      key: "actions",
      label: "",
      className: "w-32",
      render: (r) => (
        <div className="flex items-center justify-end">
		  <AdminResourceActions
			resource={resource}
			onEdit={
			  r.status === "archived" ? undefined : () => void openEditor(r)
			}
		  />
          <MaterialLifecycleActions material={r} onChanged={crud.refresh} />
        </div>
      ),
    },
  ];

  return (
    <DataTableShell
      title={ADMIN_TEXT.MATERIALS.TITLE}
      subtitle={ADMIN_TEXT.MATERIALS.SUBTITLE}
      createLabel={ADMIN_TEXT.MATERIALS.CREATE}
      canCreate={
        can(resource, AUTHORIZATION_ACTION.CREATE) &&
        canReadCategories &&
        canReadDifficulties
      }
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      loadError={
        crud.loadError ??
        (catalogLoadFailed ? ADMIN_TEXT.RELATED_DATA_LOAD_ERROR : null)
      }
      onRetryLoad={() => {
        crud.refresh();
        categoryCatalog.retry();
        difficultyCatalog.retry();
        tagCatalog.retry();
      }}
      searchPlaceholder={ADMIN_TEXT.MATERIALS.SEARCH_PLACEHOLDER}
      extraFilters={
        <div className="grid w-full gap-2 sm:grid-cols-2 lg:w-auto lg:grid-cols-3">
          <select
            aria-label={ADMIN_TEXT.MATERIALS.FILTER_CATEGORY_ALL}
            className="h-11 w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            disabled={
              !canReadCategories || categoryCatalog.status === "loading"
            }
            onChange={(e) => {
              const v = e.target.value;
              if (v) crud.setFilter("category_id", v, "exact");
              else crud.clearFilter("category_id");
            }}
          >
            <option value="">{ADMIN_TEXT.MATERIALS.FILTER_CATEGORY_ALL}</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <select
            aria-label={ADMIN_TEXT.MATERIALS.FILTER_DIFFICULTY_ALL}
            className="h-11 w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            disabled={
              !canReadDifficulties || difficultyCatalog.status === "loading"
            }
            onChange={(e) => {
              const v = e.target.value;
              if (v) crud.setFilter("difficulty_id", v, "exact");
              else crud.clearFilter("difficulty_id");
            }}
          >
            <option value="">{ADMIN_TEXT.MATERIALS.FILTER_DIFFICULTY_ALL}</option>
            {difficulties.map((d) => (
              <option key={d.id} value={d.id}>{d.name}</option>
            ))}
          </select>
          <select
            aria-label={ADMIN_TEXT.MATERIALS.FILTER_STATUS_ALL}
            className="h-11 w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            onChange={(e) => {
              const v = e.target.value;
              if (v) crud.setFilter("status", v, "exact");
              else crud.clearFilter("status");
            }}
          >
            <option value="">{ADMIN_TEXT.MATERIALS.FILTER_STATUS_ALL}</option>
            <option value="draft">{ADMIN_TEXT.MATERIALS.STATUS_DRAFT}</option>
            <option value="in_review">
              {ADMIN_TEXT.MATERIALS.STATUS_IN_REVIEW}
            </option>
            <option value="published">{ADMIN_TEXT.MATERIALS.STATUS_PUBLISHED}</option>
            <option value="archived">
              {ADMIN_TEXT.MATERIALS.STATUS_ARCHIVED}
            </option>
          </select>
        </div>
      }
      table={
        crud.isLoading ? (
          <LoadingSpinner />
        ) : (
          <div className="space-y-4">
            <AdminDataTable
              columns={columns}
              data={crud.data}
              keyExtractor={(r) => r.id}
              emptyMessage={ADMIN_TEXT.MATERIALS.EMPTY}
            />
            {crud.pagination ? (
              <MaterialCursorPagination
                page={crud.pagination.current_page}
                hasPrevious={crud.pagination.has_prev}
                hasNext={crud.pagination.has_next}
                onPageChange={crud.onPageChange}
              />
            ) : null}
          </div>
        )
      }
      dialog={
        <MaterialDialog
          key={`${crud.dialogOpen}:${crud.editing?.id ?? "create"}`}
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
          categories={categories}
          difficulties={difficulties}
          tags={allTags}
        />
      }
      confirmDialog={null}
    />
  );
}

function materialStatusLabel(status: MaterialArticle["status"]): string {
  return {
    draft: ADMIN_TEXT.MATERIALS.STATUS_DRAFT,
    in_review: ADMIN_TEXT.MATERIALS.STATUS_IN_REVIEW,
    published: ADMIN_TEXT.MATERIALS.STATUS_PUBLISHED,
    archived: ADMIN_TEXT.MATERIALS.STATUS_ARCHIVED,
  }[status];
}
