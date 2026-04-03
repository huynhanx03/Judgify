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

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  title?: string;
  description?: string;
  confirmLabel?: string;
  loading?: boolean;
}

export function ConfirmDialog({
  open, onOpenChange, onConfirm,
  title = "Xác nhận xoá",
  description = "Bạn có chắc chắn muốn xoá? Hành động này không thể hoàn tác.",
  confirmLabel = "Xoá",
  loading = false,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" className="cursor-pointer" onClick={() => onOpenChange(false)} disabled={loading}>Hủy</Button>
          <Button variant="destructive" className="cursor-pointer" onClick={onConfirm} disabled={loading}>{confirmLabel}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
