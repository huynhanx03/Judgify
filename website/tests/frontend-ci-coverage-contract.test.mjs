import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

test("the CI unit target enforces the configured coverage thresholds", async () => {
  const [script, packageJson] = await Promise.all([
    readFile(
      path.resolve(process.cwd(), "../scripts/verify/frontend.sh"),
      "utf8",
    ),
    readFile(path.resolve(process.cwd(), "package.json"), "utf8").then(JSON.parse),
  ]);

  assert.match(script, /verify_unit\(\)\s*\{\s*npm run test:coverage\s*\}/);
  assert.doesNotMatch(script, /verify_unit\(\)\s*\{\s*npm run test\s*\}/);
  assert.match(packageJson.scripts.verify, /npm run test:coverage/);
});
