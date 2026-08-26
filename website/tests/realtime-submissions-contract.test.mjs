import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("protected profile exposes the current user UUID to the frontend", async () => {
  const [userType, authContext, userService] = await Promise.all([
    source("types/user.ts"),
    source("contexts/auth-context.tsx"),
    source("services/user.service.ts"),
  ]);

  assert.match(userType, /interface ProfileAttrs\s*{[\s\S]*?id:\s*EntityID/);
  assert.match(userType, /type UserProfile = ReadyUserProfile \| UnprovisionedUserProfile/);
  assert.match(authContext, /user:\s*SessionPrincipal \| null/);
  assert.match(
    userService,
    /getProfileAttributes\([\s\S]*?Promise<ProfileAttributesResponse>/,
  );
});

test("problem submissions use the private ticketed topic as an invalidation channel", async () => {
  const [config, provider, hook, page, service, eventTypes] = await Promise.all([
    source("constants/realtime.ts"),
    source("contexts/realtime-context.tsx"),
    source("hooks/use-problem-submissions.ts"),
    source("modules/problem/problem-workspace-page.tsx"),
    source("services/realtime.service.ts"),
    source("types/realtime.ts"),
  ]);

  assert.match(config, /EVENT_SUBMISSION_JUDGED_V1/);
  assert.match(config, /userSubmissionsTopic/);
  assert.match(config, /resolveTicketedRealtimeWebSocketUrl/);
  assert.match(eventTypes, /interface SubmissionJudgedV1/);
  assert.match(provider, /mode:\s*REALTIME_CONNECTION_MODE\.TICKET/);
  assert.match(provider, /realtimeService\.issueTicket\(signal\)/);
  assert.match(hook, /useRealtime\(\)/);
  assert.match(hook, /userSubmissionsTopic\(userId\)/);
  assert.match(hook, /EVENT_SUBMISSION_JUDGED_V1/);
  assert.match(hook, /event\.problem_id !== problemId/);
  assert.match(hook, /submissionService\.getMyByProblem\(/);
  assert.match(hook, /connectionState === "open"[\s\S]*void refresh\(\)/);
  assert.match(page, /useProblemSubmissions/);
  assert.match(page, /user\?\.id/);
  assert.match(page, /isLoading:\s*isLoadingAuth/);
  assert.match(
    page,
    /isSessionLoading=\{isLoadingAuth\}/,
  );
  assert.doesNotMatch(page, /setInterval|TERMINAL_STATUSES/);
  assert.match(service, /retryUnauthorized:\s*false/);
});

test("submission hooks rely on shared realtime and explicit REST reconciliation", async () => {
  const hook = await source("hooks/use-problem-submissions.ts");

  assert.match(hook, /onTopicStateChange/);
  assert.match(hook, /EVENT_SUBMISSION_JUDGED_V1[\s\S]*void refresh\(\)/);
  assert.match(hook, /onResyncRequired[\s\S]*void refresh\(\)/);
  assert.match(hook, /refresh,/);
  assert.doesNotMatch(hook, /setInterval|clearInterval|new WebSocketClient/);
});

test("realtime event and topic wire strings live only in the protocol catalog", async () => {
  const [config, hook, client, eventTypes] = await Promise.all([
    source("constants/realtime.ts"),
    source("hooks/use-problem-submissions.ts"),
    source("lib/realtime/websocket-client.ts"),
    source("types/realtime.ts"),
  ]);

  assert.match(config, /"submission\.judged\.v1"/);
  for (const consumer of [hook, client, eventTypes]) {
    assert.doesNotMatch(consumer, /"submission\.judged\.v1"/);
  }
});
