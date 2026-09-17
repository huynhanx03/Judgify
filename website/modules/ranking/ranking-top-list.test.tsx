import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { test, vi } from "vitest";

import { TEXT } from "@/constants/text";
import { RankingTopList } from "@/modules/ranking/ranking-top-list";
import type { EntityID } from "@/types/api";

const baseProps = {
  title: TEXT.RANKING.RATING_SECTION,
  subtitle: TEXT.RANKING.RATING_SECTION_SUBTITLE,
  icon: "crown" as const,
  accentColor: "amber" as const,
  mode: "rating" as const,
  onRetry: vi.fn(),
  hasMore: false,
  onLoadMore: vi.fn(),
};

test("renders initial ranking failures as an unframed, retryable collection state", () => {
  render(
    <RankingTopList
      {...baseProps}
      data={[]}
      status="error"
    />,
  );

  const error = screen.getByRole("alert");
  assert.ok(!error.className.includes("border"));
  fireEvent.click(screen.getByRole("button", { name: TEXT.COMMON.RETRY }));
  assert.equal(baseProps.onRetry.mock.calls.length, 1);
});

test("keeps ranking entries and pagination actions semantic after data loads", () => {
  const onLoadMore = vi.fn();
  const view = render(
    <RankingTopList
      {...baseProps}
      data={[{
        id: "ranking-user-1" as EntityID,
        rank: 1,
        name: "Linh Nhi",
        displayMode: "public",
        realm: "Kim Đan",
        points: 1280,
      }]}
      status="ready"
      hasMore
      onLoadMore={onLoadMore}
    />,
  );

  assert.ok(screen.getByRole("list"));
  assert.ok(screen.getByRole("listitem").textContent?.includes("Linh Nhi"));
  fireEvent.click(screen.getByRole("button", { name: TEXT.RANKING.LOAD_MORE }));
  assert.equal(onLoadMore.mock.calls.length, 1);

  view.rerender(
    <RankingTopList
      {...baseProps}
      data={[{
        id: "ranking-user-1" as EntityID,
        rank: 1,
        name: "Linh Nhi",
        displayMode: "public",
        points: 1280,
      }]}
      status="loading_more"
      hasMore
      onLoadMore={onLoadMore}
    />,
  );
  assert.equal(
    screen.getByRole("heading", { name: TEXT.RANKING.RATING_SECTION }).closest("section")?.getAttribute("aria-busy"),
    "true",
  );
  assert.ok(screen.getByRole("button", { name: TEXT.RANKING.LOADING_MORE }).hasAttribute("disabled"));
});
