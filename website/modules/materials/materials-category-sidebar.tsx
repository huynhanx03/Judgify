/**
 * Category sidebar navigation for the Knowledge Base.
 */

import { BookOpen, FileText, Loader2 } from "lucide-react";
import type { MaterialCategory } from "@/types/material";
import type { EntityID } from "@/types/api";
import { cn } from "@/lib/utils";
import { TEXT } from "@/constants/text";

interface MaterialsCategorySidebarProps {
  categories: MaterialCategory[];
  activeCategory: EntityID | null;
  onSelectCategory: (id: EntityID | null) => void;
  loading?: boolean;
}

export function MaterialsCategorySidebar({
  categories,
  activeCategory,
  onSelectCategory,
  loading = false,
}: MaterialsCategorySidebarProps) {
  return (
    <nav
      className="space-y-1"
      aria-busy={loading}
      aria-label={TEXT.MATERIALS.CATEGORY_MENU}
    >
      <h2 className="mb-3 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        {TEXT.MATERIALS.CATEGORY_TITLE}
      </h2>

      <button
        type="button"
        aria-pressed={activeCategory === null}
        onClick={() => onSelectCategory(null)}
        className={cn(
          "flex min-h-11 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
          activeCategory === null
            ? "bg-primary/10 text-primary"
            : "text-muted-foreground hover:bg-muted hover:text-foreground",
        )}
      >
        <BookOpen className="size-4 shrink-0" aria-hidden="true" />
        <span className="truncate">{TEXT.MATERIALS.ALL}</span>
        <span className="ml-auto text-xs tabular-nums opacity-70">
          {categories.reduce(
            (total, category) => total + category.article_count,
            0,
          )}
        </span>
      </button>

      {loading && categories.length === 0 ? (
        <p
          className="flex min-h-11 items-center gap-2 px-3 text-xs text-muted-foreground"
          role="status"
          aria-live="polite"
        >
          <Loader2
            className="size-4 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />
          {TEXT.MATERIALS.CATEGORIES_LOADING}
        </p>
      ) : null}

      {categories.map((category) => (
        <button
          key={category.id}
          type="button"
          aria-pressed={activeCategory === category.id}
          onClick={() => onSelectCategory(category.id)}
          className={cn(
            "flex min-h-11 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
            activeCategory === category.id
              ? "bg-primary/10 text-primary"
              : "text-muted-foreground hover:bg-muted hover:text-foreground",
          )}
        >
          <FileText className="size-4 shrink-0" aria-hidden="true" />
          <span className="truncate">{category.name}</span>
          <span className="ml-auto text-xs tabular-nums opacity-70">
            {category.article_count}
          </span>
        </button>
      ))}
    </nav>
  );
}
