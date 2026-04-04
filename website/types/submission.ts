/**
 * Submission-related types mirroring backend submission DTOs.
 * Backend status values: pending, judging, accepted, wrong_answer,
 * time_limit_exceeded, memory_limit_exceeded, runtime_error, compile_error.
 */

/** Verdict/status of a submission — matches backend enum values. */
export type SubmissionStatus =
  | "pending"
  | "judging"
  | "accepted"
  | "wrong_answer"
  | "time_limit_exceeded"
  | "memory_limit_exceeded"
  | "runtime_error"
  | "compile_error";

/** Available programming languages for submissions. */
export type Language = "cpp" | "java" | "python" | "go";

/**
 * Submission response from backend.
 * Fields time_ms and memory_kb are nullable (null while pending/judging).
 * Field error_message is nullable (only present on errors).
 */
export interface Submission {
  id: number;
  problem_id: number;
  user_id: number;
  language: Language;
  source_code: string;
  status: SubmissionStatus;
  passed_count: number;
  total_count: number;
  time_ms: number | null;
  memory_kb: number | null;
  error_message: string | null;
  created_at: string;
}

/** Request body for creating a submission. */
export interface CreateSubmissionRequest {
  problem_id: number;
  language: Language;
  source_code: string;
}

/** Test case sample shown in problem description. */
export interface TestCase {
  input: string;
  output: string;
  explanation?: string;
}

/**
 * Test case response from backend GET /problems/:id/test-cases.
 * Maps expected_output -> output for display compatibility.
 */
export interface TestCaseResponse {
  id: number;
  problem_id: number;
  input: string;
  expected_output: string;
  is_hidden: boolean;
  order_index: number;
}
