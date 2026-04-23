"use client";

/**
 * Material detail page — displays full article content
 * with back navigation, difficulty badge, tags, and markdown body.
 */

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, BookOpen, Clock, Eye } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { getMaterialById } from "@/services/material.service";
import type { MaterialArticle } from "@/types/material";
import { DIFFICULTY_SLUG } from "@/types/difficulty";
import { MarkdownRenderer } from "@/modules/shared/markdown-renderer";

const DIFFICULTY_STYLES: Record<string, string> = {
  easy: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  medium: "bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20",
  hard: "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
};

export default function MaterialDetailPage() {
  const params = useParams();
  const [article, setArticle] = useState<MaterialArticle | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const id = Number(params.id);
    if (isNaN(id)) {
      setIsLoading(false);
      return;
    }
    getMaterialById(id)
      .then(setArticle)
      .catch(console.error)
      .finally(() => setIsLoading(false));
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
  const slug = DIFFICULTY_SLUG[article.difficulty?.level] ?? "medium";

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
              {article.category && (
                <>
                  <span className="text-muted-foreground/50">·</span>
                  <span>{article.category.name}</span>
                </>
              )}
              {article.estimated_read_time > 0 && (
                <>
                  <span className="text-muted-foreground/50">·</span>
                  <span className="inline-flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {article.estimated_read_time} phút đọc
                  </span>
                </>
              )}
              <span className="text-muted-foreground/50">·</span>
              <span className="inline-flex items-center gap-1">
                <Eye className="h-3 w-3" />
                {article.view_count} lượt xem
              </span>
            </div>

            <h1 className="text-2xl font-bold text-foreground">
              {article.title}
            </h1>

            <div className="flex items-center gap-2 flex-wrap">
              {article.difficulty && (
                <Badge
                  variant="outline"
                  className={`text-xs px-2.5 py-0.5 font-medium border ${DIFFICULTY_STYLES[slug]}`}
                >
                  {article.difficulty.name}
                </Badge>
              )}
              {article.tags.map((tag) => (
                <span
                  key={tag.id}
                  className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
                >
                  {tag.name}
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
