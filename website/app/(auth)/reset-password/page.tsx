import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import ResetPasswordPage from "@/modules/auth/reset-password-page";

export const metadata: Metadata = {
  title: TEXT.META.RESET_PASSWORD_TITLE,
};

export default function ResetPasswordRoute() {
  return <ResetPasswordPage />;
}
