import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  nullableSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import { paginatedSchema } from "@/lib/api/contracts";
import {
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import { tagSchema } from "@/lib/problems/public-problem-schema";
import type {
  AdminMaterialSearchRequest,
  AdminMaterialSearchResponse,
  MaterialArticle,
  MaterialCategory,
  MaterialDifficulty,
  MaterialRevision,
  MaterialRevisionPage,
  MaterialLifecyclePreview,
  MaterialLifecycleProjection,
  MaterialLifecycleReceipt,
  MaterialMutationReceipt,
  MaterialSearchResponse,
  MaterialSlugResponse,
} from "@/types/material";

const MAXIMUM_MATERIAL_PAGE = 100;
const MAXIMUM_MATERIAL_TAGS = 64;
const MAXIMUM_MATERIAL_CONTENT_CHARACTERS = 2 * 1024 * 1024;

const rawMaterialCategorySchema = strictObjectSchema({
  id: entityIDSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 100 }),
  description: optionalSchema(stringSchema({ maximumLength: 500 })),
  article_count: integerSchema({
    minimum: 0,
    label: "material category article count",
  }),
});

export const materialCategorySchema: Schema<MaterialCategory> = {
  parse(value: unknown, path = "$"): MaterialCategory {
    const category = rawMaterialCategorySchema.parse(value, path);
    return {
      id: category.id,
      name: category.name,
      ...(category.description === undefined
        ? {}
        : { description: category.description }),
      article_count: category.article_count,
    };
  },
};

export const materialCategoryListSchema = arraySchema(materialCategorySchema, {
  maximumLength: 1_000,
});

export const materialCategoryPageSchema = paginatedSchema(materialCategorySchema);

export const createMaterialCategoryRequestSchema = strictObjectSchema({
  name: stringSchema({ minimumLength: 1, maximumLength: 100 }),
  description: optionalSchema(stringSchema({ maximumLength: 500 })),
});

export const updateMaterialCategoryRequestSchema = strictObjectSchema({
  name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  description: optionalSchema(stringSchema({ maximumLength: 500 })),
});

const rawMaterialDifficultySchema = strictObjectSchema({
  id: entityIDSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 50 }),
  level: integerSchema({
    minimum: 1,
    maximum: 100,
    label: "material difficulty level",
  }),
  exp_reward: integerSchema({
    minimum: 0,
    label: "material difficulty experience reward",
  }),
  description: optionalSchema(stringSchema({ maximumLength: 255 })),
  // The material mapper currently emits the nested DTO version as zero, so
  // accept a non-negative version while retaining the field as part of the
  // strict response contract.
  version: integerSchema({ minimum: 0, label: "material difficulty version" }),
});

const materialDifficultySchema: Schema<MaterialDifficulty> = {
  parse(value: unknown, path = "$"): MaterialDifficulty {
    const difficulty = rawMaterialDifficultySchema.parse(value, path);
    return {
      id: difficulty.id,
      name: difficulty.name,
      level: difficulty.level,
      exp_reward: difficulty.exp_reward,
      ...(difficulty.description === undefined
        ? {}
        : { description: difficulty.description }),
      version: difficulty.version,
    };
  },
};

const rawMaterialSchema = strictObjectSchema({
  id: entityIDSchema,
  slug: stringSchema({
    minimumLength: 1,
    maximumLength: 160,
    pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
  }),
  title: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  description: stringSchema({ maximumLength: 1_000 }),
  content: optionalSchema(
    stringSchema({ maximumLength: MAXIMUM_MATERIAL_CONTENT_CHARACTERS }),
  ),
  sanitized_html: optionalSchema(
    stringSchema({ maximumLength: MAXIMUM_MATERIAL_CONTENT_CHARACTERS }),
  ),
  difficulty: materialDifficultySchema,
  category: materialCategorySchema,
  // The Go DTO emits null when an article has no tags; React always receives
  // an array so rendering cannot fail on an otherwise valid public article.
  tags: nullableSchema(
    arraySchema(tagSchema, { maximumLength: MAXIMUM_MATERIAL_TAGS }),
  ),
  author_id: entityIDSchema,
  status: enumSchema(["draft", "in_review", "published", "archived"] as const),
  visibility: enumSchema(["public", "authenticated"] as const),
  view_count: integerSchema({ minimum: 0, label: "material view count" }),
  estimated_read_time: integerSchema({
    minimum: 0,
    label: "material estimated read time",
  }),
  version: integerSchema({ minimum: 1, label: "material version" }),
	revision_id: optionalSchema(entityIDSchema),
	published_revision_id: optionalSchema(entityIDSchema),
	published_revision_number: optionalSchema(
	  integerSchema({ minimum: 1, label: "published material revision" }),
	),
  revision_number: optionalSchema(
    integerSchema({ minimum: 1, label: "material revision number" }),
  ),
  render_artifact_id: optionalSchema(entityIDSchema),
  renderer_version: optionalSchema(stringSchema({ maximumLength: 128 })),
  sanitizer_policy_version: optionalSchema(
    stringSchema({ maximumLength: 128 }),
  ),
  published_at: optionalSchema(isoDateTimeSchema),
  created_at: isoDateTimeSchema,
  updated_at: isoDateTimeSchema,
});

