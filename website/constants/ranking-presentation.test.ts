import { describe, expect, it } from "vitest";
import {
  DEFAULT_RANKING_ACCENT,
  getPodiumPresentation,
} from "@/constants/ranking-presentation";

describe("ranking presentation", () => {
  it("keeps the default medal indicator aligned with the amber leaderboard", () => {
    expect(DEFAULT_RANKING_ACCENT).toBe("amber");
    expect(getPodiumPresentation(1)?.text).toContain("amber");
    expect(getPodiumPresentation(2)?.text).toContain("slate");
    expect(getPodiumPresentation(3)?.text).toContain("orange");
  });

  it("allows success-oriented leaderboards to use jade first place without changing shared silver", () => {
    expect(getPodiumPresentation(1, "emerald")?.text).toContain("emerald");
    expect(getPodiumPresentation(2, "emerald")).toEqual(
      getPodiumPresentation(2, "amber"),
    );
  });

  it("does not assign podium decoration to unranked positions", () => {
    expect(getPodiumPresentation(0)).toBeUndefined();
    expect(getPodiumPresentation(4)).toBeUndefined();
  });
});
