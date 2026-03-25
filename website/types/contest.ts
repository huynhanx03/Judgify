/**
 * Contest-related types for the Arena Assembly (Đại Hội Tỷ Thí).
 */

export type ContestStatus = "Đang diễn ra" | "Sắp bắt đầu" | "Đã kết thúc";

export interface Contest {
  id: number;
  title: string;
  status: ContestStatus;
  participants: number;
  startTime?: string;
  timeLeft?: string;
  difficulty: "Easy" | "Medium" | "Hard" | "Insane";
  prize: string;
  isActive: boolean;
}
