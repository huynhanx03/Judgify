import { api } from "@/lib/api/client";
import { PROBLEM_API } from "@/constants/api/problem";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  Problem,
  ProblemAuthoringCatalog,
  ProblemAuthoringDraft,
  ProblemDraftReceipt,
  ProblemPublicationReceipt,
  SaveProblemDraftInput,
} from "@/types/problem";
import type { SampleTestCaseResponse } from "@/types/submission";
import { entityIDSchema } from "@/lib/api/contracts";
import {
  problemPageSchema,
  problemSchema,
  sampleTestCaseListSchema,
} from "@/lib/problems/public-problem-schema";
import {
  problemAuthoringCatalogSchema,
  problemAuthoringDraftSchema,
  problemDraftReceiptSchema,
  problemPublicationReceiptSchema,
} from "@/lib/problems/authoring-schema";

export const problemService = {
  // Read
  async find(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<Problem>> {
    return api<Paginated<Problem>, QueryOptions | undefined>(
      PROBLEM_API.FIND,
      {
        method: "POST",
        body: query,
        signal,
        schema: problemPageSchema,
      },
    );
  },
  async getById(id: string, signal?: AbortSignal): Promise<Problem> {
    return api<Problem, never>(
      PROBLEM_API.GET(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: problemSchema,
      },
    );
  },
  async getPublicSamples(
    id: string,
    signal?: AbortSignal,
  ): Promise<SampleTestCaseResponse[]> {
    return api<SampleTestCaseResponse[], never>(
      PROBLEM_API.SAMPLES(entityIDSchema.parse(id)),
      {
        method: "GET",
        auth: "none",
        signal,
        schema: sampleTestCaseListSchema,
      },
    );
  },

  // Administrative reads include draft and published records.
  async findAdmin(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<Problem>> {
    return api<Paginated<Problem>, QueryOptions | undefined>(
      PROBLEM_API.ADMIN_FIND,
      {
        method: "POST",
        body: query,
        signal,
        schema: problemPageSchema,
      },
    );
  },
  async getAdminById(id: string, signal?: AbortSignal): Promise<Problem> {
    return api<Problem, never>(
      PROBLEM_API.ADMIN_GET(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: problemSchema,
      },
    );
  },
  async getAuthoringDraft(
    id: string,
    signal?: AbortSignal,
  ): Promise<ProblemAuthoringDraft> {
    return api<ProblemAuthoringDraft, never>(
      PROBLEM_API.ADMIN_AUTHORING(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: problemAuthoringDraftSchema,
      },
    );
  },
  async getAuthoringCatalog(
    signal?: AbortSignal,
  ): Promise<ProblemAuthoringCatalog> {
    return api<ProblemAuthoringCatalog, never>(
      PROBLEM_API.ADMIN_AUTHORING_CATALOG,
      {
        method: "GET",
        signal,
        schema: problemAuthoringCatalogSchema,
      },
    );
  },

  // Stable aggregate creation. Authored content is saved atomically through
  // saveDraft; testcase mutations exist only inside that revision command.
  async create(data: {
    slug: string;
    title: string;
    description: string;
    difficulty_id: string;
    time_limit_ms?: number;
    memory_limit_kb?: number;
    tag_ids?: string[];
  }): Promise<Problem> {
    return api<Problem, typeof data>(PROBLEM_API.CREATE, {
      method: "POST",
      body: {
        ...data,
        difficulty_id: entityIDSchema.parse(data.difficulty_id),
        tag_ids: data.tag_ids?.map((tagID) => entityIDSchema.parse(tagID)),
      },
      schema: problemSchema,
    });
  },
  async saveDraft(
    id: string,
    data: SaveProblemDraftInput,
  ): Promise<ProblemDraftReceipt> {
    return api<ProblemDraftReceipt, SaveProblemDraftInput>(
      PROBLEM_API.DRAFT(entityIDSchema.parse(id)),
      {
        method: "POST",
        body: {
          ...data,
          difficulty_id: entityIDSchema.parse(data.difficulty_id),
          tag_ids: data.tag_ids.map((tagID) =>
            entityIDSchema.parse(tagID),
          ),
        },
        schema: problemDraftReceiptSchema,
      },
    );
  },
  async publish(
    id: string,
    data: {
      draft_revision_id: string;
      testset_revision_id: string;
      expected_version: number;
      catalog_version: string;
      reason: string;
    },
  ): Promise<ProblemPublicationReceipt> {
    return api<
      ProblemPublicationReceipt,
      {
        draft_revision_id: string;
        testset_revision_id: string;
        expected_version: number;
        catalog_version: string;
        reason: string;
      }
    >(
      PROBLEM_API.PUBLISH(entityIDSchema.parse(id)),
      {
        method: "POST",
        body: {
          ...data,
          draft_revision_id: entityIDSchema.parse(data.draft_revision_id),
          testset_revision_id: entityIDSchema.parse(data.testset_revision_id),
        },
        schema: problemPublicationReceiptSchema,
      },
    );
  },
  async archive(
    id: string,
    data: { expected_version: number; reason: string },
  ): Promise<ProblemPublicationReceipt> {
    return api<
      ProblemPublicationReceipt,
      { expected_version: number; reason: string }
    >(
      PROBLEM_API.ARCHIVE(entityIDSchema.parse(id)),
      {
        method: "POST",
        body: data,
        schema: problemPublicationReceiptSchema,
      },
    );
  },
};
