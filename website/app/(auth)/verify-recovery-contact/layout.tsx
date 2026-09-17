import type { Metadata } from "next";

import { TEXT } from "@/constants/text";

export const metadata: Metadata = {
  title: TEXT.META.VERIFY_RECOVERY_CONTACT_TITLE,
};

export default function VerifyRecoveryContactLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return children;
}
