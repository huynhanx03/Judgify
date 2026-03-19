"use client";

/**
 * Admin user management page — list all users with role info.
 */

import { useEffect, useState } from "react";
import { Loader2, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { adminService } from "@/services/admin.service";
import type { AdminUser } from "@/types/admin";

const ROLE_COLORS: Record<string, string> = {
  Admin: "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
  Teacher: "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20",
  Student: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
};

const columns: AdminColumn<AdminUser>[] = [
  {
    key: "id",
    label: "ID",
    className: "w-16",
    render: (u) => <span className="text-xs tabular-nums text-muted-foreground">{u.id}</span>,
  },
  {
    key: "username",
    label: "Username",
    render: (u) => <span className="font-medium">{u.username}</span>,
  },
  {
    key: "role",
    label: "Vai Trò",
    render: (u) => (
      <Badge variant="outline" className={`text-[11px] border ${ROLE_COLORS[u.role?.name ?? ""] ?? ""}`}>
        {u.role?.name ?? "N/A"}
      </Badge>
    ),
  },
  {
    key: "created_at",
    label: "Ngày Tạo",
    render: (u) => <span className="text-xs text-muted-foreground">{u.created_at}</span>,
  },
  {
    key: "actions",
    label: "",
    className: "w-12",
    render: () => (
      <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer">
        <Trash2 className="h-4 w-4" />
      </Button>
    ),
  },
];

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    adminService
      .getUsers()
      .then(setUsers)
      .catch(console.error)
      .finally(() => setIsLoading(false));
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Người Dùng</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Quản lý tài khoản người dùng trong hệ thống
        </p>
      </div>

      <AdminDataTable
        columns={columns}
        data={users}
        keyExtractor={(u) => u.id}
        emptyMessage="Chưa có người dùng nào"
      />
    </div>
  );
}
