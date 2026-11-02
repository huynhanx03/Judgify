import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("profile and difficulty projections use abortable independent resources", async () => {
  const [page, users] = await Promise.all([
    source("modules/profile/profile-page.tsx"),
    source("services/user.service.ts"),
  ]);

  assert.match(page, /useRetryableResource<UserProfile \\| null>/);
  assert.match(page, /useRetryableResource<DifficultyResponse\[\]>/);
  assert.match(page, /profileResource\.retry/);
  assert.match(page, /difficultyResource\.retry/);
  assert.doesNotMatch(page, /loadAttempt|difficultyLoadAttempt|let active/);
  assert.match(users, /getProfile\(signal\?: AbortSignal\)/);
  assert.match(users, /getProfile[\s\S]*signal,[\s\S]*schema: profileSchema/);
});

test("a locally saved profile snapshot stays fenced to the active user", async () => {
  const page = await source("modules/profile/profile-page.tsx");

  assert.match(page, /savedProfile\?\.userID === userID/);
  assert.match(page, /setSavedProfile\(\{ userID, profile: updatedProfile \}\)/);
  assert.doesNotMatch(page, /const \\[profile, setProfile\\]/);
});
