import type { Metadata } from "next";

import { TEXT } from "@/constants/text";

export const metadata: Metadata = {
  title: TEXT.META.ACCEPT_INVITATION_TITLE,
  description: TEXT.META.ACCEPT_INVITATION_DESCRIPTION,
  robots: {
    index: false,
    follow: false,
  },
};

export default function AcceptInvitationLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return children;
}
