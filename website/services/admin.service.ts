/**
 * Admin service for dashboard management.
 * All calls use real backend API endpoints.
 */

import { apiClient } from "@/lib/api-client";
import {
  ROLE_API,
  PERMISSION_API,
  RESOURCE_API,
  PROBLEM_API,
  TAG_API,
  DIFFICULTY_API,
  ELEMENT_API,
  TRAIT_API,
  LEVEL_API,
  RARITY_API,
  RANK_API,
  USER_API,
  CONTEST_API,
  MATERIAL_CATEGORY_API,
  MATERIAL_API,
} from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { Role, Permission, Resource, AdminUser } from "@/types/admin";
import type { Problem } from "@/types/problem";
import type { Contest } from "@/types/contest";
import type { Tag } from "@/types/tag";
import type { DifficultyResponse } from "@/types/difficulty";
import type { TestCaseResponse } from "@/types/submission";
import type { ElementResponse, LevelResponse, RankResponse, RarityResponse, TraitResponse } from "@/types/cultivation";
import type { MaterialArticle, MaterialCategory } from "@/types/material";

export const adminService = {
  // Roles
  async getAllRoles(): Promise<Role[]> {
    return apiClient.get<Role[]>(ROLE_API.FIND_ALL);
  },
  async createRole(data: { name: string; level: number; parent_id?: number }): Promise<Role> {
    return apiClient.post<Role>(ROLE_API.CREATE, data);
  },
  async updateRole(id: number, data: { name?: string; level?: number; parent_id?: number }): Promise<Role> {
    return apiClient.put<Role>(ROLE_API.UPDATE(id), data);
  },
  async deleteRole(id: number): Promise<void> {
    await apiClient.delete(ROLE_API.DELETE(id));
  },

  // Permissions
  async getAllPermissions(): Promise<Permission[]> {
    return apiClient.get<Permission[]>(PERMISSION_API.FIND_ALL);
  },
  async createPermission(data: { role_id: number; resource_id: number; scopes: number }): Promise<Permission> {
    return apiClient.post<Permission>(PERMISSION_API.CREATE, data);
  },
  async updatePermission(id: number, data: { scopes?: number }): Promise<Permission> {
    return apiClient.put<Permission>(PERMISSION_API.UPDATE(id), data);
  },
  async deletePermission(id: number): Promise<void> {
    await apiClient.delete(PERMISSION_API.DELETE(id));
  },

  // Resources
  async getAllResources(): Promise<Resource[]> {
    return apiClient.get<Resource[]>(RESOURCE_API.FIND_ALL);
  },
  async createResource(data: { key: string; description?: string }): Promise<Resource> {
    return apiClient.post<Resource>(RESOURCE_API.CREATE, data);
  },
  async updateResource(id: number, data: { key?: string; description?: string }): Promise<Resource> {
    return apiClient.put<Resource>(RESOURCE_API.UPDATE(id), data);
  },
  async deleteResource(id: number): Promise<void> {
    await apiClient.delete(RESOURCE_API.DELETE(id));
  },

  // Problems
  async findProblems(query: QueryOptions): Promise<Paginated<Problem>> {
    return apiClient.post<Paginated<Problem>>(PROBLEM_API.FIND, query);
  },
  async getProblem(id: number): Promise<Problem> {
    return apiClient.get<Problem>(PROBLEM_API.GET(id));
  },
  async createProblem(data: {
    title: string; description: string; difficulty_id: number;
    time_limit_ms?: number; memory_limit_kb?: number; tag_ids?: number[];
  }): Promise<Problem> {
    return apiClient.post<Problem>(PROBLEM_API.CREATE, data);
  },
  async updateProblem(id: number, data: {
    title?: string; description?: string; difficulty_id?: number;
    time_limit_ms?: number; memory_limit_kb?: number; is_published?: boolean; tag_ids?: number[];
  }): Promise<Problem> {
    return apiClient.put<Problem>(PROBLEM_API.UPDATE(id), data);
  },
  async deleteProblem(id: number): Promise<void> {
    await apiClient.delete(PROBLEM_API.DELETE(id));
  },

  // Test Cases
  async getTestCases(problemId: number): Promise<TestCaseResponse[]> {
    return apiClient.get<TestCaseResponse[]>(PROBLEM_API.TEST_CASES(problemId));
  },
  async createTestCase(problemId: number, data: {
    input: string; expected_output: string; is_hidden?: boolean; order_index?: number;
  }): Promise<TestCaseResponse> {
    return apiClient.post<TestCaseResponse>(PROBLEM_API.TEST_CASES(problemId), data);
  },
  async updateTestCase(id: number, data: {
    input?: string; expected_output?: string; is_hidden?: boolean; order_index?: number;
  }): Promise<TestCaseResponse> {
    return apiClient.put<TestCaseResponse>(PROBLEM_API.TEST_CASE_UPDATE(id), data);
  },
  async deleteTestCase(id: number): Promise<void> {
    await apiClient.delete(PROBLEM_API.TEST_CASE_DELETE(id));
  },

  // Tags
  async findTags(query: QueryOptions): Promise<Paginated<Tag>> {
    return apiClient.post<Paginated<Tag>>(TAG_API.FIND, query);
  },
  async getAllTags(): Promise<Tag[]> {
    return apiClient.get<Tag[]>(TAG_API.FIND_ALL);
  },
  async createTag(data: { name: string; element_ids?: number[] }): Promise<Tag> {
    return apiClient.post<Tag>(TAG_API.CREATE, data);
  },
  async updateTag(id: number, data: { name?: string; element_ids?: number[] }): Promise<Tag> {
    return apiClient.put<Tag>(TAG_API.UPDATE(id), data);
  },
  async deleteTag(id: number): Promise<void> {
    await apiClient.delete(TAG_API.DELETE(id));
  },

  // Difficulties
  async findDifficulties(query: QueryOptions): Promise<Paginated<DifficultyResponse>> {
    return apiClient.post<Paginated<DifficultyResponse>>(DIFFICULTY_API.FIND, query);
  },
  async getAllDifficulties(): Promise<DifficultyResponse[]> {
    return apiClient.get<DifficultyResponse[]>(DIFFICULTY_API.FIND_ALL);
  },
  async createDifficulty(data: { name: string; level: number; exp_reward?: number; description?: string }): Promise<DifficultyResponse> {
    return apiClient.post<DifficultyResponse>(DIFFICULTY_API.CREATE, data);
  },
  async updateDifficulty(id: number, data: { name?: string; level?: number; exp_reward?: number; description?: string }): Promise<DifficultyResponse> {
    return apiClient.put<DifficultyResponse>(DIFFICULTY_API.UPDATE(id), data);
  },
  async deleteDifficulty(id: number): Promise<void> {
    await apiClient.delete(DIFFICULTY_API.DELETE(id));
  },

  // Elements
  async findElements(query: QueryOptions): Promise<Paginated<ElementResponse>> {
    return apiClient.post<Paginated<ElementResponse>>(ELEMENT_API.FIND, query);
  },
  async getAllElements(): Promise<ElementResponse[]> {
    return apiClient.get<ElementResponse[]>(ELEMENT_API.FIND_ALL);
  },
  async createElement(data: { name: string; code: string; description?: string }): Promise<ElementResponse> {
    return apiClient.post<ElementResponse>(ELEMENT_API.CREATE, data);
  },
  async updateElement(id: number, data: { name?: string; code?: string; description?: string }): Promise<ElementResponse> {
    return apiClient.put<ElementResponse>(ELEMENT_API.UPDATE(id), data);
  },
  async deleteElement(id: number): Promise<void> {
    await apiClient.delete(ELEMENT_API.DELETE(id));
  },

  // Traits
  async findTraits(query: QueryOptions): Promise<Paginated<TraitResponse>> {
    return apiClient.post<Paginated<TraitResponse>>(TRAIT_API.FIND, query);
  },
  async getTraits(): Promise<TraitResponse[]> {
    return apiClient.get<TraitResponse[]>(TRAIT_API.FIND_ALL);
  },
  async createTrait(data: { type: string; name: string; rarity_id: number; description?: string; metadata?: Record<string, unknown> }): Promise<TraitResponse> {
    return apiClient.post<TraitResponse>(TRAIT_API.CREATE, data);
  },
  async updateTrait(id: number, data: { type?: string; name?: string; rarity_id?: number; description?: string; metadata?: Record<string, unknown> }): Promise<TraitResponse> {
    return apiClient.put<TraitResponse>(TRAIT_API.UPDATE(id), data);
  },
  async deleteTrait(id: number): Promise<void> {
    await apiClient.delete(TRAIT_API.DELETE(id));
  },

  // Levels
  async findLevels(query: QueryOptions): Promise<Paginated<LevelResponse>> {
    return apiClient.post<Paginated<LevelResponse>>(LEVEL_API.FIND, query);
  },
  async getAllLevels(): Promise<LevelResponse[]> {
    return apiClient.get<LevelResponse[]>(LEVEL_API.FIND_ALL);
  },
  async createLevel(data: { name: string; min_exp: number; description?: string }): Promise<LevelResponse> {
    return apiClient.post<LevelResponse>(LEVEL_API.CREATE, data);
  },
  async updateLevel(id: number, data: { name?: string; min_exp?: number; description?: string }): Promise<LevelResponse> {
    return apiClient.put<LevelResponse>(LEVEL_API.UPDATE(id), data);
  },
  async deleteLevel(id: number): Promise<void> {
    await apiClient.delete(LEVEL_API.DELETE(id));
  },

  // Rarities
  async findRarities(query: QueryOptions): Promise<Paginated<RarityResponse>> {
    return apiClient.post<Paginated<RarityResponse>>(RARITY_API.FIND, query);
  },
  async getAllRarities(): Promise<RarityResponse[]> {
    return apiClient.get<RarityResponse[]>(RARITY_API.FIND_ALL);
  },
  async createRarity(data: { name: string; code: string; weight: number; description?: string }): Promise<RarityResponse> {
    return apiClient.post<RarityResponse>(RARITY_API.CREATE, data);
  },
  async updateRarity(id: number, data: { name?: string; code?: string; weight?: number; description?: string }): Promise<RarityResponse> {
    return apiClient.put<RarityResponse>(RARITY_API.UPDATE(id), data);
  },
  async deleteRarity(id: number): Promise<void> {
    await apiClient.delete(RARITY_API.DELETE(id));
  },

  // Ranks
  async findRanks(query: QueryOptions): Promise<Paginated<RankResponse>> {
    return apiClient.post<Paginated<RankResponse>>(RANK_API.FIND, query);
  },
  async getAllRanks(): Promise<RankResponse[]> {
    return apiClient.get<RankResponse[]>(RANK_API.FIND_ALL);
  },
  async createRank(data: { name: string; min_rating: number; description?: string }): Promise<RankResponse> {
    return apiClient.post<RankResponse>(RANK_API.CREATE, data);
  },
  async updateRank(id: number, data: { name?: string; min_rating?: number; description?: string }): Promise<RankResponse> {
    return apiClient.put<RankResponse>(RANK_API.UPDATE(id), data);
  },
  async deleteRank(id: number): Promise<void> {
    await apiClient.delete(RANK_API.DELETE(id));
  },

  // Users
  async getUsers(query: QueryOptions = { pagination: { page: 1, page_size: 200 } }): Promise<Paginated<AdminUser>> {
    return apiClient.post<Paginated<AdminUser>>(USER_API.FIND, query);
  },
  async createUser(data: {
    username: string; password: string; role_id: number;
    first_name: string; last_name: string; gender: number; birthday: string;
  }): Promise<void> {
    await apiClient.post(USER_API.CREATE, data);
  },
  async updateUserRole(id: number, data: { role_id: number }): Promise<AdminUser> {
    return apiClient.patch<AdminUser>(USER_API.UPDATE(id), data);
  },
  async deleteUser(id: number): Promise<void> {
    await apiClient.delete(USER_API.DELETE(id));
  },

  // Contests
  async findContests(query: QueryOptions): Promise<Paginated<Contest>> {
    return apiClient.post<Paginated<Contest>>(CONTEST_API.FIND, query);
  },
  async createContest(data: {
    title: string; description?: string; start_time: string; end_time: string;
    max_participants?: number; problem_ids?: number[];
  }): Promise<Contest> {
    return apiClient.post<Contest>(CONTEST_API.CREATE, data);
  },
  async updateContest(id: number, data: {
    title?: string; description?: string; start_time?: string; end_time?: string;
    max_participants?: number; problem_ids?: number[];
  }): Promise<Contest> {
    return apiClient.put<Contest>(CONTEST_API.UPDATE(id), data);
  },
  async deleteContest(id: number): Promise<void> {
    await apiClient.delete(CONTEST_API.DELETE(id));
  },

  // Material Categories
  async findMaterialCategories(query: QueryOptions): Promise<Paginated<MaterialCategory>> {
    return apiClient.post(MATERIAL_CATEGORY_API.FIND, query);
  },
  async getAllMaterialCategories(): Promise<MaterialCategory[]> {
    return apiClient.get(MATERIAL_CATEGORY_API.FIND_ALL);
  },
  async createMaterialCategory(data: { name: string; description?: string }): Promise<MaterialCategory> {
    return apiClient.post(MATERIAL_CATEGORY_API.CREATE, data);
  },
  async updateMaterialCategory(id: number, data: { name?: string; description?: string }): Promise<MaterialCategory> {
    return apiClient.put(MATERIAL_CATEGORY_API.UPDATE(id), data);
  },
  async deleteMaterialCategory(id: number): Promise<void> {
    await apiClient.delete(MATERIAL_CATEGORY_API.DELETE(id));
  },

  // Materials
  async findMaterials(query: QueryOptions): Promise<Paginated<MaterialArticle>> {
    return apiClient.post(MATERIAL_API.FIND, query);
  },
  async createMaterial(data: Record<string, unknown>): Promise<MaterialArticle> {
    return apiClient.post(MATERIAL_API.CREATE, data);
  },
  async updateMaterial(id: number, data: Record<string, unknown>): Promise<MaterialArticle> {
    return apiClient.put(MATERIAL_API.UPDATE(id), data);
  },
  async deleteMaterial(id: number): Promise<void> {
    await apiClient.delete(MATERIAL_API.DELETE(id));
  },
};
