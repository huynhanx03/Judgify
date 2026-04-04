"use client";

/**
 * Admin user management — list, create, change role, delete.
 */

import { useEffect, useState } from "react";
import { Loader2, Plus, Trash2, UserCog } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { adminService } from "@/services/admin.service";
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
  const [editRoleId, setEditRoleId] = useState<string>("");
  const [isSavingRole, setIsSavingRole] = useState(false);

  // Create user state
  const [createOpen, setCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [form, setForm] = useState({
    username: "", password: "", role_id: "",
    first_name: "", last_name: "",
    gender: "0", birthday: "",
  });

  useEffect(() => {
    Promise.all([adminService.getUsers(), adminService.getAllRoles()])
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
      await adminService.deleteUser(deleteTarget.id);
      setUsers((prev) => prev.filter((u) => u.id !== deleteTarget.id));
      notify.success(TEXT.ADMIN.USERS_DELETE_SUCCESS);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.USERS_DELETE_ERROR));
    } finally {
      setIsDeleting(false);
      setDeleteTarget(null);
    }
  }

  function openEditRole(user: AdminUser) {
    setEditTarget(user);
    setEditRoleId(String(user.role_id));
  }

  async function handleSaveRole() {
    if (!editTarget || !editRoleId) return;
    setIsSavingRole(true);
    try {
      const updated = await adminService.updateUserRole(editTarget.id, { role_id: Number(editRoleId) });
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
      notify.success(TEXT.ADMIN.USERS_EDIT_ROLE_SUCCESS);
      setEditTarget(null);
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.ADMIN.USERS_EDIT_ROLE_ERROR));
    } finally {
      setIsSavingRole(false);
    }
  }

  async function handleCreate() {
    if (!form.username || !form.password || !form.role_id || !form.first_name || !form.last_name || !form.birthday) return;
    setIsCreating(true);
    try {
      await adminService.createUser({
        username: form.username,
        password: form.password,
        role_id: Number(form.role_id),
        first_name: form.first_name,
        last_name: form.last_name,
        gender: Number(form.gender),
        birthday: form.birthday,
      });
      const res = await adminService.getUsers();
      setUsers(res.records);
      notify.success(TEXT.ADMIN.USERS_CREATE_SUCCESS);
      setCreateOpen(false);
      setForm({ username: "", password: "", role_id: "", first_name: "", last_name: "", gender: "0", birthday: "" });
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
            onClick={() => openEditRole(u)}
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

      {/* Delete confirm */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        onConfirm={handleDelete}
        loading={isDeleting}
        title={TEXT.ADMIN.USERS_DELETE_TITLE}
        description={TEXT.ADMIN.USERS_DELETE_DESC(deleteTarget?.username ?? "")}
      />

      {/* Edit role dialog */}
      <Dialog open={!!editTarget} onOpenChange={(open) => !open && setEditTarget(null)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{TEXT.ADMIN.USERS_EDIT_ROLE_TITLE}</DialogTitle>
            <DialogDescription>{TEXT.ADMIN.USERS_EDIT_ROLE_DESC(editTarget?.username ?? "")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2 py-2">
            <Label>{TEXT.ADMIN.USERS_FORM_ROLE}</Label>
            <Select value={editRoleId} onValueChange={setEditRoleId}>
              <SelectTrigger><SelectValue placeholder={TEXT.ADMIN.USERS_ROLE_PLACEHOLDER} /></SelectTrigger>
              <SelectContent>
                {roles.map((r) => (
                  <SelectItem key={r.id} value={String(r.id)}>{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditTarget(null)} disabled={isSavingRole} className="cursor-pointer">{TEXT.COMMON.CANCEL}</Button>
            <Button onClick={handleSaveRole} disabled={isSavingRole || !editRoleId} className="cursor-pointer">
              {isSavingRole ? <Loader2 className="h-4 w-4 animate-spin" /> : TEXT.COMMON.SAVE}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create user dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{TEXT.ADMIN.USERS_CREATE_TITLE}</DialogTitle>
            <DialogDescription>{TEXT.ADMIN.USERS_CREATE_DESC}</DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3 py-2">
            <div className="space-y-1.5 col-span-2">
              <Label>{TEXT.ADMIN.USERS_FORM_USERNAME}</Label>
              <Input placeholder={TEXT.ADMIN.USERS_FORM_USERNAME_PLACEHOLDER} value={form.username} onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))} />
            </div>
            <div className="space-y-1.5 col-span-2">
              <Label>{TEXT.ADMIN.USERS_FORM_PASSWORD}</Label>
              <Input type="password" placeholder={TEXT.ADMIN.USERS_FORM_PASSWORD_PLACEHOLDER} value={form.password} onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label>{TEXT.ADMIN.USERS_FORM_LAST_NAME}</Label>
              <Input placeholder={TEXT.AUTH.LAST_NAME_PLACEHOLDER} value={form.last_name} onChange={(e) => setForm((f) => ({ ...f, last_name: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label>{TEXT.ADMIN.USERS_FORM_FIRST_NAME}</Label>
              <Input placeholder={TEXT.AUTH.FIRST_NAME_PLACEHOLDER} value={form.first_name} onChange={(e) => setForm((f) => ({ ...f, first_name: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label>{TEXT.ADMIN.USERS_FORM_BIRTHDAY}</Label>
              <Input type="date" value={form.birthday} onChange={(e) => setForm((f) => ({ ...f, birthday: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label>{TEXT.ADMIN.USERS_FORM_GENDER}</Label>
              <Select value={form.gender} onValueChange={(v) => setForm((f) => ({ ...f, gender: v }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">{TEXT.ADMIN.USERS_FORM_GENDER_OTHER}</SelectItem>
                  <SelectItem value="1">{TEXT.ADMIN.USERS_FORM_GENDER_MALE}</SelectItem>
                  <SelectItem value="2">{TEXT.ADMIN.USERS_FORM_GENDER_FEMALE}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5 col-span-2">
              <Label>{TEXT.ADMIN.USERS_FORM_ROLE}</Label>
              <Select value={form.role_id} onValueChange={(v) => setForm((f) => ({ ...f, role_id: v }))}>
                <SelectTrigger><SelectValue placeholder={TEXT.ADMIN.USERS_FORM_ROLE_PLACEHOLDER} /></SelectTrigger>
                <SelectContent>
                  {roles.map((r) => (
                    <SelectItem key={r.id} value={String(r.id)}>{r.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)} disabled={isCreating} className="cursor-pointer">{TEXT.COMMON.CANCEL}</Button>
            <Button
              onClick={handleCreate}
              disabled={isCreating || !form.username || !form.password || !form.role_id || !form.first_name || !form.last_name || !form.birthday}
              className="cursor-pointer"
            >
              {isCreating ? <Loader2 className="h-4 w-4 animate-spin" /> : TEXT.COMMON.CREATE}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
