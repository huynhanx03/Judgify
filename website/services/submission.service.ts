/**
 * Submission service layer — calls backend submission module endpoints.
 */

import { apiClient } from "@/lib/api-client";
import { SUBMISSION_API, PROBLEM_API } from "@/constants/api";
import type {
  Submission,
  CreateSubmissionRequest,
  TestCaseResponse,
} from "@/types/submission";

/** Submit code for judging. Returns the created submission (status: pending). */
export async function createSubmission(
  body: CreateSubmissionRequest
): Promise<Submission> {
  return apiClient.post<Submission>(SUBMISSION_API.CREATE, body);
}

/** Fetch a single submission by ID. */
export async function getSubmission(id: number): Promise<Submission> {
  return apiClient.get<Submission>(SUBMISSION_API.GET(id));
}

/** Fetch current user's submissions for a specific problem. */
export async function getMySubmissions(
  problemId: number
): Promise<Submission[]> {
  return apiClient.get<Submission[]>(SUBMISSION_API.MY_BY_PROBLEM(problemId));
}

/** Fetch all submissions for a specific problem (admin view). */
export async function getSubmissionsByProblem(
  problemId: number
): Promise<Submission[]> {
  return apiClient.get<Submission[]>(SUBMISSION_API.BY_PROBLEM(problemId));
}

/** Fetch visible (non-hidden) test cases for a problem. */
export async function getTestCases(
  problemId: number
): Promise<TestCaseResponse[]> {
  return apiClient.get<TestCaseResponse[]>(PROBLEM_API.TEST_CASES(problemId));
}
