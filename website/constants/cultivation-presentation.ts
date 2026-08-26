import {
  Droplets,
  Flame,
  Leaf,
  Mountain,
  Sword,
  Tag,
  type LucideIcon,
} from "lucide-react";
import { TEXT } from "@/constants/text";

export interface RarityStyle {
  color: string;
  bgColor: string;
  borderColor: string;
}

const RARITY_FALLBACK: RarityStyle = {
  color: "text-muted-foreground",
  bgColor: "bg-muted/50",
  borderColor: "border-border",
};

const RARITY_STYLES: Record<string, RarityStyle> = {
  common: { color: "text-slate-500", bgColor: "bg-slate-500/10", borderColor: "border-slate-500/40" },
  uncommon: { color: "text-green-600 dark:text-green-400", bgColor: "bg-green-500/10", borderColor: "border-green-500/40" },
  rare: { color: "text-blue-600 dark:text-blue-400", bgColor: "bg-blue-500/10", borderColor: "border-blue-500/40" },
  epic: { color: "text-purple-600 dark:text-purple-400", bgColor: "bg-purple-500/10", borderColor: "border-purple-500/40" },
  legendary: { color: "text-amber-700 dark:text-amber-400", bgColor: "bg-amber-500/10", borderColor: "border-amber-500/40" },
  mythic: { color: "text-rose-600 dark:text-rose-400", bgColor: "bg-rose-500/10", borderColor: "border-rose-500/40" },
  transcendent: { color: "text-cyan-700 dark:text-cyan-300", bgColor: "bg-cyan-500/10", borderColor: "border-cyan-500/40" },
  origin: { color: "text-red-600 dark:text-red-400", bgColor: "bg-red-500/10", borderColor: "border-red-500/40" },
};

export function getRarityStyle(code?: string): RarityStyle {
  return (code && RARITY_STYLES[code.toLowerCase()]) || RARITY_FALLBACK;
}

export interface TierColors {
  bar: string;
  badge: string;
  text: string;
}

const TIER_PALETTE: TierColors[] = [
  { bar: "from-slate-500 to-slate-400", badge: "text-white bg-gradient-to-r from-slate-500 to-slate-400", text: "text-slate-500 dark:text-slate-400" },
  { bar: "from-stone-500 to-stone-400", badge: "text-white bg-gradient-to-r from-stone-500 to-stone-400", text: "text-stone-500 dark:text-stone-400" },
  { bar: "from-emerald-500 to-emerald-400", badge: "text-white bg-gradient-to-r from-emerald-500 to-emerald-400", text: "text-emerald-600 dark:text-emerald-400" },
  { bar: "from-teal-500 to-teal-400", badge: "text-white bg-gradient-to-r from-teal-500 to-teal-400", text: "text-teal-600 dark:text-teal-400" },
  { bar: "from-cyan-500 to-cyan-400", badge: "text-white bg-gradient-to-r from-cyan-500 to-cyan-400", text: "text-cyan-600 dark:text-cyan-400" },
  { bar: "from-sky-500 to-sky-400", badge: "text-white bg-gradient-to-r from-sky-500 to-sky-400", text: "text-sky-600 dark:text-sky-400" },
  { bar: "from-blue-500 to-blue-400", badge: "text-white bg-gradient-to-r from-blue-500 to-blue-400", text: "text-blue-600 dark:text-blue-400" },
  { bar: "from-indigo-500 to-indigo-400", badge: "text-white bg-gradient-to-r from-indigo-500 to-indigo-400", text: "text-indigo-600 dark:text-indigo-400" },
  { bar: "from-violet-600 to-violet-400", badge: "text-white bg-gradient-to-r from-violet-600 to-violet-400", text: "text-violet-600 dark:text-violet-400" },
  { bar: "from-purple-600 to-purple-400", badge: "text-white bg-gradient-to-r from-purple-600 to-purple-400", text: "text-purple-600 dark:text-purple-400" },
  { bar: "from-fuchsia-600 to-fuchsia-400", badge: "text-white bg-gradient-to-r from-fuchsia-600 to-fuchsia-400", text: "text-fuchsia-600 dark:text-fuchsia-400" },
  { bar: "from-pink-500 to-pink-400", badge: "text-white bg-gradient-to-r from-pink-500 to-pink-400", text: "text-pink-600 dark:text-pink-400" },
  { bar: "from-rose-600 to-rose-400", badge: "text-white bg-gradient-to-r from-rose-600 to-rose-400", text: "text-rose-600 dark:text-rose-400" },
  { bar: "from-amber-500 to-amber-400", badge: "text-white bg-gradient-to-r from-amber-500 to-amber-400", text: "text-amber-700 dark:text-amber-400" },
  { bar: "from-orange-500 to-orange-400", badge: "text-white bg-gradient-to-r from-orange-500 to-orange-400", text: "text-orange-600 dark:text-orange-400" },
  { bar: "from-red-600 to-red-400", badge: "text-white bg-gradient-to-r from-red-600 to-red-400", text: "text-red-600 dark:text-red-400" },
  { bar: "from-red-700 to-rose-500", badge: "text-white bg-gradient-to-r from-red-700 to-rose-500", text: "text-rose-600 dark:text-rose-400" },
  { bar: "from-yellow-400 via-amber-400 to-orange-500", badge: "text-zinc-900 bg-gradient-to-r from-yellow-400 via-amber-400 to-orange-500", text: "text-amber-700 dark:text-amber-300" },
];

