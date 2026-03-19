/**
 * Material-related types for the Knowledge Base (Tàng Kinh Các).
 */

export type MaterialDifficulty = "Nhập Môn" | "Cơ Bản" | "Nâng Cao" | "Chuyên Sâu";

export interface MaterialArticle {
  id: string;
  title: string;
  description: string;
  difficulty: MaterialDifficulty;
  tags: string[];
  categoryId: string;
  url?: string;
  /** Full markdown content for detail page */
  content?: string;
}

export interface MaterialCategory {
  id: string;
  name: string;
  description: string;
  iconName: string;
  articleCount: number;
}
