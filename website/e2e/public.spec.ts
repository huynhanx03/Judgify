import { expect, test } from "@playwright/test";
import matrix from "@/architecture/browser-matrix.json";
import { TEXT } from "@/constants/text";
import {
  resolveMatrixRoute,
  type MatrixRoute,
} from "./support/environment";
import {
  assertAccessibility,
  assertPageQuality,
  assertVisualAcceptance,
  attachWebSocketEvidence,
  captureBrowserDiagnostics,
  captureWebSocketEvidence,
  prepareVisualAcceptance,
  visualAcceptanceLabel,
  visualAcceptanceVariants,
} from "./support/quality";

const anonymousRoutes = (matrix.routes as MatrixRoute[]).filter((entry) =>
  ["public", "anonymous"].includes(entry.audience),
);

test.describe("public and anonymous route matrix", () => {
  for (const entry of anonymousRoutes) {
    for (const variant of visualAcceptanceVariants) {
      test(`${entry.route} renders a real, accessible product state at ${visualAcceptanceLabel(variant)}`, async ({ page }, testInfo) => {
        const route = resolveMatrixRoute(entry);
        test.skip(!route, `seed fixture for ${entry.route} is not configured`);

        await prepareVisualAcceptance(page, variant);
        const diagnostics = captureBrowserDiagnostics(page);
        const sockets = captureWebSocketEvidence(page);
        await page.goto(route!, { waitUntil: "domcontentloaded" });
        await assertPageQuality(page, testInfo, diagnostics);
        await assertVisualAcceptance(page, variant);
        await attachWebSocketEvidence(testInfo, sockets);
        await assertAccessibility(page, testInfo);
      });
    }
  }

  test("keyboard skip link reaches the primary content", async ({ page }) => {
    await page.goto("/arena");
    await page.keyboard.press("Tab");
    const skipLink = page.getByRole("link", {
      name: TEXT.NAV.SKIP_TO_CONTENT,
    });
    await expect(skipLink).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.locator("#main-content")).toBeFocused();
  });

  test("mobile navigation traps and restores focus", async ({ page }) => {
    await page.setViewportSize(matrix.viewports.mobile);
    await page.goto("/arena");
    const trigger = page.getByRole("button", { name: TEXT.NAV.OPEN_MENU });
    await trigger.click();
    await expect(
      page.getByRole("dialog", { name: TEXT.NAV.MOBILE_MENU_TITLE }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(trigger).toBeFocused();
  });
});
