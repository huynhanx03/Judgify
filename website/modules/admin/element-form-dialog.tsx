"use client";

/**
 * Dialog form for creating and editing cultivation elements (Nguyên Tố).
 * Used by AdminElementsPage for both create and edit flows.
 */

import { useState, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { elementService } from "@/services/element.service";
import { notify } from "@/lib/toast";
import type { ElementResponse } from "@/types/cultivation";

interface ElementFormDialogProps {
  open: boolean;
  editItem: ElementResponse | null;
  onClose: () => void;
  onSuccess: () => void;
}

const EMPTY_FORM = { name: "", code: "", description: "" };

export function ElementFormDialog({
  open,
  editItem,
  onClose,
  onSuccess,
}: ElementFormDialogProps) {
  const [form, setForm] = useState(EMPTY_FORM);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    if (open) {
      setForm(
        editItem
          ? { name: editItem.name, code: editItem.code, description: editItem.description ?? "" }
          : EMPTY_FORM
      );
    }
  }, [open, editItem]);

  const handleChange = (key: keyof typeof EMPTY_FORM) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((prev) => ({ ...prev, [key]: e.target.value }));

  const handleSubmit = async () => {
    if (!form.name.trim() || !form.code.trim()) {
      notify.error("Tên và Code không được để trống");
      return;
    }
    setIsSaving(true);
    try {
      const payload = {
        name: form.name.trim(),
        code: form.code.trim(),
        description: form.description.trim() || undefined,
      };
      if (editItem) {
        await elementService.update(editItem.id, payload);
        notify.success("Cập nhật thành công");
      } else {
        await elementService.create(payload);
        notify.success("Tạo thành công");
      }
      onSuccess();
      onClose();
    } catch {
      notify.error(editItem ? "Không thể cập nhật" : "Không thể tạo");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editItem ? "Chỉnh Sửa Nguyên Tố" : "Tạo Nguyên Tố Mới"}</DialogTitle>
          <DialogDescription>
            {editItem ? "Cập nhật thông tin nguyên tố tu luyện." : "Thêm nguyên tố tu luyện mới vào hệ thống."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="el-name">Tên <span className="text-destructive">*</span></Label>
            <Input id="el-name" placeholder="Nhập tên nguyên tố" value={form.name} onChange={handleChange("name")} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="el-code">Code <span className="text-destructive">*</span></Label>
            <Input id="el-code" placeholder="Ví dụ: FIRE, WATER" value={form.code} onChange={handleChange("code")} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="el-desc">Mô Tả</Label>
            <Input id="el-desc" placeholder="Mô tả ngắn (tuỳ chọn)" value={form.description} onChange={handleChange("description")} />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" className="cursor-pointer" onClick={onClose} disabled={isSaving}>
            Huỷ
          </Button>
          <Button className="cursor-pointer" onClick={handleSubmit} disabled={isSaving}>
            {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {editItem ? "Lưu Thay Đổi" : "Tạo Mới"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