export const materialArticleSchema: Schema<MaterialArticle> = {
  parse(value: unknown, path = "$"): MaterialArticle {
    const article = rawMaterialSchema.parse(value, path);
    return {
      id: article.id,
      slug: article.slug,
      title: article.title,
      description: article.description,
      ...(article.content === undefined ? {} : { content: article.content }),
      ...(article.sanitized_html === undefined
        ? {}
        : { sanitized_html: article.sanitized_html }),
      difficulty: article.difficulty,
      category: article.category,
      tags: article.tags ?? [],
      author_id: article.author_id,
      status: article.status,
      visibility: article.visibility,
      view_count: article.view_count,
      estimated_read_time: article.estimated_read_time,
      version: article.version,
      ...(article.revision_id === undefined
        ? {}
        : { revision_id: article.revision_id }),
	  ...(article.published_revision_id === undefined
		? {}
		: { published_revision_id: article.published_revision_id }),
	  ...(article.published_revision_number === undefined
		? {}
		: { published_revision_number: article.published_revision_number }),
      ...(article.revision_number === undefined
        ? {}
        : { revision_number: article.revision_number }),
      ...(article.render_artifact_id === undefined
        ? {}
        : { render_artifact_id: article.render_artifact_id }),
      ...(article.renderer_version === undefined
        ? {}
        : { renderer_version: article.renderer_version }),
      ...(article.sanitizer_policy_version === undefined
        ? {}
        : { sanitizer_policy_version: article.sanitizer_policy_version }),
      ...(article.published_at === undefined
        ? {}
        : { published_at: article.published_at }),
      created_at: article.created_at,
      updated_at: article.updated_at,
    };
  },
};

export const materialArticleListSchema = arraySchema(materialArticleSchema, {
  maximumLength: MAXIMUM_MATERIAL_PAGE,
});

export const adminMaterialSearchRequestSchema: Schema<AdminMaterialSearchRequest> =
  strictObjectSchema({
    text: optionalSchema(stringSchema({ maximumLength: 256 })),
    category_id: optionalSchema(entityIDSchema),
    tag_ids: optionalSchema(
      arraySchema(entityIDSchema, { maximumLength: MAXIMUM_MATERIAL_TAGS }),
    ),
    difficulty_id: optionalSchema(entityIDSchema),
    status: optionalSchema(
      enumSchema(["draft", "in_review", "published", "archived"] as const),
    ),
    cursor: optionalSchema(stringSchema({ maximumLength: 512 })),
    limit: optionalSchema(
      integerSchema({ minimum: 1, maximum: MAXIMUM_MATERIAL_PAGE }),
    ),
  });

export const adminMaterialSearchResponseSchema: Schema<AdminMaterialSearchResponse> =
  strictObjectSchema({
    records: materialArticleListSchema,
    next_cursor: optionalSchema(stringSchema({ maximumLength: 512 })),
  });

export const materialRevisionSchema: Schema<MaterialRevision> =
  strictObjectSchema({
    id: entityIDSchema,
    material_id: entityIDSchema,
    revision_number: integerSchema({ minimum: 1 }),
    title: stringSchema({ minimumLength: 1, maximumLength: 255 }),
    description: stringSchema({ maximumLength: 1_000 }),
    content_markdown: optionalSchema(
      stringSchema({ maximumLength: MAXIMUM_MATERIAL_CONTENT_CHARACTERS }),
    ),
    sanitized_html: optionalSchema(
      stringSchema({ maximumLength: MAXIMUM_MATERIAL_CONTENT_CHARACTERS }),
    ),
    category_id: entityIDSchema,
    difficulty_id: entityIDSchema,
    tag_ids: arraySchema(entityIDSchema, {
      maximumLength: MAXIMUM_MATERIAL_TAGS,
    }),
    author_id: entityIDSchema,
    estimated_read_minutes: integerSchema({ minimum: 1 }),
    change_summary: stringSchema({ minimumLength: 1, maximumLength: 500 }),
    created_at: isoDateTimeSchema,
    render_source_id: entityIDSchema,
    render_artifact_id: entityIDSchema,
    normalized_source_checksum: stringSchema({
      minimumLength: 64,
      maximumLength: 64,
      pattern: /^[0-9a-f]{64}$/,
    }),
    renderer_version: optionalSchema(stringSchema({ maximumLength: 128 })),
    sanitizer_policy_version: optionalSchema(
      stringSchema({ maximumLength: 128 }),
    ),
  });

