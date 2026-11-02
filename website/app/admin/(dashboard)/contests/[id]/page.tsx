import { notFound } from "next/navigation";

import { tryEntityID } from "@/lib/api/contracts";
import { AdminContestOverview } from "@/modules/admin/contest-overview/admin-contest-overview";

export default async function AdminContestOverviewPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const contestID = tryEntityID(id);
  if (!contestID) notFound();
  return <AdminContestOverview contestID={contestID} />;
}
