import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

async function loadTypeScriptModule(relativePath, dependencies = {}) {
  const compiled = ts.transpileModule(await source(relativePath), {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  const require = (specifier) => {
    if (specifier in dependencies) return dependencies[specifier];
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

test("OAuth establishes only a cookie session and never accepts a browser token", async () => {
  const [service, types] = await Promise.all([
    source("services/auth.service.ts"),
    source("types/auth.ts"),
  ]);

  assert.match(service, /startOAuth\(provider:\s*OAuthProvider\)/);
  assert.match(service, /method:\s*["']POST["']/);
  assert.match(service, /oauthStartSchema/);
  assert.match(service, /finalizeOAuthLogin\(/);
  assert.match(service, /finalizeOAuthLink\(/);
  assert.match(service, /commandSuccessSchema/);
  assert.match(service, /runRecoverableIssuance/);
  assert.match(types, /OAuthFinalizeLoginResponse\s*=\s*LoginResponse/);
  assert.match(types, /OAuthFinalizeLinkResponse\s*=\s*LoginResponse/);
  assert.doesNotMatch(
    service + types,
    /access_token|refresh_token|acceptAccessToken|sessionTransport/,
  );
});

test("OAuth callback query parsing rejects provider errors and incomplete callbacks", async () => {
  const { parseOAuthCallbackQuery } = await loadTypeScriptModule(
    "lib/auth/oauth-callback.ts",
  );

  assert.deepEqual(
    parseOAuthCallbackQuery({ status: " ready ", purpose: " login " }),
    { ok: true, purpose: "login" },
  );
  assert.deepEqual(
    parseOAuthCallbackQuery({ status: "ready", purpose: "link" }),
    { ok: true, purpose: "link" },
  );
  assert.deepEqual(
    parseOAuthCallbackQuery({ status: "error", reason: "provider_denied" }),
    {
      ok: false,
      reason: "provider_denied",
    },
  );
  assert.deepEqual(parseOAuthCallbackQuery({ code: "code", state: "state" }), {
    ok: false,
    reason: "invalid_callback",
  });
});

test("OAuth callback converges through the canonical session projection", async () => {
  const [auth, session] = await Promise.all([
    source("contexts/auth-context.tsx"),
    source("contexts/session-context.tsx"),
  ]);
  assert.match(auth, /session\.establish\(["']oauth-finalize-login["']/);
  assert.match(auth, /authService\.finalizeOAuthLogin/);
  assert.match(session, /const session = await sessionService\.me\(\)/);
  assert.doesNotMatch(auth + session, /access_token|acceptAccessToken/);
});

test("profile editing follows the server-owned revisioned attribute contract", async () => {
  const [model, dialog, field] = await Promise.all([
    source("modules/profile/profile-attribute-model.ts"),
    source("modules/profile/profile-edit-dialog.tsx"),
    source("modules/profile/profile-attribute-field.tsx"),
  ]);

  assert.match(model, /planProfileAttributeMutations/);
  assert.match(model, /expected_definition_revision_id/);
  assert.match(model, /expected_value_version/);
  assert.match(model, /ProfileAttributeFieldErrors/);
  assert.match(dialog, /userService[\s\S]*\.getProfileAttributes\(controller\.signal\)/);
  assert.match(dialog, /userService\.updateProfileAttributes/);
  assert.match(dialog, /response\.profile\.id !== profile\.id/);
  assert.match(dialog, /error instanceof ApiError && error\.status === 409/);
  assert.match(field, /<Label htmlFor=\{inputID\}/);
  assert.match(field, /aria-invalid=\{Boolean\(error\)\}/);
});

test("public UI wires recoverable OAuth, profile, and ranking states", async () => {
  const [login, callback, context, profile, ranking, rankingList] = await Promise.all([
    source("modules/auth/LoginForm.tsx"),
    source("modules/auth/oauth-completion-client.tsx"),
    source("contexts/auth-context.tsx"),
    source("modules/profile/profile-page.tsx"),
    source("modules/ranking/ranking-page.tsx"),
    source("modules/ranking/ranking-top-list.tsx"),
  ]);

  assert.match(login, /OAUTH_PROVIDERS\.map/);
  assert.match(login, /authService\.startOAuth\(provider\)/);
  assert.match(login, /window\.location\.assign\(response\.auth_url\)/);
  assert.match(context, /completeOAuth:/);
  assert.match(callback, /auth\.completeOAuth\(/);
  assert.match(callback, /authService\.finalizeOAuthLink\(\)/);
  assert.match(callback, /role="status"/);
  assert.match(callback, /role="alert"/);
  assert.match(profile, /ProfileEditDialog/);
  assert.match(profile, /setSavedProfile\(\{ userID, profile: updatedProfile \}\)/);
  assert.match(rankingList, /role="alert"/);
  assert.match(rankingList, /TEXT\.COMMON\.RETRY/);
  assert.doesNotMatch(ranking, /isLoading \|\| !data/);
});

test("header does not advertise unread notifications without a real notification source", async () => {
  const header = await source("modules/layout/header.tsx");

  assert.doesNotMatch(header, /\bBell\b/);
  assert.doesNotMatch(header, /glow-amber/);
});

test("public pages use the shared typography and copy boundaries", async () => {
  const [about, adminLogin] = await Promise.all([
    source("modules/about/about-page.tsx"),
    source("modules/admin/admin-login-page.tsx"),
  ]);

  assert.doesNotMatch(about, /dangerouslySetInnerHTML|fonts\.googleapis\.com/);
  assert.match(about, /font-playfair/);
  assert.match(adminLogin, /placeholder=\{TEXT\.AUTH\.PASSWORD_PLACEHOLDER\}/);
});
