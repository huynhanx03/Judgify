import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import ContestPage from "@/modules/contest/contest-page";

export const metadata: Metadata = {
  title: TEXT.NAV.CONTEST,
};

export default function ContestRoute() {
  return <ContestPage />;
}
