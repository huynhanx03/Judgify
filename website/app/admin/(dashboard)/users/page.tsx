"use client";

/**
 * Admin user management — list, create, change role, delete.
 */

import { useEffect, useState } from "react";
import { Loader2, Plus, Trash2, UserCog } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EditRoleDialog } from "@/modules/admin/dialogs/edit-role-dialog";
import { CreateUserDialog } from "@/modules/admin/dialogs/create-user-dialog";
import { userService } from "@/services/user.service";
import { roleService } from "@/services/role.service";
import { notify, getErrorMessage } from "@/lib/toast";
import { TEXT } from "@/constants/text";
import type { AdminUser, Role } from "@/types/admin";

const ROLE_COLORS: Record<string, string> = {
  admin: "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
  teacher: "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20",
  student: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
};

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  // Delete state
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  // Edit role state
  const [editTarget, setEditTarget] = useState<AdminUser | null>(null);
  const [isSavingRole, setIsSavingRole] = useState(false);

  // Create user state
  const [createOpen, setCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    Promise.all([userService.find(), roleService.getAll()])
      .then(([res, rls]) => {
        setUsers(res.records);
        setRoles(rls);
      })
      .catch(console.error)
      .finally(() => setIsLoading(false));
  }, []);

  async function handleDelete() {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await userService.delete(deleteTarget.id);
      setUsers((prev) => prev.filter((u) => u.id !== deleteTarget.id));
      notify.success(TEXT.ADMIN.USERS_DELETE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.USERS_DELETE_ERROR));
    } finally {
      setIsDeleting(false);
      setDeleteTarget(null);
    }
  }

  async function handleSaveRole(userId: number, roleId: number) {
    setIsSavingRole(true);
    try {
      const updated = await userService.updateRole(userId, { role_id: roleId });
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
      notify.success(TEXT.ADMIN.USERS_EDIT_ROLE_SUCCESS);
      setEditTarget(null);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.USERS_EDIT_ROLE_ERROR));
    } finally {
      setIsSavingRole(false);
    }
  }

  async function handleCreate(data: {
    username: string; password: string; role_id: number;
    first_name: string; last_name: string; gender: number; birthday: string;
  }) {
    setIsCreating(true);
    try {
      await userService.create(data);
      const res = await userService.find();
      setUsers(res.records);
      notify.success(TEXT.ADMIN.USERS_CREATE_SUCCESS);
      setCreateOpen(false);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.USERS_CREATE_ERROR));
    } finally {
      setIsCreating(false);
    }
  }

  const columns: AdminColumn<AdminUser>[] = [
    {
      key: "id",
      label: TEXT.ADMIN.USERS_COL_ID,
      className: "w-16",
      render: (u) => <span className="text-xs tabular-nums text-muted-foreground">{u.id}</span>,
    },
    {
      key: "username",
      label: TEXT.ADMIN.USERS_COL_USERNAME,
      render: (u) => <span className="font-medium">{u.username}</span>,
    },
    {
      key: "role",
      label: TEXT.ADMIN.USERS_COL_ROLE,
      render: (u) => (
        <Badge variant="outline" className={`text-[11px] border ${ROLE_COLORS[u.role_name?.toLowerCase() ?? ""] ?? ""}`}>
          {u.role_name || "N/A"}
        </Badge>
      ),
    },
    {
      key: "created_at",
      label: TEXT.ADMIN.USERS_COL_CREATED,
      render: (u) => <span className="text-xs text-muted-foreground">{u.created_at?.slice(0, 10)}</span>,
    },
    {
      key: "actions",
      label: "",
      className: "w-20",
      render: (u) => (
        <div className="flex items-center gap-1 justify-end">
          <Button
            variant="ghost" size="icon"
            className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer"
            onClick={() => setEditTarget(u)}
          >
            <UserCog className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost" size="icon"
            className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer"
            onClick={() => setDeleteTarget(u)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ),
    },
  ];

  if (isLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{TEXT.ADMIN.USERS_TITLE}</h1>
          <p className="text-sm text-muted-foreground mt-1">{TEXT.ADMIN.USERS_SUBTITLE}</p>
        </div>
        <Button className="gap-2 cursor-pointer" onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" />
          {TEXT.ADMIN.USERS_CREATE}
        </Button>
      </div>

      <AdminDataTable
        columns={columns}
        data={users}
        keyExtractor={(u) => u.id}
        emptyMessage={TEXT.ADMIN.USERS_EMPTY}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        onConfirm={handleDelete}
        loading={isDeleting}
        title={TEXT.ADMIN.USERS_DELETE_TITLE}
        description={TEXT.ADMIN.USERS_DELETE_DESC(deleteTarget?.username ?? "")}
      />

      <EditRoleDialog
        user={editTarget}
        roles={roles}
        onSave={handleSaveRole}
        onClose={() => setEditTarget(null)}
        isSaving={isSavingRole}
      />

      <CreateUserDialog
        open={createOpen}
        roles={roles}
        onSave={handleCreate}
        onClose={() => setCreateOpen(false)}
        isSaving={isCreating}
      />
    </div>
  );
}
