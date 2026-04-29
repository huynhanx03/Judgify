"use client";

/**
 * Admin contests page — paginated list with search, create, edit, delete.
 */

import { useMemo } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { DataTableShell } from "@/modules/admin/data-table-shell";
import { usePaginatedCRUD } from "@/modules/admin/hooks/use-paginated-crud";
import { ContestDialog } from "@/modules/admin/dialogs/contest-dialog";
import { contestService } from "@/services/contest.service";
import { TEXT } from "@/constants/text";
import type { Contest } from "@/types/contest";

const STATUS_STYLES: Record<string, string> = {
  draft: "bg-zinc-500/10 text-zinc-500",
  upcoming: "bg-blue-500/10 text-blue-500",
  running: "bg-green-500/10 text-green-500",
  ended: "bg-zinc-500/10 text-zinc-500",
};

const STATUS_LABELS: Record<string, string> = {
  draft: TEXT.CONTEST.STATUS_DRAFT,
  upcoming: TEXT.CONTEST.STATUS_UPCOMING,
  running: TEXT.CONTEST.STATUS_RUNNING,
  ended: TEXT.CONTEST.STATUS_ENDED,
};

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("vi-VN", {
    day: "2-digit", month: "2-digit", year: "numeric",
    hour: "2-digit", minute: "2-digit",
  });
}

export default function AdminContestsPage() {
  const service = useMemo(() => ({
    find: contestService.find.bind(contestService),
    create: contestService.create.bind(contestService),
    update: contestService.update.bind(contestService),
    delete: contestService.delete.bind(contestService),
  }), []);

  const crud = usePaginatedCRUD<Contest>({ service });

  const columns: AdminColumn<Contest>[] = [
    { key: "id", label: "ID", className: "w-16",
      render: (c) => <span className="text-xs tabular-nums text-muted-foreground">{c.id}</span> },
    { key: "title", label: TEXT.ADMIN.CONTESTS.COL_TITLE,
      render: (c) => <span className="font-medium">{c.title}</span> },
    { key: "status", label: TEXT.ADMIN.CONTESTS.COL_STATUS, className: "w-32",
      render: (c) => (
        <Badge variant="secondary" className={STATUS_STYLES[c.status] ?? ""}>
          {STATUS_LABELS[c.status] ?? c.status}
        </Badge>
      ) },
    { key: "start_time", label: TEXT.ADMIN.CONTESTS.COL_START, className: "w-36",
      render: (c) => <span className="text-sm text-muted-foreground">{formatDate(c.start_time)}</span> },
    { key: "end_time", label: TEXT.ADMIN.CONTESTS.COL_END, className: "w-36",
      render: (c) => <span className="text-sm text-muted-foreground">{formatDate(c.end_time)}</span> },
    { key: "participants", label: TEXT.ADMIN.CONTESTS.COL_PARTICIPANTS, className: "w-28",
      render: (c) => (
        <span className="text-sm tabular-nums">
          {c.participant_count}{c.max_participants > 0 ? `/${c.max_participants}` : ""}
        </span>
      ) },
    { key: "actions", label: "", className: "w-20",
      render: (c) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground cursor-pointer" onClick={() => crud.openEdit(c)}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => crud.confirmDelete(c.id)}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) },
  ];

  if (crud.isLoading) return <LoadingSpinner />;

  return (
    <DataTableShell
      title={TEXT.ADMIN.CONTESTS.TITLE}
      subtitle={TEXT.ADMIN.CONTESTS.SUBTITLE}
      createLabel={TEXT.ADMIN.CONTESTS.CREATE}
      onCreateClick={crud.openCreate}
      searchValue={crud.search}
      onSearch={crud.onSearch}
      searchPlaceholder={TEXT.ADMIN.CONTESTS.SEARCH_PLACEHOLDER}
      table={
        <AdminDataTable
          columns={columns}
          data={crud.data}
          keyExtractor={(c) => c.id}
          emptyMessage={TEXT.ADMIN.CONTESTS.EMPTY}
          pagination={crud.pagination}
          onPageChange={crud.onPageChange}
        />
      }
      dialog={
        <ContestDialog
          open={crud.dialogOpen}
          editing={crud.editing}
          onSave={crud.handleSave}
          onClose={crud.closeDialog}
          isSaving={crud.isSaving}
        />
      }
      confirmDialog={
        <ConfirmDialog
          open={crud.deleteId !== null}
          onOpenChange={(open) => { if (!open) crud.cancelDelete(); }}
          onConfirm={crud.handleDelete}
        />
      }
    />
  );
}
