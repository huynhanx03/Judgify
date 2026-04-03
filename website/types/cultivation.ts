/**
 * Cultivation module types mirroring backend DTOs.
 * Covers traits (root bones & talents), rarities, and user-trait assignments.
 */

/** Nested rarity info in trait response. */
export interface TraitRarityInfo {
  id: number;
  name: string;
  code: string;
  weight: number;
}

/** Trait response from GET/POST /traits endpoints. */
export interface TraitResponse {
  id: number;
  type: "root_bone" | "talent";
  name: string;
  rarity?: TraitRarityInfo;
  description?: string;
  metadata?: TraitMetadata;
}

/** Metadata attached to a trait for gameplay effects. */
export interface TraitMetadata {
  exp_multiplier?: number;
  exp_bonus?: number;
  target_elements?: string[];
  [key: string]: unknown;
}

/** Response from GET /traits/roll. */
export interface GachaRollResponse {
  root_bones: TraitResponse[];
  talents: TraitResponse[];
}

/** Request body for POST /user-traits. */
export interface CreateUserTraitRequest {
  user_id: number;
  trait_id: number;
}

/** Response from POST /user-traits. */
export interface UserTraitResponse {
  id: number;
  user_id: number;
  trait_id: number;
}

/** Rarity response from BE CRUD endpoints. */
export interface RarityResponse {
  id: number;
  name: string;
  code: string;
  weight: number;
  description?: string;
}

/** Rarity info (used for display purposes). */
export interface RarityInfo {
  code: string;
  name: string;
  color: string;
  bgColor: string;
  borderColor: string;
}

/** Element response from BE. */
export interface ElementResponse {
  id: number;
  name: string;
  code: string;
  description?: string;
}

/** Level response from BE. */
export interface LevelResponse {
  id: number;
  name: string;
  min_exp: number;
  description?: string;
}

/** Rank response from BE. */
export interface RankResponse {
  id: number;
  name: string;
  min_rating: number;
  description?: string;
}

/** Map rarity codes to display info — keyed by code from BE nested rarity. */
export const RARITY_DISPLAY: Record<string, RarityInfo> = {
  common: { code: "common", name: "Phàm Phẩm", color: "text-slate-400", bgColor: "bg-slate-400/10", borderColor: "border-slate-400/40" },
  uncommon: { code: "uncommon", name: "Nhân Phẩm", color: "text-green-400", bgColor: "bg-green-400/10", borderColor: "border-green-400/40" },
  rare: { code: "rare", name: "Địa Phẩm", color: "text-blue-400", bgColor: "bg-blue-400/10", borderColor: "border-blue-400/40" },
  epic: { code: "epic", name: "Thiên Phẩm", color: "text-purple-400", bgColor: "bg-purple-400/10", borderColor: "border-purple-400/40" },
  legendary: { code: "legendary", name: "Tiên Phẩm", color: "text-amber-400", bgColor: "bg-amber-400/10", borderColor: "border-amber-400/40" },
  mythic: { code: "mythic", name: "Thần Phẩm", color: "text-rose-400", bgColor: "bg-rose-400/10", borderColor: "border-rose-400/40" },
  transcendent: { code: "transcendent", name: "Thánh Phẩm", color: "text-cyan-300", bgColor: "bg-cyan-300/10", borderColor: "border-cyan-300/40" },
  origin: { code: "origin", name: "Hỗn Độn Phẩm", color: "text-red-500", bgColor: "bg-red-500/10", borderColor: "border-red-500/40" },
};

/** Tier color palette — index matches tier_index from BE (0 = weakest, 17 = Đạo Tổ). */
export interface TierColors {
  bar: string    // Tailwind gradient classes for progress bar
  badge: string  // full badge classes (bg + text)
  text: string   // text color class for stat value
}

