import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

async function loadPureTypeScriptModule(relativePath, dependencies = {}) {
  const compiled = ts.transpileModule(await source(relativePath), {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    (specifier) => {
      if (specifier in dependencies) return dependencies[specifier];
      throw new Error(`unexpected import: ${specifier}`);
    },
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

test("canonical server-owned role metadata is localized without hiding administrator edits", async () => {
  const { admin: TEXT } = await loadPureTypeScriptModule("i18n/catalog.admin.vi.ts", {
    "@/i18n/locale": { APP_LOCALE: "vi-VN" },
  });

  assert.equal(
    TEXT.ROLES_DISPLAY_NAME("student", "Student"),
    "Học viên",
  );
  assert.equal(
    TEXT.ROLES_DISPLAY_NAME("superadmin", "Super Administrator"),
    "Quản trị viên tối cao",
  );
  assert.equal(
    TEXT.ROLES_DISPLAY_DESCRIPTION(
      "superadmin",
      "Protected break-glass administrator role",
    ),
    "Vai trò quản trị khẩn cấp được hệ thống bảo vệ.",
  );
  assert.equal(
    TEXT.ROLES_DISPLAY_NAME("student", "Học Viên Nội Bộ"),
    "Học Viên Nội Bộ",
  );
  assert.equal(
    TEXT.ROLES_DISPLAY_NAME("moderator", "Moderator"),
    "Moderator",
  );
});

test("role lists use the shared presentation boundary instead of raw server labels", async () => {
  const [presentation, rolePage, usersPage, createDialog, editDialog] =
    await Promise.all([
      source("lib/auth/role-presentation.ts"),
      source("modules/admin/roles-page.tsx"),
      source("modules/admin/users-page.tsx"),
      source("modules/admin/dialogs/create-user-dialog.tsx"),
      source("modules/admin/dialogs/edit-role-dialog.tsx"),
    ]);

  assert.match(presentation, /ADMIN_TEXT\.ROLES_DISPLAY_NAME/);
  assert.match(presentation, /ADMIN_TEXT\.ROLES_DISPLAY_DESCRIPTION/);
  for (const consumer of [rolePage, usersPage, createDialog, editDialog]) {
    assert.match(consumer, /roleDisplayName/);
  }
  assert.doesNotMatch(rolePage, /\{role\.name\}|\{selectedRole\.name\}/);
  assert.doesNotMatch(createDialog, /\{role\.name\}/);
  assert.doesNotMatch(editDialog, /\{role\.name\}/);
});

test("server status codes never fall through as English presentation copy", async () => {
  const [{ vi: TEXT }, contestCard, contestDetail, adminContests] =
    await Promise.all([
      loadPureTypeScriptModule("i18n/catalog.vi.ts", {
        "@/i18n/locale": { APP_LOCALE: "vi-VN" },
      }),
      source("modules/contest/contest-card.tsx"),
      source("modules/contest/contest-detail-page.tsx"),
      source("modules/admin/contests-page.tsx"),
    ]);

  assert.equal(TEXT.PROBLEM.SUBMISSION_STATUS.ACCEPTED, "Đã chấp nhận");
  assert.equal(TEXT.PROBLEM.SUBMISSION_STATUS.WRONG_ANSWER, "Sai đáp án");
  assert.equal(TEXT.PROBLEM.SUBMISSION_STATUS.PENDING, "Đang chờ");
  assert.equal(TEXT.PROBLEM.SUBMISSION_STATUS.JUDGING, "Đang chấm...");
  assert.doesNotMatch(contestCard, /label:\s*contest\.status/);
  assert.doesNotMatch(contestDetail, /\?\?\s*contest\.status/);
  assert.doesNotMatch(adminContests, /\?\?\s*c\.status/);
});
