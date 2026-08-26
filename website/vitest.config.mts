import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

const root = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  resolve: {
    alias: {
      "@": root,
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./test/setup.ts"],
    include: ["**/*.test.{ts,tsx}"],
    exclude: ["node_modules/**", ".next/**", "e2e/**"],
    clearMocks: true,
    restoreMocks: true,
    passWithNoTests: false,
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary", "lcov"],
      reportsDirectory: "./coverage",
      include: [
        "contexts/session-context.tsx",
        "design/tokens.ts",
        "i18n/text.ts",
        "lib/api/{client,contracts,csrf,error,schema}.ts",
        "lib/audit/audit-schema.ts",
        "lib/auth/{profile-schema,refresh-coordinator,safe-navigation}.ts",
        "lib/cultivation/{catalog-schema,trait-schema}.ts",
      ],
      thresholds: {
        branches: 75,
        functions: 80,
        lines: 80,
        statements: 80,
      },
    },
  },
});
