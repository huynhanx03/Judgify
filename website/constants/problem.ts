/** Interaction timings shared by the public problem experience. */
export const PROBLEM_UI_POLICY = Object.freeze({
  COPY_FEEDBACK_DURATION_MS: 1_800,
});

/** Closed authoring vocabulary mirrored from the server contract. */
export const PROBLEM_TEST_KIND = Object.freeze({
  SAMPLE: "sample",
  HIDDEN: "hidden",
} as const);

export const PROBLEM_CHECKER_CONFIG_KIND = Object.freeze({
  NONE: "none",
  FLOAT_TOLERANCE: "float_tolerance",
} as const);
