"use client";

/**
 * Innate Talents (Thiên Phú) display — shows cultivator's unlocked talents
 * with progress bars for each talent level.
 */

import { Zap, Target, Flame, Swords, Star, Droplets, Brain } from "lucide-react";
import type { InnateTalent } from "@/mock/profile-stats";

interface InnateTalentsCardProps {
  talents: InnateTalent[];
}

const ICON_MAP: Record<string, React.ReactNode> = {
  zap: <Zap className="h-4 w-4" />,
  target: <Target className="h-4 w-4" />,
  flame: <Flame className="h-4 w-4" />,
  swords: <Swords className="h-4 w-4" />,
  star: <Star className="h-4 w-4" />,
  droplets: <Droplets className="h-4 w-4" />,
  brain: <Brain className="h-4 w-4" />,
};

export function InnateTalentsCard({ talents }: InnateTalentsCardProps) {
  return (
    <div className="space-y-4">
      {talents.map((talent) => (
        <div key={talent.id} className="flex items-start gap-3">
          {/* Icon */}
          <div
            className="mt-0.5 p-2 rounded-lg shrink-0"
            style={{ backgroundColor: `${talent.color}15`, color: talent.color }}
          >
            {ICON_MAP[talent.icon] ?? <Star className="h-4 w-4" />}
          </div>

          {/* Info */}
          <div className="flex-1 min-w-0 space-y-1.5">
            <div className="flex items-center justify-between">
              <span className="text-sm font-bold">{talent.name}</span>
              <span className="text-xs font-bold" style={{ color: talent.color }}>
                Lv.{talent.level}/{talent.maxLevel}
              </span>
            </div>
            <p className="text-xs text-muted-foreground">{talent.description}</p>
            {/* Level dots */}
            <div className="flex gap-1">
              {Array.from({ length: talent.maxLevel }, (_, i) => (
                <div
                  key={i}
                  className={`h-1.5 flex-1 rounded-full ${i < talent.level ? "" : "bg-muted-foreground/20"}`}
                  style={i < talent.level ? { backgroundColor: talent.color } : undefined}
                />
              ))}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
