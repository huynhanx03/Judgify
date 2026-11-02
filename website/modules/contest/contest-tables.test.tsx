import assert from "node:assert/strict";
import { render, screen } from "@testing-library/react";
import { test } from "vitest";

import { TEXT } from "@/constants/text";
import { ContestRatingTable } from "@/modules/contest/contest-rating-table";
import { ContestStandingsTable } from "@/modules/contest/contest-standings-table";
import type { EntityID } from "@/types/api";

const USER_ID = "user-1" as EntityID;

test("contest standings use a labelled, keyboard-reachable table viewport", () => {
  render(
    <ContestStandingsTable
      standings={[
        {
          user_id: USER_ID,
          username: "DaoHuu",
          rank: 1,
          solved_count: 3,
          penalty: 42,
          pending_attempts: 0,
          pending_by_problem: {},
          problem_results: {},
        },
      ]}
    />,
  );

  assert.ok(screen.getByRole("region", { name: TEXT.CONTEST.STANDINGS }));
  assert.ok(screen.getByRole("columnheader", { name: TEXT.CONTEST.RANK }));
  assert.ok(screen.getByRole("cell", { name: "DaoHuu" }));
});

test("contest rating changes use the same labelled viewport contract", () => {
  render(
    <ContestRatingTable
      ratingChanges={[
        { user_id: USER_ID, username: "DaoHuu", rank: 1, old_rating: 1200, new_rating: 1250, delta: 50 },
      ]}
    />,
  );

  assert.ok(screen.getByRole("region", { name: TEXT.CONTEST.RATING_CHANGES }));
  assert.ok(screen.getByRole("columnheader", { name: TEXT.CONTEST.RATING_DELTA }));
  assert.ok(screen.getByRole("cell", { name: "+50" }));
});