export const materialRevisionPageSchema: Schema<MaterialRevisionPage> =
  strictObjectSchema({
    records: arraySchema(materialRevisionSchema, {
      maximumLength: MAXIMUM_MATERIAL_PAGE,
    }),
    next_cursor: optionalSchema(stringSchema({ maximumLength: 32 })),
  });

export const materialSearchResponseSchema: Schema<MaterialSearchResponse> =
  strictObjectSchema({
    records: materialArticleListSchema,
    next_cursor: optionalSchema(stringSchema({ maximumLength: 512 })),
  });

export const materialSlugResponseSchema: Schema<MaterialSlugResponse> =
  strictObjectSchema({
    material: materialArticleSchema,
    redirect: booleanSchema,
    canonical_slug: stringSchema({
      minimumLength: 1,
      maximumLength: 160,
      pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
    }),
  });

export const recordMaterialViewResponseSchema = strictObjectSchema({
  counted: booleanSchema,
});

const rawMaterialLifecycleProjectionSchema = strictObjectSchema({
  version: integerSchema({ minimum: 1, label: "material lifecycle version" }),
  status: enumSchema(["draft", "in_review", "published", "archived"] as const),
  visibility: enumSchema(["public", "authenticated"] as const),
  slug: stringSchema({
    minimumLength: 1,
    maximumLength: 160,
    pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
  }),
  revision_id: optionalSchema(entityIDSchema),
  revision_number: optionalSchema(
    integerSchema({ minimum: 1, label: "material lifecycle revision" }),
  ),
  title: optionalSchema(
    stringSchema({ minimumLength: 1, maximumLength: 255 }),
  ),
  category_id: entityIDSchema,
  difficulty_id: entityIDSchema,
  tag_ids: arraySchema(entityIDSchema, {
    maximumLength: MAXIMUM_MATERIAL_TAGS,
  }),
  render_source_id: entityIDSchema,
  render_artifact_id: entityIDSchema,
  source_checksum: stringSchema({
    minimumLength: 64,
    maximumLength: 64,
    pattern: /^[0-9a-f]{64}$/,
  }),
  renderer_version: stringSchema({ minimumLength: 1, maximumLength: 128 }),
  sanitizer_policy_version: stringSchema({
    minimumLength: 1,
    maximumLength: 128,
  }),
});

export const materialLifecycleProjectionSchema: Schema<MaterialLifecycleProjection> =
  rawMaterialLifecycleProjectionSchema;

export const materialLifecyclePreviewSchema: Schema<MaterialLifecyclePreview> =
  strictObjectSchema({
    action: enumSchema(["publish", "archive", "rollback", "submit_review", "return_to_draft"] as const),
    material_id: entityIDSchema,
    target_revision_id: optionalSchema(entityIDSchema),
    expected_version: integerSchema({ minimum: 1 }),
    before: materialLifecycleProjectionSchema,
    after: materialLifecycleProjectionSchema,
    confirmation_token: stringSchema({
      minimumLength: 16,
      maximumLength: 2_048,
      pattern: /^[A-Za-z0-9._~-]+$/,
    }),
    confirmation_expires_at: isoDateTimeSchema,
  });

export const materialLifecycleReceiptSchema: Schema<MaterialLifecycleReceipt> =
  strictObjectSchema({
    material_id: entityIDSchema,
    action: enumSchema(["publish", "archive", "rollback", "submit_review", "return_to_draft"] as const),
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    version: integerSchema({ minimum: 2 }),
    after: materialLifecycleProjectionSchema,
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

export const materialMutationReceiptSchema: Schema<MaterialMutationReceipt> =
  strictObjectSchema({
    material_id: entityIDSchema,
    action: enumSchema([
      "material.draft_created.v1",
      "material.draft_saved.v1",
      "material.slug_changed.v1",
    ] as const),
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    version: integerSchema({ minimum: 2 }),
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });
