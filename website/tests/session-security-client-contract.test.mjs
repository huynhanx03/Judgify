import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("session API constants expose the protected account security contract", async () => {
  const api = await source("constants/api/auth.ts");
  assert.match(api, /SESSION_ME:\s*["']\/session\/me["']/);
  assert.match(api, /SESSIONS:\s*["']\/auth\/sessions["']/);
  assert.match(api, /SESSION:\s*\(id:\s*EntityID\).*\/auth\/sessions/);
  assert.match(api, /LOGOUT_ALL_SESSIONS:\s*["']\/auth\/logout-all["']/);
});

test("session service lists, revokes, and logs out all sessions with stable idempotency", async () => {
  const service = await source("services/session.service.ts");
  assert.match(service, /normalizeLimit/);
  assert.match(service, /MAX_SESSION_CURSOR_LENGTH/);
  assert.match(service, /entityIDSchema\.parse/);
  assert.match(service, /AUTH_API\.SESSION_ME/);
  assert.match(service, /authSessionListSchema/);
  assert.match(service, /sessionMutationSchema/);
  assert.match(service, /commandAttemptStore\.getOrCreate/);
  assert.match(service, /idempotencyKey:\s*attempt\.attempt_id|idempotencyKey,/);
  assert.doesNotMatch(service, /localStorage|sessionStorage|document\.cookie/);
});

test("profile renders a real, accessible session security surface", async () => {
  const [profile, component, text, types] = await Promise.all([
    source("modules/profile/profile-page.tsx"),
    source("modules/profile/session-security-card.tsx"),
    source("i18n/catalog.vi.ts"),
    source("types/session.ts"),
  ]);

  assert.match(profile, /SessionSecurityCard/);
  assert.match(component, /sessionService\s*\.\s*list/);
  assert.match(component, /sessionService\s*\.\s*revoke/);
  assert.match(component, /sessionService\s*\.\s*logoutAll/);
  assert.match(component, /aria-live=/);
  assert.match(component, /role=["']alert["']/);
  assert.match(component, /motion-reduce/);
  assert.match(component, /TEXT\.PROFILE\.SESSIONS/);
  assert.match(component, /useSession/);
  assert.match(component, /revalidate/);
  assert.match(component, /APP_ROUTES\.LOGIN/);
  assert.match(component, /sessionService/);
  assert.doesNotMatch(component, /fetch\s*\(/);
  assert.doesNotMatch(component, /navigator\.clipboard|console\./);
  const service = await source("services/session.service.ts");
  assert.match(service, /commandAttemptStore\.getOrCreate/);
  assert.match(service, /commandAttemptStore\.resolve/);
  assert.doesNotMatch(service, /localStorage|sessionStorage|document\.cookie/);
  assert.match(types, /user_agent\?:\s*string/);
  assert.doesNotMatch(component, /localStorage|sessionStorage|document\.cookie|ip_hash|family_id/);
  assert.match(text, /SESSIONS:\s*\{/);
});

test("current-session revoke copy does not imply logout from every device", async () => {
  const [component, text] = await Promise.all([
    source("modules/profile/session-security-card.tsx"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(text, /CURRENT_REVOKE_DESCRIPTION:/);
  assert.match(
    text,
    /CURRENT_REVOKE_DESCRIPTION:\s*[\s\S]*?Các thiết bị khác vẫn duy trì phiên đăng nhập\./,
  );
  const firstDialogStart = component.indexOf("<ConfirmDialog");
  const secondDialogStart = component.indexOf("<ConfirmDialog", firstDialogStart + 1);
  assert.notEqual(firstDialogStart, -1);
  assert.notEqual(secondDialogStart, -1);
  const revokeDialog = component.slice(firstDialogStart, secondDialogStart);
  assert.match(revokeDialog, /selectedSession\?\.current[\s\S]*?CURRENT_REVOKE_DESCRIPTION/);
  assert.doesNotMatch(revokeDialog, /LOGOUT_ALL_DESCRIPTION/);
});

test("session pagination keeps a previous action available when a page load fails", async () => {
  const component = await source("modules/profile/session-security-card.tsx");
  const errorStart = component.indexOf('status === "error"');
  const emptyStart = component.indexOf("page.items.length === 0", errorStart);
  assert.notEqual(errorStart, -1);
  assert.notEqual(emptyStart, -1);
  const errorBranch = component.slice(errorStart, emptyStart);

  assert.match(errorBranch, /hasPrevious/);
  assert.match(errorBranch, /TEXT\.PROFILE\.SESSIONS\.PREVIOUS/);
  assert.match(errorBranch, /ChevronLeft/);
});

test("session pagination controls stay named when responsive text is hidden", async () => {
  const component = await source("modules/profile/session-security-card.tsx");
  const navigationStart = component.indexOf(
    'aria-label={TEXT.PROFILE.SESSIONS.PAGE_LABEL}',
  );
  const dialogsStart = component.indexOf("<ConfirmDialog", navigationStart);
  assert.notEqual(navigationStart, -1);
  assert.notEqual(dialogsStart, -1);
  const pagination = component.slice(navigationStart, dialogsStart);

  assert.match(
    pagination,
    /aria-label=\{TEXT\.PROFILE\.SESSIONS\.PREVIOUS\}/,
  );
  assert.match(
    pagination,
    /aria-label=\{TEXT\.PROFILE\.SESSIONS\.NEXT\}/,
  );
});
