"use client";

/**
 * Registration step 2 — durable trait-offer selection.
 * Receives all trait state + handlers from RegisterFlow.
 */

import { useEffect } from "react";
import dynamic from "next/dynamic";
import { Dices, BookOpen, Clock3 } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";
import { REGISTRATION_TRAIT_RULES } from "@/constants/registration";
import type { TraitPresentation } from "@/types/cultivation";
import type { EntityID } from "@/types/api";
import { formatDateTime } from "@/lib/format";

/**
 * TraitCard is deferred: cards render only after a roll produces an offer, and
 * `traitOffer` starts null, so no card is ever part of the first paint. Keeping
 * it out of the static graph keeps framer-motion out of /register's initial JS.
 * `next/dynamic` has no `.preload()` in the App Router, so the loader is held
 * here and called directly to warm webpack's chunk cache ahead of the roll.
 */
const importTraitCard = () => import("@/modules/cultivation/trait-card");

const TraitCard = dynamic(() => importTraitCard().then((m) => m.TraitCard), {
  ssr: false,
});

function preloadTraitCard() {
  void importTraitCard();
}

interface TraitSelectionSectionProps {
  rolledRootBone: TraitPresentation | null;
  rolledTalents: TraitPresentation[];
  selectedTalents: EntityID[];
  rollTicketExpiresAt: string;
  remainingOffers?: number;
  rollExpired: boolean;
  isRolling: boolean;
  onRoll: () => void;
  onToggleTalent: (id: EntityID) => void;
  onOpenCodex: () => void;
  disabled?: boolean;
}

export function TraitSelectionSection({
  rolledRootBone, rolledTalents, selectedTalents,
  rollTicketExpiresAt, remainingOffers, rollExpired,
  isRolling, onRoll, onToggleTalent, onOpenCodex,
  disabled = false,
}: TraitSelectionSectionProps) {
  // The roll request is a network round-trip; fetching the card chunk alongside
  // it means the cards are resident by the time the offer resolves.
  useEffect(() => {
    if (isRolling) preloadTraitCard();
  }, [isRolling]);

  return (
    <Card
      className="mt-6 space-y-4 border-border bg-card/70 p-5 backdrop-blur-sm"
      aria-busy={disabled || isRolling}
    >
      <div className="flex items-center justify-between border-b border-border pb-2">
        <h3 className="text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground">
          {TEXT.AUTH.STEP_TRAITS}
        </h3>
        <button
          type="button"
          onClick={onOpenCodex}
          disabled={disabled}
          className="flex size-11 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
          title={TEXT.CULTIVATION.TRAIT_CODEX}
          aria-label={TEXT.CULTIVATION.TRAIT_CODEX}
        >
          <BookOpen className="w-4 h-4" />
        </button>
      </div>

      {!rolledRootBone && rolledTalents.length === 0 ? (
        <div className="flex h-[200px] flex-col items-center justify-center space-y-3 rounded-xl border-2 border-dashed border-border">
          <p className="text-sm text-muted-foreground">{TEXT.AUTH.TRAIT_SUBTITLE}</p>
          <Button
            type="button"
            onClick={onRoll}
            onPointerEnter={preloadTraitCard}
            onFocus={preloadTraitCard}
            disabled={disabled || isRolling}
            className="min-h-11 gap-2 font-bold tracking-widest uppercase"
          >
            <Dices className="w-4 h-4" />
            {isRolling ? TEXT.AUTH.TRAIT_ROLLING : TEXT.AUTH.TRAIT_ROLL_BUTTON}
          </Button>
        </div>
      ) : (
        <div className="space-y-4">
          {rolledRootBone && (
            <div>
              <p className="mb-2 text-xs font-bold uppercase tracking-wider text-cultivation">{TEXT.AUTH.TRAIT_ROOT_BONE}</p>
              <TraitCard trait={rolledRootBone} selected animationDelay={0} />
            </div>
          )}
          <div>
            <p className="mb-2 text-xs font-bold uppercase tracking-wider text-primary">
              {TEXT.AUTH.TRAIT_SELECT_TALENTS} ({selectedTalents.length}/{REGISTRATION_TRAIT_RULES.SELECTED_TALENT_COUNT})
            </p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {rolledTalents.map((trait, i) => (
                <TraitCard
                  key={trait.id}
                  trait={trait}
                  selected={selectedTalents.includes(trait.id)}
                  onClick={disabled ? undefined : () => onToggleTalent(trait.id)}
                  animationDelay={0.15 + i * 0.08}
                />
              ))}
            </div>
          </div>
          <div className="flex justify-center">
            <Button
              type="button"
              variant="outline"
              onClick={onRoll}
              onPointerEnter={preloadTraitCard}
              onFocus={preloadTraitCard}
              disabled={disabled || isRolling || remainingOffers === 0}
              className="min-h-11 gap-2 border-primary/40 px-8 py-2 text-sm text-primary transition-colors hover:border-primary/60 hover:bg-primary/10 motion-reduce:transition-none"
            >
              <Dices className="w-4 h-4" />
              {isRolling ? TEXT.AUTH.TRAIT_ROLLING : TEXT.AUTH.TRAIT_REROLL}
            </Button>
          </div>
          {rollTicketExpiresAt ? (
            <div
              className="space-y-1 text-center"
              aria-live="polite"
              aria-atomic="true"
            >
              <p
                className={rollExpired
                  ? "flex items-center justify-center gap-1.5 text-xs font-medium text-destructive"
                  : "flex items-center justify-center gap-1.5 text-xs text-muted-foreground"}
                role={rollExpired ? "alert" : undefined}
              >
                <Clock3 className="size-3.5" aria-hidden="true" />
                {rollExpired
                  ? remainingOffers === 0
                    ? TEXT.AUTH.TRAIT_ROLL_EXHAUSTED
                    : TEXT.AUTH.TRAIT_ROLL_EXPIRED
                  : `${TEXT.AUTH.TRAIT_ROLL_EXPIRES} ${formatDateTime(rollTicketExpiresAt)}`}
              </p>
              {remainingOffers === undefined ? null : (
                <p className="text-[11px] text-muted-foreground">
                  {TEXT.AUTH.TRAIT_ROLL_REMAINING(remainingOffers)}
                </p>
              )}
            </div>
          ) : null}
        </div>
      )}
    </Card>
  );
}
