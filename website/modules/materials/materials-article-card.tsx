/**
 * Single article card for the Knowledge Base article list.
 * Clicking navigates to the article detail page.
 */

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { ArrowRight, Clock, Eye } from "lucide-react";
import type { MaterialArticle } from "@/types/material";
import { DIFFICULTY_SLUG } from "@/types/difficulty";

const DIFFICULTY_STYLES: Record<string, string> = {
  easy: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  medium: "bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20",
  hard: "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
};

interface MaterialsArticleCardProps {
  article: MaterialArticle;
}

export function MaterialsArticleCard({ article }: MaterialsArticleCardProps) {
  const slug = DIFFICULTY_SLUG[article.difficulty?.level] ?? "medium";

  return (
    <Link href={`/materials/${article.id}`}>
      <div className="group flex items-start gap-4 rounded-xl border border-border bg-card p-4 transition-all duration-200 hover:border-primary/30 hover:shadow-sm cursor-pointer">
        <div className="flex-1 min-w-0 space-y-2">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="text-sm font-semibold text-foreground group-hover:text-primary transition-colors">
              {article.title}
            </h3>
            {article.difficulty && (
              <Badge
                variant="outline"
                className={`text-[11px] px-2 py-0 h-5 font-medium border ${DIFFICULTY_STYLES[slug]}`}
              >
                {article.difficulty.name}
              </Badge>
            )}
          </div>

          <p className="text-xs text-muted-foreground leading-relaxed line-clamp-2">
            {article.description}
          </p>

          <div className="flex items-center gap-3 flex-wrap pt-0.5">
            {article.tags.map((tag) => (
              <span
                key={tag.id}
                className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
              >
                {tag.name}
              </span>
            ))}
            {article.estimated_read_time > 0 && (
              <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground/70">
                <Clock className="h-3 w-3" />
                {article.estimated_read_time} phút
              </span>
            )}
            {article.view_count > 0 && (
              <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground/70">
                <Eye className="h-3 w-3" />
                {article.view_count}
              </span>
            )}
          </div>
        </div>

        <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground/40 group-hover:text-primary transition-colors mt-1" />
      </div>
    </Link>
  );
}
