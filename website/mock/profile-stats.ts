/**
 * Mock profile statistics and cultivation data for UI development.
 * Includes EXP/Level system, Rating/Realm system, and rating history.
 */

/** Cultivation realms ordered by rating threshold. */
export const CULTIVATION_REALMS = [
  { name: "Luyện Khí", minRating: 0, color: "#6B7280", gradient: "from-gray-500 to-gray-600" },
  { name: "Trúc Cơ", minRating: 800, color: "#3B82F6", gradient: "from-blue-500 to-blue-600" },
  { name: "Kim Đan", minRating: 1200, color: "#8B5CF6", gradient: "from-violet-500 to-violet-600" },
  { name: "Nguyên Anh", minRating: 1600, color: "#F59E0B", gradient: "from-amber-500 to-amber-600" },
  { name: "Hóa Thần", minRating: 2000, color: "#EF4444", gradient: "from-red-500 to-red-600" },
  { name: "Luyện Hư", minRating: 2400, color: "#EC4899", gradient: "from-pink-500 to-pink-600" },
  { name: "Hợp Thể", minRating: 2800, color: "#F97316", gradient: "from-orange-500 to-orange-600" },
  { name: "Độ Kiếp", minRating: 3200, color: "#14B8A6", gradient: "from-teal-500 to-teal-600" },
  { name: "Đại Thừa", minRating: 3600, color: "#F59E0B", gradient: "from-amber-400 to-yellow-500" },
];

/** Disciple ranks ordered by EXP threshold. */
export const DISCIPLE_RANKS = [
  { name: "Tạp Dịch Đệ Tử", minExp: 0, icon: "seedling" },
  { name: "Ngoại Môn Đệ Tử", minExp: 100, icon: "sprout" },
  { name: "Nội Môn Đệ Tử", minExp: 500, icon: "flame" },
  { name: "Chân Truyền Đệ Tử", minExp: 1500, icon: "star" },
  { name: "Trưởng Lão", minExp: 5000, icon: "crown" },
  { name: "Thái Thượng Trưởng Lão", minExp: 15000, icon: "gem" },
];

/** Gets the current realm based on rating. */
export function getRealmByRating(rating: number) {
  let realm = CULTIVATION_REALMS[0];
  for (const r of CULTIVATION_REALMS) {
    if (rating >= r.minRating) realm = r;
    else break;
  }
  const idx = CULTIVATION_REALMS.indexOf(realm);
  const nextRealm = CULTIVATION_REALMS[idx + 1] ?? null;
  return { current: realm, next: nextRealm, index: idx };
}

/** Gets the current disciple rank based on EXP. */
export function getRankByExp(exp: number) {
  let rank = DISCIPLE_RANKS[0];
  for (const r of DISCIPLE_RANKS) {
    if (exp >= r.minExp) rank = r;
    else break;
  }
  const idx = DISCIPLE_RANKS.indexOf(rank);
  const nextRank = DISCIPLE_RANKS[idx + 1] ?? null;
  return { current: rank, next: nextRank, index: idx, level: idx + 1 };
}

/**
 * Spiritual Roots (Linh Căn) — five elements mapped to coding skill categories.
 * Each root represents proficiency in a problem domain.
 */
export interface SpiritualRoot {
  name: string;
  element: string;
  color: string;
  gradient: string;
  bgGlow: string;
  /** Proficiency level 0-100. */
  value: number;
  /** Number of problems solved in this category. */
  solved: number;
}

export const MOCK_SPIRITUAL_ROOTS: SpiritualRoot[] = [
  {
    name: "Kim Linh Căn",
    element: "Kim",
    color: "#F59E0B",
    gradient: "from-amber-400 to-yellow-600",
    bgGlow: "rgba(245,158,11,0.15)",
    value: 78,
    solved: 15,
  },
  {
    name: "Mộc Linh Căn",
    element: "Mộc",
    color: "#22C55E",
    gradient: "from-green-400 to-emerald-600",
    bgGlow: "rgba(34,197,94,0.15)",
    value: 62,
    solved: 12,
  },
  {
    name: "Thủy Linh Căn",
    element: "Thủy",
    color: "#3B82F6",
    gradient: "from-blue-400 to-blue-600",
    bgGlow: "rgba(59,130,246,0.15)",
    value: 85,
    solved: 18,
  },
  {
    name: "Hỏa Linh Căn",
    element: "Hỏa",
    color: "#EF4444",
    gradient: "from-red-400 to-red-600",
    bgGlow: "rgba(239,68,68,0.15)",
    value: 45,
    solved: 8,
  },
  {
    name: "Thổ Linh Căn",
    element: "Thổ",
    color: "#A855F7",
    gradient: "from-purple-400 to-purple-600",
    bgGlow: "rgba(168,85,247,0.15)",
    value: 55,
    solved: 10,
  },
];

