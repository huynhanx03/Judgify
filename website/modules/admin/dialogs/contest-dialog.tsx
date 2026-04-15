"use client";

/**
 * Create / edit dialog for Contest entities.
 * Form fields: title, description, start_time, end_time, max_participants, problem_ids.
 */

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogDescription,
  DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TEXT } from "@/constants/text";
import type { Contest } from "@/types/contest";

interface ContestDialogProps {
  open: boolean;
  editing: Contest | null;
  onSave: (input: {
    title: string;
    description?: string;
    start_time: string;
    end_time: string;
    max_participants?: number;
    problem_ids?: number[];
  }) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

function toDatetimeLocal(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function ContestDialog({ open, editing, onSave, onClose, isSaving }: ContestDialogProps) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [maxParticipants, setMaxParticipants] = useState(0);
  const [problemIds, setProblemIds] = useState("");

  useEffect(() => {
    if (editing) {
      setTitle(editing.title);
      setDescription(editing.description ?? "");
      setStartTime(toDatetimeLocal(editing.start_time));
      setEndTime(toDatetimeLocal(editing.end_time));
      setMaxParticipants(editing.max_participants);
      setProblemIds(editing.problem_ids?.join(", ") ?? "");
    } else {
      setTitle("");
      setDescription("");
      setStartTime("");
      setEndTime("");
      setMaxParticipants(0);
      setProblemIds("");
    }
  }, [editing, open]);

  function handleSubmit() {
    if (!title.trim() || !startTime || !endTime) return;
    const ids = problemIds
      .split(",")
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n));
    onSave({
      title: title.trim(),
      description: description.trim() || undefined,
      start_time: new Date(startTime).toISOString(),
      end_time: new Date(endTime).toISOString(),
      max_participants: maxParticipants > 0 ? maxParticipants : undefined,
      problem_ids: ids.length > 0 ? ids : undefined,
    });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {editing ? TEXT.ADMIN.CONTESTS.DIALOG_EDIT_TITLE : TEXT.ADMIN.CONTESTS.DIALOG_CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {editing ? TEXT.ADMIN.CONTESTS.DIALOG_EDIT_DESC : TEXT.ADMIN.CONTESTS.DIALOG_CREATE_DESC}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.CONTESTS.FORM_TITLE}</Label>
            <Input placeholder={TEXT.ADMIN.CONTESTS.FORM_TITLE_PLACEHOLDER} value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>{TEXT.ADMIN.CONTESTS.FORM_DESCRIPTION}</Label>
            <Input placeholder={TEXT.ADMIN.CONTESTS.FORM_DESCRIPTION_PLACEHOLDER} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.CONTESTS.FORM_START_TIME}</Label>
              <Input type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.CONTESTS.FORM_END_TIME}</Label>
              <Input type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.CONTESTS.FORM_MAX_PARTICIPANTS}</Label>
              <Input type="number" min={0} value={maxParticipants} onChange={(e) => setMaxParticipants(Number(e.target.value))} />
            </div>
            <div className="space-y-2">
              <Label>{TEXT.ADMIN.CONTESTS.FORM_PROBLEM_IDS}</Label>
              <Input placeholder={TEXT.ADMIN.CONTESTS.FORM_PROBLEM_IDS_PLACEHOLDER} value={problemIds} onChange={(e) => setProblemIds(e.target.value)} />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" className="cursor-pointer" onClick={onClose} disabled={isSaving}>
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            className="cursor-pointer"
            disabled={!title.trim() || !startTime || !endTime || isSaving}
            onClick={handleSubmit}
          >
            {isSaving && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
