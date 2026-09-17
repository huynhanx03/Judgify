"use client";

/**
 * Admin edit problem page — fetches problem by ID, then renders shared form.
 */

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { ProblemForm } from "@/modules/admin/problem-form";
import { DataLoadingFeedback } from "@/modules/problem/data-load-feedback";
import { problemService } from "@/services/problem.service";
import { notify } from "@/lib/toast";
import { ApiError } from "@/lib/api/error";
import { APP_ROUTES } from "@/constants/routes";
import { useAuth } from "@/contexts/auth-context";
import { AdminAccessState } from "@/modules/admin/admin-access-state";
import type { Problem, ProblemAuthoringDraft } from "@/types/problem";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { tryEntityID } from "@/lib/api/contracts";
import { useRetryableResource } from "@/hooks/use-retryable-resource";

type ProblemSource =
  | { kind: "draft"; value: ProblemAuthoringDraft }
  | { kind: "aggregate"; value: Problem };

export default function AdminEditProblemPage() {
  const params = useParams();
  const router = useRouter();
  const id = tryEntityID(typeof params.id === "string" ? params.id : null);
  const { refreshCapabilities } = useAuth();

  const problemResource = useRetryableResource<ProblemSource | null>({
    resetKey: id,
    enabled: id !== null,
    initialData: null,
    load: async (signal) => {
      if (!id) throw new TypeError("problem ID is required");
      try {
        return {
          kind: "draft",
          value: await problemService.getAuthoringDraft(id, signal),
        };
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 404) throw error;
        return {
          kind: "aggregate",
          value: await problemService.getAdminById(id, signal),
        };
      }
    },
  });
  const loadError = problemResource.error;
  const forbidden = loadError instanceof ApiError && loadError.status === 403;
  const notFound = loadError instanceof ApiError && loadError.status === 404;
  const source = problemResource.data;
  const draft = source?.kind === "draft" ? source.value : null;
  const problem = source?.kind === "aggregate" ? source.value : null;

  useEffect(() => {
    if (id === null) {
      router.replace(APP_ROUTES.ADMIN_PROBLEMS);
      return;
    }
    if (forbidden) {
      void refreshCapabilities();
      return;
    }
    if (notFound) {
      notify.error(ADMIN_TEXT.PROBLEM_NOT_FOUND);
      router.replace(APP_ROUTES.ADMIN_PROBLEMS);
    }
  }, [forbidden, id, notFound, refreshCapabilities, router]);

  if (
    problemResource.status === "idle" ||
    problemResource.status === "loading" ||
    (problemResource.status === "ready" &&
      draft?.problem_id !== id &&
      problem?.id !== id)
  ) {
    return (
      <DataLoadingFeedback
        label={ADMIN_TEXT.PROBLEM_LOADING}
        className="min-h-[50vh]"
      />
    );
  }

  if (forbidden) {
    return <AdminAccessState kind="forbidden" />;
  }

  if (problemResource.status === "error" && !notFound) {
    return (
      <AdminAccessState
        kind="unavailable"
        onRetry={problemResource.retry}
      />
    );
  }

  if (!draft && !problem) return null;

  return (
    <ProblemForm
      key={`${id}:${draft?.version ?? problem?.version ?? 0}`}
      draft={draft}
      initialProblem={problem}
    />
  );
}
