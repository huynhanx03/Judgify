import AuthContainer from "@/modules/auth/AuthContainer";
import { Metadata } from "next";
import { TEXT } from "@/constants/text";

export const metadata: Metadata = {
  title: TEXT.META.LOGIN_TITLE,
};

export default function LoginPage() {
  return <AuthContainer />;
}
