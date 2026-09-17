import type { Metadata } from "next";
import { text } from "@/i18n/text";
import AboutPage from "@/modules/about/about-page";

export const metadata: Metadata = { title: text("META.ABOUT_TITLE") };

export default function AboutRoute() {
  return <AboutPage />;
}
