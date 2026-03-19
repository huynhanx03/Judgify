/**
 * Ranking-related types for the Leaderboard (Bảng Phong Thần).
 */

/** Spiritual root types */
export type SpiritualRoot = "kim" | "moc" | "thuy" | "hoa" | "tho" | "loi" | "bang" | "phong";

export interface Cultivator {
  rank: number;
  name: string;
  realm: string;
  sect: string;
  points: number;
  avatar?: string;
  color?: string;
  /** Level of the cultivator */
  level?: number;
  /** Experience points */
  exp?: number;
  /** Max experience for current level */
  maxExp?: number;
  /** Spiritual root type */
  spiritualRoot?: SpiritualRoot;
  /** Spiritual root purity percentage */
  rootPurity?: number;
}

export interface Leaderboard {
  topThree: Cultivator[];
  others: Cultivator[];
}

/** Spiritual root display info */
export interface SpiritualRootInfo {
  key: SpiritualRoot;
  name: string;
  icon: string;
  color: string;
  bgColor: string;
  borderColor: string;
  glowColor: string;
}

export const SPIRITUAL_ROOT_MAP: Record<SpiritualRoot, SpiritualRootInfo> = {
  kim: { key: "kim", name: "Kim Linh Căn", icon: "⚔️", color: "text-yellow-300", bgColor: "bg-yellow-500/10", borderColor: "border-yellow-500/30", glowColor: "shadow-yellow-500/20" },
  moc: { key: "moc", name: "Mộc Linh Căn", icon: "🌿", color: "text-green-400", bgColor: "bg-green-500/10", borderColor: "border-green-500/30", glowColor: "shadow-green-500/20" },
  thuy: { key: "thuy", name: "Thủy Linh Căn", icon: "💧", color: "text-blue-400", bgColor: "bg-blue-500/10", borderColor: "border-blue-500/30", glowColor: "shadow-blue-500/20" },
  hoa: { key: "hoa", name: "Hỏa Linh Căn", icon: "🔥", color: "text-red-400", bgColor: "bg-red-500/10", borderColor: "border-red-500/30", glowColor: "shadow-red-500/20" },
  tho: { key: "tho", name: "Thổ Linh Căn", icon: "🏔️", color: "text-amber-600", bgColor: "bg-amber-600/10", borderColor: "border-amber-600/30", glowColor: "shadow-amber-600/20" },
  loi: { key: "loi", name: "Lôi Linh Căn", icon: "⚡", color: "text-purple-400", bgColor: "bg-purple-500/10", borderColor: "border-purple-500/30", glowColor: "shadow-purple-500/20" },
  bang: { key: "bang", name: "Băng Linh Căn", icon: "❄️", color: "text-cyan-300", bgColor: "bg-cyan-500/10", borderColor: "border-cyan-500/30", glowColor: "shadow-cyan-500/20" },
  phong: { key: "phong", name: "Phong Linh Căn", icon: "🌪️", color: "text-teal-400", bgColor: "bg-teal-500/10", borderColor: "border-teal-500/30", glowColor: "shadow-teal-500/20" },
};
