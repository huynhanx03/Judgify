/**
 * Material-related types for the Library (Tàng Kinh Các).
 */

export type MaterialRank = "Thần Cấp" | "Thiên Cấp" | "Địa Cấp" | "Huyền Cấp" | "Hoàng Cấp";

export type MaterialType = "Công Pháp" | "Kiếm Thuật" | "Tâm Pháp" | "Bí Tịch" | "Trận Pháp" | "Đan Đạo";

export interface Material {
  id: number;
  title: string;
  description: string;
  type: MaterialType;
  rank: MaterialRank;
  iconName: string; // Storing icon name as string for service portability
  color: string;
  border: string;
  text: string;
}
