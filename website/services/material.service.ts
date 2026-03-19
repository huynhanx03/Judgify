import { MOCK_ARTICLES, MOCK_CATEGORIES } from "@/mock/materials";
import type { MaterialArticle, MaterialCategory } from "@/types/material";

/**
 * Service for managing Knowledge Base materials (Tàng Kinh Các).
 * Simulated async behavior for future API integration.
 */
export const materialService = {
  async getCategories(): Promise<MaterialCategory[]> {
    await new Promise((resolve) => setTimeout(resolve, 300));
    return MOCK_CATEGORIES;
  },

  async getArticles(): Promise<MaterialArticle[]> {
    await new Promise((resolve) => setTimeout(resolve, 400));
    return MOCK_ARTICLES;
  },

  async getArticlesByCategory(categoryId: string): Promise<MaterialArticle[]> {
    await new Promise((resolve) => setTimeout(resolve, 300));
    return MOCK_ARTICLES.filter((a) => a.categoryId === categoryId);
  },

  async getArticleById(id: string): Promise<MaterialArticle | null> {
    await new Promise((resolve) => setTimeout(resolve, 200));
    return MOCK_ARTICLES.find((a) => a.id === id) ?? null;
  },
};
