import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import ContestDetailPage from "@/modules/contest/contest-detail-page";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: TEXT.NAV.CONTEST,
};

export default function ContestDetailRoute({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  return <ContestDetailPage params={params} />;
}
