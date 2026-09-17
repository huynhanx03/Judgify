"use client";

/**
 * Generic hook for any paginated CRUD resource.
 * Owns: data fetching, pagination, search (debounced), extra filters,
 * dialog open/close, create/edit/delete operations.
 */

import { useState, useEffect, useCallback, useMemo } from "react";
import { notify, getErrorMessage } from "@/lib/toast";
import type { Paginated, QueryOptions, SearchFilter, PaginationMeta } from "@/types/api";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { AUTHORIZATION_ACTION } from "@/constants/authorization";
import { ADMIN_RESOURCE_LIST } from "@/constants/admin";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";

export interface PaginatedCRUDService<
  T,
  TCreateInput = never,
  TUpdateInput = TCreateInput,
> {
  find: (query: QueryOptions, signal?: AbortSignal) => Promise<Paginated<T>>;
  create?: (input: TCreateInput) => Promise<unknown>;
  update?: (id: string, input: TUpdateInput) => Promise<unknown>;
  delete?: (id: string) => Promise<void>;
}

interface UsePaginatedCRUDOptions<
  T,
  TCreateInput = never,
  TUpdateInput = TCreateInput,
> {
  service: PaginatedCRUDService<T, TCreateInput, TUpdateInput>;
  resource: string;
  pageSize?: number;
  /** Field name used for the text search filter. Default: "name" */
  searchKey?: string;
  /** Predicate used by the search input. Default: case-insensitive text search. */
  searchType?: SearchFilter["type"];
  /** Reject incomplete or malformed searches before they reach the API. */
  validateSearch?: (value: string) => boolean;
  invalidSearchMessage?: string;
}

export interface UsePaginatedCRUDReturn<
  T,
  TCreateInput = never,
  TUpdateInput = TCreateInput,
> {
  data: T[];
  isLoading: boolean;
  hasSettled: boolean;
  loadError: string | null;
  searchError: string | null;
  pagination: PaginationMeta | undefined;
  onPageChange: (page: number) => void;
  refresh: () => void;
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
  handleCreate: (input: TCreateInput) => Promise<void>;
  handleUpdate: (input: TUpdateInput) => Promise<void>;
  /** Compatibility helper for resources whose create and update contracts match. */
  handleSave: (input: TCreateInput | TUpdateInput) => Promise<void>;
  deleteId: string | null;
  confirmDelete: (id: string) => void;
  cancelDelete: () => void;
  handleDelete: () => Promise<void>;
}

export function usePaginatedCRUD<
  T extends { id: string },
  TCreateInput = never,
  TUpdateInput = TCreateInput,
>({
  service,
  resource,
  pageSize = ADMIN_RESOURCE_LIST.PAGE_SIZE,
  searchKey = "name",
  searchType = "search",
  validateSearch,
  invalidSearchMessage = ADMIN_TEXT.FIELDS.UUID_INVALID,
}: UsePaginatedCRUDOptions<T, TCreateInput, TUpdateInput>): UsePaginatedCRUDReturn<
  T,
  TCreateInput,
  TUpdateInput
