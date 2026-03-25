/**
 * Mock submission history and test cases for UI development.
 */

import type { Submission, TestCase } from "@/types/submission";

/** Mock test cases for problem #1 (Hai Tổng Linh Thạch). */
export const MOCK_TEST_CASES: TestCase[] = [
  {
    input: "4\n2 7 11 15\n9",
    output: "0 1",
    explanation: "Vì nums[0] + nums[1] = 2 + 7 = 9, trả về [0, 1].",
  },
  {
    input: "3\n3 2 4\n6",
    output: "1 2",
    explanation: "nums[1] + nums[2] = 2 + 4 = 6.",
  },
  {
    input: "2\n3 3\n6",
    output: "0 1",
  },
];

/** Mock submission history for current user on problem #1. */
export const MOCK_SUBMISSIONS: Submission[] = [
  {
    id: 1001,
    problem_id: 1,
    language: "cpp",
    verdict: "accepted",
    time_ms: 12,
    memory_kb: 5120,
    created_at: "2025-03-15T14:32:00Z",
  },
  {
    id: 1000,
    problem_id: 1,
    language: "cpp",
    verdict: "wrong_answer",
    time_ms: 8,
    memory_kb: 4980,
    created_at: "2025-03-15T14:25:00Z",
  },
  {
    id: 999,
    problem_id: 1,
    language: "python",
    verdict: "time_limit_exceeded",
    time_ms: 1050,
    memory_kb: 32000,
    created_at: "2025-03-14T10:10:00Z",
  },
  {
    id: 998,
    problem_id: 1,
    language: "python",
    verdict: "runtime_error",
    time_ms: 0,
    memory_kb: 0,
    created_at: "2025-03-14T09:55:00Z",
  },
  {
    id: 997,
    problem_id: 1,
    language: "java",
    verdict: "compilation_error",
    time_ms: 0,
    memory_kb: 0,
    created_at: "2025-03-13T16:20:00Z",
  },
];

/** Mock detailed problem description with markdown for problem #1. */
export const MOCK_PROBLEM_DESCRIPTION = `Cho một mảng số nguyên \`nums\` và một số nguyên \`target\`, hãy trả về **chỉ số của hai phần tử** sao cho tổng của chúng bằng \`target\`.

Bạn có thể giả sử rằng mỗi đầu vào chỉ có **đúng một lời giải**, và bạn không được sử dụng cùng một phần tử hai lần.

Bạn có thể trả về kết quả theo bất kỳ thứ tự nào.

### Định dạng đầu vào
- Dòng đầu tiên: số nguyên \`n\` — kích thước mảng (2 ≤ n ≤ 10⁴)
- Dòng thứ hai: \`n\` số nguyên \`nums[i]\` (-10⁹ ≤ nums[i] ≤ 10⁹)
- Dòng thứ ba: số nguyên \`target\` (-10⁹ ≤ target ≤ 10⁹)

### Định dạng đầu ra
- Một dòng chứa hai chỉ số cách nhau bởi dấu cách`;
