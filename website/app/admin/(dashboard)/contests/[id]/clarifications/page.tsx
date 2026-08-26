import { notFound } from "next/navigation";

import { tryEntityID } from "@/lib/api/contracts";
import { AdminContestCommunications } from "@/modules/admin/contest-communications/admin-contest-communications";

export default async function AdminContestClarificationsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const contestID = tryEntityID(id);
  if (!contestID) notFound();
  return <AdminContestCommunications contestID={contestID} />;
}
