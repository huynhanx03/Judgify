import AxeBuilder from "@axe-core/playwright";
import {
  expect,
  type Page,
  type TestInfo,
  type WebSocket,
} from "@playwright/test";
import matrix from "@/architecture/browser-matrix.json";
import { THEME_STORAGE_KEY, type ThemePreference } from "@/design/tokens";

interface BrowserDiagnostics {
  consoleErrors: string[];
  pageErrors: string[];
  serverErrors: Array<{ status: number; url: string }>;
}

export interface WebSocketEvidence {
  opened: number;
  active: number;
  maximumActive: number;
  urls: string[];
}

export interface VisualAcceptanceVariant {
  theme: ThemePreference;
  viewportName: keyof typeof matrix.viewports;
  viewport: { width: number; height: number };
  colorScheme: "light" | "dark";
}

/** Canonical Cartesian product used by every route-matrix acceptance suite. */
export const visualAcceptanceVariants: readonly VisualAcceptanceVariant[] =
  (matrix.themes as ThemePreference[]).flatMap((theme) =>
    Object.entries(matrix.viewports).map(([viewportName, viewport]) => ({
      theme,
      viewportName: viewportName as keyof typeof matrix.viewports,
      viewport,
      colorScheme: theme === "light" ? "light" : "dark",
    })),
  );

export function visualAcceptanceLabel(
  variant: VisualAcceptanceVariant,
): string {
  return `${variant.theme}/${variant.viewportName}/${variant.viewport.width}px`;
}

export async function prepareVisualAcceptance(
  page: Page,
  variant: VisualAcceptanceVariant,
): Promise<void> {
  await page.setViewportSize(variant.viewport);
  await page.emulateMedia({
    colorScheme: variant.colorScheme,
    reducedMotion: "no-preference",
  });
  await installTheme(page, variant.theme);
}

export async function assertVisualAcceptance(
  page: Page,
  variant: VisualAcceptanceVariant,
): Promise<void> {
  await expect(page.locator("html")).toHaveClass(
    variant.colorScheme === "dark" ? /\bdark\b/ : /^(?!.*\bdark\b)/,
  );
}

// The session cookie is HttpOnly, so an anonymous page load cannot know whether
// it is signed in without asking the server. That probe answers 401, and the
// browser always writes a resource-load error to the console for it. Treating
// that one expected line as a defect would fail every anonymous route forever,
// so it is excluded while every other console error still fails the run.
const EXPECTED_ANONYMOUS_PROBE_ERROR =
  /Failed to load resource: the server responded with a status of 401/;

export function captureBrowserDiagnostics(page: Page): BrowserDiagnostics {
  const diagnostics: BrowserDiagnostics = {
    consoleErrors: [],
    pageErrors: [],
    serverErrors: [],
  };
  page.on("console", (message) => {
    if (message.type() !== "error") return;
    const text = message.text();
    if (EXPECTED_ANONYMOUS_PROBE_ERROR.test(text)) return;
    diagnostics.consoleErrors.push(text);
  });
  page.on("pageerror", (error) => diagnostics.pageErrors.push(error.message));
  page.on("response", (response) => {
    const url = new URL(response.url());
    if (url.pathname.startsWith("/api/") && response.status() >= 500) {
      diagnostics.serverErrors.push({ status: response.status(), url: url.pathname });
    }
  });
  return diagnostics;
}

export function captureWebSocketEvidence(page: Page): WebSocketEvidence {
  const evidence: WebSocketEvidence = {
    opened: 0,
    active: 0,
    maximumActive: 0,
    urls: [],
  };
  page.on("websocket", (socket: WebSocket) => {
    evidence.opened += 1;
    evidence.active += 1;
    evidence.maximumActive = Math.max(evidence.maximumActive, evidence.active);
    evidence.urls.push(new URL(socket.url()).pathname);
    socket.on("close", () => {
      evidence.active = Math.max(0, evidence.active - 1);
    });
  });
  return evidence;
}

export async function installTheme(
  page: Page,
  preference: ThemePreference,
): Promise<void> {
  await page.addInitScript(
    ({ key, value }) => window.localStorage.setItem(key, value),
    { key: THEME_STORAGE_KEY, value: preference },
  );
}

export async function assertPageQuality(
  page: Page,
  testInfo: TestInfo,
  diagnostics: BrowserDiagnostics,
): Promise<void> {
  await expect(page.locator("main")).toHaveCount(1);
  await expect(page.locator("main")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

  const geometry = await page.evaluate(() => ({
    documentWidth: document.documentElement.scrollWidth,
    viewportWidth: document.documentElement.clientWidth,
  }));
  expect(
    geometry.documentWidth,
    `document overflows viewport by ${geometry.documentWidth - geometry.viewportWidth}px`,
  ).toBeLessThanOrEqual(geometry.viewportWidth + 1);

  await testInfo.attach("browser-diagnostics.json", {
    body: JSON.stringify(diagnostics, null, 2),
    contentType: "application/json",
  });
  expect(diagnostics.pageErrors, "uncaught browser errors").toEqual([]);
  expect(diagnostics.consoleErrors, "browser console errors").toEqual([]);
  expect(diagnostics.serverErrors, "API 5xx responses").toEqual([]);
}

export async function assertAccessibility(
  page: Page,
  testInfo: TestInfo,
): Promise<void> {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  await testInfo.attach("axe-results.json", {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  });
  expect(results.violations).toEqual([]);
}

export async function attachWebSocketEvidence(
  testInfo: TestInfo,
  evidence: WebSocketEvidence,
): Promise<void> {
  await testInfo.attach("websocket-evidence.json", {
    body: JSON.stringify(evidence, null, 2),
    contentType: "application/json",
  });
  expect(
    evidence.maximumActive,
    "a browser tab must never own concurrent WebSocket connections",
  ).toBeLessThanOrEqual(1);
}