export const TIER_PALETTE: TierColors[] = [
  { bar: "from-slate-500 to-slate-400",     badge: "text-white bg-gradient-to-r from-slate-500 to-slate-400",     text: "text-slate-400"   },  // 0  Luyện Khí / Phàm Nhân
  { bar: "from-stone-500 to-stone-400",     badge: "text-white bg-gradient-to-r from-stone-500 to-stone-400",     text: "text-stone-400"   },  // 1  Trúc Cơ / Tán Tu
  { bar: "from-emerald-500 to-emerald-400", badge: "text-white bg-gradient-to-r from-emerald-500 to-emerald-400", text: "text-emerald-400" },  // 2  Kết Đan / Ngoại Môn
  { bar: "from-teal-500 to-teal-400",       badge: "text-white bg-gradient-to-r from-teal-500 to-teal-400",       text: "text-teal-400"    },  // 3  Nguyên Anh / Nội Môn
  { bar: "from-cyan-500 to-cyan-400",       badge: "text-white bg-gradient-to-r from-cyan-500 to-cyan-400",       text: "text-cyan-400"    },  // 4  Hóa Thần / Hạch Tâm
  { bar: "from-sky-500 to-sky-400",         badge: "text-white bg-gradient-to-r from-sky-500 to-sky-400",         text: "text-sky-400"     },  // 5  Luyện Hư / Chân Truyền
  { bar: "from-blue-500 to-blue-400",       badge: "text-white bg-gradient-to-r from-blue-500 to-blue-400",       text: "text-blue-400"    },  // 6  Hợp Thể / Chấp Sự
  { bar: "from-indigo-500 to-indigo-400",   badge: "text-white bg-gradient-to-r from-indigo-500 to-indigo-400",   text: "text-indigo-400"  },  // 7  Đại Thừa / Trưởng Lão
  { bar: "from-violet-600 to-violet-400",   badge: "text-white bg-gradient-to-r from-violet-600 to-violet-400",   text: "text-violet-400"  },  // 8  Độ Kiếp / Thái Thượng
  { bar: "from-purple-600 to-purple-400",   badge: "text-white bg-gradient-to-r from-purple-600 to-purple-400",   text: "text-purple-400"  },  // 9  Bán Tiên / Chưởng Môn
  { bar: "from-fuchsia-600 to-fuchsia-400", badge: "text-white bg-gradient-to-r from-fuchsia-600 to-fuchsia-400", text: "text-fuchsia-400" },  // 10 Địa Tiên / Vô Thượng
  { bar: "from-pink-500 to-pink-400",       badge: "text-white bg-gradient-to-r from-pink-500 to-pink-400",       text: "text-pink-400"    },  // 11 Thiên Tiên
  { bar: "from-rose-600 to-rose-400",       badge: "text-white bg-gradient-to-r from-rose-600 to-rose-400",       text: "text-rose-400"    },  // 12 Chân Tiên
  { bar: "from-amber-500 to-amber-400",     badge: "text-white bg-gradient-to-r from-amber-500 to-amber-400",     text: "text-amber-400"   },  // 13 Huyền Tiên
  { bar: "from-orange-500 to-orange-400",   badge: "text-white bg-gradient-to-r from-orange-500 to-orange-400",   text: "text-orange-400"  },  // 14 Đại La Kim Tiên
  { bar: "from-red-600 to-red-400",         badge: "text-white bg-gradient-to-r from-red-600 to-red-400",         text: "text-red-400"     },  // 15 Tiên Tôn
  { bar: "from-red-700 to-rose-500",        badge: "text-white bg-gradient-to-r from-red-700 to-rose-500",        text: "text-rose-500"    },  // 16 Tiên Đế
  { bar: "from-yellow-400 via-amber-400 to-orange-500", badge: "text-zinc-900 bg-gradient-to-r from-yellow-400 via-amber-400 to-orange-500", text: "text-amber-300" },  // 17 Đạo Tổ ✨
]

/** Returns tier colors by index, falling back to the last entry if out of range. */
export function getTierColors(tierIndex: number): TierColors {
  return TIER_PALETTE[Math.min(tierIndex, TIER_PALETTE.length - 1)] ?? TIER_PALETTE[0]
}

/** Element display info for consistent styling across the app. */
export interface ElementInfo {
  name: string;
  icon: string;
  /** Tailwind text color class */
  color: string;
  /** Tailwind bg color class */
  bgColor: string;
  /** Tailwind border color class */
  borderColor: string;
  /** Hex color — use for SVG fills, inline styles, canvas (Tailwind JIT can't handle dynamic classes) */
  hex: string;
}

/** Map element codes to display info — single source of truth for all element styling. */
export const ELEMENT_DISPLAY: Record<string, ElementInfo> = {
  fire:  { name: "Hỏa",  icon: "🔥", color: "text-red-400",    bgColor: "bg-red-400/10",    borderColor: "border-red-400/30",  hex: "#f87171" },
  water: { name: "Thủy", icon: "💧", color: "text-cyan-400",   bgColor: "bg-cyan-400/10",   borderColor: "border-cyan-400/30", hex: "#22d3ee" },
  metal: { name: "Kim",  icon: "⚔️",  color: "text-slate-300",  bgColor: "bg-slate-300/10",  borderColor: "border-slate-300/30",hex: "#cbd5e1" },
  wood:  { name: "Mộc",  icon: "🌿", color: "text-emerald-400", bgColor: "bg-emerald-400/10", borderColor: "border-emerald-400/30", hex: "#34d399" },
  earth: { name: "Thổ",  icon: "🪨", color: "text-amber-600",  bgColor: "bg-amber-600/10",  borderColor: "border-amber-600/30", hex: "#d97706" },
};
