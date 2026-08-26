import ContestDetailPage from "@/modules/contest/contest-detail-page";

export const dynamic = "force-dynamic";

export default function ContestDetailRoute({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  return <ContestDetailPage params={params} />;
}
