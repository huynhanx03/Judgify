"use client";

import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import {
  Archive,
  ArrowLeft,
  CheckCircle2,
  FileCode2,
  FlaskConical,
  Gauge,
  Layers3,
  Loader2,
  Plus,
  Rocket,
  Save,
  ShieldAlert,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import rehypeKatex from "rehype-katex";
import remarkMath from "remark-math";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  PROBLEM_CHECKER_CONFIG_KIND,
  PROBLEM_TEST_KIND,
} from "@/constants/problem";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { entityIDSchema } from "@/lib/api/contracts";
import {
  AUTHORING_CATALOG_CHANGED_REASON,
  authoringCatalogsMatch,
  checkerIdentity,
  compatibleCheckerIdentities,
  compatibleRuntimeKeys,
  isCompatibleAuthoringSelection,
  splitCheckerIdentity,
} from "@/lib/problems/authoring-capabilities";
import { cn } from "@/lib/utils";
import { getErrorMessage, notify } from "@/lib/toast";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import { difficultyService } from "@/services/difficulty.service";
import { problemService } from "@/services/problem.service";
import { tagService } from "@/services/tag.service";
import type { DifficultyResponse } from "@/types/difficulty";
import type {
  Problem,
  ProblemAuthoringCatalog,
  ProblemAuthoringDraft,
  ProblemAuthoringTestcaseGroup,
  ProblemDraftReceipt,
  ProblemLifecycle,
  SaveProblemDraftInput,
} from "@/types/problem";
import type { Tag } from "@/types/tag";
import type { EntityID } from "@/types/api";
import "katex/dist/katex.min.css";

const MDEditor = dynamic(() => import("@uiw/react-md-editor"), {
  ssr: false,
  loading: () => (
    <DataLoadingFeedback
      label={ADMIN_TEXT.PROBLEM_FORM.EDITOR_LOADING}
      className="min-h-[32rem]"
    />
  ),
});

const markdownPreviewOptions = {
  rehypePlugins: [rehypeKatex],
  remarkPlugins: [remarkMath],
};

type BusyAction = "save" | "publish" | "archive" | "reload" | null;
type ConfirmAction = "leave" | "archive" | null;
type NumericInputValue = number | "";

interface EditorTestcase {
  clientKey: string;
  testCaseID?: string;
  kind: "sample" | "hidden";
  input: string;
  expectedOutput: string;
}

interface EditorGroup {
  clientKey: string;
  stopOnFailure: boolean;
  cases: EditorTestcase[];
}

interface EditorState {
  slug: string;
  title: string;
  statementMarkdown: string;
  difficultyID: string;
  tagIDs: string[];
  cpuTimeMs: NumericInputValue;
  wallTimeMs: NumericInputValue;
  memoryLimitKb: NumericInputValue;
  outputLimitBytes: NumericInputValue;
  processLimit: NumericInputValue;
  checkerIdentity: string;
  absoluteTolerance: string;
  relativeTolerance: string;
  allowedRuntimeKeys: string[];
  groups: EditorGroup[];
  reason: string;
}

interface ProblemFormProps {
  draft?: ProblemAuthoringDraft | null;
  initialProblem?: Problem | null;
}

interface SavedSelection {
  revisionID: EntityID;
  testsetRevisionID: EntityID;
  version: number;
  artifactStatus: "ready" | "rejected";
  artifactDiagnostics: string[];
}

function initialGroups(
  groups?: ProblemAuthoringTestcaseGroup[],
): EditorGroup[] {
  if (groups?.length) {
    return groups.map((group, groupIndex) => ({
      clientKey: `persisted-group:${groupIndex}`,
      stopOnFailure: group.stop_on_failure,
      cases: group.cases.map((testcase, caseIndex) => ({
        clientKey: `persisted-case:${testcase.test_case_id}:${caseIndex}`,
        testCaseID: testcase.test_case_id,
        kind: testcase.kind,
        input: testcase.input,
        expectedOutput: testcase.expected_output,
      })),
    }));
  }
  return [
    {
      clientKey: "initial-sample-group",
      stopOnFailure: false,
      cases: [{
        clientKey: "initial-sample-case",
        kind: PROBLEM_TEST_KIND.SAMPLE,
        input: "",
        expectedOutput: "",
      }],
    },
    {
      clientKey: "initial-hidden-group",
      stopOnFailure: true,
      cases: [{
        clientKey: "initial-hidden-case",
        kind: PROBLEM_TEST_KIND.HIDDEN,
        input: "",
        expectedOutput: "",
      }],
    },
  ];
}

function toleranceValue(
  config: Record<string, unknown> | undefined,
  key: "abs_tol" | "rel_tol",
): string {
  const value = config?.[key];
  return typeof value === "number" && Number.isFinite(value)
    ? String(value)
    : "";
}

function createInitialState(
  draft?: ProblemAuthoringDraft | null,
  problem?: Problem | null,
): EditorState {
  return {
    slug: draft?.slug ?? problem?.slug ?? "",
    title: draft?.title ?? problem?.title ?? "",
    statementMarkdown:
      draft?.statement_markdown ?? problem?.description ?? "",
    difficultyID: draft?.difficulty_id ?? problem?.difficulty_id ?? "",
    tagIDs:
      draft?.tag_ids.slice() ?? problem?.tags?.map((tag) => tag.id) ?? [],
    cpuTimeMs: draft?.cpu_time_ms ?? problem?.time_limit_ms ?? "",
    wallTimeMs: draft?.wall_time_ms ?? "",
    memoryLimitKb: draft?.memory_limit_kb ?? problem?.memory_limit_kb ?? "",
    outputLimitBytes: draft?.output_limit_bytes ?? "",
    processLimit: draft?.process_limit ?? "",
    checkerIdentity: draft
      ? checkerIdentity(draft.checker_key, draft.checker_version)
      : "",
    absoluteTolerance: toleranceValue(draft?.checker_config, "abs_tol"),
    relativeTolerance: toleranceValue(draft?.checker_config, "rel_tol"),
    allowedRuntimeKeys: draft?.allowed_runtime_keys.slice() ?? [],
    groups: initialGroups(draft?.groups),
    reason: "",
  };
}

function authoredSignature(value: EditorState): string {
  return JSON.stringify({
    ...value,
    reason: undefined,
    tagIDs: [...value.tagIDs].sort(),
    allowedRuntimeKeys: [...value.allowedRuntimeKeys].sort(),
    groups: value.groups.map((group) => ({
      stopOnFailure: group.stopOnFailure,
      cases: group.cases.map((testcase) => ({
        testCaseID: testcase.testCaseID,
        kind: testcase.kind,
        input: testcase.input,
        expectedOutput: testcase.expectedOutput,
      })),
    })),
  });
}

function byteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}

function isBoundedInteger(
  value: NumericInputValue,
  minimum: number,
  maximum: number,
): value is number {
  return (
    typeof value === "number" &&
    Number.isInteger(value) &&
    value >= minimum &&
    value <= maximum
  );
}

function mergeReferences<T extends { id: string }>(
  catalog: readonly T[],
  fallbacks: readonly T[],
): T[] {
  const merged = new Map(catalog.map((item) => [item.id, item]));
  for (const item of fallbacks) {
    if (!merged.has(item.id)) merged.set(item.id, item);
  }
  return [...merged.values()];
}

function lifecycleLabel(value: ProblemLifecycle): string {
  switch (value) {
    case "published":
      return ADMIN_TEXT.PROBLEM_FORM.PUBLISHED;
    case "archived":
      return ADMIN_TEXT.PROBLEM_FORM.ARCHIVED;
    default:
      return ADMIN_TEXT.PROBLEM_FORM.DRAFT;
  }
}

function lifecycleVariant(
  value: ProblemLifecycle,
): "default" | "secondary" | "outline" {
  if (value === "published") return "default";
  if (value === "archived") return "outline";
  return "secondary";
}

