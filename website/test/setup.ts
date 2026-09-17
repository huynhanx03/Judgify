import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Vitest globals are disabled, so register cleanup explicitly. This prevents
// portals, timers, and DOM state from one component test leaking into another.
afterEach(() => cleanup());
