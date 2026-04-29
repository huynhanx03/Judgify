import { apiClient } from "@/lib/api-client";
import { SUBMISSION_API } from "@/constants/api";
import type { Submission, CreateSubmissionRequest } from "@/types/submission";

export const submissionService = {
  async submit(body: CreateSubmissionRequest): Promise<Submission> {
    return apiClient.post<Submission>(SUBMISSION_API.CREATE, body);
  },
  async getById(id: number): Promise<Submission> {
    return apiClient.get<Submission>(SUBMISSION_API.GET(id));
  },
  async getMyByProblem(problemId: number): Promise<Submission[]> {
    return apiClient.get<Submission[]>(SUBMISSION_API.MY_BY_PROBLEM(problemId));
  },
  async getByProblem(problemId: number): Promise<Submission[]> {
    return apiClient.get<Submission[]>(SUBMISSION_API.BY_PROBLEM(problemId));
  },
};
