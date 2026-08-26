"use client";

import { useMemo, useState } from "react";
import {
  AlertCircle,
  CalendarClock,
  ChevronDown,
  ChevronUp,
  Eye,
  Loader2,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  Trophy,
  UsersRound,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import {
  CONTEST_ADMIN_CATALOG,
  CONTEST_REASON_LIMITS,
} from "@/constants/contest";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { isContestReasonValid } from "@/lib/contest/publish";
import { problemService } from "@/services/problem.service";
import type {
  Contest,
  ContestDraftInput,
  ContestRegistrationMode,
} from "@/types/contest";
import type { Problem } from "@/types/problem";

export type ContestEditorInput = ContestDraftInput & {
  expected_version?: number;
};

interface ContestDialogProps {
  open: boolean;
  editing: Contest | null;
  onSave: (input: ContestEditorInput) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

interface SelectedProblem {
  id: string;
  slug: string;
  title: string;
  alias: string;
  visibleBeforeStart: boolean;
}

function toDatetimeLocal(iso?: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function defaultSchedule(): {
  start: string;
  end: string;
  opens: string;
  closes: string;
} {
  const now = new Date();
  const start = new Date(now);
  start.setDate(start.getDate() + 1);
  start.setMinutes(0, 0, 0);
  const end = new Date(start.getTime() + 2 * 60 * 60 * 1000);
  const closes = new Date(start.getTime() - 15 * 60 * 1000);
  return {
    start: toDatetimeLocal(start.toISOString()),
    end: toDatetimeLocal(end.toISOString()),
    opens: toDatetimeLocal(now.toISOString()),
    closes: toDatetimeLocal(closes.toISOString()),
  };
}

function generatedAlias(index: number): string {
  let cursor = index + 1;
  let alias = "";
  while (cursor > 0) {
    cursor -= 1;
    alias = String.fromCharCode(65 + (cursor % 26)) + alias;
    cursor = Math.floor(cursor / 26);
  }
  return alias;
}

function initialSelectedProblems(editing: Contest | null): SelectedProblem[] {
  return [...(editing?.draft_problems ?? [])]
    .sort((left, right) => left.display_order - right.display_order)
    .map((problem) => ({
      id: problem.problem_id,
      slug: problem.problem_slug,
      title: problem.problem_title,
      alias: problem.alias,
      visibleBeforeStart: problem.visible_before_start,
    }));
}

function problemOption(problem: Problem): SelectedProblem {
  return {
    id: problem.id,
    slug: problem.slug,
    title: problem.title,
    alias: "",
    visibleBeforeStart: true,
  };
}

export function ContestDialog({
  open,
  editing,
  onSave,
  onClose,
  isSaving,
}: ContestDialogProps) {
  const defaults = useMemo(() => defaultSchedule(), []);
  const [title, setTitle] = useState(editing?.title ?? "");
  const [description, setDescription] = useState(
    editing?.description ?? "",
  );
  const [reason, setReason] = useState("");
  const [startTime, setStartTime] = useState(
    toDatetimeLocal(editing?.start_time) || defaults.start,
  );
  const [endTime, setEndTime] = useState(
    toDatetimeLocal(editing?.end_time) || defaults.end,
  );
  const [registrationMode, setRegistrationMode] =
    useState<ContestRegistrationMode>(
      editing?.registration_mode ?? "registered",
    );
  const [registrationOpensAt, setRegistrationOpensAt] = useState(
    toDatetimeLocal(editing?.registration_opens_at) || defaults.opens,
  );
  const [registrationClosesAt, setRegistrationClosesAt] = useState(
    toDatetimeLocal(editing?.registration_closes_at) || defaults.closes,
  );
  const [maxParticipants, setMaxParticipants] = useState(
    editing?.max_participants?.toString() ?? "",
  );
  const [freezeStandingsAt, setFreezeStandingsAt] = useState(
    toDatetimeLocal(editing?.freeze_standings_at),
  );
  const [rated, setRated] = useState(editing?.rated ?? true);
  const [selectedProblems, setSelectedProblems] = useState(
    () => initialSelectedProblems(editing),
  );
  const [catalogSearch, setCatalogSearch] = useState("");
  const debouncedCatalogSearch = useDebouncedValue(
    catalogSearch.trim(),
    CONTEST_ADMIN_CATALOG.SEARCH_DEBOUNCE_MS,
  );
  const catalogResource = useRetryableResource<Problem[]>({
    resetKey: debouncedCatalogSearch,
    enabled: open,
    initialData: [],
    keepPreviousData: true,
    load: async (signal) => {
      const page = await problemService.find(
        {
          pagination: {
            page: 1,
            page_size: CONTEST_ADMIN_CATALOG.PAGE_SIZE,
          },
          filters: debouncedCatalogSearch
            ? [
                {
                  key: "name",
                  value: debouncedCatalogSearch,
                  type: "search",
                },
              ]
            : [],
        },
        signal,
      );
      return page.records;
    },
  });
  const catalog = useMemo(
    () => catalogResource.isPreviousData ? [] : catalogResource.data,
    [catalogResource.data, catalogResource.isPreviousData],
  );
  const catalogLoading = catalogResource.status === "loading";
  const catalogFailed = catalogResource.status === "error";

  const selectedIDs = useMemo(
    () => new Set(selectedProblems.map((problem) => problem.id)),
    [selectedProblems],
  );
  const availableProblems = useMemo(
    () => catalog.filter((problem) => !selectedIDs.has(problem.id)),
    [catalog, selectedIDs],
  );
  const capacity = Number(maxParticipants);
  const aliases = selectedProblems.map((problem) =>
    problem.alias.trim().toUpperCase()
  );
  const validAliases =
    aliases.every((alias) => /^[A-Z][A-Z0-9]{0,15}$/.test(alias)) &&
    new Set(aliases).size === aliases.length;
  const validWindow =
    Boolean(startTime && endTime) &&
    Date.parse(startTime) < Date.parse(endTime);
  const validRegistration =
    registrationMode === "open" ||
    (
      Boolean(registrationOpensAt && registrationClosesAt) &&
      Date.parse(registrationOpensAt) <=
        Date.parse(registrationClosesAt) &&
      Date.parse(registrationClosesAt) <= Date.parse(startTime)
    );
  const validFreeze =
    !freezeStandingsAt ||
    (
      Date.parse(freezeStandingsAt) >= Date.parse(startTime) &&
      Date.parse(freezeStandingsAt) < Date.parse(endTime)
    );
  const validCapacity =
    maxParticipants === "" ||
    (
      Number.isSafeInteger(capacity) &&
      capacity >= 1 &&
      capacity <= 1_000_000
    );
  const canSubmit =
    title.trim().length > 0 &&
    validWindow &&
    validRegistration &&
    validFreeze &&
    validCapacity &&
    validAliases &&
    isContestReasonValid(reason) &&
    !isSaving;

  function addProblem(problem: Problem) {
    setSelectedProblems((current) => {
      const usedAliases = new Set(
        current.map((selected) => selected.alias.toUpperCase()),
      );
      let index = current.length;
      let alias = generatedAlias(index);
      while (usedAliases.has(alias)) {
        index += 1;
        alias = generatedAlias(index);
      }
      return [
        ...current,
        { ...problemOption(problem), alias },
      ];
    });
  }

  function updateSelected(
    index: number,
    update: Partial<SelectedProblem>,
  ) {
    setSelectedProblems((current) =>
      current.map((problem, currentIndex) =>
        currentIndex === index ? { ...problem, ...update } : problem
      )
    );
  }

  function moveProblem(index: number, direction: -1 | 1) {
    setSelectedProblems((current) => {
      const destination = index + direction;
      if (destination < 0 || destination >= current.length) return current;
      const next = [...current];
      [next[index], next[destination]] = [
        next[destination]!,
        next[index]!,
      ];
      return next;
    });
  }

  function handleSubmit() {
    if (!canSubmit) return;
    const input: ContestEditorInput = {
      reason: reason.trim(),
      title: title.trim(),
      description: description.trim(),
      start_time: isoDateTimeSchema.parse(new Date(startTime).toISOString()),
      end_time: isoDateTimeSchema.parse(new Date(endTime).toISOString()),
      registration_mode: registrationMode,
      ...(registrationMode === "registered"
        ? {
            registration_opens_at:
              isoDateTimeSchema.parse(
                new Date(registrationOpensAt).toISOString(),
              ),
            registration_closes_at:
              isoDateTimeSchema.parse(
                new Date(registrationClosesAt).toISOString(),
              ),
          }
        : {}),
      ...(maxParticipants === ""
        ? {}
        : { max_participants: capacity }),
      ...(freezeStandingsAt
        ? {
            freeze_standings_at:
              isoDateTimeSchema.parse(
                new Date(freezeStandingsAt).toISOString(),
              ),
          }
        : {}),
      rated,
      problems: selectedProblems.map((problem) => ({
        problem_id: entityIDSchema.parse(problem.id),
        alias: problem.alias.trim().toUpperCase(),
        visible_before_start: problem.visibleBeforeStart,
      })),
      ...(editing ? { expected_version: editing.version } : {}),
    };
    void onSave(input);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !isSaving) onClose();
      }}
    >
      <DialogContent
        className="grid max-h-[92dvh] gap-0 overflow-hidden p-0 sm:max-w-5xl"
        showCloseButton={!isSaving}
      >
        <DialogHeader className="border-b px-6 py-5">
          <div className="flex items-start gap-3 pr-8">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Trophy className="size-5" />
            </div>
            <div className="space-y-1">
              <DialogTitle className="text-lg">
                {editing
                  ? ADMIN_TEXT.CONTESTS.DIALOG_EDIT_TITLE
                  : ADMIN_TEXT.CONTESTS.DIALOG_CREATE_TITLE}
              </DialogTitle>
              <DialogDescription>
                {editing
                  ? ADMIN_TEXT.CONTESTS.DIALOG_EDIT_DESC
                  : ADMIN_TEXT.CONTESTS.DIALOG_CREATE_DESC}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="overflow-y-auto">
          <div className="grid gap-6 p-6 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.8fr)]">
            <div className="space-y-6">
              <section className="space-y-4 rounded-2xl border bg-card p-5">
                <div className="flex items-center gap-2">
                  <Trophy className="size-4 text-primary" />
                  <h3 className="font-semibold">
                    {ADMIN_TEXT.CONTESTS.SECTION_OVERVIEW}
                  </h3>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="contest-title">
                    {ADMIN_TEXT.CONTESTS.FORM_TITLE}
                  </Label>
                  <Input
                    id="contest-title"
                    autoFocus
                    maxLength={300}
                    placeholder={ADMIN_TEXT.CONTESTS.FORM_TITLE_PLACEHOLDER}
                    value={title}
                    disabled={isSaving}
                    onChange={(event) => setTitle(event.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="contest-description">
                    {ADMIN_TEXT.CONTESTS.FORM_DESCRIPTION}
                  </Label>
                  <Textarea
                    id="contest-description"
                    maxLength={100_000}
                    className="min-h-32 resize-y"
                    placeholder={
                      ADMIN_TEXT.CONTESTS.FORM_DESCRIPTION_PLACEHOLDER
                    }
                    value={description}
                    disabled={isSaving}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </div>
              </section>

              <section className="space-y-4 rounded-2xl border bg-card p-5">
                <div className="flex items-center gap-2">
                  <CalendarClock className="size-4 text-primary" />
                  <h3 className="font-semibold">
                    {ADMIN_TEXT.CONTESTS.SECTION_SCHEDULE}
                  </h3>
                </div>
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="contest-start">
                      {ADMIN_TEXT.CONTESTS.FORM_START_TIME}
                    </Label>
                    <Input
                      id="contest-start"
                      type="datetime-local"
                      value={startTime}
                      aria-invalid={!validWindow}
                      disabled={isSaving}
                      onChange={(event) => setStartTime(event.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="contest-end">
                      {ADMIN_TEXT.CONTESTS.FORM_END_TIME}
                    </Label>
                    <Input
                      id="contest-end"
                      type="datetime-local"
                      value={endTime}
                      aria-invalid={!validWindow}
                      disabled={isSaving}
                      onChange={(event) => setEndTime(event.target.value)}
                    />
                  </div>
                </div>
                {!validWindow ? (
                  <p className="flex items-center gap-1.5 text-xs text-destructive">
                    <AlertCircle className="size-3.5" />
                    {ADMIN_TEXT.CONTESTS.ERROR_TIME_WINDOW}
                  </p>
                ) : null}
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="contest-freeze">
                      {ADMIN_TEXT.CONTESTS.FORM_FREEZE_TIME}
                      <span className="font-normal text-muted-foreground">
                        {ADMIN_TEXT.OPTIONAL}
                      </span>
                    </Label>
                    <Input
                      id="contest-freeze"
                      type="datetime-local"
                      value={freezeStandingsAt}
                      aria-invalid={!validFreeze}
                      disabled={isSaving}
                      onChange={(event) =>
                        setFreezeStandingsAt(event.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <p className="text-sm font-medium leading-none">
                      {ADMIN_TEXT.CONTESTS.FORM_SCORING_POLICY}
                    </p>
                    <div className="flex h-11 items-center justify-between rounded-[var(--control-radius)] border bg-muted/30 px-3">
                      <span className="text-sm font-medium">
                        {ADMIN_TEXT.CONTESTS.SCORING_ICPC}
                      </span>
                      <Badge variant="secondary">
                        {ADMIN_TEXT.CONTESTS.SERVER_MANAGED}
                      </Badge>
                    </div>
                  </div>
                </div>
                <div className="flex items-start justify-between gap-4 rounded-xl border bg-muted/20 p-4">
                  <div className="space-y-1">
                    <Label htmlFor="contest-rated">
                      {ADMIN_TEXT.CONTESTS.FORM_RATED}
                    </Label>
                    <p className="max-w-xl text-xs leading-5 text-muted-foreground">
                      {ADMIN_TEXT.CONTESTS.FORM_RATED_HINT}
                    </p>
                  </div>
                  <Switch
                    id="contest-rated"
                    checked={rated}
                    disabled={isSaving}
                    onCheckedChange={setRated}
                  />
                </div>
              </section>

              <section className="space-y-4 rounded-2xl border bg-card p-5">
                <div className="flex items-center gap-2">
                  <UsersRound className="size-4 text-primary" />
                  <h3 className="font-semibold">
                    {ADMIN_TEXT.CONTESTS.SECTION_REGISTRATION}
                  </h3>
                </div>
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="contest-registration-mode">
                      {ADMIN_TEXT.CONTESTS.FORM_REGISTRATION_MODE}
                    </Label>
                    <Select
                      value={registrationMode}
                      onValueChange={(value) =>
                        setRegistrationMode(
                          value as ContestRegistrationMode,
                        )}
                      disabled={isSaving}
                    >
                      <SelectTrigger id="contest-registration-mode">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="registered">
                          {ADMIN_TEXT.CONTESTS.REGISTRATION_REGISTERED}
                        </SelectItem>
                        <SelectItem value="open">
                          {ADMIN_TEXT.CONTESTS.REGISTRATION_OPEN}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">
                      {registrationMode === "registered"
                        ? ADMIN_TEXT.CONTESTS.REGISTRATION_REGISTERED_HINT
                        : ADMIN_TEXT.CONTESTS.REGISTRATION_OPEN_HINT}
                    </p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="contest-capacity">
                      {ADMIN_TEXT.CONTESTS.FORM_MAX_PARTICIPANTS}
                    </Label>
                    <Input
                      id="contest-capacity"
                      type="number"
                      min={1}
                      max={1_000_000}
                      placeholder={
                        ADMIN_TEXT.CONTESTS.FORM_CAPACITY_UNLIMITED
                      }
                      value={maxParticipants}
                      aria-invalid={!validCapacity}
                      disabled={isSaving}
                      onChange={(event) =>
                        setMaxParticipants(event.target.value)}
                    />
                  </div>
                </div>
                {registrationMode === "registered" ? (
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="contest-registration-opens">
                        {ADMIN_TEXT.CONTESTS.FORM_REGISTRATION_OPENS}
                      </Label>
                      <Input
                        id="contest-registration-opens"
                        type="datetime-local"
                        value={registrationOpensAt}
                        aria-invalid={!validRegistration}
                        disabled={isSaving}
                        onChange={(event) =>
                          setRegistrationOpensAt(event.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="contest-registration-closes">
                        {ADMIN_TEXT.CONTESTS.FORM_REGISTRATION_CLOSES}
                      </Label>
                      <Input
                        id="contest-registration-closes"
                        type="datetime-local"
                        value={registrationClosesAt}
                        aria-invalid={!validRegistration}
                        disabled={isSaving}
                        onChange={(event) =>
                          setRegistrationClosesAt(event.target.value)}
                      />
                    </div>
                  </div>
                ) : null}
              </section>
            </div>

            <section className="flex min-h-[560px] flex-col overflow-hidden rounded-2xl border bg-card">
              <div className="space-y-3 border-b p-5">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="font-semibold">
                      {ADMIN_TEXT.CONTESTS.SECTION_PROBLEMS}
                    </h3>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {ADMIN_TEXT.CONTESTS.PROBLEM_SELECTION_HINT}
                    </p>
                  </div>
                  <Badge variant="secondary">
                    {ADMIN_TEXT.CONTESTS.PROBLEM_SELECTED_COUNT(
                      selectedProblems.length,
                    )}
                  </Badge>
                </div>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    className="pl-9"
                    value={catalogSearch}
                    placeholder={
                      ADMIN_TEXT.CONTESTS.PROBLEM_SEARCH_PLACEHOLDER
                    }
                    aria-label={ADMIN_TEXT.CONTESTS.PROBLEM_SEARCH_LABEL}
                    disabled={isSaving}
                    onChange={(event) =>
                      setCatalogSearch(event.target.value)}
                  />
                </div>
                <div className="max-h-48 space-y-1 overflow-y-auto rounded-xl border bg-muted/20 p-1.5">
                  {catalogLoading ? (
                    <div className="flex h-20 items-center justify-center gap-2 text-sm text-muted-foreground">
                      <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                      {ADMIN_TEXT.CONTESTS.PROBLEM_SEARCHING}
                    </div>
                  ) : catalogFailed ? (
                    <div className="flex min-h-20 flex-col items-center justify-center gap-2 px-4 text-center text-sm text-destructive">
                      <span>{ADMIN_TEXT.CONTESTS.PROBLEM_SEARCH_ERROR}</span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={catalogResource.retry}
                      >
                        <RefreshCw className="size-4" aria-hidden="true" />
                        {TEXT.COMMON.RETRY}
                      </Button>
                    </div>
                  ) : availableProblems.length === 0 ? (
                    <div className="flex h-20 items-center justify-center px-4 text-center text-sm text-muted-foreground">
                      {ADMIN_TEXT.CONTESTS.PROBLEM_SEARCH_EMPTY}
                    </div>
                  ) : (
                    availableProblems.map((problem) => (
                      <button
                        key={problem.id}
                        type="button"
                        className="flex min-h-11 w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors hover:bg-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        disabled={isSaving}
                        onClick={() => addProblem(problem)}
                      >
                        <Plus className="size-4 shrink-0 text-primary" />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium">
                            {problem.title}
                          </span>
                          <span className="block truncate text-xs text-muted-foreground">
                            {problem.slug}
                          </span>
                        </span>
                        {problem.difficulty ? (
                          <Badge variant="outline">
                            {problem.difficulty.name}
                          </Badge>
                        ) : null}
                      </button>
                    ))
                  )}
                </div>
              </div>

              <div className="min-h-0 flex-1 overflow-y-auto p-3">
                {selectedProblems.length === 0 ? (
                  <div className="flex h-full min-h-48 flex-col items-center justify-center gap-2 rounded-xl border border-dashed px-6 text-center">
                    <Trophy className="size-8 text-muted-foreground/50" />
                    <p className="text-sm font-medium">
                      {ADMIN_TEXT.CONTESTS.PROBLEM_SELECTED_EMPTY}
                    </p>
                    <p className="max-w-xs text-xs text-muted-foreground">
                      {ADMIN_TEXT.CONTESTS.PROBLEM_SELECTED_EMPTY_HINT}
                    </p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {selectedProblems.map((problem, index) => (
                      <div
                        key={problem.id}
                        className="rounded-xl border bg-background p-3 shadow-xs"
                      >
                        <div className="flex items-start gap-3">
                          <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-sm font-bold text-primary">
                            {index + 1}
                          </div>
                          <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-semibold">
                              {problem.title}
                            </p>
                            <p className="truncate text-xs text-muted-foreground">
                              {problem.slug}
                            </p>
                          </div>
                          <div className="flex items-center">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-xs"
                              aria-label={
                                ADMIN_TEXT.CONTESTS.PROBLEM_MOVE_UP
                              }
                              disabled={index === 0 || isSaving}
                              onClick={() => moveProblem(index, -1)}
                            >
                              <ChevronUp />
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-xs"
                              aria-label={
                                ADMIN_TEXT.CONTESTS.PROBLEM_MOVE_DOWN
                              }
                              disabled={
                                index === selectedProblems.length - 1 ||
                                isSaving
                              }
                              onClick={() => moveProblem(index, 1)}
                            >
                              <ChevronDown />
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-xs"
                              className="text-destructive"
                              aria-label={
                                ADMIN_TEXT.CONTESTS.PROBLEM_REMOVE
                              }
                              disabled={isSaving}
                              onClick={() =>
                                setSelectedProblems((current) =>
                                  current.filter(
                                    (selected) =>
                                      selected.id !== problem.id,
                                  )
                                )}
                            >
                              <Trash2 />
                            </Button>
                          </div>
                        </div>
                        <div className="mt-3 grid grid-cols-[112px_1fr] items-end gap-3">
                          <div className="space-y-1.5">
                            <Label
                              htmlFor={`contest-problem-alias-${problem.id}`}
                              className="text-xs"
                            >
                              {ADMIN_TEXT.CONTESTS.PROBLEM_ALIAS}
                            </Label>
                            <Input
                              id={`contest-problem-alias-${problem.id}`}
                              maxLength={16}
                              className="h-10 uppercase"
                              value={problem.alias}
                              aria-invalid={!/^[A-Z][A-Z0-9]{0,15}$/.test(
                                problem.alias.trim().toUpperCase(),
                              )}
                              disabled={isSaving}
                              onChange={(event) =>
                                updateSelected(index, {
                                  alias: event.target.value.toUpperCase(),
                                })}
                            />
                          </div>
                          <div className="flex min-h-10 items-center justify-between rounded-lg border bg-muted/20 px-3">
                            <div className="flex items-center gap-2">
                              <Eye className="size-4 text-muted-foreground" />
                              <span className="text-xs font-medium">
                                {ADMIN_TEXT.CONTESTS.PROBLEM_VISIBLE_BEFORE}
                              </span>
                            </div>
                            <Switch
                              size="sm"
                              checked={problem.visibleBeforeStart}
                              disabled={isSaving}
                              aria-label={
                                ADMIN_TEXT.CONTESTS.PROBLEM_VISIBLE_BEFORE
                              }
                              onCheckedChange={(checked) =>
                                updateSelected(index, {
                                  visibleBeforeStart: checked,
                                })}
                            />
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </section>
          </div>
        </div>

        <div className="space-y-2 border-t bg-muted/20 px-6 py-4">
          <Label htmlFor="contest-draft-reason">
            {ADMIN_TEXT.CONTESTS.DRAFT_REASON_LABEL}
          </Label>
          <Textarea
            id="contest-draft-reason"
            className="min-h-20 resize-y bg-background"
            value={reason}
            maxLength={CONTEST_REASON_LIMITS.MAXIMUM_CHARACTERS}
            placeholder={ADMIN_TEXT.CONTESTS.DRAFT_REASON_PLACEHOLDER}
            disabled={isSaving}
            aria-invalid={reason.length > 0 && !isContestReasonValid(reason)}
            onChange={(event) => setReason(event.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            {ADMIN_TEXT.CONTESTS.DRAFT_REASON_HINT(
              new TextEncoder().encode(reason.trim()).byteLength,
              CONTEST_REASON_LIMITS.MAXIMUM_UTF8_BYTES,
            )}
          </p>
        </div>

        <DialogFooter className="m-0">
          {!validAliases && selectedProblems.length > 0 ? (
            <p className="mr-auto flex items-center gap-1.5 text-xs text-destructive">
              <AlertCircle className="size-3.5" />
              {ADMIN_TEXT.CONTESTS.ERROR_PROBLEM_ALIAS}
            </p>
          ) : (
            <p className="mr-auto text-xs text-muted-foreground">
              {ADMIN_TEXT.CONTESTS.DRAFT_VERSION_HINT}
            </p>
          )}
          <Button
            variant="outline"
            onClick={onClose}
            disabled={isSaving}
          >
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button disabled={!canSubmit} onClick={handleSubmit}>
            {isSaving ? <Loader2 className="animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
            {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