function editorValidationErrors(
  value: EditorState,
  catalog: ProblemAuthoringCatalog | null,
): string[] {
  if (!catalog) return [ADMIN_TEXT.PROBLEM_FORM.CATALOG_REQUIRED];

  const errors: string[] = [];
  const policy = catalog.policy;
  const normalizedSlug = value.slug.trim();
  const normalizedTitle = value.title.trim();
  const normalizedStatement = value.statementMarkdown.trim();
  const normalizedReason = value.reason.trim();
  const activeRuntimeKeys = new Set(
    catalog.runtimes.map((runtime) => runtime.key),
  );
  const checker = catalog.checkers.find(
    (option) =>
      checkerIdentity(option.key, option.version) === value.checkerIdentity,
  );

  if (
    !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(normalizedSlug) ||
    byteLength(normalizedSlug) > policy.slug_maximum_bytes
  ) {
    errors.push(
      ADMIN_TEXT.PROBLEM_FORM.SLUG_INVALID(policy.slug_maximum_bytes),
    );
  }
  if (
    normalizedTitle.length === 0 ||
    byteLength(normalizedTitle) > policy.title_maximum_bytes
  ) {
    errors.push(
      ADMIN_TEXT.PROBLEM_FORM.TITLE_INVALID(policy.title_maximum_bytes),
    );
  }
  if (
    normalizedStatement.length === 0 ||
    byteLength(value.statementMarkdown) > policy.statement_maximum_bytes
  ) {
    errors.push(
      ADMIN_TEXT.PROBLEM_FORM.STATEMENT_INVALID(
        policy.statement_maximum_bytes,
      ),
    );
  }
  if (!value.difficultyID) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.DIFFICULTY_REQUIRED);
  }
  if (value.tagIDs.length > policy.maximum_tags) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.TAGS_LIMIT(policy.maximum_tags));
  }
  if (
    !isBoundedInteger(
      value.cpuTimeMs,
      policy.cpu_time_ms.minimum,
      policy.cpu_time_ms.maximum,
    ) ||
    !isBoundedInteger(
      value.wallTimeMs,
      policy.wall_time_ms.minimum,
      policy.wall_time_ms.maximum,
    ) ||
    !isBoundedInteger(
      value.memoryLimitKb,
      policy.memory_limit_kb.minimum,
      policy.memory_limit_kb.maximum,
    ) ||
    !isBoundedInteger(
      value.outputLimitBytes,
      policy.output_limit_bytes.minimum,
      policy.output_limit_bytes.maximum,
    ) ||
    !isBoundedInteger(
      value.processLimit,
      policy.process_limit.minimum,
      policy.process_limit.maximum,
    ) ||
    (typeof value.cpuTimeMs === "number" &&
      typeof value.wallTimeMs === "number" &&
      value.wallTimeMs < value.cpuTimeMs)
  ) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.LIMITS_INVALID);
  }
  if (
    value.allowedRuntimeKeys.length === 0 ||
    value.allowedRuntimeKeys.length > policy.maximum_runtimes ||
    value.allowedRuntimeKeys.some((key) => !activeRuntimeKeys.has(key))
  ) {
    errors.push(
      value.allowedRuntimeKeys.length > policy.maximum_runtimes
        ? ADMIN_TEXT.PROBLEM_FORM.RUNTIME_LIMIT(policy.maximum_runtimes)
        : ADMIN_TEXT.PROBLEM_FORM.RUNTIME_REQUIRED,
    );
  }
  if (!checker) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.CHECKER_REQUIRED);
  } else if (
    value.allowedRuntimeKeys.length > 0 &&
    value.allowedRuntimeKeys.every((key) => activeRuntimeKeys.has(key)) &&
    !isCompatibleAuthoringSelection(
      catalog,
      value.allowedRuntimeKeys,
      value.checkerIdentity,
    )
  ) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.CAPABILITY_PAIR_UNAVAILABLE);
  }
  if (
    checker &&
    checker.config_kind ===
      PROBLEM_CHECKER_CONFIG_KIND.FLOAT_TOLERANCE &&
    (![value.absoluteTolerance, value.relativeTolerance].every((candidate) => {
      const parsed = Number(candidate);
      return candidate.trim() !== "" &&
        Number.isFinite(parsed) &&
        parsed >= 0 &&
        parsed <= 1;
    }))
  ) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.TOLERANCE_INVALID);
  }

  const testCount = value.groups.reduce(
    (total, group) => total + group.cases.length,
    0,
  );
  if (
    value.groups.length === 0 ||
    value.groups.length > policy.maximum_groups ||
    testCount === 0 ||
    testCount > policy.maximum_testcases ||
    value.groups.some((group) => group.cases.length === 0)
  ) {
    errors.push(
      ADMIN_TEXT.PROBLEM_FORM.TESTSET_LIMIT(
        policy.maximum_groups,
        policy.maximum_testcases,
      ),
    );
  }
  if (
    !value.groups.some((group) =>
      group.cases.some((testcase) =>
        testcase.kind === PROBLEM_TEST_KIND.HIDDEN
      )
    )
  ) {
    errors.push(ADMIN_TEXT.PROBLEM_FORM.HIDDEN_TEST_REQUIRED);
  }
  if (
    value.groups.some((group) =>
      group.cases.some(
        (testcase) =>
          testcase.input.includes("\u0000") ||
          testcase.expectedOutput.includes("\u0000") ||
          byteLength(testcase.input) > policy.test_asset_maximum_bytes ||
          byteLength(testcase.expectedOutput) >
            policy.test_asset_maximum_bytes,
      )
    )
  ) {
    errors.push(
      ADMIN_TEXT.PROBLEM_FORM.TEST_CONTENT_INVALID(
        policy.test_asset_maximum_bytes,
      ),
    );
  }
  if (
    normalizedReason.length === 0 ||
    byteLength(normalizedReason) > policy.reason_maximum_bytes
  ) {
    errors.push(
      ADMIN_TEXT.PROBLEM_FORM.REASON_INVALID(policy.reason_maximum_bytes),
    );
  }
  return [...new Set(errors)];
}

function draftInput(
  value: EditorState,
  expectedVersion: number,
  catalog: ProblemAuthoringCatalog,
): SaveProblemDraftInput {
  const [checkerKey, checkerVersion] = splitCheckerIdentity(
    value.checkerIdentity,
  );
  const checker = catalog.checkers.find(
    (option) =>
      option.key === checkerKey && option.version === checkerVersion,
  );
  const checkerConfig =
    checker?.config_kind === PROBLEM_CHECKER_CONFIG_KIND.FLOAT_TOLERANCE
      ? {
        abs_tol: Number(value.absoluteTolerance),
        rel_tol: Number(value.relativeTolerance),
      }
      : {};

  return {
    expected_version: expectedVersion,
    catalog_version: catalog.catalog_version,
    title: value.title.trim(),
    statement_markdown: value.statementMarkdown,
    difficulty_id: entityIDSchema.parse(value.difficultyID),
    tag_ids: value.tagIDs.map((id) => entityIDSchema.parse(id)).sort(),
    cpu_time_ms: value.cpuTimeMs as number,
    wall_time_ms: value.wallTimeMs as number,
    memory_limit_kb: value.memoryLimitKb as number,
    output_limit_bytes: value.outputLimitBytes as number,
    process_limit: value.processLimit as number,
    checker_key: checkerKey,
    checker_version: checkerVersion,
    checker_config: checkerConfig,
    allowed_runtime_keys: [...value.allowedRuntimeKeys].sort(),
    groups: value.groups.map((group) => ({
      stop_on_failure: group.stopOnFailure,
      cases: group.cases.map((testcase) => ({
        ...(testcase.testCaseID
          ? { test_case_id: entityIDSchema.parse(testcase.testCaseID) }
          : {}),
        kind: testcase.kind,
        input: testcase.input,
        expected_output: testcase.expectedOutput,
      })),
    })),
    reason: value.reason.trim(),
  };
}

