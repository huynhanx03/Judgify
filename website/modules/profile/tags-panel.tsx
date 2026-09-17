"use client"

import { getElementPresentation } from "@/constants/cultivation-presentation"
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
        const display = getElementPresentation(primaryElem?.code ?? "")
        const ElementIcon = display.Icon

        return (
          <span
            key={tag.name}
            className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium border
              ${display.bgColor} ${display.borderColor} ${display.color}`}
          >
            <ElementIcon className="size-3" aria-hidden="true" />
            {tag.name}
            <span className="opacity-60">×{tag.solved_count}</span>
          </span>
        )
      })}
    </div>
  )
}
