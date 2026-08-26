import { expect, test } from "@playwright/test";
import matrix from "@/architecture/browser-matrix.json";

test.describe("theme and responsive acceptance", () => {
  test("reduced motion removes nonessential transitions", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "dark" });
    await page.goto("/arena");
    const duration = await page.locator("body").evaluate(() => {
      const probe = document.createElement("div");
      probe.className = "animate-pulse transition-all";
      document.body.append(probe);
      const style = getComputedStyle(probe);
      const result = {
        animationDuration: style.animationDuration,
        transitionDuration: style.transitionDuration,
      };
      probe.remove();
      return result;
    });
    expect(duration.animationDuration).toBe("0.01ms");
    expect(duration.transitionDuration).toBe("0.01ms");
  });

  test("public shell reflows at the 200 percent zoom equivalent", async ({ page }) => {
    await page.setViewportSize({
      width: Math.floor(matrix.viewports.desktop.width / 2),
      height: matrix.viewports.desktop.height,
    });
    await page.goto("/arena");
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
    await expect(page.locator("main")).toBeVisible();
  });
});
