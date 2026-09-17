/**
 * Single article card for the Knowledge Base article list.
 * Clicking navigates to the article detail page.
 */

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { ArrowRight, Clock, Eye } from "lucide-react";
import type { MaterialArticle } from "@/types/material";
import { DIFFICULTY_SLUG } from "@/types/difficulty";
import { TEXT } from "@/constants/text";
import { APP_ROUTES } from "@/constants/routes";

const DIFFICULTY_STYLES: Record<string, string> = {
  easy: "border-difficulty-1/25 bg-difficulty-1/10 text-difficulty-1",
  medium: "border-difficulty-2/25 bg-difficulty-2/10 text-difficulty-2",
  hard: "border-difficulty-3/25 bg-difficulty-3/10 text-difficulty-3",
};

interface MaterialsArticleCardProps {
  article: MaterialArticle;
}

export function MaterialsArticleCard({ article }: MaterialsArticleCardProps) {
  const slug = DIFFICULTY_SLUG[article.difficulty?.level] ?? "medium";

  return (
    <Link
      href={APP_ROUTES.MATERIAL_DETAIL(article.slug)}
      className="group flex items-start gap-4 rounded-xl border border-border bg-card p-4 outline-none transition-[border-color,box-shadow] duration-200 hover:border-primary/30 hover:shadow-sm focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <div className="min-w-0 flex-1 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="text-sm font-semibold text-foreground transition-colors group-hover:text-primary">
            {article.title}
          </h3>
          {article.difficulty ? (
            <Badge
              variant="outline"
              className={`h-5 border px-2 py-0 text-[11px] font-medium ${DIFFICULTY_STYLES[slug]}`}
            >
              {article.difficulty.name}
            </Badge>
          ) : null}
        </div>

        <p className="line-clamp-2 text-xs leading-relaxed text-muted-foreground">
          {article.description}
        </p>

        <div className="flex flex-wrap items-center gap-3 pt-0.5">
          {article.tags.map((tag) => (
            <span
              key={tag.id}
              className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
            >
              {tag.name}
            </span>
          ))}
          {article.estimated_read_time > 0 ? (
            <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground/70">
              <Clock className="size-3" aria-hidden="true" />
              {article.estimated_read_time} {TEXT.MATERIALS.MINUTES_SHORT}
            </span>
          ) : null}
          {article.view_count > 0 ? (
            <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground/70">
              <Eye className="size-3" aria-hidden="true" />
              {article.view_count}
            </span>
          ) : null}
        </div>
      </div>

      <ArrowRight
        className="mt-1 size-4 shrink-0 text-muted-foreground/40 transition-colors group-hover:text-primary"
        aria-hidden="true"
      />
    </Link>
  );
}
