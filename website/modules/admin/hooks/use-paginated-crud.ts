"use client";

/**
 * Generic hook for any paginated CRUD resource.
 * Owns: data fetching, pagination, search (debounced), extra filters,
 * dialog open/close, create/edit/delete operations.
 */

import { useState, useEffect, useCallback, useRef } from "react";
import { notify, getErrorMessage } from "@/lib/toast";
import type { Paginated, QueryOptions, SearchFilter, PaginationMeta } from "@/types/api";
import { TEXT } from "@/constants/text";

const DEFAULT_PAGE_SIZE = 10;
const SEARCH_DEBOUNCE_MS = 300;

export interface PaginatedCRUDService<T> {
  find: (query: QueryOptions) => Promise<Paginated<T>>;
  create?: (input: any) => Promise<T>;
  update?: (id: number, input: any) => Promise<T>;
  delete: (id: number) => Promise<void>;
}

interface UsePaginatedCRUDOptions<T> {
  service: PaginatedCRUDService<T>;
  pageSize?: number;
  /** Field name used for the text search filter. Default: "name" */
  searchKey?: string;
}

export interface UsePaginatedCRUDReturn<T> {
  data: T[];
  isLoading: boolean;
  pagination: PaginationMeta | undefined;
  onPageChange: (page: number) => void;
  search: string;
  onSearch: (value: string) => void;
  /** Set an extra filter (non-search). Resets to page 1. */
  setFilter: (key: string, value: unknown, type?: "exact" | "search") => void;
  /** Remove an extra filter by key. Resets to page 1. */
  clearFilter: (key: string) => void;
  dialogOpen: boolean;
  editing: T | null;
  openCreate: () => void;
  openEdit: (item: T) => void;
  closeDialog: () => void;
  isSaving: boolean;
  handleSave: (input: any) => Promise<void>;
  deleteId: number | null;
  confirmDelete: (id: number) => void;
  cancelDelete: () => void;
  handleDelete: () => Promise<void>;
}

export function usePaginatedCRUD<T>({
  service,
  pageSize = DEFAULT_PAGE_SIZE,
  searchKey = "name",
}: UsePaginatedCRUDOptions<T>): UsePaginatedCRUDReturn<T> {
  const [data, setData] = useState<T[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [extraFilters, setExtraFilters] = useState<SearchFilter[]>([]);
  const [refreshKey, setRefreshKey] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<T | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  // Keep stable service ref — avoids infinite re-renders when caller passes inline object
  const serviceRef = useRef(service);
  serviceRef.current = service;

  // Debounce search input — resets to page 1 when triggered
  useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedSearch(search);
      setPage(1);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [search]);

  // Main fetch effect — runs on page / search / filters / manual refresh
  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);

    const query: QueryOptions = {
      pagination: { page, page_size: pageSize },
      filters: [
        ...(debouncedSearch.trim()
          ? [{ key: searchKey, value: debouncedSearch.trim(), type: "search" }]
          : []),
        ...extraFilters.filter((f) => f.value !== "" && f.value != null),
      ],
    };

    serviceRef.current
      .find(query)
      .then((res) => {
        if (!cancelled) {
          setData(res.records);
          setPagination(res.pagination);
        }
      })
      .catch(() => {
        if (!cancelled) notify.error(TEXT.ADMIN.LOAD_ERROR);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [page, debouncedSearch, extraFilters, pageSize, searchKey, refreshKey]);

  const onSearch = useCallback((value: string) => setSearch(value), []);

  const setFilter = useCallback(
    (key: string, value: unknown, type: "exact" | "search" = "exact") => {
      setExtraFilters((prev) => {
        const next = prev.filter((f) => f.key !== key);
        if (value !== "" && value != null) next.push({ key, value, type });
        return next;
      });
      setPage(1);
    },
    []
  );

  const clearFilter = useCallback((key: string) => {
    setExtraFilters((prev) => prev.filter((f) => f.key !== key));
    setPage(1);
  }, []);

  const onPageChange = useCallback((p: number) => setPage(p), []);

  const openCreate = useCallback(() => {
    setEditing(null);
    setDialogOpen(true);
  }, []);

  const openEdit = useCallback((item: T) => {
    setEditing(item);
    setDialogOpen(true);
  }, []);

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    setEditing(null);
  }, []);

  const handleSave = useCallback(async (input: any) => {
    setIsSaving(true);
    try {
      if (editing) {
        await serviceRef.current.update!((editing as any).id, input);
        notify.success(TEXT.ADMIN.UPDATE_SUCCESS);
      } else {
        await serviceRef.current.create!(input);
        notify.success(TEXT.ADMIN.CREATE_SUCCESS);
      }
      setDialogOpen(false);
      setEditing(null);
      setRefreshKey((k) => k + 1);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.SAVE_ERROR));
    } finally {
      setIsSaving(false);
    }
  }, [editing]);

  const confirmDelete = useCallback((id: number) => setDeleteId(id), []);
  const cancelDelete = useCallback(() => setDeleteId(null), []);

  const handleDelete = useCallback(async () => {
    if (deleteId == null) return;
    try {
      await serviceRef.current.delete(deleteId);
      notify.success(TEXT.ADMIN.DELETE_SUCCESS);
      setRefreshKey((k) => k + 1);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.DELETE_ERROR));
    } finally {
      setDeleteId(null);
    }
  }, [deleteId]);

  return {
    data, isLoading, pagination, onPageChange,
    search, onSearch,
    setFilter, clearFilter,
    dialogOpen, editing, openCreate, openEdit, closeDialog,
    isSaving, handleSave,
    deleteId, confirmDelete, cancelDelete, handleDelete,
  };
}
