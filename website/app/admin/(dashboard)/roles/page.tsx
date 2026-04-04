"use client";

/**
 * Admin role & permission management.
 * Top: horizontal role tabs. Below: flat permission matrix for selected role.
 * Dialogs: create/edit role, manage resources.
 */

import { useEffect, useState, useCallback, useRef } from "react";
import { Loader2, Plus, Pencil, Package, Save, Trash2, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog";
import { PermissionMatrix } from "@/modules/admin/permission-matrix";
import { adminService } from "@/services/admin.service";
import { notify, getErrorMessage } from "@/lib/toast";
import { TEXT } from "@/constants/text";
import type { Role, Permission, Resource } from "@/types/admin";
import { cn } from "@/lib/utils";

export default function AdminRolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [resources, setResources] = useState<Resource[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSavingPerms, setIsSavingPerms] = useState(false);

  // Snapshot of server permissions — used to diff on save
  const originalPermsRef = useRef<Permission[]>([]);

  // Dialog states
  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [roleName, setRoleName] = useState("");
  const [roleLevel, setRoleLevel] = useState(0);
  const [isSavingRole, setIsSavingRole] = useState(false);

  const [resourceDialogOpen, setResourceDialogOpen] = useState(false);
  const [newResourceKey, setNewResourceKey] = useState("");
  const [newResourceDesc, setNewResourceDesc] = useState("");
  const [resourceSearch, setResourceSearch] = useState("");
  const [editingResourceId, setEditingResourceId] = useState<number | null>(null);
  const [editResourceKey, setEditResourceKey] = useState("");
  const [editResourceDesc, setEditResourceDesc] = useState("");

  useEffect(() => {
    Promise.all([
      adminService.getAllRoles(),
      adminService.getAllPermissions(),
      adminService.getAllResources(),
    ])
      .then(([rls, perms, res]) => {
        setRoles(rls);
        setPermissions(perms);
        originalPermsRef.current = perms;
        setResources(res);
        if (rls.length > 0) setSelectedRoleId(rls[0].id);
      })
      .catch(console.error)
      .finally(() => setIsLoading(false));
  }, []);

  const handleToggleScope = useCallback(
    (resourceId: number, scope: number, currentlyEnabled: boolean) => {
      if (!selectedRoleId) return;

      setPermissions((prev) => {
        const idx = prev.findIndex(
          (p) => p.role_id === selectedRoleId && p.resource_id === resourceId
        );

        if (idx === -1) {
          return [
            ...prev,
            {
              id: Date.now(), // temp ID — identified as "new" on save
              role_id: selectedRoleId,
              resource_id: resourceId,
              scopes: scope,
              description: "",
            },
          ];
        }

        const updated = [...prev];
        const current = updated[idx];
        const newScopes = currentlyEnabled
          ? current.scopes & ~scope
          : current.scopes | scope;

        if (newScopes === 0) {
          updated.splice(idx, 1);
        } else {
          updated[idx] = { ...current, scopes: newScopes };
        }

        return updated;
      });
    },
    [selectedRoleId]
  );

  const selectedRole = roles.find((r) => r.id === selectedRoleId);

  function openCreateRole() {
    setEditingRole(null);
    setRoleName("");
    setRoleLevel(0);
    setRoleDialogOpen(true);
  }

  function openEditRole() {
    if (!selectedRole) return;
    setEditingRole(selectedRole);
    setRoleName(selectedRole.name);
    setRoleLevel(selectedRole.level);
    setRoleDialogOpen(true);
  }

  async function handleSaveRole() {
    if (!roleName.trim()) return;
    setIsSavingRole(true);
    try {
      if (editingRole) {
        const updated = await adminService.updateRole(editingRole.id, { name: roleName, level: roleLevel });
        setRoles((prev) => prev.map((r) => (r.id === editingRole.id ? updated : r)));
        notify.success(TEXT.ADMIN.ROLES_UPDATE_SUCCESS);
      } else {
        const created = await adminService.createRole({ name: roleName, level: roleLevel });
        setRoles((prev) => [...prev, created]);
        setSelectedRoleId(created.id);
        notify.success(TEXT.ADMIN.ROLES_CREATE_SUCCESS);
      }
      setRoleDialogOpen(false);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.ROLES_SAVE_ERROR));
    } finally {
      setIsSavingRole(false);
    }
  }

  async function handleSavePermissions() {
    if (!selectedRoleId) return;
    setIsSavingPerms(true);
    try {
      const original = originalPermsRef.current.filter((p) => p.role_id === selectedRoleId);
      const current = permissions.filter((p) => p.role_id === selectedRoleId);

      const originalMap = new Map(original.map((p) => [p.id, p]));
      const currentIds = new Set(current.map((p) => p.id));

      const toDelete = original.filter((p) => !currentIds.has(p.id));
      const toCreate = current.filter((p) => !originalMap.has(p.id));
      const toUpdate = current.filter((p) => {
        const orig = originalMap.get(p.id);
        return orig && orig.scopes !== p.scopes;
      });

      await Promise.all([
        ...toDelete.map((p) => adminService.deletePermission(p.id)),
        ...toCreate.map((p) =>
          adminService.createPermission({ role_id: p.role_id, resource_id: p.resource_id, scopes: p.scopes })
        ),
        ...toUpdate.map((p) => adminService.updatePermission(p.id, { scopes: p.scopes })),
      ]);

      const fresh = await adminService.getAllPermissions();
      setPermissions(fresh);
      originalPermsRef.current = fresh;
      notify.success(TEXT.ADMIN.ROLES_PERMS_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.ROLES_PERMS_ERROR));
    } finally {
      setIsSavingPerms(false);
    }
  }

  async function handleAddResource() {
    if (!newResourceKey.trim()) return;
    try {
      const created = await adminService.createResource({
        key: newResourceKey.trim().toLowerCase().replace(/\s+/g, "_"),
        description: newResourceDesc.trim() || undefined,
      });
      setResources((prev) => [...prev, created]);
      setNewResourceKey("");
      setNewResourceDesc("");
      notify.success(TEXT.ADMIN.RESOURCES_CREATE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.RESOURCES_CREATE_ERROR));
    }
  }

  function startEditResource(res: Resource) {
    setEditingResourceId(res.id);
    setEditResourceKey(res.key);
    setEditResourceDesc(res.description ?? "");
  }

  async function saveEditResource() {
    if (!editingResourceId || !editResourceKey.trim()) return;
    try {
      const updated = await adminService.updateResource(editingResourceId, {
        key: editResourceKey.trim(),
        description: editResourceDesc.trim() || undefined,
      });
      setResources((prev) => prev.map((r) => (r.id === editingResourceId ? updated : r)));
      setEditingResourceId(null);
      notify.success(TEXT.ADMIN.RESOURCES_UPDATE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.RESOURCES_UPDATE_ERROR));
    }
  }

  function cancelEditResource() {
    setEditingResourceId(null);
  }

  async function handleRemoveResource(id: number) {
    try {
      await adminService.deleteResource(id);
      setResources((prev) => prev.filter((r) => r.id !== id));
      setPermissions((prev) => prev.filter((p) => p.resource_id !== id));
      originalPermsRef.current = originalPermsRef.current.filter((p) => p.resource_id !== id);
      notify.success(TEXT.ADMIN.RESOURCES_DELETE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.RESOURCES_DELETE_ERROR));
    }
  }

  const filteredResources = resourceSearch
    ? resources.filter(
        (r) =>
          r.key.includes(resourceSearch.toLowerCase()) ||
          (r.description ?? "").toLowerCase().includes(resourceSearch.toLowerCase())
      )
    : resources;

  if (isLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{TEXT.ADMIN.ROLES_TITLE}</h1>
          <p className="text-sm text-muted-foreground mt-1">{TEXT.ADMIN.ROLES_SUBTITLE}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            className="gap-2 cursor-pointer"
            onClick={() => {
              setResourceSearch("");
              setEditingResourceId(null);
              setResourceDialogOpen(true);
            }}
          >
            <Package className="h-4 w-4" />
            {TEXT.ADMIN.ROLES_MANAGE_RESOURCES}
          </Button>
          <Button className="gap-2 cursor-pointer" onClick={openCreateRole}>
            <Plus className="h-4 w-4" />
            {TEXT.ADMIN.ROLES_CREATE}
          </Button>
        </div>
      </div>

      {/* Role tabs */}
      <div className="flex items-center gap-2 flex-wrap">
        {roles.map((role) => {
          const isSelected = selectedRoleId === role.id;
          return (
            <button
              key={role.id}
              onClick={() => setSelectedRoleId(role.id)}
              className={cn(
                "rounded-lg px-4 py-2 text-sm font-medium transition-colors cursor-pointer border",
                isSelected
                  ? "bg-primary text-primary-foreground border-primary shadow-sm"
                  : "bg-background border-border text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              {role.name}
            </button>
          );
        })}
      </div>

      {/* Selected role + permission matrix */}
      {selectedRole && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold">{selectedRole.name}</h2>
            <div className="flex items-center gap-1">
              <Button
                variant="ghost" size="sm"
                className="gap-2 cursor-pointer text-muted-foreground"
                onClick={openEditRole}
              >
                <Pencil className="h-3.5 w-3.5" />
                {TEXT.ADMIN.ROLES_EDIT}
              </Button>
              <Button
                size="sm"
                className="gap-2 cursor-pointer"
                onClick={handleSavePermissions}
                disabled={isSavingPerms}
              >
                {isSavingPerms ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Save className="h-3.5 w-3.5" />
                )}
                {TEXT.ADMIN.ROLES_SAVE_PERMS}
              </Button>
            </div>
          </div>

          <PermissionMatrix
            roleId={selectedRole.id}
            resources={resources}
            permissions={permissions}
            onToggleScope={handleToggleScope}
          />
        </div>
      )}

      {/* Dialog: Create / Edit Role */}
      <Dialog open={roleDialogOpen} onOpenChange={setRoleDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editingRole ? TEXT.ADMIN.ROLES_EDIT_DIALOG_TITLE : TEXT.ADMIN.ROLES_CREATE_DIALOG_TITLE}
            </DialogTitle>
            <DialogDescription>
              {editingRole ? TEXT.ADMIN.ROLES_EDIT_DIALOG_DESC : TEXT.ADMIN.ROLES_CREATE_DIALOG_DESC}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="role-name">{TEXT.ADMIN.ROLES_FORM_NAME}</Label>
              <Input
                id="role-name"
                placeholder={TEXT.ADMIN.ROLES_FORM_NAME_PLACEHOLDER}
                value={roleName}
                onChange={(e) => setRoleName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="role-level">{TEXT.ADMIN.ROLES_FORM_LEVEL}</Label>
              <Input
                id="role-level"
                type="number"
                min={0}
                max={10}
                value={roleLevel}
                onChange={(e) => setRoleLevel(Number(e.target.value))}
              />
              <p className="text-xs text-muted-foreground">{TEXT.ADMIN.ROLES_FORM_LEVEL_HINT}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRoleDialogOpen(false)} disabled={isSavingRole} className="cursor-pointer">
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button onClick={handleSaveRole} disabled={!roleName.trim() || isSavingRole} className="cursor-pointer">
              {isSavingRole ? <Loader2 className="h-4 w-4 animate-spin" /> : editingRole ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Dialog: Manage Resources */}
      <Dialog open={resourceDialogOpen} onOpenChange={setResourceDialogOpen}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>{TEXT.ADMIN.RESOURCES_DIALOG_TITLE}</DialogTitle>
            <DialogDescription>{TEXT.ADMIN.RESOURCES_DIALOG_DESC}</DialogDescription>
          </DialogHeader>

          {/* Add new resource */}
          <div className="flex items-end gap-2">
            <div className="flex-1 space-y-1.5">
              <Label className="text-xs text-muted-foreground">{TEXT.ADMIN.RESOURCES_KEY_LABEL}</Label>
              <Input
                placeholder={TEXT.ADMIN.RESOURCES_KEY_PLACEHOLDER}
                value={newResourceKey}
                onChange={(e) => setNewResourceKey(e.target.value)}
              />
            </div>
            <div className="flex-1 space-y-1.5">
              <Label className="text-xs text-muted-foreground">{TEXT.ADMIN.RESOURCES_DESC_LABEL}</Label>
              <Input
                placeholder={TEXT.ADMIN.RESOURCES_DESC_PLACEHOLDER}
                value={newResourceDesc}
                onChange={(e) => setNewResourceDesc(e.target.value)}
              />
            </div>
            <Button
              onClick={handleAddResource}
              disabled={!newResourceKey.trim()}
              className="gap-2 cursor-pointer shrink-0"
            >
              <Plus className="h-4 w-4" />
              {TEXT.ADMIN.RESOURCES_ADD}
            </Button>
          </div>

          {/* Search */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder={TEXT.ADMIN.RESOURCES_SEARCH_PLACEHOLDER}
              value={resourceSearch}
              onChange={(e) => setResourceSearch(e.target.value)}
              className="pl-9"
            />
          </div>

          {/* Resource table */}
          <div className="rounded-lg border border-border overflow-hidden">
            <div className="grid grid-cols-[1fr_1.5fr_72px] bg-muted/30 border-b border-border px-4 py-2">
              <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ADMIN.RESOURCES_COL_KEY}</span>
              <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ADMIN.RESOURCES_COL_DESC}</span>
              <span />
            </div>

            <div className="max-h-[320px] overflow-y-auto divide-y divide-border">
              {filteredResources.map((res) => (
                <div key={res.id} className="grid grid-cols-[1fr_1.5fr_72px] items-center px-4 py-2.5 hover:bg-muted/30 transition-colors">
                  {editingResourceId === res.id ? (
                    <>
                      <Input value={editResourceKey} onChange={(e) => setEditResourceKey(e.target.value)} className="h-8 text-sm font-mono" />
                      <Input value={editResourceDesc} onChange={(e) => setEditResourceDesc(e.target.value)} className="h-8 text-sm" />
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="icon" className="h-7 w-7 text-primary cursor-pointer" onClick={saveEditResource}>
                          <Save className="h-3.5 w-3.5" />
                        </Button>
                        <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground cursor-pointer" onClick={cancelEditResource}>
                          ✕
                        </Button>
                      </div>
                    </>
                  ) : (
                    <>
                      <span className="text-sm font-medium font-mono">{res.key}</span>
                      <span className="text-sm text-muted-foreground">{res.description}</span>
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => startEditResource(res)}>
                          <Pencil className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => handleRemoveResource(res.id)}>
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                    </>
                  )}
                </div>
              ))}

              {filteredResources.length === 0 && (
                <div className="px-4 py-8 text-center text-sm text-muted-foreground">
                  {TEXT.ADMIN.RESOURCES_EMPTY}
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
