import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  entityIDSchema,
  isoDateTimeSchema,
  paginationMetaSchema,
} from "@/lib/api/contracts";
import { JUDGE_CONTENT_LIMITS } from "@/constants/submission";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Paginated } from "@/types/api";
import type { Problem } from "@/types/problem";
import type { SampleTestCaseResponse } from "@/types/submission";
import type { Tag } from "@/types/tag";

const MAXIMUM_PUBLIC_SAMPLES = 100;
const MAXIMUM_PUBLIC_PROBLEM_PAGE = 100;
const MAXIMUM_TAGS = 64;
const MAXIMUM_TAG_ELEMENTS = 16;
const MAXIMUM_DESCRIPTION_CHARACTERS = 2 * 1024 * 1024;

const boundedPercentageSchema: Schema<number> = {
  parse(value: unknown, path = "$"): number {
    if (
      typeof value !== "number" ||
      !Number.isFinite(value) ||
      value < 0 ||
      value > 100
    ) {
      throw new TypeError(`${path}: invalid percentage`);
    }
    return value;
  },
};

function utf8StringSchema(maximumBytes: number): Schema<string> {
  const boundedCharacters = stringSchema({
    maximumLength: maximumBytes,
  });
  return {
    parse(value: unknown, path = "$"): string {
      const result = boundedCharacters.parse(value, path);
      if (new TextEncoder().encode(result).byteLength > maximumBytes) {
        throw new TypeError(`${path}: UTF-8 content exceeds byte boundary`);
      }
      return result;
    },
  };
}

const tagElementSchema = strictObjectSchema({
  id: entityIDSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 100 }),
  code: stringSchema({ minimumLength: 1, maximumLength: 64 }),
});

export const tagSchema = strictObjectSchema({
  id: entityIDSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 50 }),
  elements: arraySchema(tagElementSchema, {
    maximumLength: MAXIMUM_TAG_ELEMENTS,
  }),
  version: integerSchema({ minimum: 1, label: "tag version" }),
}) as Schema<Tag>;

export const tagListSchema = arraySchema(tagSchema, {
  maximumLength: 1_000,
});

export const difficultySchema = strictObjectSchema({
  id: entityIDSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 50 }),
  level: integerSchema({
    minimum: 1,
    maximum: 100,
    label: "difficulty level",
  }),
  exp_reward: integerSchema({
    minimum: 0,
    label: "difficulty experience reward",
  }),
  description: optionalSchema(
    stringSchema({ maximumLength: 255 }),
  ),
  version: integerSchema({ minimum: 1, label: "difficulty version" }),
}) as Schema<DifficultyResponse>;

export const difficultyListSchema = arraySchema(difficultySchema, {
  maximumLength: 100,
});

export const difficultyPageSchema: Schema<Paginated<DifficultyResponse>> =
  strictObjectSchema({
    records: arraySchema(difficultySchema, { maximumLength: 100 }),
    pagination: paginationMetaSchema,
  });

export const tagPageSchema: Schema<Paginated<Tag>> = strictObjectSchema({
  records: arraySchema(tagSchema, { maximumLength: 1_000 }),
  pagination: paginationMetaSchema,
});

const rawProblemSchema = strictObjectSchema({
  id: entityIDSchema,
  slug: stringSchema({
    minimumLength: 1,
    maximumLength: 128,
    pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
    label: "problem slug",
  }),
  lifecycle: enumSchema(["draft", "published", "archived"] as const),
  version: integerSchema({ minimum: 1, label: "problem version" }),
  title: stringSchema({ minimumLength: 1, maximumLength: 300 }),
  description: stringSchema({
    minimumLength: 1,
    maximumLength: MAXIMUM_DESCRIPTION_CHARACTERS,
  }),
  difficulty_id: entityIDSchema,
  difficulty: optionalSchema(difficultySchema),
  time_limit_ms: integerSchema({
    minimum: 1,
    maximum: 120_000,
    label: "problem time limit",
  }),
  memory_limit_kb: integerSchema({
    minimum: 1,
    maximum: 16 * 1024 * 1024,
    label: "problem memory limit",
  }),
  author_id: entityIDSchema,
  is_published: booleanSchema,
  submission_count: integerSchema({
    minimum: 0,
    label: "problem submission count",
  }),
  accepted_count: integerSchema({
    minimum: 0,
    label: "problem accepted count",
  }),
  acceptance_rate: boundedPercentageSchema,
  is_solved: optionalSchema(booleanSchema),
  tags: optionalSchema(
    arraySchema(tagSchema, { maximumLength: MAXIMUM_TAGS }),
  ),
  created_at: isoDateTimeSchema,
  updated_at: isoDateTimeSchema,
});

export const problemSchema: Schema<Problem> = {
  parse(value: unknown, path = "$"): Problem {
    const problem = rawProblemSchema.parse(value, path);
    if (problem.accepted_count > problem.submission_count) {
      throw new TypeError(
        `${path}: accepted count exceeds submission count`,
      );
    }
    const expectedRate =
      problem.submission_count === 0
        ? 0
        : (problem.accepted_count / problem.submission_count) * 100;
    if (Math.abs(problem.acceptance_rate - expectedRate) > 0.01) {
      throw new TypeError(`${path}: inconsistent acceptance rate`);
    }
    return {
      ...problem,
      tags: problem.tags ?? null,
    };
  },
};

export const problemPageSchema: Schema<Paginated<Problem>> =
  strictObjectSchema({
    records: arraySchema(problemSchema, {
      maximumLength: MAXIMUM_PUBLIC_PROBLEM_PAGE,
    }),
    pagination: paginationMetaSchema,
  });

const sampleTestCaseSchema = strictObjectSchema({
  input: utf8StringSchema(JUDGE_CONTENT_LIMITS.MAXIMUM_TEST_INPUT_BYTES),
  expected_output: utf8StringSchema(
    JUDGE_CONTENT_LIMITS.MAXIMUM_EXPECTED_OUTPUT_BYTES,
  ),
  order_index: integerSchema({
    minimum: 0,
    label: "sample order",
  }),
}) as Schema<SampleTestCaseResponse>;

export const sampleTestCaseListSchema: Schema<
  SampleTestCaseResponse[]
> = {
  parse(value: unknown, path = "$"): SampleTestCaseResponse[] {
    const samples = arraySchema(sampleTestCaseSchema, {
      maximumLength: MAXIMUM_PUBLIC_SAMPLES,
    }).parse(value, path);
    const orderIndexes = new Set(
      samples.map((sample) => sample.order_index),
    );
    if (orderIndexes.size !== samples.length) {
      throw new TypeError(`${path}: duplicate sample order`);
    }
    return samples;
  },
};
