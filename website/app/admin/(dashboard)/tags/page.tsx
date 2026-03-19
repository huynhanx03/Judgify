"use client";

/**
 * Admin tag management page — list and manage problem tags.
 */

import { useEffect, useState } from "react";
import { Loader2, Plus, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { adminService } from "@/services/admin.service";
import type { Tag } from "@/types/tag";

const columns: AdminColumn<Tag>[] = [
  {
    key: "id",
    label: "ID",
    className: "w-16",
    render: (t) => <span className="text-xs tabular-nums text-muted-foreground">{t.id}</span>,
  },
  {
    key: "name",
    label: "Tên Tag",
    render: (t) => <span className="font-medium">{t.name}</span>,
  },
  {
    key: "actions",
    label: "",
    className: "w-20",
    render: () => (
      <div className="flex gap-1">
        <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer">
          <Pencil className="h-3.5 w-3.5" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer">
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
    ),
  },
];

export default function AdminTagsPage() {
  const [tags, setTags] = useState<Tag[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    adminService
      .getTags()
      .then(setTags)
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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Tags</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Quản lý thẻ phân loại bài tập
          </p>
        </div>
        <Button className="gap-2 cursor-pointer">
          <Plus className="h-4 w-4" />
          Tạo Tag
        </Button>
      </div>

      <AdminDataTable
        columns={columns}
        data={tags}
        keyExtractor={(t) => t.id}
        emptyMessage="Chưa có tag nào"
      />
    </div>
  );
}
