"use client";

/**
 * Presentational layout shell for admin CRUD pages.
 * Zero business logic — arranges header, search, filters, table, and dialogs.
 */

import type { ReactNode } from "react";
import { Search, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface DataTableShellProps {
  title: string;
  subtitle?: string;
  createLabel: string;
  onCreateClick: () => void;
  searchValue: string;
  onSearch: (value: string) => void;
  searchPlaceholder?: string;
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
  onCreateClick,
  searchValue,
  onSearch,
  searchPlaceholder = "Tìm kiếm...",
  extraFilters,
  table,
  dialog,
  confirmDialog,
}: DataTableShellProps) {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
          {subtitle && (
            <p className="text-sm text-muted-foreground mt-1">{subtitle}</p>
          )}
        </div>
        <Button className="gap-2 cursor-pointer" onClick={onCreateClick}>
          <Plus className="h-4 w-4" />
          {createLabel}
        </Button>
      </div>

      {/* Search + extra filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={searchPlaceholder}
            className="pl-9"
            value={searchValue}
            onChange={(e) => onSearch(e.target.value)}
          />
        </div>
        {extraFilters}
      </div>

      {/* Data table */}
      {table}

      {/* Dialogs rendered outside table to avoid stacking issues */}
      {dialog}
      {confirmDialog}
    </div>
  );
}
