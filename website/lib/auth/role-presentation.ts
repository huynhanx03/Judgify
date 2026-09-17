import { ADMIN_TEXT } from "@/constants/admin-text";
import type { Role } from "@/types/admin";

type RolePresentation = Pick<Role, "key" | "name" | "description">;

/** Localizes canonical bootstrap metadata while preserving administrator edits. */
export function roleDisplayName(role: RolePresentation): string {
  return ADMIN_TEXT.ROLES_DISPLAY_NAME(role.key, role.name);
}

export function roleDisplayDescription(
  role: RolePresentation,
): string | undefined {
  return ADMIN_TEXT.ROLES_DISPLAY_DESCRIPTION(role.key, role.description);
}
