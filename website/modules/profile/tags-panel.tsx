"use client"

import { ELEMENT_DISPLAY } from "@/types/cultivation"
import { TEXT } from "@/constants/text"
import type { TagStat } from "@/types/user"

interface TagsPanelProps {
  tags: TagStat[]
}

export function TagsPanel({ tags }: TagsPanelProps) {
  if (tags.length === 0) {
    return (
      <p className="text-sm text-muted-foreground italic text-center py-4">
        {TEXT.STATS.NO_TAGS}
      </p>
    )
  }

  const sorted = [...tags].sort((a, b) => b.solved_count - a.solved_count)

  return (
    <div className="flex flex-wrap gap-2">
      {sorted.map((tag) => {
        const primaryElem = tag.elements[0]
        const display = primaryElem ? ELEMENT_DISPLAY[primaryElem.code] : null

        return (
          <span
            key={tag.name}
            className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium border
              ${display
                ? `${display.bgColor} ${display.borderColor} ${display.color}`
                : "bg-muted/40 border-border/30 text-muted-foreground"
              }`}
          >
            {display?.icon ?? "🏷️"}
            {tag.name}
            <span className="opacity-60">×{tag.solved_count}</span>
          </span>
        )
      })}
    </div>
  )
}
