import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

function executeCommonJs(code, dependencies = {}) {
  const compiled = ts.transpileModule(code, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  const require = (specifier) => {
    if (Object.hasOwn(dependencies, specifier)) return dependencies[specifier];
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

test("recovery-contact time model treats server timestamps as authoritative", async () => {
  const model = executeCommonJs(
    await source("modules/profile/recovery-contact-model.ts"),
  );
  const now = Date.parse("2026-07-22T10:00:00.000Z");

  assert.equal(
    model.isRecentReauthenticationActive("2026-07-22T10:00:01.000Z", now),
    true,
  );
  assert.equal(
    model.isRecentReauthenticationActive("2026-07-22T10:00:00.000Z", now),
    false,
  );
  assert.equal(model.isRecentReauthenticationActive("not-a-date", now), false);
  assert.equal(
    model.secondsUntil("2026-07-22T10:02:01.001Z", now),
    122,
  );
  assert.equal(model.formatCountdown(122), "02:02");
  assert.equal(model.formatCountdown(0), "00:00");
});

test("recovery-contact errors map transport status to safe UI decisions", async () => {
  const model = executeCommonJs(
    await source("modules/profile/recovery-contact-model.ts"),
  );

  assert.equal(
    model.classifyRecoveryContactError({ status: 403 }),
    "reauthentication_required",
  );
  assert.equal(model.classifyRecoveryContactError({ status: 409 }), "conflict");
  assert.equal(
    model.classifyRecoveryContactError({ status: 429 }),
    "rate_limited",
  );
  assert.equal(model.classifyRecoveryContactError(new Error("offline")), "unknown");
});

test("authenticated recovery-contact API contract is centralized and versioned", async () => {
  const [api, types, service] = await Promise.all([
    source("constants/api/auth.ts"),
    source("types/auth.ts"),
    source("services/auth.service.ts"),
  ]);

  for (const endpoint of [
    ["RECOVERY_CONTACT", "/auth/recovery-contact"],
    ["REAUTHENTICATE", "/auth/reauthenticate"],
    ["RECOVERY_CONTACT_RESEND", "/auth/recovery-contact/resend"],
    ["RECOVERY_CONTACT_PENDING", "/auth/recovery-contact/pending"],
  ]) {
    assert.match(api, new RegExp(`${endpoint[0]}:\\s*["']${endpoint[1]}["']`));
  }

  for (const field of [
    "can_remove",
    "masked_address",
    "version",
    "verification_expires_at",
    "resend_available_at",
    "delivery_state",
    "reauthenticated_until",
    "expected_current_version",
    "expected_pending_version",
    "current_password",
  ]) {
    assert.match(types, new RegExp(`${field}[?:]`));
  }

  for (const method of [
    "getRecoveryContact",
    "reauthenticateRecoveryContact",
    "replaceRecoveryContact",
    "resendRecoveryContactVerification",
    "cancelPendingRecoveryContact",
    "removeRecoveryContact",
  ]) {
    assert.match(service, new RegExp(`${method}\\(`));
  }
  assert.ok(
    (service.match(/runRecoverableIssuance/g) ?? []).length >= 6,
    "every security mutation, including existing auth mutations, keeps one stable key",
  );
  assert.match(service, /recoveryContactResponseSchema/);
  assert.match(service, /AUTH_API\.RECOVERY_CONTACT_PENDING/);
  assert.match(service, /expected_pending_version/);
  assert.match(service, /expected_current_version/);
});

test("DELETE bodies and step-up 401 policy stay explicit at the central transport", async () => {
  const [client, service] = await Promise.all([
    source("lib/api/client.ts"),
    source("services/auth.service.ts"),
  ]);
  assert.match(client, /SAFE_METHODS\.has\(method\)\s*&&\s*requestOptions\.body\s*!==\s*undefined/);
  assert.match(client, /const serializedBody\s*=[\s\S]*JSON\.stringify\(requestOptions\.body\)/);
  const cancelStart = service.indexOf("cancelPendingRecoveryContact(");
  const removalStart = service.indexOf("removeRecoveryContact(", cancelStart);
  const logoutStart = service.indexOf("async logout(", removalStart);
  assert.notEqual(cancelStart, -1);
  assert.notEqual(removalStart, -1);
  assert.notEqual(logoutStart, -1);
  const cancellation = service.slice(cancelStart, removalStart);
  const removal = service.slice(removalStart, logoutStart);
  assert.match(cancellation, /method:\s*["']DELETE["']/);
  assert.match(cancellation, /body:\s*\{ expected_pending_version: expectedPendingVersion \}/);
  assert.match(cancellation, /schema:\s*recoveryContactResponseSchema/);
  assert.match(removal, /method:\s*["']DELETE["']/);
  assert.match(removal, /body:\s*\{ expected_current_version: expectedCurrentVersion \}/);
  assert.match(removal, /schema:\s*recoveryContactResponseSchema/);
  const stepUpStart = service.indexOf("reauthenticateRecoveryContact(");
  const replaceStart = service.indexOf("replaceRecoveryContact(", stepUpStart);
  const stepUp = service.slice(stepUpStart, replaceStart);
  assert.match(stepUp, /retryUnauthorized:\s*false/);
  assert.match(stepUp, /reauthenticateRecoveryContactResponseSchema/);
  assert.doesNotMatch(stepUp, /invalidate|clear|logout/);
});

test("profile recovery card is masked-only, accessible, and resilient", async () => {
  const [profile, card, dialogs, text] = await Promise.all([
    source("modules/profile/profile-page.tsx"),
    source("modules/profile/recovery-contact-card.tsx"),
    source("modules/profile/recovery-contact-dialogs.tsx"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(profile, /<RecoveryContactCard\s*\/>/);
  assert.match(card, /authService\.getRecoveryContact/);
  assert.match(card, /masked_address/);
  assert.match(card, /aria-live=["']polite["']/);
  assert.match(card, /role=["']alert["']/);
  assert.match(card, /motion-reduce:/);
  assert.match(card, /projection\.can_remove/);
  assert.match(card, /requestIntent\(["']remove["']\)/);
  assert.match(
    card,
    /authService\.removeRecoveryContact\(current\.version\)/,
  );
  assert.match(card, /REMOVE_CONFIRM/);
  assert.match(card, /disabled=.*recovery-contact-remove-explanation/s);
  assert.match(dialogs, /htmlFor=["']recovery-current-password["']/);
  assert.match(dialogs, /htmlFor=["']recovery-contact-email["']/);
  assert.match(dialogs, /autoComplete=["']current-password["']/);
  assert.match(dialogs, /autoComplete=["']email["']/);
  assert.match(dialogs, /RECOVERY_EMAIL_MAX_LENGTH/);
  assert.doesNotMatch(card + dialogs, /localStorage|sessionStorage|document\.cookie/);
  assert.doesNotMatch(card + dialogs, /\/auth\/recovery-contact/);
  assert.match(text, /RECOVERY_CONTACT:\s*{/);
  assert.match(text, /REMOVE_DISABLED:/);
  assert.match(text, /REMOVE_CONFIRM:/);
});

test("forgot-password consumes the non-enumerating accepted contract", async () => {
  const [types, service, page, identity, text] = await Promise.all([
    source("types/auth.ts"),
    source("services/auth.service.ts"),
    source("modules/auth/forgot-password-page.tsx"),
    source("constants/identity.ts"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(identity, /PASSWORD_RESET_REQUEST_STATUS/);
  assert.match(identity, /ACCEPTED:\s*["']accepted["']/);
  assert.match(types, /status:\s*typeof PASSWORD_RESET_REQUEST_STATUS\.ACCEPTED/);
  assert.match(service, /Promise<ForgotPasswordResponse>/);
  assert.match(page, /response\.status/);
  assert.match(page, /PASSWORD_RESET_REQUEST_STATUS\.ACCEPTED/);
  assert.match(page, /setError\(TEXT\.AUTH\.FORGOT_PASSWORD_FAILED\)/);
  assert.match(page, /APP_ROUTES\.LOGIN/);
  assert.doesNotMatch(page, /href=["']\/login["']/);
  assert.match(text, /Nếu tài khoản này có email khôi phục đã xác minh/);
});
