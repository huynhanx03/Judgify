/**
 * Material service layer — calls backend material module endpoints.
 */

import { apiClient } from "@/lib/api-client";
import { MATERIAL_API, MATERIAL_CATEGORY_API } from "@/constants/api";
import type {
  MaterialArticle,
  MaterialCategory,
  CreateMaterialRequest,
  UpdateMaterialRequest,
  CreateMaterialCategoryRequest,
  UpdateMaterialCategoryRequest,
} from "@/types/material";
import type { Paginated, QueryOptions } from "@/types/api";

// --- Public: Categories ---

/** Fetches all material categories. */
export async function getMaterialCategories(): Promise<MaterialCategory[]> {
  return apiClient.get<MaterialCategory[]>(MATERIAL_CATEGORY_API.FIND_ALL);
}

/** Fetches a paginated list of material categories. */
export async function findMaterialCategories(
  query?: QueryOptions
): Promise<Paginated<MaterialCategory>> {
  return apiClient.post<Paginated<MaterialCategory>>(
    MATERIAL_CATEGORY_API.FIND,
    query
  );
}

/** Fetches a single material category by ID. */
export async function getMaterialCategory(
  id: number
): Promise<MaterialCategory> {
  return apiClient.get<MaterialCategory>(MATERIAL_CATEGORY_API.GET(id));
}

// --- Public: Materials ---

/** Fetches a paginated list of materials. */
export async function findMaterials(
  query?: QueryOptions
): Promise<Paginated<MaterialArticle>> {
  return apiClient.post<Paginated<MaterialArticle>>(
    MATERIAL_API.FIND,
    query
  );
}

/** Fetches a single material by ID. */
export async function getMaterialById(
  id: number
): Promise<MaterialArticle> {
  return apiClient.get<MaterialArticle>(MATERIAL_API.GET(id));
}

// --- Protected: Categories CRUD ---

/** Creates a new material category. */
export async function createMaterialCategory(
  data: CreateMaterialCategoryRequest
): Promise<MaterialCategory> {
  return apiClient.post<MaterialCategory>(MATERIAL_CATEGORY_API.CREATE, data);
}

/** Updates a material category. */
export async function updateMaterialCategory(
  id: number,
  data: UpdateMaterialCategoryRequest
): Promise<MaterialCategory> {
  return apiClient.put<MaterialCategory>(
    MATERIAL_CATEGORY_API.UPDATE(id),
    data
  );
}

/** Deletes a material category. */
export async function deleteMaterialCategory(id: number): Promise<void> {
  await apiClient.delete(MATERIAL_CATEGORY_API.DELETE(id));
}

// --- Protected: Materials CRUD ---

/** Creates a new material article. */
export async function createMaterial(
  data: CreateMaterialRequest
): Promise<MaterialArticle> {
  return apiClient.post<MaterialArticle>(MATERIAL_API.CREATE, data);
}

/** Updates a material article. */
export async function updateMaterial(
  id: number,
  data: UpdateMaterialRequest
): Promise<MaterialArticle> {
  return apiClient.put<MaterialArticle>(MATERIAL_API.UPDATE(id), data);
}

/** Deletes a material article. */
export async function deleteMaterial(id: number): Promise<void> {
  await apiClient.delete(MATERIAL_API.DELETE(id));
}
