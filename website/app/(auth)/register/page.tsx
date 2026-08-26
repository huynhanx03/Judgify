import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import RegisterPage from "@/modules/auth/register-page";

export const metadata: Metadata = { title: TEXT.META.REGISTER_TITLE };

export default function RegisterRoute() {
  return <RegisterPage />;
}
