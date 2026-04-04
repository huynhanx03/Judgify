"use client";

/**
 * Registration step 2 — trait gacha card.
 * Receives all trait state + handlers from RegisterFlow.
 */

import { Loader2, Dices, BookOpen } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";
import { TraitCard } from "@/modules/cultivation/trait-card";
import type { TraitResponse } from "@/types/cultivation";

interface TraitSelectionSectionProps {
  rolledRootBone: TraitResponse | null;
  rolledTalents: TraitResponse[];
  selectedTalents: number[];
  isRolling: boolean;
  traitsLoading: boolean;
  onRoll: () => void;
  onToggleTalent: (id: number) => void;
  onOpenCodex: () => void;
}

export function TraitSelectionSection({
  rolledRootBone, rolledTalents, selectedTalents,
  isRolling, traitsLoading, onRoll, onToggleTalent, onOpenCodex,
}: TraitSelectionSectionProps) {
  return (
    <Card className="border-white/10 bg-white/[0.02] backdrop-blur-sm p-5 space-y-4 mt-6">
      <div className="flex items-center justify-between border-b border-white/10 pb-2">
        <h3 className="text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground">
          {TEXT.AUTH.STEP_TRAITS}
        </h3>
        <button onClick={onOpenCodex} className="text-muted-foreground hover:text-primary transition-colors" title="Xem tất cả traits">
          <BookOpen className="w-4 h-4" />
        </button>
      </div>

      {traitsLoading ? (
        <div className="h-[200px] flex items-center justify-center">
          <Loader2 className="w-8 h-8 animate-spin text-primary" />
        </div>
      ) : !rolledRootBone && rolledTalents.length === 0 ? (
        <div className="h-[200px] flex flex-col items-center justify-center border-2 border-dashed border-white/10 rounded-xl space-y-3">
          <p className="text-sm text-muted-foreground">{TEXT.AUTH.TRAIT_SUBTITLE}</p>
          <Button onClick={onRoll} disabled={isRolling}
            className="bg-amber-600 hover:bg-amber-500 text-white font-bold tracking-widest uppercase gap-2">
            <Dices className="w-4 h-4" />
            {isRolling ? TEXT.AUTH.TRAIT_ROLLING : TEXT.AUTH.TRAIT_ROLL_BUTTON}
          </Button>
        </div>
      ) : (
        <div className="space-y-4">
          {rolledRootBone && (
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-amber-400 mb-2">{TEXT.AUTH.TRAIT_ROOT_BONE}</p>
              <TraitCard trait={rolledRootBone} selected animationDelay={0} />
            </div>
          )}
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-purple-400 mb-2">
              {TEXT.AUTH.TRAIT_SELECT_TALENTS} ({selectedTalents.length}/3)
            </p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {rolledTalents.map((trait, i) => (
                <TraitCard
                  key={trait.id}
                  trait={trait}
                  selected={selectedTalents.includes(trait.id)}
                  onClick={() => onToggleTalent(trait.id)}
                  animationDelay={0.15 + i * 0.08}
                />
              ))}
            </div>
          </div>
          <div className="flex justify-center">
            <Button variant="outline" onClick={onRoll} disabled={isRolling}
              className="px-8 py-2 text-sm gap-2 border-purple-500/40 text-purple-300 hover:bg-purple-500/10 hover:border-purple-400/60 transition-all">
              <Dices className="w-4 h-4" />
              {isRolling ? TEXT.AUTH.TRAIT_ROLLING : TEXT.AUTH.TRAIT_REROLL}
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}
