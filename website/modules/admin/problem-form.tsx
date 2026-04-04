"use client";

/**
 * Shared problem form — used by both create and edit pages.
 * Single-column layout: metadata fields on top, markdown editor below.
 */

import { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import { Loader2, ArrowLeft } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { adminService } from "@/services/admin.service";
import { notify, getErrorMessage } from "@/lib/toast";
import type { Problem } from "@/types/problem";
import type { Tag } from "@/types/tag";
import type { DifficultyResponse } from "@/types/difficulty";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import "katex/dist/katex.min.css";

/** Lazy-load MD editor to avoid SSR issues */
const MDEditor = dynamic(() => import("@uiw/react-md-editor"), { ssr: false });

/** Preview options — same KaTeX math rendering as client MarkdownRenderer */
const mdPreviewOptions = {
  rehypePlugins: [rehypeKatex],
  remarkPlugins: [remarkMath],
};

const DEFAULT_TIME_LIMIT = 1000;
const DEFAULT_MEMORY_LIMIT = 262144;

interface ProblemFormProps {
  /** Existing problem data for edit mode. Null = create mode. */
  problem?: Problem | null;
}

export function ProblemForm({ problem }: ProblemFormProps) {
  const router = useRouter();
  const isEditing = !!problem;

  const [title, setTitle] = useState(problem?.title ?? "");
  const [description, setDescription] = useState(problem?.description ?? "");
  const [difficultyId, setDifficultyId] = useState(problem?.difficulty_id ?? 0);
  const [timeLimitMs, setTimeLimitMs] = useState(problem?.time_limit_ms ?? DEFAULT_TIME_LIMIT);
  const [memoryLimitKb, setMemoryLimitKb] = useState(problem?.memory_limit_kb ?? DEFAULT_MEMORY_LIMIT);
  const [tagIds, setTagIds] = useState<number[]>(problem?.tags?.map((t) => t.id) ?? []);
  const [isPublished, setIsPublished] = useState(problem?.is_published ?? false);
  const [saving, setSaving] = useState(false);

  // Reference data
  const [difficulties, setDifficulties] = useState<DifficultyResponse[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);

  useEffect(() => {
    Promise.all([
      adminService.getAllDifficulties(),
      adminService.getAllTags(),
    ]).then(([diffs, tagList]) => {
      setDifficulties(diffs);
      setTags(tagList);
      if (!problem && diffs.length > 0 && !difficultyId) {
        setDifficultyId(diffs[0].id);
      }
    }).catch(() => {});
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function toggleTag(id: number) {
    setTagIds((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);
  }

  async function handleSubmit() {
    if (!title.trim() || !difficultyId) return;
    setSaving(true);
    try {
      if (isEditing && problem) {
        await adminService.updateProblem(problem.id, {
          title, description, difficulty_id: difficultyId,
          time_limit_ms: timeLimitMs, memory_limit_kb: memoryLimitKb,
          tag_ids: tagIds, is_published: isPublished,
        });
        notify.success("Cập nhật thành công");
      } else {
        await adminService.createProblem({
          title, description, difficulty_id: difficultyId,
          time_limit_ms: timeLimitMs, memory_limit_kb: memoryLimitKb,
          tag_ids: tagIds,
        });
        notify.success("Tạo thành công");
      }
      router.push("/admin/problems");
    } catch (err) {
      notify.error(getErrorMessage(err, "Thao tác thất bại"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Header with back button */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" className="h-8 w-8 cursor-pointer" onClick={() => router.push("/admin/problems")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{isEditing ? "Chỉnh Sửa Bài Tập" : "Tạo Bài Tập Mới"}</h1>
          <p className="text-sm text-muted-foreground mt-0.5">{isEditing ? "Cập nhật thông tin bài tập." : "Thêm bài tập mới vào hệ thống."}</p>
        </div>
      </div>

      {/* Metadata fields */}
      <div className="rounded-lg border border-border bg-card p-4 space-y-4">
        <div className="space-y-2">
          <Label>Tiêu đề</Label>
          <Input placeholder="VD: Two Sum" value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="space-y-2">
            <Label>Độ khó</Label>
            <select value={difficultyId} onChange={(e) => setDifficultyId(Number(e.target.value))}
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm">
              <option value={0} disabled>Chọn</option>
              {difficulties.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
            </select>
          </div>
          <div className="space-y-2">
            <Label>Time Limit (ms)</Label>
            <Input type="number" min={100} max={30000} value={timeLimitMs}
              onChange={(e) => setTimeLimitMs(Number(e.target.value))} />
          </div>
          <div className="space-y-2">
            <Label>Memory Limit (KB)</Label>
            <Input type="number" min={1024} max={1048576} value={memoryLimitKb}
              onChange={(e) => setMemoryLimitKb(Number(e.target.value))} />
          </div>
          {isEditing && (
            <div className="space-y-2">
              <Label>Trạng thái</Label>
              <div className="flex items-center gap-2 h-10">
                <Switch checked={isPublished} onCheckedChange={setIsPublished} />
                <span className="text-sm text-muted-foreground">{isPublished ? "Published" : "Draft"}</span>
              </div>
            </div>
          )}
        </div>

        <div className="space-y-2">
          <Label>Tags</Label>
          <div className="flex flex-wrap gap-2">
            {tags.map((t) => {
              const active = tagIds.includes(t.id);
              return (
                <button key={t.id} type="button" onClick={() => toggleTag(t.id)}
                  className={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-semibold transition-colors cursor-pointer ${active ? "bg-primary/10 text-primary border-primary" : "text-muted-foreground border-border hover:border-foreground/30"}`}>
                  {t.name}
                </button>
              );
            })}
          </div>
        </div>
      </div>

      {/* Markdown editor */}
      <div className="rounded-lg border border-border bg-card p-4 space-y-2">
        <Label>Mô tả (Markdown)</Label>
        <div data-color-mode="light">
          <MDEditor
            value={description}
            onChange={(val) => setDescription(val ?? "")}
            height={500}
            preview="live"
            previewOptions={mdPreviewOptions}
          />
        </div>
      </div>

      {/* Actions */}
      <div className="flex justify-end gap-3 pb-6">
        <Button variant="outline" className="cursor-pointer" onClick={() => router.push("/admin/problems")}>Hủy</Button>
        <Button className="cursor-pointer" disabled={!title.trim() || !difficultyId || saving} onClick={handleSubmit}>
          {saving && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
          {isEditing ? "Lưu thay đổi" : "Tạo bài tập"}
        </Button>
      </div>
    </div>
  );
}
