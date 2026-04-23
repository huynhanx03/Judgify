/**
 * Category sidebar navigation for the Knowledge Base.
 */

import { BookOpen, FileText } from "lucide-react";
import type { MaterialCategory } from "@/types/material";
import { cn } from "@/lib/utils";

interface MaterialsCategorySidebarProps {
  categories: MaterialCategory[];
  activeCategory: number | null;
  onSelectCategory: (id: number | null) => void;
}

export function MaterialsCategorySidebar({
  categories,
  activeCategory,
  onSelectCategory,
}: MaterialsCategorySidebarProps) {
  return (
    <nav className="space-y-1">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground px-3 mb-3">
        Danh Mục
      </h2>

      <button
        onClick={() => onSelectCategory(null)}
        className={cn(
          "flex items-center gap-3 w-full rounded-lg px-3 py-2.5 text-sm font-medium transition-colors cursor-pointer",
          activeCategory === null
            ? "bg-primary/10 text-primary"
            : "text-muted-foreground hover:bg-muted hover:text-foreground"
        )}
      >
        <BookOpen className="h-4 w-4 shrink-0" />
        <span className="truncate">Tất Cả</span>
        <span className="ml-auto text-xs tabular-nums opacity-70">
          {categories.reduce((sum, c) => sum + c.article_count, 0)}
        </span>
      </button>

      {categories.map((category) => (
        <button
          key={category.id}
          onClick={() => onSelectCategory(category.id)}
          className={cn(
            "flex items-center gap-3 w-full rounded-lg px-3 py-2.5 text-sm font-medium transition-colors cursor-pointer",
            activeCategory === category.id
              ? "bg-primary/10 text-primary"
              : "text-muted-foreground hover:bg-muted hover:text-foreground"
          )}
        >
          <FileText className="h-4 w-4 shrink-0" />
          <span className="truncate">{category.name}</span>
          <span className="ml-auto text-xs tabular-nums opacity-70">
            {category.article_count}
          </span>
        </button>
      ))}
    </nav>
  );
}
