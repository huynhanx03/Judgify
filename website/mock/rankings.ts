import type { Cultivator } from "@/types/ranking";

export const MOCK_TOP_CULTIVATORS: Cultivator[] = [
  {
    rank: 1,
    name: "0x4epoll",
    realm: "Đại Thừa Kỳ",
    sect: "Judgify Tông",
    points: 12540,
    avatar: "https://github.com/0x4epoll.png",
    color: "from-amber-400 to-yellow-600",
  },
  {
    rank: 2,
    name: "Huyền Cơ Tử",
    realm: "Độ Kiếp Kỳ",
    sect: "Thiên Đạo Viện",
    points: 11200,
    avatar: "",
    color: "from-slate-300 to-slate-500",
  },
  {
    rank: 3,
    name: "Tiêu Viêm",
    realm: "Hợp Thể Kỳ",
    sect: "Viêm Minh",
    points: 10850,
    avatar: "",
    color: "from-orange-400 to-orange-700",
  },
];

export const MOCK_RANKINGS: Cultivator[] = [
  ...MOCK_TOP_CULTIVATORS,
  { rank: 4, name: "Lâm Động", realm: "Hợp Thể Kỳ", sect: "Võ Tổ", points: 9500 },
  { rank: 5, name: "Thạch Hạo", realm: "Luyện Hư Kỳ", sect: "Hoàng Đình", points: 9200 },
  { rank: 6, name: "Diệp Phàm", realm: "Luyện Hư Kỳ", sect: "Thiên Đình", points: 8900 },
  { rank: 7, name: "Hàn Lập", realm: "Hóa Thần Kỳ", sect: "Thanh Nguyên Môn", points: 8500 },
  { rank: 8, name: "Trần Bắc Huyền", realm: "Hóa Thần Kỳ", sect: "Bắc Huyền Phái", points: 8100 },
  { rank: 9, name: "Tần Vũ", realm: "Nguyên Anh Kỳ", sect: "Tần Gia", points: 7800 },
  { rank: 10, name: "Lục Tuyết Kỳ", realm: "Nguyên Anh Kỳ", sect: "Thanh Vân Môn", points: 7500 },
];
