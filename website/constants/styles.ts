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
    text: "text-difficulty-1",
    bg: "bg-difficulty-1/10",
    border: "border-difficulty-1/20",
    stroke: "var(--difficulty-1)",
    active: "bg-difficulty-1/10 border-difficulty-1 text-difficulty-1",
    hover: "hover:border-difficulty-1/50 text-muted-foreground",
  },
  2: {
    text: "text-difficulty-2",
    bg: "bg-difficulty-2/10",
    border: "border-difficulty-2/20",
    stroke: "var(--difficulty-2)",
    active: "bg-difficulty-2/10 border-difficulty-2 text-difficulty-2",
    hover: "hover:border-difficulty-2/50 text-muted-foreground",
  },
  3: {
    text: "text-difficulty-3",
    bg: "bg-difficulty-3/10",
    border: "border-difficulty-3/20",
    stroke: "var(--difficulty-3)",
    active: "bg-difficulty-3/10 border-difficulty-3 text-difficulty-3",
    hover: "hover:border-difficulty-3/50 text-muted-foreground",
  },
  4: {
    text: "text-difficulty-4",
    bg: "bg-difficulty-4/10",
    border: "border-difficulty-4/20",
    stroke: "var(--difficulty-4)",
    active: "bg-difficulty-4/10 border-difficulty-4 text-difficulty-4",
    hover: "hover:border-difficulty-4/50 text-muted-foreground",
  },
  5: {
    text: "text-difficulty-5",
    bg: "bg-difficulty-5/10",
    border: "border-difficulty-5/20",
    stroke: "var(--difficulty-5)",
    active: "bg-difficulty-5/10 border-difficulty-5 text-difficulty-5",
    hover: "hover:border-difficulty-5/50 text-muted-foreground",
  },
};

export const DIFFICULTY_FALLBACK: DifficultyStyle = {
  text: "text-difficulty-fallback",
  bg: "bg-difficulty-fallback/10",
  border: "border-difficulty-fallback/20",
  stroke: "var(--difficulty-fallback)",
  active: "bg-difficulty-fallback/10 border-difficulty-fallback text-difficulty-fallback",
  hover: "hover:border-difficulty-fallback/50 text-muted-foreground",
};

/** Returns the style for a given difficulty level, falling back to DIFFICULTY_FALLBACK. */
export function getDifficultyStyle(level: number): DifficultyStyle {
  return DIFFICULTY_STYLES[level] ?? DIFFICULTY_FALLBACK;
}
