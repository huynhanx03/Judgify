"use client";

import { useMemo, useRef, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  decodeSourceUTF8,
  formatJudgeBytes,
  inspectJudgeContent,
} from "@/lib/submissions/content-limits";
import {
  runtimeKeySchema,
  sourceFileMatchesRuntime,
} from "@/lib/submissions/runtime-catalog";
import { submissionService } from "@/services/submission.service";
import type {
  JudgeRuntimeCatalog,
  JudgeRuntime,
  RuntimeKey,
} from "@/types/submission";
import {
  AlertCircle,
  CheckCircle2,
  Code2,
  FileCode2,
  Loader2,
  LogIn,
  RotateCcw,
  Send,
  Upload,
} from "lucide-react";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";

interface FileSubmissionProps {
  onSubmit: (code: string, runtime: JudgeRuntime) => Promise<boolean>;
  isSubmitting: boolean;
  submissionDisabledReason?: string;
  loginRequired?: boolean;
  allowedRuntimeKeys?: readonly RuntimeKey[];
}

const EMPTY_RUNTIME_CATALOG: JudgeRuntimeCatalog = Object.freeze({
  version: 0,
  maximum_source_code_bytes: 0,
  runtimes: Object.freeze([]),
});

function runtimeLabel(runtime: JudgeRuntime): string {
  return runtime.display_name;
}

function inspectionError(
  reason: "required" | "contains_nul" | "too_large" | null,
  maximumBytes: number,
): string | null {
  switch (reason) {
    case "required":
      return TEXT.PROBLEM.SOURCE_CODE_REQUIRED;
    case "contains_nul":
      return TEXT.PROBLEM.SOURCE_FILE_CONTAINS_NUL;
    case "too_large":
      return TEXT.PROBLEM.SOURCE_FILE_TOO_LARGE(formatJudgeBytes(maximumBytes));
    default:
      return null;
  }
}

