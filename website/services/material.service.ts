import { api } from "@/lib/api/client";
import { voidSchema, type Schema } from "@/lib/api/schema";
import { MATERIAL_API, MATERIAL_CATEGORY_API } from "@/constants/api/material";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  MaterialArticle,
  MaterialCategory,
  CreateMaterialRequest,
  UpdateMaterialRequest,
  CreateMaterialCategoryRequest,
  UpdateMaterialCategoryRequest,
  MaterialSearchRequest,
  MaterialSearchResponse,
  AdminMaterialSearchRequest,
  AdminMaterialSearchResponse,
  MaterialRevision,
  MaterialRevisionPage,
  MaterialSlugResponse,
  MaterialLifecycleRequest,
  MaterialLifecyclePreview,
  MaterialLifecycleReceipt,
  MaterialMutationReceipt,
  MaterialOptimisticMutationRequest,
} from "@/types/material";
import { entityIDSchema } from "@/lib/api/contracts";
import { commandPurpose } from "@/lib/api/command-purpose";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import {
  materialArticleSchema,
  materialArticleListSchema,
  materialCategorySchema,
  materialCategoryListSchema,
  materialCategoryPageSchema,
  createMaterialCategoryRequestSchema,
  updateMaterialCategoryRequestSchema,
  adminMaterialSearchRequestSchema,
  adminMaterialSearchResponseSchema,
  materialRevisionPageSchema,
  materialRevisionSchema,
  materialSearchResponseSchema,
  materialSlugResponseSchema,
  recordMaterialViewResponseSchema,
  materialLifecyclePreviewSchema,
  materialLifecycleReceiptSchema,
  materialMutationReceiptSchema,
} from "@/lib/materials/public-material-schema";

const adminMaterialCursors = new Map<string, string[]>();
const MAXIMUM_ADMIN_CURSOR_QUERIES = 32;

interface MaterialLifecycleTransportBody {
  command_id: string;
  expected_version: number;
  reason: string;
  confirmation_token?: string;
}

function materialLifecycleBody(
  request: MaterialLifecycleRequest,
  requireConfirmation: boolean,
): MaterialLifecycleTransportBody {
  const commandID = entityIDSchema.parse(request.command_id);
  const reason = request.reason.trim();
  if (
    !Number.isSafeInteger(request.expected_version) ||
    request.expected_version < 1 ||
    reason.length < 3 ||
    reason.length > 500
  ) {
    throw new TypeError("invalid material lifecycle command");
  }
  const base = {
    command_id: commandID,
    expected_version: request.expected_version,
    reason,
  };
  if (!requireConfirmation) return base;
  const confirmationToken = request.confirmation_token;
  if (!confirmationToken || confirmationToken.length > 2_048) {
    throw new TypeError("material lifecycle confirmation is required");
  }
  return { ...base, confirmation_token: confirmationToken };
}

function lifecycleCall<TResponse, TBody extends object>(
  path: string,
  commandID: string,
  body: TBody,
  schema: Schema<TResponse>,
): Promise<TResponse> {
  return api<TResponse, TBody>(path, {
    method: "POST",
    body,
    idempotencyKey: commandID,
    schema,
  });
}

async function optimisticMaterialMutation<TBody extends object>(
  namespace: string,
  path: string,
  method: "POST" | "PUT",
  body: TBody,
): Promise<MaterialMutationReceipt> {
  const purpose = await commandPurpose(namespace, [path, method, body]);
  return runIdempotentCommand(purpose, (commandID) => {
    const commandBody = { ...body, command_id: commandID };
    return api<MaterialMutationReceipt, typeof commandBody>(path, {
      method,
      body: commandBody,
      idempotencyKey: commandID,
      schema: materialMutationReceiptSchema,
    });
  });
}

