import type { Metadata } from "next"

import { TEXT } from "@/constants/text"

export const metadata: Metadata = {
  title: TEXT.META.PUBLIC_PROFILE_TITLE,
  description: TEXT.META.PUBLIC_PROFILE_DESCRIPTION,
}

export default function PublicProfileLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return children
}