export function getTierColors(tierIndex: number): TierColors {
  const safeIndex = Math.min(Math.max(tierIndex, 0), TIER_PALETTE.length - 1);
  return TIER_PALETTE[safeIndex] ?? TIER_PALETTE[0];
}

export interface ElementPresentation {
  name: string;
  shortLabel: string;
  Icon: LucideIcon;
  color: string;
  bgColor: string;
  borderColor: string;
  chartColor: string;
}

const ELEMENT_FALLBACK: ElementPresentation = {
  name: TEXT.COMMON.UNKNOWN,
  shortLabel: "?",
  Icon: Tag,
  color: "text-muted-foreground",
  bgColor: "bg-muted/50",
  borderColor: "border-border",
  chartColor: "var(--muted-foreground)",
};

const ELEMENT_PRESENTATION: Record<string, ElementPresentation> = {
  fire: {
    name: TEXT.CULTIVATION.ELEMENT_FIRE,
    shortLabel: TEXT.CULTIVATION.ELEMENT_FIRE_SHORT,
    Icon: Flame,
    color: "text-red-600 dark:text-red-400",
    bgColor: "bg-red-500/10",
    borderColor: "border-red-500/30",
    chartColor: "var(--element-fire)",
  },
  water: {
    name: TEXT.CULTIVATION.ELEMENT_WATER,
    shortLabel: TEXT.CULTIVATION.ELEMENT_WATER_SHORT,
    Icon: Droplets,
    color: "text-cyan-700 dark:text-cyan-400",
    bgColor: "bg-cyan-500/10",
    borderColor: "border-cyan-500/30",
    chartColor: "var(--element-water)",
  },
  metal: {
    name: TEXT.CULTIVATION.ELEMENT_METAL,
    shortLabel: TEXT.CULTIVATION.ELEMENT_METAL_SHORT,
    Icon: Sword,
    color: "text-slate-600 dark:text-slate-300",
    bgColor: "bg-slate-500/10",
    borderColor: "border-slate-500/30",
    chartColor: "var(--element-metal)",
  },
  wood: {
    name: TEXT.CULTIVATION.ELEMENT_WOOD,
    shortLabel: TEXT.CULTIVATION.ELEMENT_WOOD_SHORT,
    Icon: Leaf,
    color: "text-emerald-700 dark:text-emerald-400",
    bgColor: "bg-emerald-500/10",
    borderColor: "border-emerald-500/30",
    chartColor: "var(--element-wood)",
  },
  earth: {
    name: TEXT.CULTIVATION.ELEMENT_EARTH,
    shortLabel: TEXT.CULTIVATION.ELEMENT_EARTH_SHORT,
    Icon: Mountain,
    color: "text-amber-800 dark:text-amber-500",
    bgColor: "bg-amber-500/10",
    borderColor: "border-amber-500/30",
    chartColor: "var(--element-earth)",
  },
};

export function getElementPresentation(code: string): ElementPresentation {
  return ELEMENT_PRESENTATION[code.toLowerCase()] ?? ELEMENT_FALLBACK;
}
