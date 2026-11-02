import { expect, type Page } from "@playwright/test";
import { TEXT } from "@/constants/text";
import { APP_ROUTES } from "@/constants/routes";
import {
  acceptanceEnvironment,
  hasAdminCredentials,
  hasMemberCredentials,
} from "./environment";

export async function loginMember(page: Page): Promise<void> {
  if (!hasMemberCredentials()) {
    throw new Error("member browser credentials are not configured");
  }
  await page.goto(APP_ROUTES.LOGIN);
  await page.getByLabel(TEXT.AUTH.USERNAME).fill(
    acceptanceEnvironment.member.username!,
  );
  await page.getByLabel(TEXT.AUTH.PASSWORD).fill(
    acceptanceEnvironment.member.password!,
  );
  await page.getByRole("button", { name: TEXT.AUTH.LOGIN_BUTTON }).click();
  await expect(page).toHaveURL(new RegExp(`${APP_ROUTES.ARENA}(?:[/?#]|$)`));
}

export async function loginAdmin(page: Page): Promise<void> {
  if (!hasAdminCredentials()) {
    throw new Error("administrator browser credentials are not configured");
  }
  await page.goto(APP_ROUTES.ADMIN_LOGIN);
  await page.getByLabel(TEXT.AUTH.ADMIN_USERNAME).fill(
    acceptanceEnvironment.admin.username!,
  );
  await page.getByLabel(TEXT.AUTH.ADMIN_PASSWORD).fill(
    acceptanceEnvironment.admin.password!,
  );
  await page.getByRole("button", { name: TEXT.AUTH.LOGIN }).click();
  await expect(page).toHaveURL(new RegExp(`${APP_ROUTES.ADMIN}(?:[/?#]|$)`));
}
