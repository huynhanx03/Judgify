"use client";

/**
 * Admin problems page — paginated list with search and difficulty filter.
 * Create/Edit navigate to dedicated pages — no dialog.
 */

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Badge } from "@/components/ui/badge";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { problemService } from "@/services/problem.service";
import { difficultyService } from "@/services/difficulty.service";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { APP_ROUTES } from "@/constants/routes";
import { formatDuration, formatMemory } from "@/lib/format";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { DataLoadFeedback } from "@/modules/problem/data-load-feedback";
import type { Problem } from "@/types/problem";
import type { DifficultyResponse } from "@/types/difficulty";

export default function AdminProblemsPage() {
  const router = useRouter();
  const { can } = useAuth();
  const resource = AUTHORIZATION_RESOURCE.PROBLEM;
  const canReadDifficulties = can(
    AUTHORIZATION_RESOURCE.DIFFICULTY,
    AUTHORIZATION_ACTION.READ,
  );
  const difficultyCatalog = useRetryableResource<DifficultyResponse[]>({
    resetKey: canReadDifficulties,
    enabled: canReadDifficulties,
    initialData: [],
    load: difficultyService.getAll,
  });
  const difficulties = difficultyCatalog.data;

  // Lifecycle transitions live in the immutable editor; there is no destructive
  // delete action for problem aggregates.
  const service = useMemo(() => ({
    find: problemService.findAdmin.bind(problemService),
  }), []);

  const crud = usePaginatedCRUD({ service, resource, searchKey: "title" });

  const columns: AdminColumn<Problem>[] = [
    { key: "id", label: ADMIN_TEXT.COLUMN_ID, className: "w-16",
      render: (p) => <span className="text-xs tabular-nums text-muted-foreground">{p.id}</span> },
    { key: "title", label: ADMIN_TEXT.PROBLEMS.COL_TITLE, className: "max-w-[300px]",
      render: (p) => <p className="font-medium truncate" title={p.title}>{p.title}</p> },
    { key: "tags", label: ADMIN_TEXT.PROBLEMS.COL_TAGS, className: "max-w-[200px]",
      render: (p) => {
        if (!p.tags?.length) return <span className="text-xs text-muted-foreground">{TEXT.COMMON.NOT_AVAILABLE}</span>;
        return (
          <div className="flex gap-1 flex-wrap">
            {p.tags.slice(0, 3).map((t) => (
              <span key={t.id} className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">{t.name}</span>
            ))}
            {p.tags.length > 3 && (
              <span className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">+{p.tags.length - 3}</span>
            )}
          </div>
        );
      } },
    { key: "difficulty", label: ADMIN_TEXT.PROBLEMS.COL_DIFFICULTY, className: "w-28",
      render: (p) => <Badge variant="outline" className="text-[11px] border">{p.difficulty?.name ?? TEXT.COMMON.NOT_AVAILABLE}</Badge> },
    { key: "published", label: ADMIN_TEXT.PROBLEMS.COL_STATUS, className: "w-28",
      render: (p) => <Badge variant={p.is_published ? "default" : "outline"} className="text-[11px]">{p.is_published ? ADMIN_TEXT.PROBLEM_FORM.PUBLISHED : ADMIN_TEXT.PROBLEM_FORM.DRAFT}</Badge> },
    { key: "limits", label: ADMIN_TEXT.PROBLEMS.COL_LIMITS, className: "w-36",
      render: (p) => (
        <div className="flex gap-2 text-xs text-muted-foreground">
          <span>{formatDuration(p.time_limit_ms)}</span>
          <span className="text-muted-foreground/40">|</span>
          <span>{formatMemory(p.memory_limit_kb)}</span>
        </div>
      ) },
    { key: "actions", label: "", className: "w-20",
      render: (p) => (
        <AdminResourceActions
          resource={resource}
          onEdit={() => router.push(APP_ROUTES.ADMIN_PROBLEM_EDIT(p.id))}
        />
      ) },
  ];

  if (crud.isLoading && !crud.hasSettled) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={ADMIN_TEXT.PROBLEMS.TITLE}
      subtitle={ADMIN_TEXT.PROBLEMS.SUBTITLE}
      createLabel={ADMIN_TEXT.PROBLEMS.CREATE}
      canCreate={can(resource, AUTHORIZATION_ACTION.CREATE)}
      onCreateClick={() => router.push(APP_ROUTES.ADMIN_PROBLEM_CREATE)}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      isLoading={crud.isLoading}
      loadError={crud.loadError}
      onRetryLoad={crud.refresh}
      searchPlaceholder={ADMIN_TEXT.PROBLEMS.SEARCH_PLACEHOLDER}
      extraFilters={canReadDifficulties ? (
        difficultyCatalog.status === "error" ? (
          <DataLoadFeedback
            compact
            title={ADMIN_TEXT.PROBLEMS.FILTER_DIFFICULTY_ERROR}
            retryLabel={ADMIN_TEXT.PROBLEMS.FILTER_DIFFICULTY_RETRY}
            onRetry={difficultyCatalog.retry}
          />
        ) : (
          <select
            aria-label={ADMIN_TEXT.PROBLEMS.FILTER_DIFFICULTY_LABEL}
            disabled={difficultyCatalog.status === "loading"}
            className="flex h-11 rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-wait disabled:opacity-60"
            onChange={(event) =>
              crud.setFilter(
                "difficulty_id",
                event.target.value || null,
                "exact",
              )
            }
          >
            <option value="">
              {difficultyCatalog.status === "loading"
                ? ADMIN_TEXT.PROBLEMS.FILTER_DIFFICULTY_LOADING
                : ADMIN_TEXT.PROBLEMS.FILTER_DIFFICULTY_ALL}
            </option>
            {difficulties.map((difficulty) => (
              <option key={difficulty.id} value={difficulty.id}>
                {difficulty.name}
              </option>
            ))}
          </select>
        )
      ) : null}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(p) => p.id}
          emptyMessage={ADMIN_TEXT.PROBLEMS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      confirmDialog={null}
    />
  );
}
