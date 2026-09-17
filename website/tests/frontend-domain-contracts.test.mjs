import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

async function optionalSource(relativePath) {
  try {
    return await source(relativePath);
  } catch (error) {
    if (error?.code === "ENOENT") return null;
    throw error;
  }
}

function executeGenderModule(code) {
  const compiled = ts.transpileModule(code, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  const require = (specifier) => {
    if (specifier === "./text") {
      return {
        TEXT: {
          AUTH: {
            GENDER_MALE: "male-label",
            GENDER_FEMALE: "female-label",
            GENDER_OTHER: "other-label",
          },
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

test("gender has one typed canonical mapping for every user form", async () => {
  const genderSource = await optionalSource("constants/gender.ts");
  assert.ok(genderSource, "constants/gender.ts must own the gender mapping");

  const gender = executeGenderModule(genderSource);
  assert.deepEqual(gender.GENDER, { MALE: 0, FEMALE: 1, OTHER: 2 });
  assert.deepEqual(
    gender.GENDER_OPTIONS.map(({ value, label }) => ({ value, label })),
    [
      { value: 0, label: "male-label" },
      { value: 1, label: "female-label" },
      { value: 2, label: "other-label" },
    ],
  );
  assert.equal(gender.parseGender("0"), 0);
  assert.equal(gender.parseGender(1), 1);
  assert.equal(gender.parseGender("2"), 2);
  assert.equal(gender.parseGender(""), null);
  assert.equal(gender.parseGender("invalid"), null);
  assert.equal(gender.parseGender(3), null);

  const [registerFlow, adminCreate, profileAttributes, userTypes] = await Promise.all([
    source("modules/auth/RegisterFlow.tsx"),
    source("modules/admin/dialogs/create-user-dialog.tsx"),
    source("constants/profile-attributes.ts"),
    source("types/user.ts"),
  ]);

  for (const consumer of [registerFlow, adminCreate]) {
    assert.match(consumer, /OnboardingProfileFields/);
    assert.match(consumer, /useOnboardingProfileForm/);
  }
  assert.match(profileAttributes, /import \{ GENDER_OPTIONS \}/);
  assert.match(profileAttributes, /GENDER_OPTIONS\.find/);
  assert.doesNotMatch(adminCreate, /Number\(form\.gender\)/);
  assert.doesNotMatch(adminCreate, /USERS_FORM_GENDER_(?:OTHER|MALE|FEMALE)/);
  assert.match(userTypes, /import type \{ Gender \}/);
  assert.match(userTypes, /gender:\s*Gender/);
  assert.doesNotMatch(userTypes, /gender:\s*number/);
});

test("UUID-backed admin filters preserve identifiers as strings", async () => {
  const [problems, tags, traits, materials] = await Promise.all([
    source("modules/admin/catalog/problems-page.tsx"),
    source("modules/admin/catalog/tags-page.tsx"),
    source("modules/admin/catalog/traits-page.tsx"),
    source("modules/admin/materials/materials-page.tsx"),
  ]);

  assert.doesNotMatch(problems, /setFilter\("difficulty_id"[^\n]*Number\(/);
  assert.doesNotMatch(tags, /setFilter\("element_id"[^\n]*Number\(/);
  assert.doesNotMatch(traits, /setFilter\("rarity_id"[^\n]*Number\(/);
  assert.doesNotMatch(materials, /setFilter\("category_id"[^\n]*Number\(/);
  assert.doesNotMatch(materials, /setFilter\("difficulty_id"[^\n]*Number\(/);
});
