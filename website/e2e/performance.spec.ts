import { expect, test } from "@playwright/test";
import { acceptanceEnvironment } from "./support/environment";

interface BrowserVitals {
  lcp: number;
  cls: number;
  inp: number | null;
  ttfb: number;
}

const PUBLIC_PERFORMANCE_ROUTES = [
  "/arena",
  "/contest",
  "/materials",
  "/ranking",
] as const;

test.describe("public Web Vitals evidence", () => {
  test.describe.configure({ mode: "serial" });

  for (const route of PUBLIC_PERFORMANCE_ROUTES) {
    test(`${route} stays inside the release performance budget`, async ({ page }, testInfo) => {
      await page.addInitScript(() => {
        const state = { lcp: 0, cls: 0, inp: null as number | null };
        Object.defineProperty(window, "__judgifyVitals", {
          configurable: false,
          enumerable: false,
          value: state,
          writable: false,
        });
        new PerformanceObserver((list) => {
          const entries = list.getEntries();
          const last = entries.at(-1);
          if (last) state.lcp = last.startTime;
        }).observe({ type: "largest-contentful-paint", buffered: true });
        new PerformanceObserver((list) => {
          for (const entry of list.getEntries()) {
            const shift = entry as PerformanceEntry & {
              hadRecentInput?: boolean;
              value?: number;
            };
            if (!shift.hadRecentInput) state.cls += shift.value ?? 0;
          }
        }).observe({ type: "layout-shift", buffered: true });
        try {
          new PerformanceObserver((list) => {
            for (const entry of list.getEntries()) {
              const event = entry as PerformanceEntry & {
                duration: number;
                interactionId?: number;
              };
              if (!event.interactionId) continue;
              state.inp = Math.max(state.inp ?? 0, event.duration);
            }
          }).observe({
            type: "event",
            buffered: true,
            durationThreshold: 16,
          } as PerformanceObserverInit & { durationThreshold: number });
        } catch {
          // INP event timing is not available in every browser build.
        }
      });

      await page.goto(route, { waitUntil: "load" });
      await page.getByRole("heading", { level: 1 }).waitFor();
      await page.keyboard.press("Tab");
      await page.waitForTimeout(1_000);

      const vitals = await page.evaluate<BrowserVitals>(() => {
        const state = (window as typeof window & {
          __judgifyVitals: Omit<BrowserVitals, "ttfb">;
        }).__judgifyVitals;
        const navigation = performance.getEntriesByType(
          "navigation",
        )[0] as PerformanceNavigationTiming | undefined;
        return {
          ...state,
          ttfb: navigation
            ? navigation.responseStart - navigation.requestStart
            : 0,
        };
      });

      await testInfo.attach("web-vitals.json", {
        body: JSON.stringify({ route, profile: "local-browser", ...vitals }, null, 2),
        contentType: "application/json",
      });

      if (!acceptanceEnvironment.enforcePerformance) return;
      expect(vitals.lcp, "LCP must be observed").toBeGreaterThan(0);
      expect(vitals.lcp, "LCP p75 budget").toBeLessThanOrEqual(2_500);
      expect(vitals.cls, "CLS budget").toBeLessThanOrEqual(0.1);
      if (vitals.inp !== null) {
        expect(vitals.inp, "INP p75 budget").toBeLessThanOrEqual(200);
      }
    });
  }
});
