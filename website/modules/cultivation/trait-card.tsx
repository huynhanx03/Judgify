"use client";

import { LazyMotion, m, useReducedMotion } from "framer-motion";

const loadDomAnimation = () =>
  import("framer-motion").then((mod) => mod.domAnimation);
import { Bone, Sparkles } from "lucide-react";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  getElementPresentation,
  getRarityStyle,
} from "@/constants/cultivation-presentation";
import { TEXT } from "@/constants/text";
import { cn } from "@/lib/utils";
import type {
  TraitEffectRevisionResponse,
  TraitPresentation,
  TraitRarityInfo,
} from "@/types/cultivation";

export type TraitCardData = Omit<TraitPresentation, "id" | "rarity"> & {
  id?: string;
  rarity?: Omit<TraitRarityInfo, "id"> & { id?: string };
  effect_revision?: TraitEffectRevisionResponse;
};

interface TraitCardProps {
  trait: TraitCardData;
  selected?: boolean;
  onClick?: () => void;
  variant?: "compact" | "full";
  animationDelay?: number;
}

function interactiveContainer(
  interactive: boolean,
  className: string,
  content: ReactNode,
  onClick?: () => void,
  selected?: boolean,
) {
  if (interactive) {
    return (
      <button
        type="button"
        className={className}
        onClick={onClick}
        aria-pressed={selected}
      >
        {content}
      </button>
    );
  }
  return <div className={className}>{content}</div>;
}

export function TraitCard({
  trait,
  selected = false,
  onClick,
  variant = "full",
  animationDelay = 0,
}: TraitCardProps) {
  const reduceMotion = useReducedMotion();
  const rarity = getRarityStyle(trait.rarity?.code);
  const rarityName =
    trait.rarity?.name ?? TEXT.CULTIVATION.DEFAULT_RARITY;
  const isRootBone = trait.type === "root_bone";
  const TraitIcon = isRootBone ? Bone : Sparkles;
  const effect = trait.effect_revision?.effect;
  const multiplierDelta =
    effect?.kind === "exp_multiplier"
      ? effect.multiplier_delta_bps
      : undefined;
  const bonus = effect?.kind === "exp_bonus" ? effect.flat_bonus : undefined;
  const elements = effect?.element_codes ?? [];
  const hasEffects = effect !== undefined;
  const interactive = onClick !== undefined;

  const title = (
    <div className="flex flex-wrap items-center gap-2">
      <span
        className={cn(
          "font-semibold text-foreground",
          variant === "full" && "text-base font-bold",
          selected && rarity.color,
        )}
      >
        {trait.name}
      </span>
      <Badge
        variant="outline"
        className={cn(
          "h-5 px-1.5 text-[10px]",
          rarity.color,
          rarity.borderColor,
        )}
      >
        {rarityName}
      </Badge>
    </div>
  );

  const effectChips = hasEffects ? (
    <div className="flex flex-wrap items-center gap-2">
      {multiplierDelta !== undefined ? (
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs",
            multiplierDelta >= 0
              ? "border-success/20 bg-success/10 text-success"
              : "border-destructive/20 bg-destructive/10 text-destructive",
          )}
        >
          <span className="font-mono font-bold">
            {multiplierDelta >= 0 ? "+" : ""}
            {multiplierDelta / 100}%
          </span>
          {variant === "full" ? (
            <span className="text-[10px] opacity-75">
              {TEXT.CULTIVATION.TRAINING_SPEED}
            </span>
          ) : null}
        </span>
      ) : null}
      {bonus !== undefined ? (
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs",
            bonus >= 0
              ? "border-info/20 bg-info/10 text-info"
              : "border-destructive/20 bg-destructive/10 text-destructive",
          )}
        >
          <span className="font-mono font-bold">
            {bonus > 0 ? `+${bonus}` : bonus}
          </span>
          <span className="text-[10px] opacity-75">
            {variant === "full"
              ? TEXT.CULTIVATION.BASE_EXP
              : TEXT.CULTIVATION.EXP}
          </span>
        </span>
      ) : null}
      {elements.map((code) => {
        const element = getElementPresentation(code);
        const ElementIcon = element.Icon;
        return (
          <span
            key={code}
            className={cn(
              "inline-flex items-center gap-1 rounded-lg border px-2.5 py-1 text-xs",
              element.bgColor,
              element.color,
              element.borderColor,
            )}
          >
            <ElementIcon className="size-3" aria-hidden="true" />
            {element.name}
          </span>
        );
      })}
    </div>
  ) : null;

  if (variant === "compact") {
    const content = (
      <>
        <div
          className={cn(
            "flex size-9 shrink-0 items-center justify-center rounded-lg",
            rarity.bgColor,
            rarity.color,
          )}
        >
          <TraitIcon className="size-4" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1 space-y-1 text-left">
          {title}
          {trait.description ? (
            <p className="text-xs leading-relaxed text-muted-foreground">
              {trait.description}
            </p>
          ) : null}
          {effectChips ? (
            <div className="pt-1">{effectChips}</div>
          ) : null}
        </div>
      </>
    );
    return interactiveContainer(
      interactive,
      cn(
        "group flex w-full items-start gap-3 rounded-xl border border-border bg-muted/15 p-3 text-left transition-colors",
        selected && "border-primary/60 bg-primary/5 ring-1 ring-primary/20",
        interactive &&
          "cursor-pointer hover:bg-muted/35 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
      ),
      content,
      onClick,
      selected,
    );
  }

  const content = (
    <div className="relative flex items-start gap-4">
      <div
        className={cn(
          "flex size-12 shrink-0 items-center justify-center rounded-xl ring-1 ring-border",
          rarity.bgColor,
          rarity.color,
        )}
      >
        <TraitIcon className="size-5" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1 space-y-1.5 text-left">
        {title}
        {trait.description ? (
          <p className="text-sm leading-relaxed text-muted-foreground">
            {trait.description}
          </p>
        ) : null}
        {effectChips ? (
          <>
            <Separator className="my-2 bg-border" />
            {effectChips}
          </>
        ) : null}
      </div>
      {selected ? (
        <div
          className="mt-1 size-4 shrink-0 rounded-full bg-primary shadow-brand-soft"
          aria-hidden="true"
        />
      ) : null}
    </div>
  );
  const className = cn(
    "group relative w-full rounded-xl border-2 p-4 text-left transition-[border-color,background-color,box-shadow] duration-200",
    selected
      ? cn(
          "bg-gradient-to-r from-muted/60 to-muted/30 shadow-brand-subtle",
          rarity.borderColor,
        )
      : "border-border bg-muted/15 hover:border-border/80 hover:bg-muted/30",
    interactive &&
      "cursor-pointer focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
  );
  const motionProps = {
    initial: reduceMotion ? false : { opacity: 0, y: 10 },
    animate: { opacity: 1, y: 0 },
    transition: reduceMotion
      ? { duration: 0 }
      : {
          delay: animationDelay,
          type: "spring" as const,
          stiffness: 200,
        },
  };

  return interactive ? (
    <LazyMotion features={loadDomAnimation}>
      <m.button
        type="button"
        className={className}
        onClick={onClick}
        aria-pressed={selected}
        {...motionProps}
      >
        {content}
      </m.button>
    </LazyMotion>
  ) : (
    <LazyMotion features={loadDomAnimation}>
      <m.div className={className} {...motionProps}>
        {content}
      </m.div>
    </LazyMotion>
  );
}
