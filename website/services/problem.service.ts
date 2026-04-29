import { apiClient } from "@/lib/api-client";
import { PROBLEM_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { Problem } from "@/types/problem";
import type { TestCaseResponse } from "@/types/submission";

export const problemService = {
  // Read
  async find(query?: QueryOptions): Promise<Paginated<Problem>> {
    return apiClient.post<Paginated<Problem>>(PROBLEM_API.FIND, query);
  },
  async getById(id: number): Promise<Problem> {
    return apiClient.get<Problem>(PROBLEM_API.GET(id));
  },

  // CRUD
  async create(data: {
    title: string;
    description: string;
    difficulty_id: number;
    time_limit_ms?: number;
    memory_limit_kb?: number;
    tag_ids?: number[];
  }): Promise<Problem> {
    return apiClient.post<Problem>(PROBLEM_API.CREATE, data);
  },
  async update(
    id: number,
    data: {
      title?: string;
      description?: string;
      difficulty_id?: number;
      time_limit_ms?: number;
      memory_limit_kb?: number;
      is_published?: boolean;
      tag_ids?: number[];
    }
  ): Promise<Problem> {
    return apiClient.put<Problem>(PROBLEM_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(PROBLEM_API.DELETE(id));
  },

  // Test Cases
  async getTestCases(problemId: number): Promise<TestCaseResponse[]> {
    return apiClient.get<TestCaseResponse[]>(PROBLEM_API.TEST_CASES(problemId));
  },
  async createTestCase(
    problemId: number,
    data: {
      input: string;
      expected_output: string;
      is_hidden?: boolean;
      order_index?: number;
    }
  ): Promise<TestCaseResponse> {
    return apiClient.post<TestCaseResponse>(
      PROBLEM_API.TEST_CASES(problemId),
      data
    );
  },
  async updateTestCase(
    id: number,
    data: {
      input?: string;
      expected_output?: string;
      is_hidden?: boolean;
      order_index?: number;
    }
  ): Promise<TestCaseResponse> {
    return apiClient.put<TestCaseResponse>(
      PROBLEM_API.TEST_CASE_UPDATE(id),
      data
    );
  },
  async deleteTestCase(id: number): Promise<void> {
    await apiClient.delete(PROBLEM_API.TEST_CASE_DELETE(id));
  },
};
