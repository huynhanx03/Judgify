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
import { TEXT } from "@/constants/text";

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
      <div className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
        <div className="min-w-0">
          <h1 className="break-words text-2xl font-bold tracking-tight">
            {title}
          </h1>
          {subtitle && (
            <p className="mt-1 break-words text-sm text-muted-foreground">
              {subtitle}
            </p>
          )}
        </div>
        {canCreate ? (
          <Button
            className="w-full gap-2 cursor-pointer sm:w-auto"
            onClick={onCreateClick}
          >
            <Plus className="h-4 w-4" />
            {createLabel}
          </Button>
        ) : null}
      </div>

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

      {loadError ? (
        <div
          className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive"
          role="alert"
        >
          <span>{loadError}</span>
          {onRetryLoad ? (
            <Button type="button" variant="outline" size="sm" onClick={onRetryLoad}>
              {TEXT.COMMON.RETRY}
            </Button>
          ) : null}
        </div>
      ) : null}

      {/* Data table */}
      {table}

      {/* Dialogs rendered outside table to avoid stacking issues */}
      {dialog}
      {confirmDialog}
    </div>
  );
}
