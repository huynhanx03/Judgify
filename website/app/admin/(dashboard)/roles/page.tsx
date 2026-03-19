"use client";

/**
 * Admin role & permission management.
 * Top: horizontal role tabs. Below: flat permission matrix for selected role.
 * Dialogs: create/edit role, manage resources.
 */

import { useEffect, useState, useCallback } from "react";
import { Loader2, Plus, Pencil, Package, Save, Trash2, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { PermissionMatrix } from "@/modules/admin/permission-matrix";
import { adminService } from "@/services/admin.service";
import type { Role, Permission, Resource } from "@/types/admin";
import { cn } from "@/lib/utils";

export default function AdminRolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [resources, setResources] = useState<Resource[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Dialog states
  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [roleName, setRoleName] = useState("");
  const [roleLevel, setRoleLevel] = useState(0);

  const [resourceDialogOpen, setResourceDialogOpen] = useState(false);
  const [newResourceKey, setNewResourceKey] = useState("");
  const [newResourceDesc, setNewResourceDesc] = useState("");
  const [resourceSearch, setResourceSearch] = useState("");
  const [editingResourceId, setEditingResourceId] = useState<number | null>(null);
  const [editResourceKey, setEditResourceKey] = useState("");
  const [editResourceDesc, setEditResourceDesc] = useState("");

  useEffect(() => {
    Promise.all([
      adminService.getRoles(),
      adminService.getPermissions(),
      adminService.getResources(),
    ])
      .then(([rls, perms, res]) => {
        setRoles(rls);
        setPermissions(perms);
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
              id: Date.now(),
              role_id: selectedRoleId,
              resource_id: resourceId,
              scopes: scope,
              description: "",
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
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

  // Open create role dialog
  function openCreateRole() {
    setEditingRole(null);
    setRoleName("");
    setRoleLevel(0);
    setRoleDialogOpen(true);
  }

  // Open edit role dialog
  function openEditRole() {
    if (!selectedRole) return;
    setEditingRole(selectedRole);
    setRoleName(selectedRole.name);
    setRoleLevel(selectedRole.level);
    setRoleDialogOpen(true);
  }

  // Save role (create or edit)
  function handleSaveRole() {
    if (!roleName.trim()) return;

    if (editingRole) {
      setRoles((prev) =>
        prev.map((r) =>
          r.id === editingRole.id ? { ...r, name: roleName, level: roleLevel } : r
        )
      );
    } else {
      const newRole: Role = {
        id: Date.now(),
        name: roleName,
        level: roleLevel,
        parent_id: -1,
        lft: 0,
        rgt: 0,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      setRoles((prev) => [...prev, newRole]);
      setSelectedRoleId(newRole.id);
    }
    setRoleDialogOpen(false);
  }

  // Save permissions (mock — just show feedback)
  function handleSavePermissions() {
    // TODO: call API to persist permissions
  }

  // Add resource
  function handleAddResource() {
    if (!newResourceKey.trim()) return;

    const newRes: Resource = {
      id: Date.now(),
      key: newResourceKey.trim().toLowerCase().replace(/\s+/g, "_"),
      description: newResourceDesc.trim(),
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    setResources((prev) => [...prev, newRes]);
    setNewResourceKey("");
    setNewResourceDesc("");
  }

  // Start editing a resource inline
  function startEditResource(res: Resource) {
    setEditingResourceId(res.id);
    setEditResourceKey(res.key);
    setEditResourceDesc(res.description);
  }

  // Save inline resource edit
  function saveEditResource() {
    if (!editingResourceId || !editResourceKey.trim()) return;
    setResources((prev) =>
      prev.map((r) =>
        r.id === editingResourceId
          ? { ...r, key: editResourceKey.trim(), description: editResourceDesc.trim() }
          : r
      )
    );
    setEditingResourceId(null);
  }

  // Cancel inline resource edit
  function cancelEditResource() {
    setEditingResourceId(null);
  }

  // Remove resource
  function handleRemoveResource(id: number) {
    setResources((prev) => prev.filter((r) => r.id !== id));
    setPermissions((prev) => prev.filter((p) => p.resource_id !== id));
  }

  // Filtered resources for dialog
  const filteredResources = resourceSearch
    ? resources.filter(
        (r) =>
          r.key.includes(resourceSearch.toLowerCase()) ||
          r.description.toLowerCase().includes(resourceSearch.toLowerCase())
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
          <h1 className="text-2xl font-bold tracking-tight">Phân Quyền</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Quản lý vai trò và ma trận quyền hạn truy cập tài nguyên
          </p>
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
            Quản Lý Tài Nguyên
          </Button>
          <Button className="gap-2 cursor-pointer" onClick={openCreateRole}>
            <Plus className="h-4 w-4" />
            Tạo Vai Trò
          </Button>
        </div>
      </div>

      {/* Role tabs — horizontal row, no level badges */}
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
                variant="ghost"
                size="sm"
                className="gap-2 cursor-pointer text-muted-foreground"
                onClick={openEditRole}
              >
                <Pencil className="h-3.5 w-3.5" />
                Chỉnh sửa
              </Button>
              <Button
                size="sm"
                className="gap-2 cursor-pointer"
                onClick={handleSavePermissions}
              >
                <Save className="h-3.5 w-3.5" />
                Lưu
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
              {editingRole ? "Chỉnh Sửa Vai Trò" : "Tạo Vai Trò Mới"}
            </DialogTitle>
            <DialogDescription>
              {editingRole
                ? "Cập nhật thông tin vai trò trong hệ thống."
                : "Thêm vai trò mới để phân quyền cho người dùng."}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="role-name">Tên vai trò</Label>
              <Input
                id="role-name"
                placeholder="VD: Moderator, Reviewer..."
                value={roleName}
                onChange={(e) => setRoleName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="role-level">Cấp độ (Level)</Label>
              <Input
                id="role-level"
                type="number"
                min={0}
                max={10}
                value={roleLevel}
                onChange={(e) => setRoleLevel(Number(e.target.value))}
              />
              <p className="text-xs text-muted-foreground">
                0 = cao nhất (Admin), số càng lớn = quyền hạn càng thấp
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRoleDialogOpen(false)}
              className="cursor-pointer"
            >
              Hủy
            </Button>
            <Button onClick={handleSaveRole} disabled={!roleName.trim()} className="cursor-pointer">
              {editingRole ? "Lưu" : "Tạo"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Dialog: Manage Resources — polished table layout */}
      <Dialog open={resourceDialogOpen} onOpenChange={setResourceDialogOpen}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>Quản Lý Tài Nguyên</DialogTitle>
            <DialogDescription>
              Thêm, sửa hoặc xóa các tài nguyên được bảo vệ trong hệ thống phân quyền.
            </DialogDescription>
          </DialogHeader>

          {/* Add new resource — simple row on top */}
          <div className="flex items-end gap-2">
            <div className="flex-1 space-y-1.5">
              <Label className="text-xs text-muted-foreground">Key</Label>
              <Input
                placeholder="VD: submission"
                value={newResourceKey}
                onChange={(e) => setNewResourceKey(e.target.value)}
              />
            </div>
            <div className="flex-1 space-y-1.5">
              <Label className="text-xs text-muted-foreground">Mô tả</Label>
              <Input
                placeholder="VD: Bài nộp của người dùng"
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
              Thêm
            </Button>
          </div>

          {/* Search */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Tìm tài nguyên..."
              value={resourceSearch}
              onChange={(e) => setResourceSearch(e.target.value)}
              className="pl-9"
            />
          </div>

          {/* Resource table */}
          <div className="rounded-lg border border-border overflow-hidden">
            {/* Table header */}
            <div className="grid grid-cols-[1fr_1.5fr_72px] bg-muted/30 border-b border-border px-4 py-2">
              <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Key</span>
              <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Mô tả</span>
              <span />
            </div>

            {/* Table body */}
            <div className="max-h-[320px] overflow-y-auto divide-y divide-border">
              {filteredResources.map((res) => (
                <div key={res.id} className="grid grid-cols-[1fr_1.5fr_72px] items-center px-4 py-2.5 hover:bg-muted/30 transition-colors">
                  {editingResourceId === res.id ? (
                    <>
                      <Input
                        value={editResourceKey}
                        onChange={(e) => setEditResourceKey(e.target.value)}
                        className="h-8 text-sm font-mono"
                      />
                      <Input
                        value={editResourceDesc}
                        onChange={(e) => setEditResourceDesc(e.target.value)}
                        className="h-8 text-sm"
                      />
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-primary cursor-pointer"
                          onClick={saveEditResource}
                        >
                          <Save className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground cursor-pointer"
                          onClick={cancelEditResource}
                        >
                          ✕
                        </Button>
                      </div>
                    </>
                  ) : (
                    <>
                      <span className="text-sm font-medium font-mono">{res.key}</span>
                      <span className="text-sm text-muted-foreground">{res.description}</span>
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-foreground cursor-pointer"
                          onClick={() => startEditResource(res)}
                        >
                          <Pencil className="h-3 w-3" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive cursor-pointer"
                          onClick={() => handleRemoveResource(res.id)}
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                    </>
                  )}
                </div>
              ))}

              {filteredResources.length === 0 && (
                <div className="px-4 py-8 text-center text-sm text-muted-foreground">
                  Không tìm thấy tài nguyên nào.
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
