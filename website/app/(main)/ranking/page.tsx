import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import RankingPage from "@/modules/ranking/ranking-page";

export const metadata: Metadata = {
  title: TEXT.NAV.RANKING,
};

export default function RankingRoute() {
  return <RankingPage />;
}