export function ProblemForm({
  draft,
  initialProblem,
}: ProblemFormProps) {
  const router = useRouter();
  const { resolvedTheme } = useTheme();
  const { can } = useAuth();
  const keySequence = useRef(0);
  const validationSummaryRef = useRef<HTMLElement | null>(null);
  const initialState = useMemo(
    () => createInitialState(draft, initialProblem),
    [draft, initialProblem],
  );
  const baseline = useRef(authoredSignature(initialState));

  const [editor, setEditor] = useState<EditorState>(initialState);
  const [problemID, setProblemID] = useState<EntityID | null>(
    draft?.problem_id ?? initialProblem?.id ?? null,
  );
  const [version, setVersion] = useState(
    draft?.version ?? initialProblem?.version ?? 0,
  );
  const [lifecycle, setLifecycle] = useState<ProblemLifecycle>(
    draft?.lifecycle ?? initialProblem?.lifecycle ?? "draft",
  );
  const [selection, setSelection] = useState<SavedSelection | null>(
    draft
      ? {
        revisionID: draft.revision_id,
        testsetRevisionID: draft.testset_revision_id,
        version: draft.version,
        artifactStatus: draft.artifact_status,
        artifactDiagnostics: draft.artifact_diagnostics.slice(),
      }
      : null,
  );
  const [busy, setBusy] = useState<BusyAction>(null);
  const [confirmAction, setConfirmAction] =
    useState<ConfirmAction>(null);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [conflicted, setConflicted] = useState(false);
  const [catalogDrifted, setCatalogDrifted] = useState(false);

  const canCreateProblem = can(
    AUTHORIZATION_RESOURCE.PROBLEM,
    AUTHORIZATION_ACTION.CREATE,
  );
  const canUpdateProblem = can(
    AUTHORIZATION_RESOURCE.PROBLEM,
    AUTHORIZATION_ACTION.UPDATE,
  );
  const canReadProblem = can(
    AUTHORIZATION_RESOURCE.PROBLEM,
    AUTHORIZATION_ACTION.READ,
  );
  const isExisting = problemID !== null;
  const canSave = isExisting
    ? canUpdateProblem
    : canCreateProblem && canUpdateProblem;
  const backRoute = canReadProblem
    ? APP_ROUTES.ADMIN_PROBLEMS
    : APP_ROUTES.ADMIN;

  const fallbackDifficulties = useMemo(
    () => initialProblem?.difficulty ? [initialProblem.difficulty] : [],
    [initialProblem],
  );
  const fallbackTags = useMemo(
    () => initialProblem?.tags ?? [],
    [initialProblem],
  );
  const catalogResource = useRetryableResource<ProblemAuthoringCatalog | null>({
    resetKey: `${draft?.problem_id ?? initialProblem?.id ?? "create"}:catalog`,
    enabled: canCreateProblem || canUpdateProblem,
    initialData: null,
    load: problemService.getAuthoringCatalog,
    onSuccess: (catalog) => {
      if (!catalog) return;
      setEditor((current) => {
        const initialCheckerIdentity = current.checkerIdentity ||
          (catalog.checkers[0]
            ? checkerIdentity(
              catalog.checkers[0].key,
              catalog.checkers[0].version,
            )
            : "");
        return {
          ...current,
          cpuTimeMs:
            current.cpuTimeMs === ""
              ? catalog.policy.cpu_time_ms.default
              : current.cpuTimeMs,
          wallTimeMs:
            current.wallTimeMs === ""
              ? Math.max(
                catalog.policy.wall_time_ms.default,
                typeof current.cpuTimeMs === "number"
                  ? current.cpuTimeMs
                  : catalog.policy.cpu_time_ms.default,
              )
              : current.wallTimeMs,
          memoryLimitKb:
            current.memoryLimitKb === ""
              ? catalog.policy.memory_limit_kb.default
              : current.memoryLimitKb,
          outputLimitBytes:
            current.outputLimitBytes === ""
              ? catalog.policy.output_limit_bytes.default
              : current.outputLimitBytes,
          processLimit:
            current.processLimit === ""
              ? catalog.policy.process_limit.default
              : current.processLimit,
          checkerIdentity: initialCheckerIdentity,
          allowedRuntimeKeys:
            current.allowedRuntimeKeys.length > 0
              ? current.allowedRuntimeKeys
              : compatibleRuntimeKeys(catalog, initialCheckerIdentity)
                .slice(0, catalog.policy.maximum_runtimes),
        };
      });
    },
  });
  const difficultyResource = useRetryableResource<DifficultyResponse[]>({
    resetKey: `${draft?.problem_id ?? initialProblem?.id ?? "create"}:difficulty`,
    // Difficulty and tag catalogs are public reference data. Authoring
    // authorization is enforced by the problem aggregate, not by a second
    // capability gate that would make a valid editor unusable.
    enabled: true,
    initialData: fallbackDifficulties,
    load: difficultyService.getAll,
    onSuccess: (items) => {
      if (items.length === 0) return;
      setEditor((current) => ({
        ...current,
        difficultyID: current.difficultyID || items[0].id,
      }));
    },
  });
  const tagResource = useRetryableResource<Tag[]>({
    resetKey: `${draft?.problem_id ?? initialProblem?.id ?? "create"}:tag`,
    enabled: true,
    initialData: fallbackTags,
    load: tagService.getAll,
  });

  const catalog = catalogResource.data;
  const difficulties = useMemo(
    () => mergeReferences(difficultyResource.data, fallbackDifficulties),
    [difficultyResource.data, fallbackDifficulties],
  );
  const tags = useMemo(
    () => mergeReferences(tagResource.data, fallbackTags),
    [tagResource.data, fallbackTags],
  );
  const selectedChecker = catalog?.checkers.find(
    (option) =>
      checkerIdentity(option.key, option.version) ===
      editor.checkerIdentity,
  );
  const compatibleCheckerIDs = useMemo(
    () =>
      catalog
        ? compatibleCheckerIdentities(catalog, editor.allowedRuntimeKeys)
        : new Set<string>(),
    [catalog, editor.allowedRuntimeKeys],
  );
  const selectedCheckerRuntimeKeys = useMemo(
    () =>
      catalog && editor.checkerIdentity
        ? new Set(compatibleRuntimeKeys(catalog, editor.checkerIdentity))
        : new Set(catalog?.runtimes.map((runtime) => runtime.key) ?? []),
    [catalog, editor.checkerIdentity],
  );
  const selectedCheckerIsCompatible =
    selectedChecker !== undefined &&
    compatibleCheckerIDs.has(editor.checkerIdentity);
  const activeRuntimeKeys = new Set(
    catalog?.runtimes.map((runtime) => runtime.key) ?? [],
  );
  const unavailableRuntimeKeys = editor.allowedRuntimeKeys.filter(
    (key) => !activeRuntimeKeys.has(key),
  );
  const activeDifficultyIDs = new Set<string>(
    difficulties.map((difficulty) => difficulty.id),
  );
  const unavailableDifficultyID =
    editor.difficultyID && !activeDifficultyIDs.has(editor.difficultyID)
      ? editor.difficultyID
      : null;
  const activeTagIDs = new Set<string>(tags.map((tag) => tag.id));
  const unavailableTagIDs = editor.tagIDs.filter(
    (tagID) => !activeTagIDs.has(tagID),
  );
  const [unavailableCheckerKey, unavailableCheckerVersion] = splitCheckerIdentity(
    editor.checkerIdentity,
  );
  const unavailableChecker =
    catalog &&
    editor.checkerIdentity &&
    !selectedChecker
      ? {
        identity: editor.checkerIdentity,
        version: unavailableCheckerVersion,
      }
      : null;
  const dirty = authoredSignature(editor) !== baseline.current;
  const disabled =
    busy !== null ||
    conflicted ||
    !canSave ||
    lifecycle === "archived" ||
    catalogResource.status !== "ready" ||
    catalog === null;
  const testCount = editor.groups.reduce(
    (total, group) => total + group.cases.length,
    0,
  );

  useEffect(() => {
    if (!dirty) return;
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty]);

  useEffect(() => {
    if (validationErrors.length > 0) {
      validationSummaryRef.current?.focus({ preventScroll: true });
    }
  }, [validationErrors]);

  function updateEditor(
    update: (current: EditorState) => EditorState,
  ) {
    setEditor(update);
    setValidationErrors([]);
  }

  function nextKey(prefix: string): string {
    keySequence.current += 1;
    return `${prefix}:${keySequence.current}`;
  }

  function toggleTag(tagID: string) {
    if (tagResource.status !== "ready") return;
    updateEditor((current) => ({
      ...current,
      tagIDs: current.tagIDs.includes(tagID)
        ? current.tagIDs.filter((value) => value !== tagID)
        : catalog &&
            current.tagIDs.length >= catalog.policy.maximum_tags
          ? current.tagIDs
        : [...current.tagIDs, tagID],
    }));
  }

  function toggleRuntime(runtimeKey: string) {
    updateEditor((current) => {
      const selected = current.allowedRuntimeKeys.includes(runtimeKey);
      if (
        !selected &&
        catalog &&
        current.checkerIdentity &&
        !compatibleRuntimeKeys(catalog, current.checkerIdentity).includes(
          runtimeKey,
        )
      ) {
        return current;
      }
      return {
        ...current,
        allowedRuntimeKeys: selected
          ? current.allowedRuntimeKeys.filter((value) => value !== runtimeKey)
          : catalog &&
              current.allowedRuntimeKeys.length >=
                catalog.policy.maximum_runtimes
            ? current.allowedRuntimeKeys
          : [...current.allowedRuntimeKeys, runtimeKey],
      };
    });
  }

  function addGroup() {
    if (
      !catalog ||
      editor.groups.length >= catalog.policy.maximum_groups
    ) return;
    updateEditor((current) => ({
      ...current,
      groups: [
        ...current.groups,
        {
          clientKey: nextKey("group"),
          stopOnFailure: false,
          cases: [{
            clientKey: nextKey("case"),
            kind: PROBLEM_TEST_KIND.HIDDEN,
            input: "",
            expectedOutput: "",
          }],
        },
      ],
    }));
  }

  function removeGroup(groupKey: string) {
    updateEditor((current) => ({
      ...current,
      groups: current.groups.filter((group) => group.clientKey !== groupKey),
    }));
  }

  function updateGroup(
    groupKey: string,
    update: (group: EditorGroup) => EditorGroup,
  ) {
    updateEditor((current) => ({
      ...current,
      groups: current.groups.map((group) =>
        group.clientKey === groupKey ? update(group) : group
      ),
    }));
  }

  function addCase(groupKey: string) {
    if (
      !catalog ||
      testCount >= catalog.policy.maximum_testcases
    ) return;
    updateGroup(groupKey, (group) => ({
      ...group,
      cases: [
        ...group.cases,
        {
          clientKey: nextKey("case"),
          kind: PROBLEM_TEST_KIND.HIDDEN,
          input: "",
          expectedOutput: "",
        },
      ],
    }));
  }

  function removeCase(groupKey: string, caseKey: string) {
    updateGroup(groupKey, (group) => ({
      ...group,
      cases: group.cases.filter(
        (testcase) => testcase.clientKey !== caseKey,
      ),
    }));
  }

  function validate(): ProblemAuthoringCatalog | null {
    const errors = editorValidationErrors(editor, catalog);
    setValidationErrors(errors);
    if (errors.length > 0 || !catalog) {
      notify.error(ADMIN_TEXT.PROBLEM_FORM.FORM_HAS_ERRORS);
      return null;
    }
    return catalog;
  }

  function markCatalogDrift() {
    setCatalogDrifted(true);
    setConflicted(false);
    setValidationErrors([ADMIN_TEXT.PROBLEM_FORM.CATALOG_STALE_ERROR]);
    catalogResource.retry();
    notify.error(ADMIN_TEXT.PROBLEM_FORM.CATALOG_STALE_ERROR);
  }

  async function verifyLatestCatalog(
    activeCatalog: ProblemAuthoringCatalog,
  ): Promise<ProblemAuthoringCatalog | null> {
    let latestCatalog: ProblemAuthoringCatalog;
    try {
      latestCatalog = await problemService.getAuthoringCatalog();
    } catch (error) {
      setValidationErrors([ADMIN_TEXT.PROBLEM_FORM.CATALOG_VERIFY_ERROR]);
      notify.error(
        getErrorMessage(
          error,
          ADMIN_TEXT.PROBLEM_FORM.CATALOG_VERIFY_ERROR,
        ),
      );
      return null;
    }

    if (!authoringCatalogsMatch(activeCatalog, latestCatalog)) {
      markCatalogDrift();
      return null;
    }
    setCatalogDrifted(false);
    return latestCatalog;
  }

  async function handleOperationError(
    error: unknown,
    fallback: string,
    activeCatalog?: ProblemAuthoringCatalog,
  ) {
    if (error instanceof ApiError && error.status === 409) {
      if (error.params?.reason === AUTHORING_CATALOG_CHANGED_REASON) {
        markCatalogDrift();
        return;
      }
      if (activeCatalog) {
        try {
          const latestCatalog = await problemService.getAuthoringCatalog();
          if (!authoringCatalogsMatch(activeCatalog, latestCatalog)) {
            markCatalogDrift();
            return;
          }
        } catch {
          // Fall through to optimistic-conflict recovery. Both paths keep the
          // editor state intact and prevent another mutation until refreshed.
        }
      }
      setConflicted(true);
      notify.error(ADMIN_TEXT.PROBLEM_FORM.CONFLICT_ERROR);
      return;
    }
    notify.error(getErrorMessage(error, fallback));
  }

  async function persistDraft(
    activeCatalog: ProblemAuthoringCatalog,
  ): Promise<{
    problemID: EntityID;
    selection: SavedSelection;
    created: boolean;
  }> {
    let activeProblemID = problemID;
    let expectedVersion = version;
    let created = false;

    if (!activeProblemID) {
      const problem = await problemService.create({
        slug: editor.slug.trim(),
        title: editor.title.trim(),
        description: editor.statementMarkdown,
        difficulty_id: editor.difficultyID,
        tag_ids: [...editor.tagIDs].sort(),
      });
      activeProblemID = problem.id;
      expectedVersion = problem.version;
      created = true;
      setProblemID(problem.id);
      setVersion(problem.version);
      setLifecycle(problem.lifecycle);
    }

    let receipt: ProblemDraftReceipt;
    try {
      receipt = await problemService.saveDraft(
        activeProblemID,
        draftInput(editor, expectedVersion, activeCatalog),
      );
    } catch (error) {
      if (created) {
        let archived = false;
        try {
          await problemService.archive(activeProblemID, {
            expected_version: expectedVersion,
            reason: ADMIN_TEXT.PROBLEM_FORM.INITIAL_DRAFT_ROLLBACK_REASON,
          });
          archived = true;
        } catch {
          // Leave the shell selected when compensation fails so the author
          // can retry saving the same draft. Do not hide the original save
          // error behind a best-effort cleanup failure.
        }
        if (archived) {
          // Keep a failed first-save shell out of the active authoring set.
          // Retrying starts a fresh aggregate instead of targeting an
          // archived shell with no immutable draft selection.
          setProblemID(null);
          setVersion(0);
          setLifecycle("draft");
        }
      }
      throw error;
    }
    const savedSelection: SavedSelection = {
      revisionID: receipt.revision_id,
      testsetRevisionID: receipt.testset_revision_id,
      version: receipt.version,
      artifactStatus: receipt.artifact_status,
      artifactDiagnostics: receipt.artifact_diagnostics.slice(),
    };
    setSelection(savedSelection);
    setVersion(receipt.version);
    baseline.current = authoredSignature(editor);
    return {
      problemID: activeProblemID,
      selection: savedSelection,
      created,
    };
  }

  async function saveDraft(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    const validatedCatalog = validate();
    if (!validatedCatalog || disabled) return;

    setBusy("save");
    let activeCatalog: ProblemAuthoringCatalog | null = null;
    try {
      activeCatalog = await verifyLatestCatalog(validatedCatalog);
      if (!activeCatalog) return;
      const saved = await persistDraft(activeCatalog);
      setEditor((current) => ({ ...current, reason: "" }));
      notify.success(ADMIN_TEXT.PROBLEM_FORM.DRAFT_SAVED);
      if (saved.created) {
        router.push(APP_ROUTES.ADMIN_PROBLEM_EDIT(saved.problemID));
      }
    } catch (error) {
      await handleOperationError(
        error,
        ADMIN_TEXT.PROBLEM_FORM.SAVE_DRAFT_ERROR,
        activeCatalog ?? validatedCatalog,
      );
    } finally {
      setBusy(null);
    }
  }

  async function publish() {
    const validatedCatalog = validate();
    if (!validatedCatalog || disabled) return;

    setBusy("publish");
    let activeCatalog: ProblemAuthoringCatalog | null = null;
    try {
      activeCatalog = await verifyLatestCatalog(validatedCatalog);
      if (!activeCatalog) return;
      let activeProblemID = problemID;
      let activeSelection = selection;
      let created = false;
      if (dirty || !activeProblemID || !activeSelection) {
        const saved = await persistDraft(activeCatalog);
        activeProblemID = saved.problemID;
        activeSelection = saved.selection;
        created = saved.created;
      }
      if (activeSelection.artifactStatus !== "ready") {
        setValidationErrors([
          ...activeSelection.artifactDiagnostics.map((diagnostic) =>
            ADMIN_TEXT.PROBLEM_FORM.RENDER_DIAGNOSTIC(diagnostic)
          ),
          ADMIN_TEXT.PROBLEM_FORM.RENDER_REJECTED,
        ]);
        notify.error(ADMIN_TEXT.PROBLEM_FORM.RENDER_REJECTED);
        return;
      }
      const receipt = await problemService.publish(activeProblemID, {
        draft_revision_id: activeSelection.revisionID,
        testset_revision_id: activeSelection.testsetRevisionID,
        expected_version: activeSelection.version,
        catalog_version: activeCatalog.catalog_version,
        reason: editor.reason.trim(),
      });
      setLifecycle(receipt.lifecycle);
      setVersion(receipt.version);
      setSelection({
        ...activeSelection,
        version: receipt.version,
      });
      setEditor((current) => ({ ...current, reason: "" }));
      notify.success(ADMIN_TEXT.PROBLEM_FORM.PUBLISH_SUCCESS);
      if (created) {
        router.push(APP_ROUTES.ADMIN_PROBLEM_EDIT(activeProblemID));
      }
    } catch (error) {
      await handleOperationError(
        error,
        ADMIN_TEXT.PROBLEM_FORM.PUBLISH_ERROR,
        activeCatalog ?? validatedCatalog,
      );
    } finally {
      setBusy(null);
    }
  }

  async function archiveProblem() {
    if (!problemID || disabled) return;
    const reasonMaximum = catalog?.policy.reason_maximum_bytes ?? 0;
    if (
      editor.reason.trim() === "" ||
      byteLength(editor.reason.trim()) > reasonMaximum
    ) {
      setValidationErrors([
        ADMIN_TEXT.PROBLEM_FORM.REASON_INVALID(reasonMaximum),
      ]);
      notify.error(ADMIN_TEXT.PROBLEM_FORM.FORM_HAS_ERRORS);
      setConfirmAction(null);
      return;
    }

    setBusy("archive");
    try {
      const receipt = await problemService.archive(problemID, {
        expected_version: version,
        reason: editor.reason.trim(),
      });
      setLifecycle(receipt.lifecycle);
      setVersion(receipt.version);
      setEditor((current) => ({ ...current, reason: "" }));
      setConfirmAction(null);
      notify.success(ADMIN_TEXT.PROBLEM_FORM.ARCHIVE_SUCCESS);
    } catch (error) {
      await handleOperationError(
        error,
        ADMIN_TEXT.PROBLEM_FORM.ARCHIVE_ERROR,
      );
    } finally {
      setBusy(null);
    }
  }

  async function reloadLatest() {
    if (!problemID || busy !== null) return;

    setBusy("reload");
    try {
      try {
        const latestDraft = await problemService.getAuthoringDraft(problemID);
        baseline.current = authoredSignature(
          createInitialState(latestDraft, null),
        );
        setVersion(latestDraft.version);
        setLifecycle(latestDraft.lifecycle);
        setSelection({
          revisionID: latestDraft.revision_id,
          testsetRevisionID: latestDraft.testset_revision_id,
          version: latestDraft.version,
          artifactStatus: latestDraft.artifact_status,
          artifactDiagnostics: latestDraft.artifact_diagnostics.slice(),
        });
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 404) throw error;
        const latestProblem = await problemService.getAdminById(problemID);
        baseline.current = authoredSignature(
          createInitialState(null, latestProblem),
        );
        setVersion(latestProblem.version);
        setLifecycle(latestProblem.lifecycle);
        setSelection(null);
      }
      setConflicted(false);
      setValidationErrors([]);
      notify.success(ADMIN_TEXT.PROBLEM_FORM.RELOAD_LATEST_SUCCESS);
    } catch (error) {
      notify.error(
        getErrorMessage(
          error,
          ADMIN_TEXT.PROBLEM_FORM.RELOAD_LATEST_ERROR,
        ),
      );
    } finally {
      setBusy(null);
    }
  }

  function requestLeave() {
    if (dirty) {
      setConfirmAction("leave");
      return;
    }
    router.push(backRoute);
  }

  const operationLabel =
    busy === "reload"
      ? ADMIN_TEXT.PROBLEM_FORM.RELOADING_LATEST
      : busy === "publish"
        ? ADMIN_TEXT.PROBLEM_FORM.PUBLISHING
        : busy === "archive"
          ? ADMIN_TEXT.PROBLEM_FORM.ARCHIVING
          : ADMIN_TEXT.PROBLEM_FORM.SAVING;

  return (
    <form
      className="mx-auto w-full max-w-[96rem] space-y-6"
      onSubmit={(event) => void saveDraft(event)}
      noValidate
      aria-busy={busy !== null}
    >
      <header className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-11 shrink-0"
            onClick={requestLeave}
            aria-label={ADMIN_TEXT.PROBLEM_FORM.BACK_LABEL}
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
          </Button>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
                {isExisting
                  ? ADMIN_TEXT.PROBLEM_FORM.EDIT_TITLE
                  : ADMIN_TEXT.PROBLEM_FORM.CREATE_TITLE}
              </h1>
              <Badge variant={lifecycleVariant(lifecycle)}>
                {lifecycleLabel(lifecycle)}
              </Badge>
            </div>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.PROBLEM_FORM.IMMUTABLE_DESCRIPTION}
            </p>
          </div>
        </div>
        {catalog ? (
          <div className="flex items-center gap-2 self-start rounded-full border border-border bg-card px-3 py-1.5 text-xs text-muted-foreground shadow-sm">
            <CheckCircle2
              className="size-3.5 text-success"
              aria-hidden="true"
            />
            {ADMIN_TEXT.PROBLEM_FORM.RELEASE(catalog.release_id)}
          </div>
        ) : null}
      </header>

      {!canSave ? (
        <div
          className="flex items-start gap-3 rounded-xl border border-warning/30 bg-warning/10 p-4 text-sm text-foreground"
          role="note"
        >
          <ShieldAlert className="mt-0.5 size-5 shrink-0" aria-hidden="true" />
          <p>{ADMIN_TEXT.PROBLEM_FORM.AUTHORING_RESTRICTED}</p>
        </div>
      ) : null}

      {lifecycle === "archived" ? (
        <div
          className="flex items-start gap-3 rounded-xl border border-border-strong/40 bg-muted/60 p-4 text-sm text-muted-foreground"
          role="status"
        >
          <Archive className="mt-0.5 size-5 shrink-0" aria-hidden="true" />
          <p>{ADMIN_TEXT.PROBLEM_FORM.ARCHIVED_READ_ONLY}</p>
        </div>
      ) : null}

      {conflicted ? (
        <div
          className="flex flex-col gap-3 rounded-xl border border-destructive/30 bg-destructive/10 p-4 sm:flex-row sm:items-center sm:justify-between"
          role="alert"
        >
          <div className="flex items-start gap-3">
            <TriangleAlert
              className="mt-0.5 size-5 shrink-0 text-destructive"
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold">
                {ADMIN_TEXT.PROBLEM_FORM.CONFLICT_TITLE}
              </p>
              <p className="mt-1 text-sm text-muted-foreground">
                {ADMIN_TEXT.PROBLEM_FORM.CONFLICT_DESCRIPTION}
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            className="h-11 shrink-0"
            disabled={busy !== null}
            onClick={() => void reloadLatest()}
          >
            {busy === "reload" ? (
              <Loader2
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : null}
            {busy === "reload"
              ? ADMIN_TEXT.PROBLEM_FORM.RELOADING_LATEST
              : ADMIN_TEXT.PROBLEM_FORM.RELOAD_LATEST}
          </Button>
        </div>
      ) : null}

      {catalogDrifted ? (
        <div
          className="flex flex-col gap-3 rounded-xl border border-warning/30 bg-warning/10 p-4 sm:flex-row sm:items-center sm:justify-between"
          role="alert"
        >
          <div className="flex items-start gap-3">
            <TriangleAlert
              className="mt-0.5 size-5 shrink-0 text-warning"
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold">
                {ADMIN_TEXT.PROBLEM_FORM.CATALOG_STALE_TITLE}
              </p>
              <p className="mt-1 text-sm text-muted-foreground">
                {ADMIN_TEXT.PROBLEM_FORM.CATALOG_STALE_DESCRIPTION}
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            className="h-11 shrink-0"
            disabled={busy !== null || catalogResource.status === "loading"}
            onClick={catalogResource.retry}
          >
            {ADMIN_TEXT.PROBLEM_FORM.RELOAD_CATALOG}
          </Button>
        </div>
      ) : null}

      {catalogResource.status === "loading" ? (
        <DataLoadingFeedback
          label={ADMIN_TEXT.PROBLEM_FORM.CATALOG_LOADING}
          className="min-h-24"
        />
      ) : catalogResource.status === "error" ? (
        <DataLoadFeedback
          title={ADMIN_TEXT.PROBLEM_FORM.CATALOG_ERROR_TITLE}
          description={ADMIN_TEXT.PROBLEM_FORM.CATALOG_ERROR_DESCRIPTION}
          retryLabel={TEXT.COMMON.RETRY}
          onRetry={catalogResource.retry}
        />
      ) : null}

      {validationErrors.length > 0 ? (
        <section
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-4"
          aria-labelledby="problem-validation-title"
          role="alert"
          tabIndex={-1}
          ref={validationSummaryRef}
        >
          <h2
            id="problem-validation-title"
            className="flex items-center gap-2 font-semibold text-destructive"
          >
            <TriangleAlert className="size-4" aria-hidden="true" />
            {ADMIN_TEXT.PROBLEM_FORM.VALIDATION_TITLE}
          </h2>
          <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-muted-foreground">
            {validationErrors.map((error) => (
              <li key={error}>{error}</li>
            ))}
          </ul>
        </section>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="min-w-0 space-y-6">
          <Card>
            <CardHeader className="border-b">
              <CardTitle className="flex items-center gap-2">
                <FileCode2 className="size-4 text-primary" aria-hidden="true" />
                {ADMIN_TEXT.PROBLEM_FORM.CONTENT_TITLE}
              </CardTitle>
              <CardDescription>
                {ADMIN_TEXT.PROBLEM_FORM.CONTENT_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(14rem,0.45fr)]">
                <div className="space-y-2">
                  <Label htmlFor="problem-title">
                    {ADMIN_TEXT.PROBLEM_FORM.TITLE}
                  </Label>
                  <Input
                    id="problem-title"
                    value={editor.title}
                    disabled={disabled}
                    placeholder={ADMIN_TEXT.PROBLEM_FORM.TITLE_PLACEHOLDER}
                    onChange={(event) =>
                      updateEditor((current) => ({
                        ...current,
                        title: event.target.value,
                      }))
                    }
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="problem-slug">
                    {ADMIN_TEXT.PROBLEM_FORM.SLUG}
                  </Label>
                  <Input
                    id="problem-slug"
                    value={editor.slug}
                    disabled={disabled || isExisting}
                    placeholder={ADMIN_TEXT.PROBLEM_FORM.SLUG_PLACEHOLDER}
                    autoCapitalize="none"
                    spellCheck={false}
                    onChange={(event) =>
                      updateEditor((current) => ({
                        ...current,
                        slug: event.target.value.toLowerCase(),
                      }))
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    {isExisting
                      ? ADMIN_TEXT.PROBLEM_FORM.SLUG_IMMUTABLE
                      : ADMIN_TEXT.PROBLEM_FORM.SLUG_HINT}
                  </p>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="problem-statement">
                  {ADMIN_TEXT.PROBLEM_FORM.STATEMENT}
                </Label>
                <div
                  data-color-mode={
                    resolvedTheme === "dark" ? "dark" : "light"
                  }
                  className="overflow-hidden rounded-xl border border-border"
                >
                  <MDEditor
                    value={editor.statementMarkdown}
                    onChange={(value) =>
                      updateEditor((current) => ({
                        ...current,
                        statementMarkdown: value ?? "",
                      }))
                    }
                    height={560}
                    preview="live"
                    previewOptions={markdownPreviewOptions}
                    textareaProps={{
                      id: "problem-statement",
                      disabled,
                      "aria-label": ADMIN_TEXT.PROBLEM_FORM.STATEMENT,
                    }}
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="border-b">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <CardTitle className="flex items-center gap-2">
                    <FlaskConical
                      className="size-4 text-primary"
                      aria-hidden="true"
                    />
                    {ADMIN_TEXT.PROBLEM_FORM.TESTSET_TITLE}
                  </CardTitle>
                  <CardDescription className="mt-1">
                    {ADMIN_TEXT.PROBLEM_FORM.TESTSET_DESCRIPTION}
                  </CardDescription>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline">
                    {ADMIN_TEXT.PROBLEM_FORM.GROUP_COUNT(editor.groups.length)}
                  </Badge>
                  <Badge variant="outline">
                    {ADMIN_TEXT.PROBLEM_FORM.CASE_COUNT(testCount)}
                  </Badge>
                  <Button
                    type="button"
                    variant="outline"
                    className="h-11"
                    disabled={
                      disabled ||
                      !catalog ||
                      editor.groups.length >= catalog.policy.maximum_groups
                    }
                    onClick={addGroup}
                  >
                    <Plus className="size-4" aria-hidden="true" />
                    {ADMIN_TEXT.PROBLEM_FORM.ADD_GROUP}
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {editor.groups.length === 0 ? (
                <div className="rounded-xl border border-dashed border-border p-8 text-center">
                  <p className="font-medium">
                    {ADMIN_TEXT.PROBLEM_FORM.NO_GROUP_TITLE}
                  </p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {ADMIN_TEXT.PROBLEM_FORM.NO_GROUP_DESCRIPTION}
                  </p>
                </div>
              ) : null}

              {editor.groups.map((group, groupIndex) => (
                <article
                  key={group.clientKey}
                  className="overflow-hidden rounded-xl border border-border bg-muted/10"
                >
                  <div className="flex flex-col gap-3 border-b border-border bg-muted/30 p-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <h3 className="font-semibold">
                        {ADMIN_TEXT.PROBLEM_FORM.GROUP_NUMBER(groupIndex + 1)}
                      </h3>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {ADMIN_TEXT.PROBLEM_FORM.GROUP_CASES(group.cases.length)}
                      </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-3">
                      <Label
                        htmlFor={`stop-on-failure-${group.clientKey}`}
                        className="flex min-h-11 cursor-pointer items-center gap-2 rounded-lg px-2 text-xs text-muted-foreground"
                      >
                        <Switch
                          id={`stop-on-failure-${group.clientKey}`}
                          checked={group.stopOnFailure}
                          disabled={disabled}
                          onCheckedChange={(checked) =>
                            updateGroup(group.clientKey, (current) => ({
                              ...current,
                              stopOnFailure: checked,
                            }))
                          }
                        />
                        {ADMIN_TEXT.PROBLEM_FORM.STOP_ON_FAILURE}
                      </Label>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="min-h-11"
                        disabled={
                          disabled ||
                          !catalog ||
                          testCount >= catalog.policy.maximum_testcases
                        }
                        onClick={() => addCase(group.clientKey)}
                      >
                        <Plus className="size-4" aria-hidden="true" />
                        {ADMIN_TEXT.PROBLEM_FORM.ADD_CASE}
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-11 text-destructive hover:bg-destructive/10 hover:text-destructive"
                        disabled={disabled}
                        onClick={() => removeGroup(group.clientKey)}
                        aria-label={
                          ADMIN_TEXT.PROBLEM_FORM.REMOVE_GROUP(groupIndex + 1)
                        }
                      >
                        <Trash2 className="size-4" aria-hidden="true" />
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-4 p-4">
                    {group.cases.map((testcase, caseIndex) => (
                      <section
                        key={testcase.clientKey}
                        className="rounded-xl border border-border/70 bg-card p-4 shadow-sm"
                      >
                        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                          <div className="flex items-center gap-2">
                            <span className="grid size-7 place-items-center rounded-full bg-primary/10 text-xs font-bold text-primary">
                              {caseIndex + 1}
                            </span>
                            <select
                              value={testcase.kind}
                              disabled={disabled}
                              className="h-11 rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                              aria-label={
                                ADMIN_TEXT.PROBLEM_FORM.CASE_KIND(
                                  groupIndex + 1,
                                  caseIndex + 1,
                                )
                              }
                              onChange={(event) =>
                                updateGroup(group.clientKey, (current) => ({
                                  ...current,
                                  cases: current.cases.map((candidate) =>
                                    candidate.clientKey === testcase.clientKey
                                      ? {
                                        ...candidate,
                                        kind: event.target.value as
                                          | "sample"
                                          | "hidden",
                                      }
                                      : candidate
                                  ),
                                }))
                              }
                            >
                              <option value={PROBLEM_TEST_KIND.SAMPLE}>
                                {ADMIN_TEXT.PROBLEM_FORM.SAMPLE_CASE}
                              </option>
                              <option value={PROBLEM_TEST_KIND.HIDDEN}>
                                {ADMIN_TEXT.PROBLEM_FORM.HIDDEN_CASE}
                              </option>
                            </select>
                          </div>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-11 text-destructive hover:bg-destructive/10 hover:text-destructive"
                            disabled={disabled}
                            onClick={() =>
                              removeCase(
                                group.clientKey,
                                testcase.clientKey,
                              )
                            }
                            aria-label={
                              ADMIN_TEXT.PROBLEM_FORM.REMOVE_CASE(
                                groupIndex + 1,
                                caseIndex + 1,
                              )
                            }
                          >
                            <Trash2 className="size-4" aria-hidden="true" />
                          </Button>
                        </div>
                        <div className="grid gap-4 lg:grid-cols-2">
                          <div className="space-y-2">
                            <Label
                              htmlFor={`case-input-${testcase.clientKey}`}
                            >
                              {ADMIN_TEXT.PROBLEM_FORM.INPUT}
                            </Label>
                            <Textarea
                              id={`case-input-${testcase.clientKey}`}
                              value={testcase.input}
                              disabled={disabled}
                              className="min-h-36 resize-y font-mono text-sm"
                              placeholder={
                                ADMIN_TEXT.PROBLEM_FORM.INPUT_PLACEHOLDER
                              }
                              onChange={(event) =>
                                updateGroup(group.clientKey, (current) => ({
                                  ...current,
                                  cases: current.cases.map((candidate) =>
                                    candidate.clientKey === testcase.clientKey
                                      ? {
                                        ...candidate,
                                        input: event.target.value,
                                      }
                                      : candidate
                                  ),
                                }))
                              }
                            />
                          </div>
                          <div className="space-y-2">
                            <Label
                              htmlFor={`case-output-${testcase.clientKey}`}
                            >
                              {ADMIN_TEXT.PROBLEM_FORM.EXPECTED_OUTPUT}
                            </Label>
                            <Textarea
                              id={`case-output-${testcase.clientKey}`}
                              value={testcase.expectedOutput}
                              disabled={disabled}
                              className="min-h-36 resize-y font-mono text-sm"
                              placeholder={
                                ADMIN_TEXT.PROBLEM_FORM
                                  .EXPECTED_OUTPUT_PLACEHOLDER
                              }
                              onChange={(event) =>
                                updateGroup(group.clientKey, (current) => ({
                                  ...current,
                                  cases: current.cases.map((candidate) =>
                                    candidate.clientKey === testcase.clientKey
                                      ? {
                                        ...candidate,
                                        expectedOutput: event.target.value,
                                      }
                                      : candidate
                                  ),
                                }))
                              }
                            />
                          </div>
                        </div>
                      </section>
                    ))}
                  </div>
                </article>
              ))}
            </CardContent>
          </Card>
        </div>

        <aside className="space-y-6 xl:sticky xl:top-24 xl:self-start">
          <Card>
            <CardHeader className="border-b">
              <CardTitle className="flex items-center gap-2">
                <Gauge className="size-4 text-primary" aria-hidden="true" />
                {ADMIN_TEXT.PROBLEM_FORM.JUDGE_POLICY_TITLE}
              </CardTitle>
              <CardDescription>
                {ADMIN_TEXT.PROBLEM_FORM.JUDGE_POLICY_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="space-y-2">
                <Label htmlFor="problem-difficulty">
                  {ADMIN_TEXT.PROBLEM_FORM.DIFFICULTY}
                </Label>
                <select
                  id="problem-difficulty"
                  value={editor.difficultyID}
                  disabled={
                    disabled ||
                    difficultyResource.status !== "ready"
                  }
                  className="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-60"
                  onChange={(event) =>
                    updateEditor((current) => ({
                      ...current,
                      difficultyID: event.target.value,
                    }))
                  }
                >
                  <option value="">
                    {ADMIN_TEXT.PROBLEM_FORM.SELECT_DIFFICULTY}
                  </option>
                  {unavailableDifficultyID ? (
                    <option value={unavailableDifficultyID} disabled>
                      {ADMIN_TEXT.PROBLEM_FORM.DIFFICULTY_UNAVAILABLE(
                        unavailableDifficultyID,
                      )}
                    </option>
                  ) : null}
                  {difficulties.map((difficulty) => (
                    <option key={difficulty.id} value={difficulty.id}>
                      {difficulty.name}
                    </option>
                  ))}
                </select>
                {difficultyResource.status === "error" ? (
                  <DataLoadFeedback
                    compact
                    title={
                      ADMIN_TEXT.PROBLEM_FORM
                        .DIFFICULTIES_LOAD_ERROR_TITLE
                    }
                    description={
                      ADMIN_TEXT.PROBLEM_FORM
                        .DIFFICULTIES_LOAD_ERROR_DESCRIPTION
                    }
                    retryLabel={TEXT.COMMON.RETRY}
                    onRetry={difficultyResource.retry}
                  />
                ) : null}
              </div>

              <div className="grid grid-cols-2 gap-3">
                {([
                  ["cpuTimeMs", "cpu_time_ms", ADMIN_TEXT.PROBLEM_FORM.CPU_LIMIT],
                  ["wallTimeMs", "wall_time_ms", ADMIN_TEXT.PROBLEM_FORM.WALL_LIMIT],
                  ["memoryLimitKb", "memory_limit_kb", ADMIN_TEXT.PROBLEM_FORM.MEMORY_LIMIT],
                  ["outputLimitBytes", "output_limit_bytes", ADMIN_TEXT.PROBLEM_FORM.OUTPUT_LIMIT],
                  ["processLimit", "process_limit", ADMIN_TEXT.PROBLEM_FORM.PROCESS_LIMIT],
                ] as const).map(([field, policyKey, label]) => {
                  const numericPolicy = catalog?.policy[policyKey];
                  return (
                    <div
                      key={field}
                      className={cn(
                        "space-y-2",
                        field === "processLimit" && "col-span-2",
                      )}
                    >
                      <Label htmlFor={`problem-${field}`}>{label}</Label>
                      <Input
                        id={`problem-${field}`}
                        type="number"
                        inputMode="numeric"
                        value={editor[field]}
                        min={numericPolicy?.minimum}
                        max={numericPolicy?.maximum}
                        step={1}
                        disabled={disabled}
                        onChange={(event) =>
                          updateEditor((current) => ({
                            ...current,
                            [field]:
                              event.target.value === ""
                                ? ""
                                : event.target.valueAsNumber,
                          }))
                        }
                      />
                    </div>
                  );
                })}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="border-b">
              <CardTitle className="flex items-center gap-2">
                <Layers3 className="size-4 text-primary" aria-hidden="true" />
                {ADMIN_TEXT.PROBLEM_FORM.RUNTIME_CHECKER_TITLE}
              </CardTitle>
              <CardDescription>
                {ADMIN_TEXT.PROBLEM_FORM.RUNTIME_CHECKER_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <fieldset className="space-y-2">
                <legend className="text-sm font-medium">
                  {ADMIN_TEXT.PROBLEM_FORM.ALLOWED_RUNTIMES}
                </legend>
                <div className="space-y-2">
                  {catalog?.runtimes.map((runtime) => {
                    const selected = editor.allowedRuntimeKeys.includes(
                      runtime.key,
                    );
                    const compatible = selectedCheckerRuntimeKeys.has(
                      runtime.key,
                    );
                    return (
                      <label
                        key={runtime.key}
                        className={cn(
                          "flex min-h-11 items-start gap-3 rounded-lg border border-border px-3 py-2.5 transition-colors has-[:checked]:border-primary/40 has-[:checked]:bg-primary/5",
                          compatible || selected
                            ? "cursor-pointer hover:bg-muted/40"
                            : "cursor-not-allowed opacity-60",
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={selected}
                          disabled={
                            disabled ||
                            (!selected &&
                              (!compatible ||
                                editor.allowedRuntimeKeys.length >=
                                  (catalog?.policy.maximum_runtimes ?? 0)))
                          }
                          className="mt-1 size-4 accent-primary"
                          onChange={() => toggleRuntime(runtime.key)}
                        />
                        <span className="min-w-0">
                          <span className="block text-sm font-medium">
                            {runtime.display_name}
                          </span>
                          <span className="block truncate text-xs text-muted-foreground">
                            {compatible || selected
                              ? ADMIN_TEXT.PROBLEM_FORM.RUNTIME_VERSION(
                                runtime.profile_version,
                              )
                              : ADMIN_TEXT.PROBLEM_FORM.RUNTIME_INCOMPATIBLE}
                          </span>
                        </span>
                      </label>
                    );
                  })}
                  {unavailableRuntimeKeys.map((runtimeKey) => (
                    <button
                      key={runtimeKey}
                      type="button"
                      className="flex min-h-11 w-full items-center justify-between rounded-lg border border-destructive/30 bg-destructive/5 px-3 text-left text-sm text-destructive"
                      disabled={disabled}
                      onClick={() => toggleRuntime(runtimeKey)}
                    >
                      <span>
                        {ADMIN_TEXT.PROBLEM_FORM.RUNTIME_UNAVAILABLE(runtimeKey)}
                      </span>
                      <Trash2 className="size-4" aria-hidden="true" />
                    </button>
                  ))}
                </div>
              </fieldset>

              <div className="space-y-2">
                <Label htmlFor="problem-checker">
                  {ADMIN_TEXT.PROBLEM_FORM.CHECKER}
                </Label>
                <select
                  id="problem-checker"
                  value={editor.checkerIdentity}
                  disabled={disabled}
                  className="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-60"
                  onChange={(event) =>
                    updateEditor((current) => ({
                      ...current,
                      checkerIdentity: event.target.value,
                      absoluteTolerance: "",
                      relativeTolerance: "",
                    }))
                  }
                >
                  <option value="">
                    {ADMIN_TEXT.PROBLEM_FORM.SELECT_CHECKER}
                  </option>
                  {unavailableChecker ? (
                    <option value={unavailableChecker.identity} disabled>
                      {ADMIN_TEXT.PROBLEM_FORM.CHECKER_UNAVAILABLE(
                        unavailableCheckerKey,
                        unavailableChecker.version,
                      )}
                    </option>
                  ) : null}
                  {catalog?.checkers.map((checker) => {
                    const identity = checkerIdentity(
                      checker.key,
                      checker.version,
                    );
                    const compatible = compatibleCheckerIDs.has(identity);
                    return (
                      <option
                        key={identity}
                        value={identity}
                        disabled={!compatible}
                      >
                        {compatible
                          ? ADMIN_TEXT.PROBLEM_FORM.CHECKER_OPTION(
                            checker.key,
                            checker.version,
                          )
                          : ADMIN_TEXT.PROBLEM_FORM.CHECKER_INCOMPATIBLE(
                            checker.key,
                            checker.version,
                          )}
                      </option>
                    );
                  })}
                </select>
                <p
                  className={cn(
                    "text-xs",
                    selectedChecker && !selectedCheckerIsCompatible
                      ? "text-destructive"
                      : "text-muted-foreground",
                  )}
                  role={
                    selectedChecker && !selectedCheckerIsCompatible
                      ? "alert"
                      : undefined
                  }
                >
                  {selectedChecker && !selectedCheckerIsCompatible
                    ? ADMIN_TEXT.PROBLEM_FORM.CAPABILITY_PAIR_UNAVAILABLE
                    : ADMIN_TEXT.PROBLEM_FORM.CHECKER_COMPATIBILITY_HINT}
                </p>
              </div>

              {selectedChecker?.config_kind ===
              PROBLEM_CHECKER_CONFIG_KIND.FLOAT_TOLERANCE ? (
                <div className="grid grid-cols-2 gap-3 rounded-lg border border-border bg-muted/20 p-3">
                  <div className="space-y-2">
                    <Label htmlFor="absolute-tolerance">
                      {ADMIN_TEXT.PROBLEM_FORM.ABSOLUTE_TOLERANCE}
                    </Label>
                    <Input
                      id="absolute-tolerance"
                      value={editor.absoluteTolerance}
                      inputMode="decimal"
                      disabled={disabled}
                      placeholder={ADMIN_TEXT.PROBLEM_FORM.TOLERANCE_PLACEHOLDER}
                      onChange={(event) =>
                        updateEditor((current) => ({
                          ...current,
                          absoluteTolerance: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="relative-tolerance">
                      {ADMIN_TEXT.PROBLEM_FORM.RELATIVE_TOLERANCE}
                    </Label>
                    <Input
                      id="relative-tolerance"
                      value={editor.relativeTolerance}
                      inputMode="decimal"
                      disabled={disabled}
                      placeholder={ADMIN_TEXT.PROBLEM_FORM.TOLERANCE_PLACEHOLDER}
                      onChange={(event) =>
                        updateEditor((current) => ({
                          ...current,
                          relativeTolerance: event.target.value,
                        }))
                      }
                    />
                  </div>
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="border-b">
              <CardTitle>{ADMIN_TEXT.PROBLEM_FORM.TAXONOMY_TITLE}</CardTitle>
              <CardDescription>
                {ADMIN_TEXT.PROBLEM_FORM.TAXONOMY_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {tagResource.status === "loading" ? (
                <DataLoadingFeedback
                  label={ADMIN_TEXT.PROBLEM_FORM.TAGS_LOADING}
                  className="min-h-20"
                />
              ) : (
                <div className="flex flex-wrap gap-2">
                  {tags.map((tag) => {
                    const active = editor.tagIDs.includes(tag.id);
                    return (
                      <button
                        key={tag.id}
                        type="button"
                        aria-pressed={active}
                        disabled={
                          disabled ||
                          tagResource.status !== "ready" ||
                          (!active &&
                            editor.tagIDs.length >=
                              (catalog?.policy.maximum_tags ?? 0))
                        }
                        className={cn(
                          "min-h-11 rounded-full border px-3 text-xs font-semibold outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-60",
                          active
                            ? "border-primary/40 bg-primary/10 text-primary"
                            : "border-border text-muted-foreground hover:bg-muted",
                        )}
                        onClick={() => toggleTag(tag.id)}
                      >
                        {tag.name}
                      </button>
                    );
                  })}
                  {unavailableTagIDs.map((tagID) => (
                    <button
                      key={tagID}
                      type="button"
                      aria-pressed="true"
                      disabled={disabled}
                      className="inline-flex min-h-11 items-center gap-2 rounded-full border border-destructive/30 bg-destructive/5 px-3 text-xs font-semibold text-destructive outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-60"
                      onClick={() => toggleTag(tagID)}
                    >
                      {ADMIN_TEXT.PROBLEM_FORM.TAG_UNAVAILABLE(tagID)}
                      <Trash2 className="size-3.5" aria-hidden="true" />
                    </button>
                  ))}
                </div>
              )}
              {tagResource.status === "error" ? (
                <DataLoadFeedback
                  compact
                  title={ADMIN_TEXT.PROBLEM_FORM.TAGS_LOAD_ERROR_TITLE}
                  description={
                    ADMIN_TEXT.PROBLEM_FORM.TAGS_LOAD_ERROR_DESCRIPTION
                  }
                  retryLabel={TEXT.COMMON.RETRY}
                  onRetry={tagResource.retry}
                />
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="border-b">
              <CardTitle id="problem-audit-reason-title">
                {ADMIN_TEXT.PROBLEM_FORM.AUDIT_REASON}
              </CardTitle>
              <CardDescription>
                {ADMIN_TEXT.PROBLEM_FORM.AUDIT_REASON_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Textarea
                id="problem-audit-reason"
                aria-labelledby="problem-audit-reason-title"
                value={editor.reason}
                disabled={disabled}
                placeholder={ADMIN_TEXT.PROBLEM_FORM.AUDIT_REASON_PLACEHOLDER}
                className="min-h-28"
                onChange={(event) =>
                  updateEditor((current) => ({
                    ...current,
                    reason: event.target.value,
                  }))
                }
              />
            </CardContent>
          </Card>
        </aside>
      </div>

      <footer className="sticky bottom-0 z-20 -mx-3 flex flex-col gap-3 border-t border-border bg-background/92 px-3 py-4 shadow-xl backdrop-blur-xl sm:flex-row sm:items-center">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">
            {dirty
              ? ADMIN_TEXT.PROBLEM_FORM.UNSAVED_CHANGES
              : ADMIN_TEXT.PROBLEM_FORM.ALL_CHANGES_SAVED}
          </p>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            {problemID
              ? ADMIN_TEXT.PROBLEM_FORM.VERSION(version)
              : ADMIN_TEXT.PROBLEM_FORM.NOT_CREATED}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-2 sm:flex">
          {lifecycle === "published" ? (
            <Button
              type="button"
              variant="outline"
              className="h-11 text-destructive hover:bg-destructive/10 hover:text-destructive"
              disabled={disabled || dirty}
              title={
                dirty
                  ? ADMIN_TEXT.PROBLEM_FORM.ARCHIVE_DIRTY_HINT
                  : undefined
              }
              onClick={() => setConfirmAction("archive")}
            >
              <Archive className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.PROBLEM_FORM.ARCHIVE}
            </Button>
          ) : null}
          <Button
            type="submit"
            variant="outline"
            className="h-11"
            disabled={disabled}
          >
            {busy === "save" ? (
              <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
            ) : (
              <Save className="size-4" aria-hidden="true" />
            )}
            {busy === "save"
              ? ADMIN_TEXT.PROBLEM_FORM.SAVING
              : ADMIN_TEXT.PROBLEM_FORM.SAVE_DRAFT}
          </Button>
          <Button
            type="button"
            className="col-span-2 h-11 sm:col-span-1"
            disabled={disabled}
            onClick={() => void publish()}
          >
            {busy === "publish" ? (
              <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
            ) : (
              <Rocket className="size-4" aria-hidden="true" />
            )}
            {busy === "publish"
              ? ADMIN_TEXT.PROBLEM_FORM.PUBLISHING
              : ADMIN_TEXT.PROBLEM_FORM.PUBLISH}
          </Button>
        </div>
        {busy ? (
          <span className="sr-only" aria-live="polite">
            {operationLabel}
          </span>
        ) : null}
      </footer>

      <ConfirmDialog
        open={confirmAction === "leave"}
        onOpenChange={(open) => {
          if (!open) setConfirmAction(null);
        }}
        onConfirm={() => router.push(backRoute)}
        confirmVariant="destructive"
        title={ADMIN_TEXT.PROBLEM_FORM.LEAVE_TITLE}
        description={ADMIN_TEXT.PROBLEM_FORM.LEAVE_DESCRIPTION}
        confirmLabel={ADMIN_TEXT.PROBLEM_FORM.LEAVE_CONFIRM}
      />
      <ConfirmDialog
        open={confirmAction === "archive"}
        onOpenChange={(open) => {
          if (!open && busy !== "archive") setConfirmAction(null);
        }}
        onConfirm={() => void archiveProblem()}
        confirmVariant="destructive"
        loading={busy === "archive"}
        title={ADMIN_TEXT.PROBLEM_FORM.ARCHIVE_TITLE}
        description={ADMIN_TEXT.PROBLEM_FORM.ARCHIVE_DESCRIPTION}
        confirmLabel={ADMIN_TEXT.PROBLEM_FORM.ARCHIVE_CONFIRM}
      />
    </form>
  );
}
