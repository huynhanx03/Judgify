import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const source = (path) => readFile(new URL(path, root), "utf8");

test("OAuth completion requires an explicit three-of-six offer selection for new users", async () => {
  const [client, service, types, schema, server, lifecycle] = await Promise.all([
    source("modules/auth/oauth-completion-client.tsx"),
    source("services/auth.service.ts"),
    source("types/auth.ts"),
    source("lib/auth/auth-schema.ts"),
    source("../api/internal/identity/usecase/oauth_durable_lifecycle.go"),
    source("../api/internal/onboarding/registration.go"),
  ]);

  assert.match(types, /interface OAuthLoginInspectionResponse/);
  assert.match(types, /onboarding_required:\s*boolean/);
  assert.match(types, /expires_at:\s*ISODateTime/);
  assert.match(types, /server_time:\s*ISODateTime/);
  assert.match(schema, /OAuth completion ticket is already expired/);
  assert.match(types, /interface OAuthOnboardingRequest[\s\S]*TraitOfferSelectionRequest/);
  assert.match(types, /onboarding\?:\s*OAuthOnboardingRequest/);
  assert.match(service, /inspectOAuthLogin/);
  assert.match(service, /body:\s*request/);
  assert.match(client, /cultivationService\.createTraitOffer/);
  assert.match(client, /TraitSelectionSection/);
  assert.match(client, /SELECTED_TALENT_COUNT/);
  assert.match(client, /onboarding:\s*\{ \.\.\.offerSelection, \.\.\.profileEvidence \}/);
  assert.match(client, /Math\.min\([\s\S]*offer\.expiresAtEpochMs[\s\S]*completionExpiresAt/);
  assert.match(client, /serverExpiresAt - serverTime/);
  assert.match(client, /setTraitOffer\(null\)/);
  assert.match(client, /result\.purpose === "link" && auth\.isLoading/);
  assert.match(client, /existingLoginFinalizeRef\.current/);
  assert.match(client, /await auth\.refreshCapabilities\(\)/);
  assert.match(client, /onboardingFinalizeRef/);
  assert.match(
    client,
    /if \(await auth\.refreshCapabilities\(\)\) \{\s*router\.replace\(APP_ROUTES\.ARENA\)/,
  );
  assert.doesNotMatch(client, /localStorage|sessionStorage/);

  assert.match(server, /s\.registration\.Authorize\(/);
  assert.match(server, /oauthFinalizeFingerprint\(snapshot, req\)/);
  assert.doesNotMatch(server, /s\.registration\.Issue\(/);
  assert.doesNotMatch(lifecycle, /offer\.Talents\[0\]/);
});

test("OAuth onboarding keeps completion-ticket inspection and finalization separate", async () => {
  const [routes, service, client] = await Promise.all([
    source("constants/api/auth.ts"),
    source("services/auth.service.ts"),
    source("modules/auth/oauth-completion-client.tsx"),
  ]);

  assert.match(routes, /OAUTH_INSPECT_LOGIN:\s*"\/auth\/oauth\/login\/inspect"/);
  assert.match(service, /oauthLoginInspectionSchema/);
  assert.match(client, /OAUTH_ONBOARDING_RETRY/);
  assert.match(client, /role="alert"/);
  assert.match(client, /aria-live="polite"/);
});
