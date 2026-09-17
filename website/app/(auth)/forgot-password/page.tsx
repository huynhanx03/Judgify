import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import ForgotPasswordPage from "@/modules/auth/forgot-password-page";

export const metadata: Metadata = {
  title: TEXT.AUTH.FORGOT_PASSWORD_TITLE,
};

export default function ForgotPasswordRoute() {
  return <ForgotPasswordPage />;
}
