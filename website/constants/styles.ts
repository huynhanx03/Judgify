/**
 * Shared difficulty level style constants.
 * Keyed by level number (1 = easy, 2 = medium, 3 = hard, 4+  = extra tiers).
 * Used by: difficulty-badge, stats-panel, arena-client.
 */

export interface DifficultyStyle {
  text: string;     // Tailwind text color
  bg: string;       // Tailwind background
  border: string;   // Tailwind border color
  stroke: string;   // Hex for SVG strokes (donut chart)
  /** Active state for filter buttons: bg + border + text combined */
  active: string;
  /** Hover border for inactive filter buttons */
  hover: string;
}

export const DIFFICULTY_STYLES: Record<number, DifficultyStyle> = {
  1: {
    text: "text-emerald-500",
    bg: "bg-emerald-500/10",
    border: "border-emerald-500/20",
    stroke: "#10b981",
    active: "bg-emerald-500/10 border-emerald-500 text-emerald-500",
    hover: "hover:border-emerald-500/50 text-muted-foreground",
  },
  2: {
    text: "text-amber-500",
    bg: "bg-amber-500/10",
    border: "border-amber-500/20",
    stroke: "#f59e0b",
    active: "bg-amber-500/10 border-amber-500 text-amber-500",
    hover: "hover:border-amber-500/50 text-muted-foreground",
  },
  3: {
    text: "text-rose-500",
    bg: "bg-rose-500/10",
    border: "border-rose-500/20",
    stroke: "#f43f5e",
    active: "bg-rose-500/10 border-rose-500 text-rose-500",
    hover: "hover:border-rose-500/50 text-muted-foreground",
  },
  4: {
    text: "text-purple-500",
    bg: "bg-purple-500/10",
    border: "border-purple-500/20",
    stroke: "#a855f7",
    active: "bg-purple-500/10 border-purple-500 text-purple-500",
    hover: "hover:border-purple-500/50 text-muted-foreground",
  },
  5: {
    text: "text-cyan-500",
    bg: "bg-cyan-500/10",
    border: "border-cyan-500/20",
    stroke: "#06b6d4",
    active: "bg-cyan-500/10 border-cyan-500 text-cyan-500",
    hover: "hover:border-cyan-500/50 text-muted-foreground",
  },
};

export const DIFFICULTY_FALLBACK: DifficultyStyle = {
  text: "text-slate-400",
  bg: "bg-slate-400/10",
  border: "border-slate-400/20",
  stroke: "#6b7280",
  active: "bg-slate-400/10 border-slate-400 text-slate-400",
  hover: "hover:border-slate-400/50 text-muted-foreground",
};

/** Returns the style for a given difficulty level, falling back to DIFFICULTY_FALLBACK. */
export function getDifficultyStyle(level: number): DifficultyStyle {
  return DIFFICULTY_STYLES[level] ?? DIFFICULTY_FALLBACK;
}
