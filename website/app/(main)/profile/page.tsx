import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import ProfilePage from "@/modules/profile/profile-page";

export const metadata: Metadata = {
  title: TEXT.NAV.PROFILE,
};

export default function ProfileRoute() {
  return <ProfilePage />;
}
