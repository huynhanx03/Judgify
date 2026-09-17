import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import ProblemWorkspacePage from "@/modules/problem/problem-workspace-page";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: TEXT.PROBLEM.WORKBENCH_TITLE,
};

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