/**
 * Căn Cốt (Innate Constitution) — the cultivator's born spiritual body type.
 * Determines base aptitude and cultivation speed.
 */
export interface Constitution {
  name: string;
  rank: string;
  description: string;
  color: string;
  gradient: string;
}

export const MOCK_CONSTITUTION: Constitution = {
  name: "Lôi Linh Thể",
  rank: "Thiên Cấp",
  description: "Thể chất bẩm sinh mang lôi thuộc tính, có khả năng dẫn lôi nhập thể, tu luyện tốc độ nhanh gấp đôi bình thường. Cực hiếm, vạn năm mới xuất hiện một lần.",
  color: "#8B5CF6",
  gradient: "from-violet-500 to-indigo-600",
};

/**
 * Thiên Phú (Innate Talents) — spiritual affinities and technique mastery.
 * Represents what the cultivator naturally excels at.
 */
export interface InnateTalent {
  id: string;
  name: string;
  description: string;
  level: number;
  maxLevel: number;
  color: string;
  icon: string;
}

export const MOCK_INNATE_TALENTS: InnateTalent[] = [
  {
    id: "water",
    name: "Thông Thiên Thuỷ Linh Căn",
    description: "Thiên phú thuỷ hệ, lĩnh ngộ thuỷ thuộc công pháp cực nhanh",
    level: 4,
    maxLevel: 5,
    color: "#3B82F6",
    icon: "droplets",
  },
  {
    id: "fire",
    name: "Viêm Hoả Thần Thông",
    description: "Am hiểu hoả hệ công pháp, uy lực hoả thuật tăng 30%",
    level: 3,
    maxLevel: 5,
    color: "#EF4444",
    icon: "flame",
  },
  {
    id: "sword",
    name: "Kiếm Đạo Thiên Tài",
    description: "Thiên phú kiếm thuật, giải bài dạng Hard nhanh hơn 40%",
    level: 2,
    maxLevel: 5,
    color: "#F59E0B",
    icon: "swords",
  },
  {
    id: "soul",
    name: "Thần Hồn Cường Đại",
    description: "Tinh thần lực mạnh mẽ, tập trung giải bài lâu không mệt",
    level: 3,
    maxLevel: 5,
    color: "#A855F7",
    icon: "brain",
  },
];

/** Mock user cultivation profile data. */
export const MOCK_PROFILE_STATS = {
  rating: 1450,
  exp: 2350,
  joinedDate: "2024-06-15",
  problemsSolved: 42,
  contestsJoined: 8,
  totalSubmissions: 128,
  acceptanceRate: 72.5,
  streak: 7,
  easy: 20,
  medium: 15,
  hard: 7,
  /** Rating history for chart — monthly snapshots. */
  ratingHistory: [
    { date: "2024-07", rating: 200 },
    { date: "2024-08", rating: 450 },
    { date: "2024-09", rating: 680 },
    { date: "2024-10", rating: 820 },
    { date: "2024-11", rating: 950 },
    { date: "2024-12", rating: 1100 },
    { date: "2025-01", rating: 1050 },
    { date: "2025-02", rating: 1200 },
    { date: "2025-03", rating: 1350 },
    { date: "2025-04", rating: 1450 },
  ],
};

/** Generate mock activity data for heatmap — covers current + previous year. */
export function generateMockActivity(): Record<string, number> {
  const data: Record<string, number> = {};
  const today = new Date();
  for (let i = 0; i < 730; i++) {
    const d = new Date(today);
    d.setDate(today.getDate() - i);
    const key = d.toISOString().slice(0, 10);
    if (Math.random() > 0.4) data[key] = Math.ceil(Math.random() * 12);
  }
  return data;
}
