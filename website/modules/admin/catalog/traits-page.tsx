"use client";

/**
 * Admin traits page — paginated list with search, type filter, rarity filter.
 */

import { useMemo } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Badge } from "@/components/ui/badge";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { TraitDialog } from "@/modules/admin/dialogs/trait-dialog";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { cultivationService } from "@/services/cultivation.service";
import { rarityService } from "@/services/rarity.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { notify } from "@/lib/toast";
import type { TraitResponse, RarityResponse } from "@/types/cultivation";
import type {
  TraitMutationInput,
  TraitUpdateInput,
} from "@/services/cultivation.service";
import { useRetryableResource } from "@/hooks/use-retryable-resource";

export default function AdminTraitsPage() {
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.TRAIT;
  const canReadRarities = can(
    AUTHORIZATION_RESOURCE.RARITY,
    AUTHORIZATION_ACTION.READ,
  );
  const rarityCatalog = useRetryableResource<RarityResponse[]>({
    resetKey: canReadRarities,
    enabled: canReadRarities,
    initialData: [],
    load: () => rarityService.getAll(),
  });
  const rarities = rarityCatalog.data;

  const service = useMemo(() => ({
    find: cultivationService.findTraits.bind(cultivationService),
    create: cultivationService.createTrait.bind(cultivationService),
    update: cultivationService.updateTrait.bind(cultivationService),
  }), []);

  const crud = usePaginatedCRUD<
    TraitResponse,
    TraitMutationInput,
    TraitUpdateInput
  >({ service, resource });

  const columns: AdminColumn<TraitResponse>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (r) => <span className="text-xs tabular-nums text-muted-foreground">{r.id}</span> },
    { key: "name", label: ADMIN_TEXT.TRAITS.COL_NAME,
      render: (r) => <span className="font-medium">{r.name}</span> },
    { key: "type", label: ADMIN_TEXT.TRAITS.COL_TYPE, className: "w-28",
      render: (r) => (
        <span className="text-xs bg-muted px-1.5 py-0.5 rounded">
          {r.type === "root_bone" ? ADMIN_TEXT.FIELDS.TYPE_ROOT_BONE : ADMIN_TEXT.FIELDS.TYPE_TALENT}
        </span>
      ) },
    { key: "rarity", label: ADMIN_TEXT.TRAITS.COL_RARITY, className: "w-28",
      render: (r) => <span className="text-sm text-muted-foreground">{r.rarity?.name ?? TEXT.COMMON.NOT_AVAILABLE}</span> },
    { key: "effect", label: ADMIN_TEXT.TRAITS.COL_EFFECT, className: "min-w-44",
      render: (r) => {
        const effect = r.effect_revision?.effect;
        if (!effect) return <span className="text-xs text-destructive">{ADMIN_TEXT.USER_TRAITS.EFFECT_UNAVAILABLE}</span>;
        const value = effect.kind === "exp_multiplier"
          ? `${effect.multiplier_delta_bps >= 0 ? "+" : ""}${effect.multiplier_delta_bps / 100}%`
          : `${effect.flat_bonus >= 0 ? "+" : ""}${effect.flat_bonus} EXP`;
        return <span className="font-mono text-xs">{value}</span>;
      } },
    { key: "revision", label: ADMIN_TEXT.TRAITS.COL_REVISION, className: "w-24",
      render: (r) => <span className="font-mono text-xs">v{r.version}</span> },
    { key: "status", label: ADMIN_TEXT.TRAITS.COL_STATUS, className: "w-32",
      render: (r) => <Badge variant={r.active ? "secondary" : "outline"}>{r.active ? ADMIN_TEXT.TRAITS.ACTIVE : ADMIN_TEXT.TRAITS.ARCHIVED}</Badge> },
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
      title={ADMIN_TEXT.TRAITS.TITLE}
      subtitle={ADMIN_TEXT.TRAITS.SUBTITLE}
      createLabel={ADMIN_TEXT.TRAITS.CREATE}
      canCreate={
        can(resource, AUTHORIZATION_ACTION.CREATE) && canReadRarities
      }
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={
        crud.loadError ??
        (rarityCatalog.status === "error"
          ? ADMIN_TEXT.RELATED_DATA_LOAD_ERROR
          : null)
      }
      onRetryLoad={() => {
        crud.refresh();
        rarityCatalog.retry();
      }}
      searchPlaceholder={ADMIN_TEXT.TRAITS.SEARCH_PLACEHOLDER}
      extraFilters={
        <>
          {/* Type filter */}
          <select
            className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
            onChange={(e) => crud.setFilter("type", e.target.value || null, "exact")}
          >
            <option value="">{ADMIN_TEXT.TRAITS.FILTER_TYPE_ALL}</option>
            <option value="root_bone">{ADMIN_TEXT.FIELDS.TYPE_ROOT_BONE}</option>
            <option value="talent">{ADMIN_TEXT.FIELDS.TYPE_TALENT}</option>
          </select>
          {/* Rarity filter */}
          <select
            className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
            onChange={(e) => crud.setFilter("rarity_id", e.target.value || null, "exact")}
            disabled={!canReadRarities || rarityCatalog.status === "loading"}
          >
            <option value="">{ADMIN_TEXT.TRAITS.FILTER_RARITY_ALL}</option>
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
          emptyMessage={ADMIN_TEXT.TRAITS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <TraitDialog
          key={`${crud.dialogOpen}:${crud.editing?.id ?? "create"}`}
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onArchive={async (expectedVersion, reason) => {
            if (!crud.editing) return;
            await cultivationService.deleteTrait(
              crud.editing.id,
              { expected_version: expectedVersion, reason },
            );
            notify.success(ADMIN_TEXT.TRAITS.ARCHIVE_SUCCESS);
            crud.closeDialog();
            crud.refresh();
          }}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
          rarities={rarities}
        />
      }
      confirmDialog={null}
    />
  );
}
