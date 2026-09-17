"use client";

import { Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/auth-context";
import { AUTHORIZATION_ACTION } from "@/constants/authorization";
import { TEXT } from "@/constants/text";
import { cn } from "@/lib/utils";

interface AdminResourceActionsProps {
  resource: string;
  onEdit?: () => void;
  onDelete?: () => void;
  editLabel?: string;
  deleteLabel?: string;
  className?: string;
}

/** Shared exact-capability row actions for all ordinary admin resources. */
export function AdminResourceActions({
  resource,
  onEdit,
  onDelete,
  editLabel = TEXT.COMMON.EDIT,
  deleteLabel = TEXT.COMMON.DELETE,
  className,
}: AdminResourceActionsProps) {
  const { can } = useAuth();
  const canUpdate = Boolean(onEdit) && can(resource, AUTHORIZATION_ACTION.UPDATE);
  const canDelete = Boolean(onDelete) && can(resource, AUTHORIZATION_ACTION.DELETE);

  if (!canUpdate && !canDelete) return null;

  return (
    <div className={cn("flex items-center justify-end gap-1", className)}>
      {canUpdate ? (
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-foreground"
          onClick={onEdit}
          aria-label={editLabel}
        >
          <Pencil className="h-3.5 w-3.5" />
        </Button>
      ) : null}
      {canDelete ? (
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-destructive"
          onClick={onDelete}
          aria-label={deleteLabel}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      ) : null}
    </div>
  );
}
