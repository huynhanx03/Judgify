/**
 * Search bar for filtering articles in the Knowledge Base.
 */

import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { TEXT } from "@/constants/text";

interface MaterialsSearchBarProps {
  value: string;
  onChange: (value: string) => void;
  resultCount: number;
}

export function MaterialsSearchBar({ value, onChange, resultCount }: MaterialsSearchBarProps) {
  return (
    <div className="flex items-center gap-3">
      <div className="relative flex-1">
        <label htmlFor="materials-search" className="sr-only">
          {TEXT.MATERIALS.SEARCH_LABEL}
        </label>
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          id="materials-search"
          type="text"
          placeholder={TEXT.MATERIALS.SEARCH_PLACEHOLDER}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="pl-9 h-9"
        />
      </div>
      <span
        className="whitespace-nowrap text-xs tabular-nums text-muted-foreground"
        aria-live="polite"
      >
        {resultCount} {TEXT.MATERIALS.ARTICLE_COUNT_LABEL}
      </span>
    </div>
  );
}
