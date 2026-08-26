import ProblemWorkspacePage from "@/modules/problem/problem-workspace-page";

export const dynamic = "force-dynamic";

export default function ProblemWorkspaceRoute({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{
    contest?: string | string[];
    contest_problem?: string | string[];
  }>;
}) {
  return <ProblemWorkspacePage params={params} searchParams={searchParams} />;
}
