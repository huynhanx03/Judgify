import { apiClient } from "@/lib/api-client";
import { MATERIAL_API, MATERIAL_CATEGORY_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  MaterialArticle,
  MaterialCategory,
  CreateMaterialRequest,
  UpdateMaterialRequest,
  CreateMaterialCategoryRequest,
  UpdateMaterialCategoryRequest,
} from "@/types/material";

export const materialService = {
  // Materials
  async find(query?: QueryOptions): Promise<Paginated<MaterialArticle>> {
    return apiClient.post<Paginated<MaterialArticle>>(MATERIAL_API.FIND, query);
  },
  async getById(id: number): Promise<MaterialArticle> {
    return apiClient.get<MaterialArticle>(MATERIAL_API.GET(id));
  },
  async create(data: CreateMaterialRequest): Promise<MaterialArticle> {
    return apiClient.post<MaterialArticle>(MATERIAL_API.CREATE, data);
  },
  async update(id: number, data: UpdateMaterialRequest): Promise<MaterialArticle> {
    return apiClient.put<MaterialArticle>(MATERIAL_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(MATERIAL_API.DELETE(id));
  },

  // Categories
  async findCategories(query?: QueryOptions): Promise<Paginated<MaterialCategory>> {
    return apiClient.post<Paginated<MaterialCategory>>(
      MATERIAL_CATEGORY_API.FIND,
      query
    );
  },
  async getAllCategories(): Promise<MaterialCategory[]> {
    return apiClient.get<MaterialCategory[]>(MATERIAL_CATEGORY_API.FIND_ALL);
  },
  async getCategoryById(id: number): Promise<MaterialCategory> {
    return apiClient.get<MaterialCategory>(MATERIAL_CATEGORY_API.GET(id));
  },
  async createCategory(data: CreateMaterialCategoryRequest): Promise<MaterialCategory> {
    return apiClient.post<MaterialCategory>(MATERIAL_CATEGORY_API.CREATE, data);
  },
  async updateCategory(id: number, data: UpdateMaterialCategoryRequest): Promise<MaterialCategory> {
    return apiClient.put<MaterialCategory>(
      MATERIAL_CATEGORY_API.UPDATE(id),
      data
    );
  },
  async deleteCategory(id: number): Promise<void> {
    await apiClient.delete(MATERIAL_CATEGORY_API.DELETE(id));
  },
};
