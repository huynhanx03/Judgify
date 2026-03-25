/**
 * Search bar for filtering articles in the Knowledge Base.
 */

import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";

interface MaterialsSearchBarProps {
  value: string;
  onChange: (value: string) => void;
  resultCount: number;
}

export function MaterialsSearchBar({ value, onChange, resultCount }: MaterialsSearchBarProps) {
  return (
    <div className="flex items-center gap-3">
      <div className="relative flex-1">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          type="text"
          placeholder="Tìm kiếm bài viết..."
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="pl-9 h-9"
        />
      </div>
      <span className="text-xs text-muted-foreground whitespace-nowrap tabular-nums">
        {resultCount} bài viết
      </span>
    </div>
  );
}
