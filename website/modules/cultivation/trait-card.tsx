"use client";

import { motion } from "framer-motion";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { RARITY_DISPLAY, ELEMENT_DISPLAY } from "@/types/cultivation";
import type { TraitResponse } from "@/types/cultivation";

interface TraitCardProps {
  trait: TraitResponse;
  selected?: boolean;
  onClick?: () => void;
  /** "compact" for codex list, "full" for roll result display. */
  variant?: "compact" | "full";
  animationDelay?: number;
}

/** Reusable trait card — used in register flow, codex modal, and profile. */
export function TraitCard({
  trait,
  selected = false,
  onClick,
  variant = "full",
  animationDelay = 0,
}: TraitCardProps) {
  const rarity = (trait.rarity ? RARITY_DISPLAY[trait.rarity.code] : undefined) ?? RARITY_DISPLAY["common"];
  const meta = trait.metadata;
  const isRootBone = trait.type === "root_bone";

  const mul = meta?.exp_multiplier as number | undefined;
  const bonus = meta?.exp_bonus as number | undefined;
  const elems = meta?.target_elements as string[] | undefined;

  const hasEffects = (mul && mul !== 1.0) || (bonus && bonus !== 0) || (elems && elems.length > 0);

  if (variant === "compact") {
    return (
      <div
        className={`group flex items-start gap-3 p-3 rounded-xl bg-muted/10 border border-white/5 transition-colors ${onClick ? "cursor-pointer hover:bg-muted/20" : ""}`}
        onClick={onClick}
      >
        <div className={`w-9 h-9 rounded-lg ${rarity.bgColor} flex items-center justify-center shrink-0`}>
          <span className="text-base">{isRootBone ? "🦴" : "✨"}</span>
        </div>
        <div className="flex-1 min-w-0 space-y-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-semibold text-foreground">{trait.name}</span>
            <Badge variant="outline" className={`text-[10px] h-4 ${rarity.color} ${rarity.borderColor}`}>
              {rarity.name}
            </Badge>
          </div>
          {trait.description && (
            <p className="text-xs text-muted-foreground leading-relaxed">{trait.description}</p>
          )}
          {hasEffects && (
            <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
              {mul && mul !== 1.0 && (
                <span className={`text-[10px] px-1.5 py-0.5 rounded-md ${mul > 1 ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20" : "bg-red-500/10 text-red-400 border border-red-500/20"}`}>
                  EXP ×{mul}
                </span>
              )}
              {bonus !== undefined && bonus !== 0 && (
                <span className={`text-[10px] px-1.5 py-0.5 rounded-md ${bonus > 0 ? "bg-sky-500/10 text-sky-400 border border-sky-500/20" : "bg-red-500/10 text-red-400 border border-red-500/20"}`}>
                  {bonus > 0 ? `+${bonus}` : bonus} EXP
                </span>
              )}
              {elems?.map((e) => {
                const el = ELEMENT_DISPLAY[e];
                return (
                  <span key={e} className={`text-[10px] px-1.5 py-0.5 rounded-md ${el?.bgColor ?? "bg-white/5"} ${el?.color ?? "text-muted-foreground"} border ${el?.borderColor ?? "border-white/10"}`}>
                    {el?.icon} {el?.name ?? e}
                  </span>
                );
              })}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Full variant — for roll results
  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: animationDelay, type: "spring", stiffness: 200 }}
      onClick={onClick}
      className={`group relative p-4 rounded-xl border-2 transition-all duration-300 ${onClick ? "cursor-pointer" : ""} ${
        selected
          ? `bg-gradient-to-r from-muted/60 to-muted/30 ${rarity.borderColor} shadow-[0_0_20px_rgba(245,158,11,0.12)]`
          : "border-white/5 bg-muted/10 hover:bg-muted/20 hover:border-white/10"
      }`}
    >
      {/* Selected indicator glow */}
      {selected && (
        <div className="absolute inset-0 rounded-xl bg-gradient-to-r from-amber-500/5 to-purple-500/5 pointer-events-none" />
      )}

      <div className="relative flex items-start gap-4">
        {/* Icon */}
        <div className={`w-12 h-12 rounded-xl ${rarity.bgColor} flex items-center justify-center shrink-0 ring-1 ring-white/10`}>
          <span className="text-xl">{isRootBone ? "🦴" : "✨"}</span>
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0 space-y-1.5">
          <div className="flex items-center gap-2 flex-wrap">
            <h4 className={`font-bold text-base ${selected ? rarity.color : "text-foreground"}`}>
              {trait.name}
            </h4>
            <Badge variant="outline" className={`text-[10px] h-4 ${rarity.color} ${rarity.borderColor}`}>
              {rarity.name}
            </Badge>
          </div>

          {trait.description && (
            <p className="text-sm text-muted-foreground leading-relaxed">{trait.description}</p>
          )}

          {/* Effects */}
          {hasEffects && (
            <>
              <Separator className="my-2 bg-white/5" />
              <div className="flex flex-wrap items-center gap-2">
                {mul && mul !== 1.0 && (
                  <div className={`flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-lg ${mul > 1 ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20" : "bg-red-500/10 text-red-400 border border-red-500/20"}`}>
                    <span className="font-mono font-bold">×{mul}</span>
                    <span className="text-[10px] opacity-70">Tu Luyện Tốc</span>
                  </div>
                )}
                {bonus !== undefined && bonus !== 0 && (
                  <div className={`flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-lg ${bonus > 0 ? "bg-sky-500/10 text-sky-400 border border-sky-500/20" : "bg-red-500/10 text-red-400 border border-red-500/20"}`}>
                    <span className="font-mono font-bold">{bonus > 0 ? `+${bonus}` : bonus}</span>
                    <span className="text-[10px] opacity-70">EXP Cơ Bản</span>
                  </div>
                )}
                {elems?.map((e) => {
                  const el = ELEMENT_DISPLAY[e];
                  return (
                    <div key={e} className={`flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg ${el?.bgColor ?? "bg-white/5"} ${el?.color ?? "text-muted-foreground"} border ${el?.borderColor ?? "border-white/10"}`}>
                      <span>{el?.icon}</span>
                      <span>{el?.name ?? e}</span>
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </div>

        {/* Selection dot */}
        {selected && (
          <div className="w-4 h-4 rounded-full bg-amber-500 shadow-[0_0_12px_rgba(245,158,11,0.6)] shrink-0 mt-1" />
        )}
      </div>
    </motion.div>
  );
}