/** Runtime-driven source workbench. No accepted language or extension is owned by the client. */
export function FileSubmission({
  onSubmit,
  isSubmitting,
  submissionDisabledReason,
  loginRequired = false,
  allowedRuntimeKeys,
}: FileSubmissionProps) {
  const [runtimeKey, setRuntimeKey] = useState<RuntimeKey | null>(null);
  const [sourceCode, setSourceCode] = useState("");
  const [importedFilename, setImportedFilename] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [isInspecting, setIsInspecting] = useState(false);
  const [didAttemptSubmit, setDidAttemptSubmit] = useState(false);
  const [indentWithTab, setIndentWithTab] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const inspectionSequence = useRef(0);

  const runtimeResource = useRetryableResource<JudgeRuntimeCatalog>({
    resetKey: "judge-runtime-catalog",
    initialData: EMPTY_RUNTIME_CATALOG,
    load: submissionService.getRuntimeCatalog,
  });
  const runtimeReady = runtimeResource.status === "ready";
  const availableRuntimes = useMemo(() => {
    if (allowedRuntimeKeys === undefined) {
      return runtimeResource.data.runtimes;
    }
    const allowed = new Set<RuntimeKey>(allowedRuntimeKeys);
    return runtimeResource.data.runtimes.filter((runtime) =>
      allowed.has(runtime.runtime_key),
    );
  }, [allowedRuntimeKeys, runtimeResource.data.runtimes]);
  const selectedRuntimeKey = availableRuntimes.some(
    (runtime) => runtime.runtime_key === runtimeKey,
  )
    ? runtimeKey
    : (availableRuntimes[0]?.runtime_key ?? null);
  const selectedRuntime = availableRuntimes.find(
    (runtime) => runtime.runtime_key === selectedRuntimeKey,
  );
  const maximumSourceBytes =
    runtimeResource.data.maximum_source_code_bytes || 1;
  const inspection = useMemo(
    () => inspectJudgeContent(sourceCode, maximumSourceBytes),
    [maximumSourceBytes, sourceCode],
  );
  const validationError =
    runtimeReady &&
    !inspection.valid &&
    (didAttemptSubmit || sourceCode.length > 0)
      ? inspectionError(inspection.reason, maximumSourceBytes)
      : null;
  const displayedError = fileError ?? validationError;
  const allExtensions = useMemo(
    () =>
      [...new Set(availableRuntimes.flatMap(
        (runtime) => runtime.file_extensions,
      ))],
    [availableRuntimes],
  );

  function updateSourceCode(value: string) {
    setSourceCode(value);
    setFileError(null);
  }

  async function handleFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const input = event.currentTarget;
    const selected = input.files?.[0];
    const sequence = ++inspectionSequence.current;
    setFileError(null);
    if (!selected || !runtimeReady) return;

    const matchingRuntimes = availableRuntimes.filter((runtime) =>
      sourceFileMatchesRuntime(selected.name, runtime),
    );
    if (matchingRuntimes.length === 0) {
      setFileError(
        TEXT.PROBLEM.SOURCE_FILE_UNSUPPORTED(allExtensions.join(", ")),
      );
      input.value = "";
      return;
    }
    if (selected.size > maximumSourceBytes) {
      setFileError(
        TEXT.PROBLEM.SOURCE_FILE_TOO_LARGE(
          formatJudgeBytes(maximumSourceBytes),
        ),
      );
      input.value = "";
      return;
    }

    const inferredRuntime =
      matchingRuntimes.find(
        (runtime) => runtime.runtime_key === selectedRuntimeKey,
      ) ??
      matchingRuntimes[0];
    setIsInspecting(true);
    try {
      const code = decodeSourceUTF8(await selected.arrayBuffer());
      if (sequence !== inspectionSequence.current) return;
      const sourceInspection = inspectJudgeContent(code, maximumSourceBytes);
      if (!sourceInspection.valid) {
        setFileError(
          inspectionError(sourceInspection.reason, maximumSourceBytes) ??
            TEXT.PROBLEM.SOURCE_FILE_INVALID,
        );
        return;
      }

      setRuntimeKey(inferredRuntime.runtime_key);
      setSourceCode(code);
      setImportedFilename(selected.name);
      setDidAttemptSubmit(false);
    } catch (error) {
      if (sequence !== inspectionSequence.current) return;
      setFileError(
        error instanceof Error && error.name === "JudgeSourceDecodeError"
          ? TEXT.PROBLEM.SOURCE_FILE_INVALID_UTF8
          : TEXT.PROBLEM.SOURCE_FILE_READ_ERROR,
      );
    } finally {
      if (sequence === inspectionSequence.current) setIsInspecting(false);
      input.value = "";
    }
  }

  function clearSource() {
    inspectionSequence.current += 1;
    setSourceCode("");
    setImportedFilename(null);
    setFileError(null);
    setDidAttemptSubmit(false);
    setIsInspecting(false);
    if (fileInputRef.current) fileInputRef.current.value = "";
    editorRef.current?.focus();
  }

  async function handleSubmit() {
    setDidAttemptSubmit(true);
    setFileError(null);
    if (
      submissionDisabledReason ||
      !runtimeReady ||
      !selectedRuntime ||
      !inspection.valid
    ) {
      return;
    }

    try {
      const succeeded = await onSubmit(sourceCode, selectedRuntime);
      if (succeeded) setDidAttemptSubmit(false);
    } catch {
      setFileError(TEXT.PROBLEM.SUBMIT_ERROR);
    }
  }

  function handleEditorKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
      event.preventDefault();
      if (!isSubmitting && !submissionDisabledReason) void handleSubmit();
      return;
    }
    if (event.key === "Escape" && indentWithTab) {
      event.preventDefault();
      setIndentWithTab(false);
      return;
    }
    if (event.key !== "Tab" || !indentWithTab) return;

    event.preventDefault();
    const editor = event.currentTarget;
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const nextCode = `${sourceCode.slice(0, start)}  ${sourceCode.slice(end)}`;
    updateSourceCode(nextCode);
    requestAnimationFrame(() => {
      editor.selectionStart = start + 2;
      editor.selectionEnd = start + 2;
    });
  }

  return (
    <section
      className="flex min-h-[34rem] flex-col overflow-hidden rounded-xl border border-border/70 bg-background shadow-sm"
      aria-labelledby="submission-workbench-title"
    >
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-border/70 bg-muted/20 px-4 py-3">
        <div className="flex min-w-0 items-start gap-3">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Code2 className="size-4" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h2
              id="submission-workbench-title"
              className="text-sm font-semibold text-foreground"
            >
              {TEXT.PROBLEM.WORKBENCH_TITLE}
            </h2>
            <p className="mt-0.5 text-xs leading-5 text-muted-foreground">
              {TEXT.PROBLEM.WORKBENCH_DESCRIPTION}
            </p>
          </div>
        </div>
        {runtimeReady && selectedRuntime ? (
          <span className="inline-flex min-h-7 items-center gap-1.5 rounded-full border border-status-success/25 bg-status-success/10 px-2.5 text-xs font-medium text-status-success">
            <CheckCircle2 className="size-3.5" aria-hidden="true" />
            {TEXT.PROBLEM.RUNTIME_READY}
          </span>
        ) : null}
      </header>

      {runtimeResource.status === "loading" ? (
        <DataLoadingFeedback
          label={TEXT.PROBLEM.RUNTIME_CATALOG_LOADING}
          className="m-4 flex-1"
        />
      ) : runtimeResource.status === "error" ? (
        <DataLoadFeedback
          title={TEXT.PROBLEM.RUNTIME_CATALOG_ERROR_TITLE}
          description={TEXT.PROBLEM.RUNTIME_CATALOG_ERROR_DESCRIPTION}
          retryLabel={TEXT.PROBLEM.RETRY}
          onRetry={runtimeResource.retry}
          compact
          className="m-4"
        />
      ) : selectedRuntime ? (
        <>
          <div className="flex flex-wrap items-end gap-3 border-b border-border/60 px-4 py-3">
            <div className="min-w-44 flex-1 sm:max-w-56">
              <label
                id="submission-language-label"
                className="mb-1.5 block text-xs font-medium text-muted-foreground"
              >
                {TEXT.PROBLEM.SELECT_LANGUAGE}
              </label>
              <Select
                value={selectedRuntimeKey ?? ""}
                onValueChange={(value) =>
                  setRuntimeKey(runtimeKeySchema.parse(value))
                }
              >
                <SelectTrigger aria-labelledby="submission-language-label">
                  <span>{runtimeLabel(selectedRuntime)}</span>
                </SelectTrigger>
                <SelectContent>
                  {availableRuntimes.map((runtime) => (
                    <SelectItem
                      key={runtime.runtime_key}
                      value={runtime.runtime_key}
                    >
                      {runtimeLabel(runtime)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="hidden min-w-0 flex-1 sm:block">
              <p className="text-xs font-medium text-muted-foreground">
                {TEXT.PROBLEM.RUNTIME_FILE}
              </p>
              <p className="mt-1 truncate font-mono text-xs text-foreground">
                {selectedRuntime.source_filename}
                <span className="ml-2 font-sans text-muted-foreground">
                  · {selectedRuntime.compiled
                    ? TEXT.PROBLEM.RUNTIME_COMPILED
                    : TEXT.PROBLEM.RUNTIME_INTERPRETED}
                </span>
              </p>
            </div>

            <input
              ref={fileInputRef}
              type="file"
              accept={allExtensions.join(",")}
              onChange={handleFileChange}
              className="hidden"
              tabIndex={-1}
            />
            <Button
              type="button"
              variant="outline"
              className="h-11 cursor-pointer gap-2"
              onClick={() => fileInputRef.current?.click()}
              disabled={isInspecting || isSubmitting}
            >
              {isInspecting ? (
                <Loader2
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : (
                <Upload className="size-4" aria-hidden="true" />
              )}
              {isInspecting
                ? TEXT.PROBLEM.CHECKING_FILE
                : TEXT.PROBLEM.IMPORT_SOURCE}
            </Button>
          </div>

          <div className="relative flex flex-1 flex-col bg-muted/10">
            <label htmlFor="submission-source-editor" className="sr-only">
              {TEXT.PROBLEM.EDITOR_LABEL}
            </label>
            <Textarea
              ref={editorRef}
              id="submission-source-editor"
              value={sourceCode}
              onChange={(event) => updateSourceCode(event.target.value)}
              onKeyDown={handleEditorKeyDown}
              placeholder={TEXT.PROBLEM.EDITOR_PLACEHOLDER(
                selectedRuntime.source_filename,
              )}
              aria-invalid={Boolean(displayedError)}
              aria-describedby="submission-indent-with-tab-hint submission-source-status submission-source-error"
              spellCheck={false}
              autoCapitalize="none"
              autoCorrect="off"
              className="min-h-[26rem] flex-1 resize-y rounded-none border-0 bg-surface-sunken/40 p-4 font-mono text-[13px] leading-6 shadow-none focus-visible:ring-2 focus-visible:ring-inset"
            />
          </div>

          <div className="border-t border-border/70 bg-background px-4 py-3">
            <div className="mb-3 flex items-start gap-3 rounded-lg border border-border bg-surface-sunken/40 px-3 py-2">
              <Switch
                id="submission-indent-with-tab"
                checked={indentWithTab}
                onCheckedChange={setIndentWithTab}
                aria-describedby="submission-indent-with-tab-hint"
              />
              <div className="min-w-0">
                <label
                  htmlFor="submission-indent-with-tab"
                  className="cursor-pointer text-xs font-medium text-foreground"
                >
                  {TEXT.PROBLEM.INDENT_WITH_TAB}
                </label>
                <p
                  id="submission-indent-with-tab-hint"
                  className="mt-0.5 text-xs leading-5 text-muted-foreground"
                >
                  {indentWithTab
                    ? TEXT.PROBLEM.EDITOR_ESCAPE_HINT
                    : TEXT.PROBLEM.INDENT_WITH_TAB_HINT}
                </p>
              </div>
            </div>

            <div
              id="submission-source-status"
              className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"
            >
              <span className="inline-flex min-w-0 items-center gap-1.5">
                <FileCode2 className="size-3.5 shrink-0" aria-hidden="true" />
                <span className="truncate">
                  {importedFilename
                    ? TEXT.PROBLEM.SOURCE_IMPORTED(importedFilename)
                    : TEXT.PROBLEM.SOURCE_MANUAL}
                </span>
              </span>
              <span
                className={
                  inspection.reason === "too_large"
                    ? "font-semibold text-destructive"
                    : "tabular-nums"
                }
              >
                {TEXT.PROBLEM.SOURCE_FILE_SIZE(
                  formatJudgeBytes(inspection.actualBytes),
                  formatJudgeBytes(maximumSourceBytes),
                )}
              </span>
            </div>

            <div id="submission-source-error" className="min-h-6 pt-1">
              {displayedError ? (
                <p
                  role="alert"
                  className="flex items-start gap-1.5 text-xs font-medium text-destructive"
                >
                  <AlertCircle
                    className="mt-0.5 size-3.5 shrink-0"
                    aria-hidden="true"
                  />
                  {displayedError}
                </p>
              ) : null}
            </div>

            {submissionDisabledReason ? (
              <div
                className="mt-2 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-status-warning/25 bg-status-warning/10 px-3 py-2 text-xs font-medium text-status-warning"
                role="status"
              >
                <span>{submissionDisabledReason}</span>
                {loginRequired ? (
                  <Link
                    href={APP_ROUTES.LOGIN}
                    className="inline-flex min-h-11 items-center gap-2 rounded-lg px-3 text-foreground outline-none transition-colors hover:bg-background/60 focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    <LogIn className="size-4" aria-hidden="true" />
                    {TEXT.PROBLEM.LOGIN_TO_SUBMIT}
                  </Link>
                ) : null}
              </div>
            ) : null}

            <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  className="h-11 cursor-pointer gap-2 text-muted-foreground"
                  onClick={clearSource}
                  disabled={sourceCode.length === 0 || isSubmitting}
                >
                  <RotateCcw className="size-4" aria-hidden="true" />
                  {TEXT.PROBLEM.CLEAR_SOURCE}
                </Button>
                <span className="hidden text-xs text-muted-foreground xl:inline">
                  {TEXT.PROBLEM.SUBMIT_SHORTCUT}
                </span>
              </div>
              <Button
                type="button"
                onClick={() => void handleSubmit()}
                disabled={
                  isSubmitting ||
                  isInspecting ||
                  !inspection.valid ||
                  Boolean(submissionDisabledReason)
                }
                className="h-11 min-w-32 cursor-pointer gap-2 px-5"
              >
                {isSubmitting ? (
                  <Loader2
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                ) : (
                  <Send className="size-4" aria-hidden="true" />
                )}
                {isSubmitting ? TEXT.PROBLEM.SUBMITTING : TEXT.PROBLEM.SUBMIT}
              </Button>
            </div>
          </div>
        </>
      ) : (
        <DataLoadFeedback
          title={TEXT.PROBLEM.RUNTIME_NOT_ALLOWED_TITLE}
          description={TEXT.PROBLEM.RUNTIME_NOT_ALLOWED_DESCRIPTION}
          retryLabel={TEXT.PROBLEM.RETRY}
          onRetry={runtimeResource.retry}
          compact
          className="m-4"
        />
      )}
    </section>
  );
}
