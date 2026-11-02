"use client";

/**
 * Reusable confirmation dialog for destructive actions (delete, etc.).
 * Renders a modal with title, description, cancel and confirm buttons.
 */

import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from "@/components/ui/dialog";
import { Loader2 } from "lucide-react";
import { text } from "@/i18n/text";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  title?: string;
  description?: string;
  confirmLabel?: string;
  confirmVariant?: "default" | "destructive";
  loading?: boolean;
  confirmDisabled?: boolean;
  children?: ReactNode;
  contentClassName?: string;
}

export function ConfirmDialog({
  open, onOpenChange, onConfirm,
  title = text("COMMON.CONFIRM_DELETE_TITLE"),
  description = text("COMMON.CONFIRM_DELETE_DESCRIPTION"),
  confirmLabel = text("COMMON.DELETE_ACCENTED"),
  confirmVariant = "destructive",
  loading = false,
  confirmDisabled = false,
  children,
  contentClassName,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn("sm:max-w-sm", contentClassName)}
        aria-busy={loading}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {children}
        <DialogFooter>
          <Button
            variant="outline"
            className="cursor-pointer"
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            {text("COMMON.CANCEL")}
          </Button>
          <Button
            variant={confirmVariant}
            className="cursor-pointer"
            onClick={onConfirm}
            disabled={loading || confirmDisabled}
          >
            {loading && <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />}
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
