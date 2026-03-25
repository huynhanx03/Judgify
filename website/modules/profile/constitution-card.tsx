"use client";

/**
 * Constitution (Căn Cốt) display — shows cultivator's innate body type
 * with rank badge and description.
 */

import { Shield } from "lucide-react";
import type { Constitution } from "@/mock/profile-stats";

interface ConstitutionCardProps {
  constitution: Constitution;
}

export function ConstitutionCard({ constitution }: ConstitutionCardProps) {
  return (
    <div className="relative overflow-hidden rounded-xl border border-border/30 p-5">
      {/* Background glow */}
      <div
        className="absolute inset-0 opacity-10"
        style={{
          background: `radial-gradient(ellipse at 30% 50%, ${constitution.color}, transparent 70%)`,
        }}
      />

      <div className="relative flex items-start gap-4">
        {/* Icon */}
        <div
          className="p-3 rounded-xl shrink-0"
          style={{ backgroundColor: `${constitution.color}15`, color: constitution.color }}
        >
          <Shield className="h-6 w-6" />
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0 space-y-2">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-base font-bold" style={{ color: constitution.color }}>
              {constitution.name}
            </span>
            <span
              className={`text-[10px] font-bold text-white px-2 py-0.5 rounded-md bg-gradient-to-r ${constitution.gradient}`}
            >
              {constitution.rank}
            </span>
          </div>
          <p className="text-sm text-muted-foreground leading-relaxed">
            {constitution.description}
          </p>
        </div>
      </div>
    </div>
  );
}
