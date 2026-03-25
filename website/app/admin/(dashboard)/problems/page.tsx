"use client";

/**
 * Admin problem management page — list, publish status, difficulty.
 */

import { useEffect, useState } from "react";
import { Loader2, Plus, Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { adminService } from "@/services/admin.service";
import type { Problem } from "@/types/problem";

const DIFF_STYLES: Record<string, string> = {
  easy: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  medium: "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
  hard: "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
};

const DIFF_LABELS: Record<string, string> = {
  easy: "Dễ",
  medium: "Trung Bình",
  hard: "Khó",
};

const columns: AdminColumn<Problem>[] = [
  {
    key: "id",
    label: "ID",
    className: "w-16",
    render: (p) => <span className="text-xs tabular-nums text-muted-foreground">{p.id}</span>,
  },
  {
    key: "title",
    label: "Tiêu Đề",
    render: (p) => (
      <div className="space-y-1">
        <span className="font-medium">{p.title}</span>
        <div className="flex gap-1 flex-wrap">
          {p.tags?.map((t) => (
            <span key={t.id} className="inline-flex rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
              {t.name}
            </span>
          ))}
        </div>
      </div>
    ),
  },
  {
    key: "difficulty",
    label: "Độ Khó",
    className: "w-28",
    render: (p) => (
      <Badge variant="outline" className={`text-[11px] border ${DIFF_STYLES[p.difficulty]}`}>
        {DIFF_LABELS[p.difficulty]}
      </Badge>
    ),
  },
  {
    key: "published",
    label: "Trạng Thái",
    className: "w-28",
    render: (p) => (
      <Badge variant={p.is_published ? "default" : "outline"} className="text-[11px]">
        {p.is_published ? "Published" : "Draft"}
      </Badge>
    ),
  },
  {
    key: "limits",
    label: "Giới Hạn",
    className: "w-32",
    render: (p) => (
      <div className="text-xs text-muted-foreground space-y-0.5">
        <div>{p.time_limit_ms}ms</div>
        <div>{(p.memory_limit_kb / 1024).toFixed(0)}MB</div>
      </div>
    ),
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

export default function AdminProblemsPage() {
  const [problems, setProblems] = useState<Problem[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    adminService
      .getProblems()
      .then(setProblems)
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
          <h1 className="text-2xl font-bold tracking-tight">Bài Tập</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Quản lý bài tập và test cases
          </p>
        </div>
        <Button className="gap-2 cursor-pointer">
          <Plus className="h-4 w-4" />
          Tạo Bài Tập
        </Button>
      </div>

      <AdminDataTable
        columns={columns}
        data={problems}
        keyExtractor={(p) => p.id}
        emptyMessage="Chưa có bài tập nào"
      />
    </div>
  );
}
