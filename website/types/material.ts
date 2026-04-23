/**
 * Material-related types mirroring backend material DTOs.
 */

import type { DifficultyResponse } from "./difficulty";
import type { Tag } from "./tag";

/** Material category entity — mirrors BE MaterialCategoryResponse. */
export interface MaterialCategory {
  id: number;
  name: string;
  description?: string;
  article_count: number;
}

/** Material article entity — mirrors BE MaterialResponse. */
export interface MaterialArticle {
  id: number;
  title: string;
  description: string;
  content?: string;
  difficulty: DifficultyResponse;
  category: MaterialCategory;
  tags: Tag[];
  author_id: number;
  status: "draft" | "published";
  visibility: "public" | "group";
  view_count: number;
  estimated_read_time: number;
  created_at: string;
  updated_at: string;
}

/** Request body for creating a material article. */
export interface CreateMaterialRequest {
  title: string;
  description?: string;
  content?: string;
  difficulty_id: number;
  category_id: number;
  tag_ids?: number[];
  status?: "draft" | "published";
  visibility?: "public" | "group";
}

/** Request body for updating a material article. */
export interface UpdateMaterialRequest {
  title?: string;
  description?: string;
  content?: string;
  difficulty_id?: number;
  category_id?: number;
  tag_ids?: number[];
  status?: "draft" | "published";
  visibility?: "public" | "group";
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