> {
  const { can, authorizationRevision } = useAuth();
  const canCreate = can(resource, AUTHORIZATION_ACTION.CREATE);
  const canUpdate = can(resource, AUTHORIZATION_ACTION.UPDATE);
  const canDelete =
    typeof service.delete === "function" &&
    can(resource, AUTHORIZATION_ACTION.DELETE);
  const [data, setData] = useState<T[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [extraFilters, setExtraFilters] = useState<SearchFilter[]>([]);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogRevision, setDialogRevision] = useState<number | null>(null);
  const [editing, setEditing] = useState<T | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleteRevision, setDeleteRevision] = useState<number | null>(null);

  // Debounce search input — resets to page 1 when triggered
  useEffect(() => {
    if (search === debouncedSearch) return;
    const t = setTimeout(() => {
      setDebouncedSearch(search);
      setPage(1);
    }, ADMIN_RESOURCE_LIST.SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [debouncedSearch, search]);

  const normalizedSearch = debouncedSearch.trim();
  const searchError =
    normalizedSearch.length > 0 &&
    validateSearch &&
    !validateSearch(normalizedSearch)
      ? invalidSearchMessage
      : null;
  const query = useMemo<QueryOptions>(
    () => ({
      pagination: { page, page_size: pageSize },
      filters: [
        ...(normalizedSearch
          ? [{ key: searchKey, value: normalizedSearch, type: searchType }]
          : []),
        ...extraFilters.filter((f) => f.value !== "" && f.value != null),
      ],
    }),
    [extraFilters, normalizedSearch, page, pageSize, searchKey, searchType],
  );
  const resourceKey = useMemo(
    () => `${authorizationRevision}:${JSON.stringify(query)}`,
    [authorizationRevision, query],
  );
  const listResource = useRetryableResource<Paginated<T> | null>({
    resetKey: resourceKey,
    enabled: searchError === null,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => service.find(query, signal),
    onSuccess: (response) => {
      if (!response) return;
      setPagination(response.pagination);
      if (response.pagination.current_page !== page) {
        setPage(response.pagination.current_page);
        return;
      }
      setData(response.records);
    },
  });
  const retryList = listResource.retry;
  const isLoading = listResource.status === "loading";
  const hasSettled = !isLoading || pagination !== undefined;
  const loadError =
    listResource.status === "error"
      ? getErrorMessage(listResource.error, ADMIN_TEXT.LOAD_ERROR)
      : null;

  const onSearch = useCallback((value: string) => {
    setSearch(value);
  }, []);

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

  const onPageChange = useCallback((p: number) => {
    setPage(p);
  }, []);
  const refresh = retryList;

  const openCreate = useCallback(() => {
    if (!canCreate) return;
    setEditing(null);
    setDialogRevision(authorizationRevision);
    setDialogOpen(true);
  }, [authorizationRevision, canCreate]);

  const openEdit = useCallback((item: T) => {
    if (!canUpdate) return;
    setEditing(item);
    setDialogRevision(authorizationRevision);
    setDialogOpen(true);
  }, [authorizationRevision, canUpdate]);

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    setDialogRevision(null);
    setEditing(null);
  }, []);

  const finishSave = useCallback(() => {
    setDialogOpen(false);
    setDialogRevision(null);
    setEditing(null);
    retryList();
  }, [retryList]);

  const handleCreate = useCallback(async (input: TCreateInput) => {
    const createItem = service.create;
    if (!canCreate || !createItem) {
      setDialogOpen(false);
      setDialogRevision(null);
      notify.error(ADMIN_TEXT.ACCESS.FORBIDDEN_TITLE);
      return;
    }
    setIsSaving(true);
    try {
      await createItem(input);
      notify.success(ADMIN_TEXT.CREATE_SUCCESS);
      finishSave();
    } catch (err) {
      notify.error(getErrorMessage(err, ADMIN_TEXT.SAVE_ERROR));
    } finally {
      setIsSaving(false);
    }
  }, [canCreate, finishSave, service]);

  const handleUpdate = useCallback(async (input: TUpdateInput) => {
    const updateItem = service.update;
    if (!canUpdate || !editing || !updateItem) {
      setDialogOpen(false);
      setDialogRevision(null);
      notify.error(ADMIN_TEXT.ACCESS.FORBIDDEN_TITLE);
      return;
    }
    setIsSaving(true);
    try {
      await updateItem(editing.id, input);
      notify.success(ADMIN_TEXT.UPDATE_SUCCESS);
      finishSave();
    } catch (err) {
      notify.error(getErrorMessage(err, ADMIN_TEXT.SAVE_ERROR));
    } finally {
      setIsSaving(false);
    }
  }, [canUpdate, editing, finishSave, service]);

  const handleSave = useCallback(
    async (input: TCreateInput | TUpdateInput) => {
      if (editing) {
        await handleUpdate(input as TUpdateInput);
      } else {
        await handleCreate(input as TCreateInput);
      }
    },
    [editing, handleCreate, handleUpdate],
  );

  const confirmDelete = useCallback((id: string) => {
    if (!canDelete) return;
    setDeleteRevision(authorizationRevision);
    setDeleteId(id);
  }, [authorizationRevision, canDelete]);
  const cancelDelete = useCallback(() => {
    setDeleteId(null);
    setDeleteRevision(null);
  }, []);

  const handleDelete = useCallback(async () => {
    const deleteItem = service.delete;
    if (deleteId == null || !canDelete || !deleteItem) {
      setDeleteId(null);
      setDeleteRevision(null);
      if (!canDelete) notify.error(ADMIN_TEXT.ACCESS.FORBIDDEN_TITLE);
      return;
    }
    try {
      await deleteItem(deleteId);
      notify.success(ADMIN_TEXT.DELETE_SUCCESS);
      if (data.length === 1 && page > 1) {
        setPage((current) => current - 1);
      } else {
        retryList();
      }
    } catch (err) {
      notify.error(getErrorMessage(err, ADMIN_TEXT.DELETE_ERROR));
    } finally {
      setDeleteId(null);
      setDeleteRevision(null);
    }
  }, [canDelete, data.length, deleteId, page, retryList, service]);

  const effectiveDialogOpen =
    dialogOpen &&
    dialogRevision === authorizationRevision &&
    (editing ? canUpdate : canCreate);
  const effectiveDeleteId =
    canDelete && deleteRevision === authorizationRevision ? deleteId : null;

  return {
    data, isLoading, hasSettled, loadError, searchError, pagination, onPageChange, refresh,
    search, onSearch,
    setFilter, clearFilter,
    dialogOpen: effectiveDialogOpen,
    editing: effectiveDialogOpen ? editing : null,
    openCreate, openEdit, closeDialog,
    isSaving, handleCreate, handleUpdate, handleSave,
    deleteId: effectiveDeleteId,
    confirmDelete, cancelDelete, handleDelete,
  };
}
