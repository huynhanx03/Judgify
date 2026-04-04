"use client";

/**
 * Material detail page — displays full article content
 * with back navigation, difficulty badge, tags, and markdown body.
 */

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, BookOpen } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { materialService } from "@/services/material.service";
import type { MaterialArticle, MaterialDifficulty } from "@/types/material";
import { MarkdownRenderer } from "@/modules/shared/markdown-renderer";

const DIFFICULTY_STYLES: Record<MaterialDifficulty, string> = {
  "Nhập Môn": "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  "Cơ Bản": "bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20",
  "Nâng Cao": "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
  "Chuyên Sâu": "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
};

export default function MaterialDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [article, setArticle] = useState<MaterialArticle | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const id = params.id as string;
    materialService.getArticleById(id).then((data) => {
      setArticle(data);
      setIsLoading(false);
    });
  }, [params.id]);

  if (isLoading) return <LoadingSpinner />;

  if (!article) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] gap-4">
        <p className="text-muted-foreground">Không tìm thấy bài viết</p>
        <Link href="/materials" className="text-primary text-sm hover:underline">
          Quay lại Tàng Kinh Các
        </Link>
      </div>
    );
  }

  const content = article.content ?? article.description;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 space-y-6">
      {/* Back link */}
      <Link
        href="/materials"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-primary transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Tàng Kinh Các
      </Link>

      {/* Article card */}
      <Card className="glass-card border-border/40">
        <CardContent className="p-6 sm:p-8 space-y-6">
          {/* Header */}
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <BookOpen className="h-4 w-4" />
              <span>Bài viết</span>
            </div>

            <h1 className="text-2xl font-bold text-foreground">
              {article.title}
            </h1>

            <div className="flex items-center gap-2 flex-wrap">
              <Badge
                variant="outline"
                className={`text-xs px-2.5 py-0.5 font-medium border ${DIFFICULTY_STYLES[article.difficulty]}`}
              >
                {article.difficulty}
              </Badge>
              {article.tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
                >
                  {tag}
                </span>
              ))}
            </div>

            <p className="text-sm text-muted-foreground">
              {article.description}
            </p>
          </div>

          {/* Separator */}
          <div className="h-px bg-border/40" />

          {/* Content */}
          <MarkdownRenderer content={content} />
        </CardContent>
      </Card>
    </div>
  );
}
