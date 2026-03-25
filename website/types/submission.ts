/**
 * Submission-related types for problem solutions.
 */

/** Verdict status of a submission. */
export type Verdict =
  | "accepted"
  | "wrong_answer"
  | "time_limit_exceeded"
  | "memory_limit_exceeded"
  | "runtime_error"
  | "compilation_error"
  | "pending";

/** Available programming languages for submissions. */
export type Language = "cpp" | "java" | "python" | "go";

/** A single submission record. */
export interface Submission {
  id: number;
  problem_id: number;
  language: Language;
  verdict: Verdict;
  /** Execution time in milliseconds. */
  time_ms: number;
  /** Memory usage in KB. */
  memory_kb: number;
  created_at: string;
}

/** Test case sample shown in problem description. */
export interface TestCase {
  input: string;
  output: string;
  explanation?: string;
}
