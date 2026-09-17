import { expect, test } from "@playwright/test";
import matrix from "@/architecture/browser-matrix.json";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import {
  acceptanceEnvironment,
  hasAdminCredentials,
  resolveMatrixRoute,
  type MatrixRoute,
} from "./support/environment";
import { loginAdmin } from "./support/session";
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

const adminRoutes = (matrix.routes as MatrixRoute[]).filter(
  (entry) => entry.audience === "admin",
);

test.describe("administrator route matrix", () => {
  test.skip(!hasAdminCredentials(), "administrator browser credentials are not configured");

  for (const entry of adminRoutes) {
    for (const variant of visualAcceptanceVariants) {
      test(`${entry.route} is capability-aware and accessible at ${visualAcceptanceLabel(variant)}`, async ({ page }, testInfo) => {
        const route = resolveMatrixRoute(entry);
        test.skip(!route, `seed fixture for ${entry.route} is not configured`);
        await prepareVisualAcceptance(page, variant);
        const diagnostics = captureBrowserDiagnostics(page);
        const sockets = captureWebSocketEvidence(page);
        await loginAdmin(page);
        await page.goto(route!, { waitUntil: "domcontentloaded" });
        expect(new URL(page.url()).pathname).toBe(route);
        await assertPageQuality(page, testInfo, diagnostics);
        await assertVisualAcceptance(page, variant);
        await expect(
          page.getByRole("heading", {
            level: 1,
            name: ADMIN_TEXT.ACCESS.FORBIDDEN_TITLE,
          }),
        ).toHaveCount(0);
        await expect(
          page.getByRole("heading", {
            level: 1,
            name: ADMIN_TEXT.ACCESS.UNAVAILABLE_TITLE,
          }),
        ).toHaveCount(0);
        await attachWebSocketEvidence(testInfo, sockets);
        await assertAccessibility(page, testInfo);
      });
    }
  }

  test("mobile administrator navigation restores focus", async ({ page }) => {
    await page.setViewportSize(matrix.viewports.mobile);
    await loginAdmin(page);
    const trigger = page.getByRole("button", { name: TEXT.NAV.OPEN_SIDEBAR });
    await trigger.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(trigger).toBeFocused();
  });
});

test.describe("administrator policy mutation", () => {
  const fixture = acceptanceEnvironment.fixtures;
  const policyFixtureReady = Boolean(
    hasAdminCredentials() &&
      fixture.policyRoleKey &&
      fixture.policyResource &&
      fixture.policyAction,
  );
  test.skip(!policyFixtureReady, "disposable policy fixture is not configured");

  test("changes and restores one permission through the revisioned matrix", async ({ page }) => {
    await loginAdmin(page);
    await page.goto("/admin/roles");

    const roleButton = page
      .getByRole("button")
      .filter({ hasText: fixture.policyRoleKey! })
      .first();
    await roleButton.click();

    const resourceRow = page
      .locator("tbody tr")
      .filter({ hasText: fixture.policyResource! })
      .first();
    const actionLabel = ADMIN_TEXT.PERMISSIONS.ACTION_LABEL(
      fixture.policyAction!,
    );
    const permission = resourceRow
      .getByRole("button")
      .filter({ hasText: actionLabel })
      .first();
    await expect(permission).toBeEnabled();
    const initial = await permission.getAttribute("aria-pressed");

    async function save(reason: string) {
      await page.getByLabel(ADMIN_TEXT.ROLES_POLICY_REASON_LABEL).fill(reason);
      await page.getByRole("button", { name: ADMIN_TEXT.ROLES_SAVE_PERMS }).click();
      await expect(page.getByText(ADMIN_TEXT.ROLES_PERMS_SUCCESS).last()).toBeVisible();
    }

    let mutationAttempted = false;
    try {
      mutationAttempted = true;
      await permission.click();
      await save("Kiểm thử E2E thay đổi quyền có đối soát revision.");
      await expect(permission).toHaveAttribute(
        "aria-pressed",
        initial === "true" ? "false" : "true",
      );
    } finally {
      if (mutationAttempted) {
        await page.reload();
        await roleButton.click();
        const restoredResourceRow = page
          .locator("tbody tr")
          .filter({ hasText: fixture.policyResource! })
          .first();
        const restoredPermission = restoredResourceRow
          .getByRole("button")
          .filter({ hasText: actionLabel })
          .first();
        await expect(restoredPermission).toBeEnabled();
        if ((await restoredPermission.getAttribute("aria-pressed")) !== initial) {
          await restoredPermission.click();
          await save("Khôi phục trạng thái quyền sau kiểm thử E2E.");
        }
        await expect(restoredPermission).toHaveAttribute(
          "aria-pressed",
          initial ?? "false",
        );
      }
    }
  });
});
