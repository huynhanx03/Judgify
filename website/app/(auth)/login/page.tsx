import AuthContainer from "@/modules/auth/AuthContainer";
import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Đăng Nhập | Judgify",
};

export default function LoginPage() {
  return <AuthContainer />;
}
