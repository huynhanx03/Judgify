"use client";

/**
 * Admin role & permission management.
 * Top: horizontal role tabs. Below: flat permission matrix for selected role.
 * Dialogs extracted to modules/admin/dialogs/.
 */

import { useEffect, useState, useCallback, useRef } from "react";
import { Loader2, Plus, Pencil, Package, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PermissionMatrix } from "@/modules/admin/permission-matrix";
import { RoleDialog } from "@/modules/admin/dialogs/role-dialog";
import { ResourceManagementDialog } from "@/modules/admin/dialogs/resource-management-dialog";
import { roleService } from "@/services/role.service";
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
  const [isSavingRole, setIsSavingRole] = useState(false);

  const [resourceDialogOpen, setResourceDialogOpen] = useState(false);

  useEffect(() => {
    Promise.all([
      roleService.getAll(),
      roleService.getAllPermissions(),
      roleService.getAllResources(),
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

  async function handleSaveRole(input: { name: string; level: number }) {
    setIsSavingRole(true);
    try {
      if (editingRole) {
        const updated = await roleService.update(editingRole.id, input);
        setRoles((prev) => prev.map((r) => (r.id === editingRole.id ? updated : r)));
        notify.success(TEXT.ADMIN.ROLES_UPDATE_SUCCESS);
      } else {
        const created = await roleService.create(input);
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
        ...toDelete.map((p) => roleService.deletePermission(p.id)),
        ...toCreate.map((p) =>
          roleService.createPermission({ role_id: p.role_id, resource_id: p.resource_id, scopes: p.scopes })
        ),
        ...toUpdate.map((p) => roleService.updatePermission(p.id, { scopes: p.scopes })),
      ]);

      const fresh = await roleService.getAllPermissions();
      setPermissions(fresh);
      originalPermsRef.current = fresh;
      notify.success(TEXT.ADMIN.ROLES_PERMS_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.ROLES_PERMS_ERROR));
    } finally {
      setIsSavingPerms(false);
    }
  }

  function handleResourcesChange(updated: Resource[]) {
    setResources(updated);
  }

  function handlePermissionsFilterRemove(resourceId: number) {
    setPermissions((prev) => prev.filter((p) => p.resource_id !== resourceId));
    originalPermsRef.current = originalPermsRef.current.filter((p) => p.resource_id !== resourceId);
  }

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
            onClick={() => setResourceDialogOpen(true)}
          >
            <Package className="h-4 w-4" />
            {TEXT.ADMIN.ROLES_MANAGE_RESOURCES}
          </Button>
          <Button
            className="gap-2 cursor-pointer"
            onClick={() => { setEditingRole(null); setRoleDialogOpen(true); }}
          >
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
                onClick={() => { setEditingRole(selectedRole); setRoleDialogOpen(true); }}
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
      <RoleDialog
        open={roleDialogOpen}
        editing={editingRole}
        onSave={handleSaveRole}
        onClose={() => setRoleDialogOpen(false)}
        isSaving={isSavingRole}
      />

      {/* Dialog: Manage Resources */}
      <ResourceManagementDialog
        open={resourceDialogOpen}
        onClose={() => setResourceDialogOpen(false)}
        resources={resources}
        onResourcesChange={handleResourcesChange}
        onResourceDeleted={(id) => {
          setPermissions((prev) => prev.filter((p) => p.resource_id !== id));
          originalPermsRef.current = originalPermsRef.current.filter((p) => p.resource_id !== id);
        }}
      />
    </div>
  );
}
