import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

function executeOneTimeTokenModule(code) {
  const compiled = ts.transpileModule(code, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  const require = (specifier) => {
    if (specifier === "@/constants/identity") {
      return {
        ONE_TIME_CREDENTIAL: {
          FRAGMENT_KEY: "token",
          MAX_LENGTH: 4096,
        },
      };
    }
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

function executeRecoveryEmailModule(code) {
  const compiled = ts.transpileModule(code, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  const require = (specifier) => {
    if (specifier === "@/constants/identity") {
      return {
        IDENTITY_INPUT_LIMITS: {
          RECOVERY_EMAIL_MAX_LENGTH: 320,
        },
      };
    }
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

function fakeBrowser(href) {
  const url = new URL(href);
  const replacements = [];
  return {
    browser: {
      location: {
        hash: url.hash,
        href: url.href,
      },
      history: {
        state: { navigation: "state" },
        replaceState(state, unused, nextURL) {
          replacements.push({ state, unused, nextURL });
        },
      },
    },
    replacements,
  };
}

test("one-time credentials are opaque, bounded, and erased from browser-visible URLs", async () => {
  const oneTimeToken = executeOneTimeTokenModule(
    await source("lib/auth/one-time-token.ts"),
  );

  const futureCredential = "jfuture2.opaque-credential";
  const { browser, replacements } = fakeBrowser(
    `https://judgify.test/reset-password?token=legacy&theme=dark#token=${encodeURIComponent(futureCredential)}`,
  );

  assert.equal(
    oneTimeToken.consumeOneTimeTokenFragment({ browser }),
    futureCredential,
    "the browser must not duplicate backend token-prefix knowledge",
  );
  assert.deepEqual(replacements, [
    {
      state: { navigation: "state" },
      unused: "",
      nextURL: "/reset-password?theme=dark",
    },
  ]);

  const tooLong = fakeBrowser(
    `https://judgify.test/verify-recovery-contact#token=${"x".repeat(17)}`,
  );
  assert.equal(
    oneTimeToken.consumeOneTimeTokenFragment({
      browser: tooLong.browser,
      maximumLength: 16,
    }),
    null,
  );
  assert.equal(tooLong.replacements[0].nextURL, "/verify-recovery-contact");
});

test("registration collects and sends the bounded recovery email contract", async () => {
  const [authTypes, registerFlow, personalInfo, identityConstants, text] =
    await Promise.all([
      source("types/auth.ts"),
      source("modules/auth/RegisterFlow.tsx"),
      source("modules/auth/sections/personal-info-section.tsx"),
      source("constants/identity.ts"),
      source("i18n/catalog.vi.ts"),
    ]);

  assert.match(authTypes, /recovery_email:\s*string/);
  assert.match(registerFlow, /recovery_email:\s*""/);
  assert.match(registerFlow, /recovery_email:\s*normalizeRecoveryEmail/);
  assert.match(registerFlow, /isRecoveryEmailValid/);
  assert.match(personalInfo, /type=["{]?["']email["']/);
  assert.match(personalInfo, /autoComplete=["']email["']/);
  assert.match(personalInfo, /RECOVERY_EMAIL_MAX_LENGTH/);
  assert.match(identityConstants, /RECOVERY_EMAIL_MAX_LENGTH:\s*320/);
  assert.match(text, /RECOVERY_EMAIL:/);
  assert.match(text, /RECOVERY_EMAIL_DESCRIPTION:/);
});

test("recovery email input is normalized before validation and transport", async () => {
  const recoveryEmail = executeRecoveryEmailModule(
    await source("lib/auth/recovery-email.ts"),
  );

  assert.equal(
    recoveryEmail.normalizeRecoveryEmail("  DaoHuu@Example.com  "),
    "DaoHuu@Example.com",
  );
  assert.equal(recoveryEmail.isRecoveryEmailValid("  user@example.com "), true);
  assert.equal(recoveryEmail.isRecoveryEmailValid("not-an-email"), false);
  assert.equal(
    recoveryEmail.isRecoveryEmailValid(`${"x".repeat(309)}@example.com`),
    false,
  );
});

test("registration identity controls remain label-associated and keyboard-accessible", async () => {
  const [personalInfo, registerFlow, onboardingFields, attributeField] =
    await Promise.all([
      source("modules/auth/sections/personal-info-section.tsx"),
      source("modules/auth/RegisterFlow.tsx"),
      source("modules/auth/onboarding-profile-fields.tsx"),
      source("modules/profile/profile-attribute-field.tsx"),
    ]);

  for (const field of [
    "registration-username",
    "registration-password",
    "registration-recovery-email",
  ]) {
    assert.match(personalInfo, new RegExp(`htmlFor=["']${field}["']`));
    assert.match(personalInfo, new RegExp(`id=["']${field}["']`));
  }
  assert.match(registerFlow, /<OnboardingProfileFields/);
  assert.match(onboardingFields, /<ProfileAttributeField/);
  assert.match(onboardingFields, /\[aria-invalid="true"\]/);
  assert.match(attributeField, /<Label htmlFor=\{inputID\}/);
  assert.match(attributeField, /id=\{inputID\}/);
  assert.match(attributeField, /aria-invalid=\{Boolean\(error\)\}/);
  assert.match(personalInfo, /focus-visible:ring-2/);
});

test("verification route consumes its credential once and calls only the typed API service", async () => {
  const [page, verifyLayout, routes, api, service, authLayout, text] = await Promise.all([
    source("modules/auth/verify-recovery-contact-page.tsx"),
    source("app/(auth)/verify-recovery-contact/layout.tsx"),
    source("constants/routes.ts"),
    source("constants/api/auth.ts"),
    source("services/auth.service.ts"),
    source("app/(auth)/layout.tsx"),
    source("constants/text.ts"),
  ]);

  assert.match(routes, /VERIFY_RECOVERY_CONTACT:\s*["']\/verify-recovery-contact["']/);
  assert.match(api, /VERIFY_RECOVERY_CONTACT:\s*["']\/auth\/recovery-contact\/verify["']/);
  assert.match(service, /verifyRecoveryContact/);
  assert.match(service, /AUTH_API\.VERIFY_RECOVERY_CONTACT/);
  assert.match(page, /consumeOneTimeTokenFragment/);
  assert.match(page, /startedRef\.current/);
  assert.match(page, /authService\.verifyRecoveryContact/);
  assert.match(page, /role=["'](?:alert|status)["']/);
  assert.match(verifyLayout, /TEXT\.META\.VERIFY_RECOVERY_CONTACT_TITLE/);
  assert.doesNotMatch(
    text,
    /VERIFY_RECOVERY_CONTACT_TITLE:\s*["'][^"']*\|\s*Judgify/,
    "the root metadata template owns the product-name suffix",
  );
  assert.match(authLayout, /referrer:\s*["']no-referrer["']/);
  assert.doesNotMatch(page, /useSearchParams/);
  assert.doesNotMatch(page, /(?:localStorage|sessionStorage|document\.cookie)/);
  assert.doesNotMatch(page, /jcv\d|jpr\d/);
});
