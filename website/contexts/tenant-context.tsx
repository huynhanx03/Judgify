"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useSession } from "@/contexts/session-context";
import { apiTransport } from "@/lib/api/client";
import { tenantService } from "@/services/tenant.service";
import type { Tenant } from "@/types/tenant";

interface TenantContextValue {
  tenants: readonly Tenant[];
  selectedTenantId: string | null;
  isAllTenants: boolean;
  loading: boolean;
  selectTenant(id: string | null): void;
  createTenant(name: string): Promise<Tenant>;
  renameTenant(id: string, name: string): Promise<Tenant>;
  refresh(): Promise<void>;
}

const TenantContext = createContext<TenantContextValue | null>(null);

export function TenantProvider({ children }: { children: ReactNode }) {
  const { state: sessionState } = useSession();
  const userID = sessionState.status === "authenticated" ? sessionState.session.user.id : null;
  const [tenants, setTenants] = useState<readonly Tenant[]>([]);
  const [selectedTenantId, setSelectedTenantId] = useState<string | null>(null);
  const [isAllTenants, setIsAllTenants] = useState(false);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    if (!userID) {
      setTenants([]);
      setSelectedTenantId(null);
      setIsAllTenants(false);
      setLoading(false);
      apiTransport.setActiveTenantId(null);
      return;
    }
    setLoading(true);
    apiTransport.setActiveTenantId(null);
    const result = await tenantService.findForUser();
    setTenants(result.tenants);
    const stored = window.localStorage.getItem(`judgify.activeTenant.${userID}`);
    if (stored === "all" && result.tenants.length > 0) {
      setSelectedTenantId(null);
      setIsAllTenants(true);
      apiTransport.setActiveTenantId(null);
      setLoading(false);
      return;
    }
    const selected = result.tenants.find((tenant) => tenant.id === stored);
    if (selected) {
      setSelectedTenantId(selected.id);
      setIsAllTenants(false);
      apiTransport.setActiveTenantId(selected.id);
    } else if (result.tenants.length > 0) {
      setSelectedTenantId(result.tenants[0].id);
      setIsAllTenants(false);
      window.localStorage.setItem(`judgify.activeTenant.${userID}`, result.tenants[0].id);
      apiTransport.setActiveTenantId(result.tenants[0].id);
    } else {
      setSelectedTenantId(null);
      setIsAllTenants(false);
      apiTransport.setActiveTenantId(null);
    }
    setLoading(false);
  }, [userID]);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    if (!userID) {
      setTenants([]);
      setSelectedTenantId(null);
      setIsAllTenants(false);
      apiTransport.setActiveTenantId(null);
      setLoading(sessionState.status === "loading");
      return () => controller.abort();
    }
    setLoading(true);
    tenantService.findForUser(controller.signal).then((result) => {
      if (!active) return;
      setTenants(result.tenants);
      const stored = window.localStorage.getItem(`judgify.activeTenant.${userID}`);
      if (stored === "all" && result.tenants.length > 0) {
        setSelectedTenantId(null);
        setIsAllTenants(true);
        apiTransport.setActiveTenantId(null);
        setLoading(false);
        return;
      }
      const selected = result.tenants.find((tenant) => tenant.id === stored);
      const nextID = selected?.id ?? result.tenants[0]?.id ?? null;
      setSelectedTenantId(nextID);
      setIsAllTenants(false);
      if (nextID) window.localStorage.setItem(`judgify.activeTenant.${userID}`, nextID);
      apiTransport.setActiveTenantId(nextID);
      setLoading(false);
    }).catch(() => {
      if (active) setLoading(false);
    });
    return () => {
      active = false;
      controller.abort();
    };
  }, [userID, sessionState.status]);

  const selectTenant = useCallback((id: string | null) => {
    if (id === null) {
      if (tenants.length === 0) return;
      setSelectedTenantId(null);
      setIsAllTenants(true);
      if (userID) window.localStorage.setItem(`judgify.activeTenant.${userID}`, "all");
      apiTransport.setActiveTenantId(null);
      window.location.reload();
      return;
    }
    if (!tenants.some((tenant) => tenant.id === id)) return;
    setSelectedTenantId(id);
    setIsAllTenants(false);
    if (userID) window.localStorage.setItem(`judgify.activeTenant.${userID}`, id);
    apiTransport.setActiveTenantId(id);
    window.location.reload();
  }, [tenants, userID]);

  const createTenant = useCallback(async (name: string) => {
    const created = await tenantService.create(name);
    setTenants((current) => [...current, created]);
    setSelectedTenantId(created.id);
    setIsAllTenants(false);
    if (userID) window.localStorage.setItem(`judgify.activeTenant.${userID}`, created.id);
    apiTransport.setActiveTenantId(created.id);
    window.location.reload();
    return created;
  }, [userID]);

  const renameTenant = useCallback(async (id: string, name: string) => {
    const updated = await tenantService.rename(id, name);
    setTenants((current) => current.map((tenant) => tenant.id === updated.id ? updated : tenant));
    return updated;
  }, []);

  const value = useMemo<TenantContextValue>(() => ({
    tenants,
    selectedTenantId,
    isAllTenants,
    loading,
    selectTenant,
    createTenant,
    renameTenant,
    refresh,
  }), [tenants, selectedTenantId, isAllTenants, loading, selectTenant, createTenant, renameTenant, refresh]);

  return <TenantContext.Provider value={value}>{children}</TenantContext.Provider>;
}

export function useTenant(): TenantContextValue {
  const context = useContext(TenantContext);
  if (!context) throw new Error("useTenant must be used within a TenantProvider");
  return context;
}

export function useOptionalTenant(): TenantContextValue | null {
  return useContext(TenantContext);
}
