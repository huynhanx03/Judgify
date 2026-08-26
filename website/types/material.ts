/**
 * Material-related types mirroring backend material DTOs.
 */

import type { Tag } from "./tag";
import type { EntityID, ISODateTime } from "@/types/api";

/** Material category entity — mirrors BE MaterialCategoryResponse. */
export interface MaterialCategory {
  id: EntityID;
  name: string;
  description?: string;
  article_count: number;
}

/** Difficulty shape embedded in the material API DTO. */
export interface MaterialDifficulty {
  id: EntityID;
  name: string;
  level: number;
  exp_reward: number;
  description?: string;
  version: number;
}

/** Material article entity — mirrors BE MaterialResponse. */
export interface MaterialArticle {
  id: EntityID;
  slug: string;
  title: string;
  description: string;
  content?: string;
  sanitized_html?: string;
  difficulty: MaterialDifficulty;
  category: MaterialCategory;
  tags: Tag[];
  author_id: EntityID;
  status: "draft" | "in_review" | "published" | "archived";
  visibility: "public" | "authenticated";
  view_count: number;
  estimated_read_time: number;
  version: number;
	revision_id?: EntityID;
	published_revision_id?: EntityID;
	published_revision_number?: number;
	revision_number?: number;
  render_artifact_id?: EntityID;
  renderer_version?: string;
  sanitizer_policy_version?: string;
  published_at?: ISODateTime;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

/** Request body for creating a material article. */
export interface CreateMaterialRequest {
  slug: string;
  title: string;
  description?: string;
  content?: string;
  difficulty_id: EntityID;
  category_id: EntityID;
  tag_ids?: EntityID[];
  change_summary: string;
  status?: "draft" | "in_review";
  visibility?: "public" | "authenticated";
}

/** Request body for updating a material article. */
export interface UpdateMaterialRequest {
  title?: string;
  description?: string;
  content?: string;
  difficulty_id?: EntityID;
  category_id?: EntityID;
  tag_ids?: EntityID[];
  expected_version: number;
  change_summary: string;
  status?: "draft" | "in_review";
  visibility?: "public" | "authenticated";
}

export interface MaterialSearchRequest {
  text?: string;
  category_id?: EntityID;
  tag_ids?: EntityID[];
  difficulty_id?: EntityID;
  sort?: "relevance" | "published" | "popularity";
  cursor?: string;
  limit?: number;
}

export interface MaterialSearchResponse {
  records: MaterialArticle[];
  next_cursor?: string;
}

export interface AdminMaterialSearchRequest {
  text?: string;
  category_id?: EntityID;
  tag_ids?: EntityID[];
  difficulty_id?: EntityID;
  status?: "draft" | "in_review" | "published" | "archived";
  cursor?: string;
  limit?: number;
}

export interface AdminMaterialSearchResponse {
  records: MaterialArticle[];
  next_cursor?: string;
}

export interface MaterialRevision {
  id: EntityID;
  material_id: EntityID;
  revision_number: number;
  title: string;
  description: string;
  content_markdown?: string;
  sanitized_html?: string;
  category_id: EntityID;
  difficulty_id: EntityID;
  tag_ids: EntityID[];
  author_id: EntityID;
  estimated_read_minutes: number;
  change_summary: string;
  created_at: string;
  render_source_id: EntityID;
  render_artifact_id: EntityID;
  normalized_source_checksum: string;
  renderer_version?: string;
  sanitizer_policy_version?: string;
}

export interface MaterialRevisionPage {
  records: MaterialRevision[];
  next_cursor?: string;
}

export interface MaterialSlugResponse {
  material: MaterialArticle;
  redirect: boolean;
  canonical_slug: string;
}

export interface MaterialLifecycleRequest {
	command_id: EntityID;
  expected_version: number;
  reason: string;
	confirmation_token?: string;
}

export type MaterialLifecycleAction = "publish" | "archive" | "rollback" | "submit_review" | "return_to_draft";

export interface MaterialLifecycleProjection {
	version: number;
	status: MaterialArticle["status"];
	visibility: MaterialArticle["visibility"];
	slug: string;
	revision_id?: EntityID;
	revision_number?: number;
	title?: string;
  category_id: EntityID;
  difficulty_id: EntityID;
  tag_ids: EntityID[];
  render_source_id: EntityID;
  render_artifact_id: EntityID;
  source_checksum: string;
  renderer_version: string;
  sanitizer_policy_version: string;
}

export interface MaterialLifecyclePreview {
	action: MaterialLifecycleAction;
	material_id: EntityID;
	target_revision_id?: EntityID;
	expected_version: number;
	before: MaterialLifecycleProjection;
	after: MaterialLifecycleProjection;
	confirmation_token: string;
	confirmation_expires_at: ISODateTime;
}

export interface MaterialLifecycleReceipt {
	material_id: EntityID;
	action: MaterialLifecycleAction;
	command_id: EntityID;
	event_id: EntityID;
	version: number;
	after: MaterialLifecycleProjection;
	committed_at: ISODateTime;
	idempotent_replay: boolean;
}

export type MaterialMutationAction =
  | "material.draft_created.v1"
  | "material.draft_saved.v1"
  | "material.slug_changed.v1";

export interface MaterialMutationReceipt {
  material_id: EntityID;
  action: MaterialMutationAction;
  command_id: EntityID;
  event_id: EntityID;
  version: number;
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface MaterialOptimisticMutationRequest {
	expected_version: number;
	reason: string;
}

/** Request body for creating a material category. */
export interface CreateMaterialCategoryRequest {
  name: string;
  description?: string;
}

/** Request body for updating a material category. */
export interface UpdateMaterialCategoryRequest {
  name?: string;
  description?: string;
}
