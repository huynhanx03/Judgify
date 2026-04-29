"use client";

/**
 * Shared problem form — used by both create and edit pages.
 * Single-column layout: metadata fields on top, markdown editor, test cases below.
 */

import { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import { Loader2, ArrowLeft, Plus, Trash2, Eye, EyeOff } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { problemService } from "@/services/problem.service";
import { difficultyService } from "@/services/difficulty.service";
import { tagService } from "@/services/tag.service";
import { notify, getErrorMessage } from "@/lib/toast";
import type { Problem } from "@/types/problem";
import type { Tag } from "@/types/tag";
import type { DifficultyResponse } from "@/types/difficulty";
import type { TestCaseResponse } from "@/types/submission";
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

/** Local test case draft for add/edit. */
interface TestCaseDraft {
  id?: number; // existing test case ID
  input: string;
  expected_output: string;
  is_hidden: boolean;
  order_index: number;
}

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

  // Test cases
  const [testCases, setTestCases] = useState<TestCaseDraft[]>([]);
  const [tcLoading, setTcLoading] = useState(false);
  const [tcSaving, setTcSaving] = useState<number | null>(null); // index being saved

  // Reference data
  const [difficulties, setDifficulties] = useState<DifficultyResponse[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);

  useEffect(() => {
    Promise.all([
      difficultyService.getAll(),
      tagService.getAll(),
    ]).then(([diffs, tagList]) => {
      setDifficulties(diffs);
      setTags(tagList);
      if (!problem && diffs.length > 0 && !difficultyId) {
        setDifficultyId(diffs[0].id);
      }
    }).catch(() => {});
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Load test cases in edit mode
  useEffect(() => {
    if (!problem) return;
    setTcLoading(true);
    problemService.getTestCases(problem.id)
      .then((data) => {
        setTestCases(data.map((tc) => ({
          id: tc.id,
          input: tc.input,
          expected_output: tc.expected_output,
          is_hidden: tc.is_hidden,
          order_index: tc.order_index,
        })));
      })
      .catch(() => {})
      .finally(() => setTcLoading(false));
  }, [problem]); // eslint-disable-line react-hooks/exhaustive-deps

  function toggleTag(id: number) {
    setTagIds((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);
  }

  // -- Test case helpers --

  function addTestCase() {
    setTestCases((prev) => [...prev, {
      input: "",
      expected_output: "",
      is_hidden: false,
      order_index: prev.length,
    }]);
  }

  function updateTestCase(index: number, field: keyof TestCaseDraft, value: string | boolean | number) {
    setTestCases((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  }

  async function removeTestCase(index: number) {
    const tc = testCases[index];
    if (tc.id) {
      try {
        await problemService.deleteTestCase(tc.id);
      } catch (err) {
        notify.error(getErrorMessage(err, "Xóa test case thất bại"));
        return;
      }
    }
    setTestCases((prev) => prev.filter((_, i) => i !== index));
  }

  async function saveTestCase(index: number) {
    const tc = testCases[index];
    if (!tc.input.trim() || !tc.expected_output.trim()) {
      notify.error("Input và Expected Output không được để trống");
      return;
    }
    if (!problem) return;

    setTcSaving(index);
    try {
      if (tc.id) {
        const updated = await problemService.updateTestCase(tc.id, {
          input: tc.input,
          expected_output: tc.expected_output,
          is_hidden: tc.is_hidden,
          order_index: tc.order_index,
        });
        setTestCases((prev) => {
          const next = [...prev];
          next[index] = { ...next[index], id: updated.id };
          return next;
        });
        notify.success("Test case đã cập nhật");
      } else {
        const created = await problemService.createTestCase(problem.id, {
          input: tc.input,
          expected_output: tc.expected_output,
          is_hidden: tc.is_hidden,
          order_index: tc.order_index,
        });
        setTestCases((prev) => {
          const next = [...prev];
          next[index] = { ...next[index], id: created.id };
          return next;
        });
        notify.success("Test case đã tạo");
      }
    } catch (err) {
      notify.error(getErrorMessage(err, "Lưu test case thất bại"));
    } finally {
      setTcSaving(null);
    }
  }

  async function handleSubmit() {
    if (!title.trim() || !difficultyId) return;
    setSaving(true);
    try {
      if (isEditing && problem) {
        await problemService.update(problem.id, {
          title, description, difficulty_id: difficultyId,
          time_limit_ms: timeLimitMs, memory_limit_kb: memoryLimitKb,
          tag_ids: tagIds, is_published: isPublished,
        });
        notify.success("Cập nhật thành công");
      } else {
        const created = await problemService.create({
          title, description, difficulty_id: difficultyId,
          time_limit_ms: timeLimitMs, memory_limit_kb: memoryLimitKb,
          tag_ids: tagIds,
        });
        notify.success("Tạo thành công");
        router.push(`/admin/problems/${created.id}/edit`);
        return;
      }
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

      {/* Test Cases — only in edit mode */}
      {isEditing && (
        <div className="rounded-lg border border-border bg-card p-4 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-base font-bold">Test Cases</Label>
              <p className="text-xs text-muted-foreground mt-0.5">Thêm, sửa hoặc xóa test case cho bài tập.</p>
            </div>
            <Button size="sm" variant="outline" className="cursor-pointer" onClick={addTestCase}>
              <Plus className="h-4 w-4 mr-1" />
              Thêm
            </Button>
          </div>

          {tcLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : testCases.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8 italic">Chưa có test case.</p>
          ) : (
            <div className="space-y-3">
              {testCases.map((tc, idx) => (
                <div key={tc.id ?? idx} className="rounded-lg border border-border/60 bg-muted/20 p-3 space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-semibold">Test Case #{tc.id ?? idx + 1}</span>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => updateTestCase(idx, "is_hidden", !tc.is_hidden)}
                        className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer p-1"
                        title={tc.is_hidden ? "Ẩn (hidden)" : "Hiện (visible)"}
                      >
                        {tc.is_hidden ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                      </button>
                      <button
                        type="button"
                        onClick={() => removeTestCase(idx)}
                        className="text-red-500 hover:text-red-400 transition-colors cursor-pointer p-1"
                        title="Xóa"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                    <div className="space-y-1">
                      <span className="text-xs font-semibold text-muted-foreground uppercase">Input</span>
                      <textarea
                        value={tc.input}
                        onChange={(e) => updateTestCase(idx, "input", e.target.value)}
                        rows={4}
                        className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-y"
                        placeholder="Dữ liệu đầu vào..."
                      />
                    </div>
                    <div className="space-y-1">
                      <span className="text-xs font-semibold text-muted-foreground uppercase">Expected Output</span>
                      <textarea
                        value={tc.expected_output}
                        onChange={(e) => updateTestCase(idx, "expected_output", e.target.value)}
                        rows={4}
                        className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-y"
                        placeholder="Kết quả mong đợi..."
                      />
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className={`text-xs px-2 py-0.5 rounded-md border ${tc.is_hidden ? "bg-amber-500/10 text-amber-500 border-amber-500/20" : "bg-green-500/10 text-green-500 border-green-500/20"}`}>
                      {tc.is_hidden ? "Hidden" : "Visible"}
                    </span>
                    <div className="flex-1" />
                    <Button
                      size="sm"
                      className="cursor-pointer"
                      disabled={!tc.input.trim() || !tc.expected_output.trim() || tcSaving === idx}
                      onClick={() => saveTestCase(idx)}
                    >
                      {tcSaving === idx && <Loader2 className="h-3.5 w-3.5 animate-spin mr-1" />}
                      {tc.id ? "Cập nhật" : "Lưu"}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {!isEditing && (
        <p className="text-sm text-muted-foreground text-center">Tạo bài tập trước, sau đó thêm test case ở trang chỉnh sửa.</p>
      )}

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
