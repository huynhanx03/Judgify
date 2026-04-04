/**
 * Single article card for the Knowledge Base article list.
 * Clicking navigates to the article detail page.
 */

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { ArrowRight } from "lucide-react";
import type { MaterialArticle, MaterialDifficulty } from "@/types/material";

const DIFFICULTY_STYLES: Record<MaterialDifficulty, string> = {
  "Nhập Môn": "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
  "Cơ Bản": "bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20",
  "Nâng Cao": "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
  "Chuyên Sâu": "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
};

interface MaterialsArticleCardProps {
  article: MaterialArticle;
}

export function MaterialsArticleCard({ article }: MaterialsArticleCardProps) {
  return (
    <Link href={`/materials/${article.id}`}>
      <div className="group flex items-start gap-4 rounded-xl border border-border bg-card p-4 transition-all duration-200 hover:border-primary/30 hover:shadow-sm cursor-pointer">
        <div className="flex-1 min-w-0 space-y-2">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="text-sm font-semibold text-foreground group-hover:text-primary transition-colors">
              {article.title}
            </h3>
            <Badge
              variant="outline"
              className={`text-[11px] px-2 py-0 h-5 font-medium border ${DIFFICULTY_STYLES[article.difficulty]}`}
            >
              {article.difficulty}
            </Badge>
          </div>

          <p className="text-xs text-muted-foreground leading-relaxed line-clamp-2">
            {article.description}
          </p>

          <div className="flex items-center gap-1.5 flex-wrap pt-0.5">
            {article.tags.map((tag) => (
              <span
                key={tag}
                className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
              >
                {tag}
              </span>
            ))}
          </div>
        </div>

        <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground/40 group-hover:text-primary transition-colors mt-1" />
      </div>
    </Link>
  );
}
