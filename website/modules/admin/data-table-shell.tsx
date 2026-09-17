"use client";

/**
 * Presentational layout shell for admin CRUD pages.
 * Zero business logic — arranges header, search, filters, table, and dialogs.
 */

import { useId, type ReactNode } from "react";
import { Loader2, Search, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PageHeader } from "@/components/page-header";
import { TEXT } from "@/constants/text";
import { ResourceCollectionFeedback } from "@/modules/shared/resource-feedback";

interface DataTableShellProps {
  title: string;
  subtitle?: string;
  createLabel: string;
  canCreate: boolean;
  onCreateClick: () => void;
  searchValue: string;
  onSearch: (value: string) => void;
  searchLabel?: string;
  searchPlaceholder?: string;
  searchError?: string | null;
  isLoading?: boolean;
  loadError?: string | null;
  onRetryLoad?: () => void;
  /** Extra filter controls rendered beside the search input (e.g. selects) */
  extraFilters?: ReactNode;
  table: ReactNode;
  /** CRUD form dialog — pass null/undefined for pages that navigate instead of dialog */
  dialog?: ReactNode;
  confirmDialog: ReactNode;
}

export function DataTableShell({
  title,
  subtitle,
  createLabel,
  canCreate,
  onCreateClick,
  searchValue,
  onSearch,
  searchLabel = TEXT.COMMON.SEARCH,
  searchPlaceholder = TEXT.COMMON.SEARCH_PLACEHOLDER,
  searchError,
  isLoading = false,
  loadError,
  onRetryLoad,
  extraFilters,
  table,
  dialog,
  confirmDialog,
}: DataTableShellProps) {
  const searchId = useId();
  const searchErrorId = `${searchId}-error`;

  return (
    <div className="space-y-6" aria-busy={isLoading}>
      {/* Header */}
      <PageHeader
        title={title}
        description={subtitle}
        actions={canCreate ? (
          <Button
            className="w-full gap-2 cursor-pointer sm:w-auto"
            onClick={onCreateClick}
          >
            <Plus className="h-4 w-4" />
            {createLabel}
          </Button>
        ) : undefined}
      />

      {/* Search + extra filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="w-full sm:max-w-sm">
          <Label htmlFor={searchId} className="sr-only">
            {searchLabel}
          </Label>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              id={searchId}
              placeholder={searchPlaceholder}
              className="pl-9"
              value={searchValue}
              aria-invalid={Boolean(searchError)}
              aria-describedby={searchError ? searchErrorId : undefined}
              onChange={(e) => onSearch(e.target.value)}
            />
          </div>
          {searchError ? (
            <p id={searchErrorId} className="mt-1.5 text-sm text-destructive">
              {searchError}
            </p>
          ) : null}
        </div>
        {extraFilters}
        {isLoading ? (
          <div
            className="flex items-center gap-2 text-sm text-muted-foreground"
            role="status"
            aria-live="polite"
          >
            <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
            <span>{TEXT.COMMON.LOADING}</span>
          </div>
        ) : null}
      </div>

      {/* A failed collection occupies the table's exact content slot. */}
      {loadError ? (
        onRetryLoad ? (
          <ResourceCollectionFeedback
            className="min-h-72"
            title={TEXT.COMMON.ERROR}
            description={loadError}
            retryLabel={TEXT.COMMON.RETRY}
            onRetry={onRetryLoad}
          />
        ) : (
          <div className="flex min-h-72 items-center justify-center px-6 text-center text-sm text-muted-foreground" role="alert">
            {loadError}
          </div>
        )
      ) : table}

      {/* Dialogs rendered outside table to avoid stacking issues */}
      {dialog}
      {confirmDialog}
    </div>
  );
}
