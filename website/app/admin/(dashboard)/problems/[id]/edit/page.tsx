"use client";

/**
 * Admin edit problem page — fetches problem by ID, then renders shared form.
 */

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { ProblemForm } from "@/modules/admin/problem-form";
import { adminService } from "@/services/admin.service";
import { notify } from "@/lib/toast";
import type { Problem } from "@/types/problem";

export default function AdminEditProblemPage() {
  const params = useParams();
  const router = useRouter();
  const id = Number(params.id);

  const [problem, setProblem] = useState<Problem | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id || isNaN(id)) {
      router.replace("/admin/problems");
      return;
    }
    adminService.getProblem(id)
      .then(setProblem)
      .catch(() => {
        notify.error("Không tìm thấy bài tập");
        router.replace("/admin/problems");
      })
      .finally(() => setLoading(false));
  }, [id, router]);

  if (loading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  if (!problem) return null;

  return <ProblemForm problem={problem} />;
}
