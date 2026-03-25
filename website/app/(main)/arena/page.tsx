/**
 * Arena page — displays a list of problems with filtering tabs.
 * Uses getProblems service to fetch mock data and renders ProblemTable.
 */

import { getProblems } from "@/services/problem.service";
import { TEXT } from "@/constants/text";
import { ArenaClient } from "./arena-client";

export const metadata = {
  title: TEXT.NAV.ARENA,
};

/** Server component that fetches problem data and passes to client component. */
export default async function ArenaPage() {
  const data = await getProblems({ pagination: { page: 1, page_size: 100 } });

  return (
    <ArenaClient initialProblems={data.records} />
  );
}
