import { expect, test } from "@playwright/test";
import matrix from "@/architecture/browser-matrix.json";
import {
  hasMemberCredentials,
  resolveMatrixRoute,
  type MatrixRoute,
} from "./support/environment";
import { loginMember } from "./support/session";
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

const memberRoutes = (matrix.routes as MatrixRoute[]).filter(
  (entry) => entry.audience === "authenticated",
);

test.describe("authenticated member journeys", () => {
  test.skip(!hasMemberCredentials(), "member browser credentials are not configured");

  for (const entry of memberRoutes) {
    for (const variant of visualAcceptanceVariants) {
      test(`${entry.route} uses the canonical cookie session at ${visualAcceptanceLabel(variant)}`, async ({ page }, testInfo) => {
        await prepareVisualAcceptance(page, variant);
        const diagnostics = captureBrowserDiagnostics(page);
        const sockets = captureWebSocketEvidence(page);
        await loginMember(page);
        const route = resolveMatrixRoute(entry);
        await page.goto(route!, { waitUntil: "domcontentloaded" });
        expect(new URL(page.url()).pathname).toBe(route);
        await assertPageQuality(page, testInfo, diagnostics);
        await assertVisualAcceptance(page, variant);
        await attachWebSocketEvidence(testInfo, sockets);
        await assertAccessibility(page, testInfo);

        const persistedKeys = await page.evaluate(() => [
          ...Object.keys(window.localStorage).map((key) => `local:${key}`),
          ...Object.keys(window.sessionStorage).map((key) => `session:${key}`),
        ]);
        expect(persistedKeys, "credentials must never enter Web Storage").not.toEqual(
          expect.arrayContaining([
            expect.stringMatching(/(?:access|refresh|bearer|credential|jwt|token)/i),
          ]),
        );
      });
    }
  }
});
