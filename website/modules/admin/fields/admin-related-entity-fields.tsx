"use client";

import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { ADMIN_USER_LOOKUP } from "@/constants/admin";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { tryEntityID } from "@/lib/api/contracts";
import { userService } from "@/services/user.service";
import type { AdminUser } from "@/types/admin";
import type { EntityID } from "@/types/api";

interface AdminUserLookupProps {
  id: string;
  value: EntityID | null;
  onChange: (value: EntityID | null) => void;
  enabled: boolean;
  disabled?: boolean;
}

export function AdminUserLookup({
  id,
  value,
  onChange,
  enabled,
  disabled = false,
}: AdminUserLookupProps) {
  const [query, setQuery] = useState("");
  const debouncedQuery = useDebouncedValue(
    query.trim(),
    ADMIN_USER_LOOKUP.SEARCH_DEBOUNCE_MS,
  );
  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null);
  const searchId = `${id}-search`;
  const selectId = `${id}-select`;

  const usersResource = useRetryableResource<AdminUser[]>({
    resetKey: `${enabled}:${debouncedQuery}`,
    enabled,
    initialData: [],
    keepPreviousData: true,
    load: async (signal) => {
      const response = await userService.find(
        {
          pagination: {
            page: 1,
            page_size: ADMIN_USER_LOOKUP.PAGE_SIZE,
          },
          filters: debouncedQuery
            ? [
                {
                  key: "username",
                  value: debouncedQuery,
                  type: "search",
                },
              ]
            : [],
        },
        signal,
      );
      return response.records;
    },
    onSuccess: (records) => {
      const current = records.find((user) => user.id === value);
      if (current) setSelectedUser(current);
    },
  });
  const users = usersResource.data;
  const isLoading = usersResource.status === "loading";
  const hasError = usersResource.status === "error";

  const options = useMemo(() => {
    if (
      selectedUser &&
      selectedUser.id === value &&
      !users.some((user) => user.id === selectedUser.id)
    ) {
      return [selectedUser, ...users];
    }
    return users;
  }, [selectedUser, users, value]);

  function selectUser(candidate: string) {
    const userID = tryEntityID(candidate);
    if (!userID) {
      setSelectedUser(null);
      onChange(null);
      return;
    }
    setSelectedUser(options.find((user) => user.id === userID) ?? null);
    onChange(userID);
  }

  return (
    <div className="space-y-2">
      <Label htmlFor={searchId}>{ADMIN_TEXT.LOOKUPS.USER_SEARCH_LABEL}</Label>
      <div className="relative">
        <Input
          id={searchId}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={ADMIN_TEXT.LOOKUPS.USER_SEARCH_PLACEHOLDER}
          disabled={disabled || !enabled}
          autoComplete="off"
          aria-describedby={hasError ? `${id}-error` : undefined}
        />
        {isLoading ? (
          <Loader2
            className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-muted-foreground motion-reduce:animate-none"
            aria-label={ADMIN_TEXT.LOOKUPS.USER_LOADING}
          />
        ) : null}
      </div>
      <Label htmlFor={selectId} className="sr-only">
        {ADMIN_TEXT.LOOKUPS.USER_RESULTS_LABEL}
      </Label>
      <select
        id={selectId}
        className="flex h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 py-2 text-sm"
        value={value ?? ""}
        onChange={(event) => selectUser(event.target.value)}
        disabled={disabled || !enabled || isLoading || hasError}
      >
        <option value="">{ADMIN_TEXT.LOOKUPS.USER_SELECT_PLACEHOLDER}</option>
        {options.map((user) => (
          <option key={user.id} value={user.id}>
            {user.username} · {user.id}
          </option>
        ))}
      </select>
      {!isLoading && !hasError && users.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {ADMIN_TEXT.LOOKUPS.USER_EMPTY}
        </p>
      ) : null}
      {hasError ? (
        <div
          id={`${id}-error`}
          className="flex items-center justify-between gap-3 text-sm text-destructive"
          role="alert"
        >
          <span>{ADMIN_TEXT.LOOKUPS.USER_LOAD_ERROR}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={usersResource.retry}
            disabled={disabled}
          >
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

export interface AdminEntityOption {
  id: EntityID;
  label: string;
}

interface AdminEntitySelectFieldProps {
  id: string;
  label: string;
  placeholder: string;
  value: EntityID | null;
  options: AdminEntityOption[];
  onChange: (value: EntityID | null) => void;
  disabled?: boolean;
}

export function AdminEntitySelectField({
  id,
  label,
  placeholder,
  value,
  options,
  onChange,
  disabled = false,
}: AdminEntitySelectFieldProps) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <select
        id={id}
        className="flex h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 py-2 text-sm"
        value={value ?? ""}
        onChange={(event) => onChange(tryEntityID(event.target.value))}
        disabled={disabled || options.length === 0}
        aria-describedby={options.length === 0 ? `${id}-empty` : undefined}
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.id} value={option.id}>
            {option.label}
          </option>
        ))}
      </select>
      {options.length === 0 ? (
        <p id={`${id}-empty`} className="text-sm text-muted-foreground">
          {ADMIN_TEXT.LOOKUPS.CATALOG_EMPTY}
        </p>
      ) : null}
    </div>
  );
}
