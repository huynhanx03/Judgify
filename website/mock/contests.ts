import type { Contest } from "@/types/contest";

export const MOCK_CONTESTS: Contest[] = [
  {
    id: 1,
    title: "Đại Hội Tỷ Thí Liên Tông - Kỳ 12",
    status: "Đang diễn ra",
    participants: 1245,
    timeLeft: "02:45:12",
    difficulty: "Hard",
    prize: "100.000 Linh Thạch",
    isActive: true,
  },
  {
    id: 2,
    title: "Sát Hạch Ngoại Môn Đệ Tử",
    status: "Sắp bắt đầu",
    participants: 4500,
    startTime: "8:00 PM, 20/03",
    difficulty: "Medium",
    prize: "Địa Cấp Linh Bảo",
    isActive: false,
  },
  {
    id: 3,
    title: "Tranh Đoạt Thiên Địa Linh Khí",
    status: "Sắp bắt đầu",
    participants: 800,
    startTime: "9:00 AM, 22/03",
    difficulty: "Insane",
    prize: "Thần Cấp Bí Tịch",
    isActive: false,
  },
  {
    id: 4,
    title: "Khảo Nghiệm Tâm Tính (Nội Môn)",
    status: "Đã kết thúc",
    participants: 320,
    startTime: "15/03/2025",
    difficulty: "Medium",
    prize: "Tâm Pháp Toàn Thư",
    isActive: false,
  }
];