export const materialService = {
  // Materials
  async search(
    query: MaterialSearchRequest,
    signal?: AbortSignal,
  ): Promise<MaterialSearchResponse> {
    return api<MaterialSearchResponse, MaterialSearchRequest>(
      MATERIAL_API.SEARCH,
      {
        method: "POST",
        body: query,
        signal,
        schema: materialSearchResponseSchema,
        auth: "none",
        csrf: "omit",
      },
    );
  },
  async getBySlug(
    slug: string,
    signal?: AbortSignal,
  ): Promise<MaterialSlugResponse> {
    return api<MaterialSlugResponse, never>(
      MATERIAL_API.GET_BY_SLUG(slug),
      {
        method: "GET",
        signal,
        schema: materialSlugResponseSchema,
        auth: "none",
      },
    );
  },
  async related(
    slug: string,
    signal?: AbortSignal,
  ): Promise<MaterialArticle[]> {
    return api<MaterialArticle[], never>(MATERIAL_API.RELATED(slug), {
      method: "GET",
      signal,
      schema: materialArticleListSchema,
      auth: "none",
    });
  },
  async recordView(
    id: string,
    signal?: AbortSignal,
  ): Promise<{ counted: boolean }> {
    return api<{ counted: boolean }, never>(
      MATERIAL_API.RECORD_VIEW(entityIDSchema.parse(id)),
      {
        method: "POST",
        signal,
        schema: recordMaterialViewResponseSchema,
        auth: "none",
        csrf: "omit",
      },
    );
  },
  async findAdmin(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<MaterialArticle>> {
    const filters = query?.filters ?? [];
    const text = filters.find((item) => item.type === "search")?.value;
    const category = filters.find((item) => item.key === "category_id")?.value;
    const difficulty = filters.find(
      (item) => item.key === "difficulty_id",
    )?.value;
    const status = filters.find((item) => item.key === "status")?.value;
    const page = query?.pagination?.page ?? 1;
    const pageSize = query?.pagination?.page_size ?? 10;
    const queryKey = JSON.stringify({
      text: typeof text === "string" ? text.trim() : "",
      category: typeof category === "string" ? category : "",
      difficulty: typeof difficulty === "string" ? difficulty : "",
      status: typeof status === "string" ? status : "",
      pageSize,
    });
    if (page === 1) {
      adminMaterialCursors.delete(queryKey);
    }
    const cursors = adminMaterialCursors.get(queryKey) ?? [""];
    const cursor = cursors[page - 1];
    if (cursor === undefined) {
      throw new TypeError("admin material cursor is unavailable");
    }
    const response = await this.searchAdmin(
      {
        ...(typeof text === "string" && text.trim()
          ? { text: text.trim() }
          : {}),
        ...(typeof category === "string" && category
          ? { category_id: entityIDSchema.parse(category) }
          : {}),
        ...(typeof difficulty === "string"
          ? { difficulty_id: entityIDSchema.parse(difficulty) }
          : {}),
        ...(status === "draft" ||
        status === "in_review" ||
        status === "published" ||
        status === "archived"
          ? { status }
          : {}),
        ...(cursor ? { cursor } : {}),
        limit: pageSize,
      },
      signal,
    );
    if (response.next_cursor) {
      cursors[page] = response.next_cursor;
    } else {
      cursors.length = page;
    }
    adminMaterialCursors.set(queryKey, cursors);
    if (adminMaterialCursors.size > MAXIMUM_ADMIN_CURSOR_QUERIES) {
      const oldest = adminMaterialCursors.keys().next().value;
      if (oldest !== undefined) adminMaterialCursors.delete(oldest);
    }
    const hasNext = Boolean(response.next_cursor);
    const totalPages = hasNext ? page + 1 : page;
    return {
      records: response.records,
      pagination: {
        current_page: page,
        page_size: pageSize,
        total_pages: totalPages,
        total_items: (page - 1) * pageSize + response.records.length +
          (hasNext ? 1 : 0),
        has_next: hasNext,
        has_prev: page > 1,
      },
    };
  },
  async searchAdmin(
    query: AdminMaterialSearchRequest,
    signal?: AbortSignal,
  ): Promise<AdminMaterialSearchResponse> {
    const body = adminMaterialSearchRequestSchema.parse(query);
    return api<AdminMaterialSearchResponse, AdminMaterialSearchRequest>(
      MATERIAL_API.ADMIN_SEARCH,
      {
        method: "POST",
        body,
        signal,
        schema: adminMaterialSearchResponseSchema,
      },
    );
  },
  async getAdmin(
    id: string,
    signal?: AbortSignal,
  ): Promise<MaterialArticle> {
    return api<MaterialArticle, never>(
      MATERIAL_API.GET_ADMIN(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: materialArticleSchema,
      },
    );
  },
  async listRevisions(
    id: string,
    cursor?: string,
    signal?: AbortSignal,
  ): Promise<MaterialRevisionPage> {
    const params = new URLSearchParams({ limit: "100" });
    if (cursor) params.set("cursor", cursor);
    return api<MaterialRevisionPage, never>(
      `${MATERIAL_API.REVISIONS(entityIDSchema.parse(id))}?${params}`,
      { method: "GET", signal, schema: materialRevisionPageSchema },
    );
  },
  async getRevision(
    id: string,
    revisionId: string,
    signal?: AbortSignal,
  ): Promise<MaterialRevision> {
    return api<MaterialRevision, never>(
      MATERIAL_API.REVISION(
        entityIDSchema.parse(id),
        entityIDSchema.parse(revisionId),
      ),
      { method: "GET", signal, schema: materialRevisionSchema },
    );
  },
  async create(data: CreateMaterialRequest): Promise<MaterialMutationReceipt> {
    return optimisticMaterialMutation(
      "material-draft-create",
      MATERIAL_API.CREATE,
      "POST",
      data,
    );
  },
  async update(id: string, data: UpdateMaterialRequest): Promise<MaterialMutationReceipt> {
    return optimisticMaterialMutation(
      "material-draft-save",
      MATERIAL_API.UPDATE(entityIDSchema.parse(id)),
      "PUT",
      data,
    );
  },
  async previewPublish(
    id: string,
    draftRevisionId: string,
    request: MaterialLifecycleRequest,
  ): Promise<MaterialLifecyclePreview> {
    const body = materialLifecycleBody(request, false);
    return lifecycleCall(
      MATERIAL_API.PUBLISH_PREVIEW(entityIDSchema.parse(id)),
      body.command_id,
      {
        ...body,
        draft_revision_id: entityIDSchema.parse(draftRevisionId),
      },
      materialLifecyclePreviewSchema,
    );
  },
  async previewSubmitReview(id: string, request: MaterialLifecycleRequest): Promise<MaterialLifecyclePreview> {
    const body = materialLifecycleBody(request, false);
    return lifecycleCall(MATERIAL_API.SUBMIT_REVIEW_PREVIEW(entityIDSchema.parse(id)), body.command_id, body, materialLifecyclePreviewSchema);
  },
  async applySubmitReview(id: string, request: MaterialLifecycleRequest): Promise<MaterialLifecycleReceipt> {
    const body = materialLifecycleBody(request, true);
    return lifecycleCall(MATERIAL_API.SUBMIT_REVIEW(entityIDSchema.parse(id)), body.command_id, body, materialLifecycleReceiptSchema);
  },
  async previewReturnToDraft(id: string, request: MaterialLifecycleRequest): Promise<MaterialLifecyclePreview> {
    const body = materialLifecycleBody(request, false);
    return lifecycleCall(MATERIAL_API.RETURN_TO_DRAFT_PREVIEW(entityIDSchema.parse(id)), body.command_id, body, materialLifecyclePreviewSchema);
  },
  async applyReturnToDraft(id: string, request: MaterialLifecycleRequest): Promise<MaterialLifecycleReceipt> {
    const body = materialLifecycleBody(request, true);
    return lifecycleCall(MATERIAL_API.RETURN_TO_DRAFT(entityIDSchema.parse(id)), body.command_id, body, materialLifecycleReceiptSchema);
  },
  async applyPublish(
    id: string,
    draftRevisionId: string,
    request: MaterialLifecycleRequest,
  ): Promise<MaterialLifecycleReceipt> {
    const body = materialLifecycleBody(request, true);
    return lifecycleCall(
      MATERIAL_API.PUBLISH(entityIDSchema.parse(id)),
      body.command_id,
      {
        ...body,
        draft_revision_id: entityIDSchema.parse(draftRevisionId),
      },
      materialLifecycleReceiptSchema,
    );
  },
  async previewArchive(
    id: string,
    request: MaterialLifecycleRequest,
  ): Promise<MaterialLifecyclePreview> {
    const body = materialLifecycleBody(request, false);
    return lifecycleCall(
      MATERIAL_API.ARCHIVE_PREVIEW(entityIDSchema.parse(id)),
      body.command_id,
      body,
      materialLifecyclePreviewSchema,
    );
  },
  async applyArchive(
    id: string,
    request: MaterialLifecycleRequest,
  ): Promise<MaterialLifecycleReceipt> {
    const body = materialLifecycleBody(request, true);
    return lifecycleCall(
      MATERIAL_API.ARCHIVE(entityIDSchema.parse(id)),
      body.command_id,
      body,
      materialLifecycleReceiptSchema,
    );
  },
  async previewRollback(
    id: string,
    targetRevisionId: string,
    request: MaterialLifecycleRequest,
  ): Promise<MaterialLifecyclePreview> {
    const body = materialLifecycleBody(request, false);
    return lifecycleCall(
      MATERIAL_API.ROLLBACK_PREVIEW(entityIDSchema.parse(id)),
      body.command_id,
      {
        ...body,
        target_revision_id: entityIDSchema.parse(targetRevisionId),
      },
      materialLifecyclePreviewSchema,
    );
  },
  async applyRollback(
    id: string,
    targetRevisionId: string,
    request: MaterialLifecycleRequest,
  ): Promise<MaterialLifecycleReceipt> {
    const body = materialLifecycleBody(request, true);
    return lifecycleCall(
      MATERIAL_API.ROLLBACK(entityIDSchema.parse(id)),
      body.command_id,
      {
        ...body,
        target_revision_id: entityIDSchema.parse(targetRevisionId),
      },
      materialLifecycleReceiptSchema,
    );
  },
  async rename(
    id: string,
    slug: string,
    request: MaterialOptimisticMutationRequest,
  ): Promise<MaterialMutationReceipt> {
    return optimisticMaterialMutation(
      "material-slug-rename",
      MATERIAL_API.RENAME(entityIDSchema.parse(id)),
      "POST",
      { ...request, slug },
    );
  },

  // Categories
  async findCategories(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<MaterialCategory>> {
    return api<Paginated<MaterialCategory>, QueryOptions | undefined>(
      MATERIAL_CATEGORY_API.FIND,
      {
        method: "POST",
        body: query,
        signal,
        schema: materialCategoryPageSchema,
      },
    );
  },
  async getAllCategories(signal?: AbortSignal): Promise<MaterialCategory[]> {
    return api<MaterialCategory[], never>(MATERIAL_CATEGORY_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: materialCategoryListSchema,
    });
  },
  async getCategoryById(
    id: string,
    signal?: AbortSignal,
  ): Promise<MaterialCategory> {
    return api<MaterialCategory, never>(
      MATERIAL_CATEGORY_API.GET(entityIDSchema.parse(id)),
      { method: "GET", signal, schema: materialCategorySchema },
    );
  },
  async createCategory(
    data: CreateMaterialCategoryRequest,
  ): Promise<MaterialCategory> {
    const body = createMaterialCategoryRequestSchema.parse(data);
    return api<MaterialCategory, typeof body>(MATERIAL_CATEGORY_API.CREATE, {
      method: "POST",
      body,
      schema: materialCategorySchema,
    });
  },
  async updateCategory(
    id: string,
    data: UpdateMaterialCategoryRequest,
  ): Promise<MaterialCategory> {
    const body = updateMaterialCategoryRequestSchema.parse(data);
    return api<MaterialCategory, typeof body>(
      MATERIAL_CATEGORY_API.UPDATE(entityIDSchema.parse(id)),
      { method: "PUT", body, schema: materialCategorySchema },
    );
  },
  async deleteCategory(id: string): Promise<void> {
    await api<void, never>(
      MATERIAL_CATEGORY_API.DELETE(entityIDSchema.parse(id)),
      { method: "DELETE", schema: voidSchema },
    );
  },
};
