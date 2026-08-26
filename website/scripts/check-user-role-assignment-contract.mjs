#!/usr/bin/env node

/**
 * Guards the reviewed admin user-role command against silently regressing to
 * a direct mutation that omits its revision, reason, confirmation or replay
 * boundary.
 */

import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const root = process.cwd();

async function source(relativePath) {
  return readFile(path.join(root, relativePath), "utf8");
}

function assertMatch(value, pattern, message) {
  if (!pattern.test(value)) throw new Error(message);
}

async function main() {
  const [adminTypes, userService, usersPage, editDialog, reviewDialog] = await Promise.all([
    source("types/admin.ts"),
    source("services/user.service.ts"),
    source("modules/admin/users-page.tsx"),
    source("modules/admin/dialogs/edit-role-dialog.tsx"),
    source("modules/admin/user-role-review-dialog.tsx"),
  ]);

  assertMatch(
    adminTypes,
    /authorization_revision:\s*number;/,
    "Admin users must retain the server authorization revision.",
  );
  assertMatch(
    adminTypes,
    /interface UserRoleCommandInput[\s\S]*expected_revision:\s*number;[\s\S]*reason:\s*string;[\s\S]*roles:\s*string\[\];[\s\S]*interface ReviewedUserRoleCommand[\s\S]*command_id:/,
    "User role commands must model revision, reason, roles, and a durable command identity.",
  );
  assertMatch(
    userService,
    /previewRoles\([\s\S]*USER_API\.ROLES_PREVIEW[\s\S]*idempotencyKey:\s*attempt\.attempt_id[\s\S]*applyRoles\([\s\S]*USER_API\.ROLES_APPLY[\s\S]*confirmation_token:[\s\S]*reviewed\.review\.confirmation_token[\s\S]*idempotencyKey:\s*commandID/,
    "The user service must preview then apply the exact idempotent reviewed command.",
  );
  assertMatch(
    editDialog,
    /expected_revision:\s*user\.authorization_revision[\s\S]*reason:[\s\S]*roles:\s*selectedRoles/,
    "The role editor must submit the loaded revision, an audit reason, and selected roles.",
  );
  assertMatch(
    usersPage,
    /handleReviewRoles[\s\S]*\.status === 409[\s\S]*refreshUserAfterRoleConflict[\s\S]*handleApplyRoles[\s\S]*\.status === 403[\s\S]*roleReauthenticationRequired/,
    "A 409 role-assignment conflict must refresh the server snapshot.",
  );
  assertMatch(
    usersPage,
    /page_size:\s*IDENTITY_INPUT_LIMITS\.ADMIN_USERS_PAGE_SIZE[\s\S]*pagination=\{visiblePagination\}[\s\S]*onPageChange=\{setPage\}/,
    "Admin users must use bounded server-side pagination.",
  );
  assertMatch(
    usersPage,
    /filters:[\s\S]*type:\s*"search"/,
    "Admin user search must remain a server-side filter.",
  );
  assertMatch(
    usersPage,
    /useDebouncedValue\([\s\S]*IDENTITY_INPUT_LIMITS\.SEARCH_DEBOUNCE_MS/,
    "Admin user search must remain debounced.",
  );
  assertMatch(
    usersPage,
    /useRetryableResource<Paginated<AdminUser>[\s\S]*load:\s*\(signal\)\s*=>\s*userService\.find\(usersQuery, signal\)/,
    "Admin users must use the shared cancellable resource boundary.",
  );
  assertMatch(
    usersPage,
    /USERS_LOAD_ERROR[\s\S]*onClick=\{usersResource\.retry\}/,
    "Admin users must expose a recoverable load state.",
  );
  assertMatch(
    usersPage,
    /u\.id !== currentUser\?\.id/,
    "The admin UI must not offer self-deletion.",
  );
  assertMatch(
    editDialog,
    /onReview:[\s\S]*Promise<void>[\s\S]*expected_revision:\s*user\.authorization_revision[\s\S]*reason:[\s\S]*roles:\s*selectedRoles/,
    "The role editor must await a reviewed command containing the loaded revision, reason, and roles.",
  );
  assertMatch(
    reviewDialog,
    /\{requiresReauthentication\s*\?\s*\([\s\S]*CURRENT_PASSWORD[\s\S]*\)\s*:\s*null\}/,
    "The review dialog must render the current-password input only when the server requires recent authentication.",
  );
  assertMatch(
    reviewDialog,
    /if\s*\(requiresReauthentication\s*&&\s*password\.length\s*===\s*0\)[\s\S]*onConfirm\(password\)/,
    "The review dialog must validate and submit the current password for reauthentication.",
  );
}

main().catch((error) => {
  process.stderr.write(`${error.message}\n`);
  process.exitCode = 1;
});
