"use client";

/**
 * Materials (Tàng Kinh Các) page — wiki-style knowledge base
 * with category sidebar, search, difficulty filter, and article list.
 */

import { useEffect, useMemo, useState } from "react";
import { Menu, X } from "lucide-react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { materialService } from "@/services/material.service";
import type { MaterialArticle, MaterialCategory, MaterialDifficulty } from "@/types/material";
import { MaterialsHeroSection } from "@/modules/materials/materials-hero-section";
import { MaterialsCategorySidebar } from "@/modules/materials/materials-category-sidebar";
import { MaterialsSearchBar } from "@/modules/materials/materials-search-bar";
import { MaterialsDifficultyFilter } from "@/modules/materials/materials-difficulty-filter";
import { MaterialsArticleCard } from "@/modules/materials/materials-article-card";

export default function MaterialsPage() {
  const [categories, setCategories] = useState<MaterialCategory[]>([]);
  const [articles, setArticles] = useState<MaterialArticle[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const [activeCategory, setActiveCategory] = useState<string | null>(null);
  const [activeDifficulty, setActiveDifficulty] = useState<MaterialDifficulty | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [cats, arts] = await Promise.all([
          materialService.getCategories(),
          materialService.getArticles(),
        ]);
        setCategories(cats);
        setArticles(arts);
      } catch (error) {
        console.error("Failed to fetch materials:", error);
      } finally {
        setIsLoading(false);
      }
    };
    fetchData();
  }, []);

  const filteredArticles = useMemo(() => {
    let result = articles;

    if (activeCategory) {
      result = result.filter((a) => a.categoryId === activeCategory);
    }
    if (activeDifficulty) {
      result = result.filter((a) => a.difficulty === activeDifficulty);
    }
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      result = result.filter(
        (a) =>
          a.title.toLowerCase().includes(q) ||
          a.description.toLowerCase().includes(q) ||
          a.tags.some((t) => t.toLowerCase().includes(q))
      );
    }

    return result;
  }, [articles, activeCategory, activeDifficulty, searchQuery]);

  const activeCategoryName = activeCategory
    ? categories.find((c) => c.id === activeCategory)?.name
    : null;

  if (isLoading) return <LoadingSpinner />;

  return (
    <div className="min-h-screen">
      <MaterialsHeroSection />

      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-6">
        <div className="flex gap-8">
          {/* Mobile sidebar toggle */}
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="lg:hidden fixed bottom-6 right-6 z-50 p-3 rounded-full bg-primary text-primary-foreground shadow-lg cursor-pointer"
            aria-label="Toggle categories"
          >
            {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>

          {/* Sidebar */}
          <aside
            className={`
              fixed inset-y-0 left-0 z-40 w-72 bg-background border-r border-border p-4 pt-20 transition-transform duration-200
              lg:static lg:w-56 lg:shrink-0 lg:border-r-0 lg:p-0 lg:pt-0 lg:translate-x-0
              ${sidebarOpen ? "translate-x-0" : "-translate-x-full"}
            `}
          >
            <MaterialsCategorySidebar
              categories={categories}
              activeCategory={activeCategory}
              onSelectCategory={(id) => {
                setActiveCategory(id);
                setSidebarOpen(false);
              }}
            />
          </aside>

          {/* Overlay for mobile sidebar */}
          {sidebarOpen && (
            <div
              className="fixed inset-0 z-30 bg-black/40 lg:hidden"
              onClick={() => setSidebarOpen(false)}
            />
          )}

          {/* Main content */}
          <main className="flex-1 min-w-0 space-y-5">
            {/* Header with category name */}
            <div className="space-y-1">
              <h2 className="text-lg font-bold text-foreground">
                {activeCategoryName ?? "Tất Cả Bài Viết"}
              </h2>
              {activeCategoryName && (
                <p className="text-xs text-muted-foreground">
                  {categories.find((c) => c.id === activeCategory)?.description}
                </p>
              )}
            </div>

            {/* Search + difficulty filter */}
            <div className="space-y-3">
              <MaterialsSearchBar
                value={searchQuery}
                onChange={setSearchQuery}
                resultCount={filteredArticles.length}
              />
              <MaterialsDifficultyFilter
                active={activeDifficulty}
                onChange={setActiveDifficulty}
              />
            </div>

            {/* Article list */}
            <div className="space-y-3">
              {filteredArticles.length > 0 ? (
                filteredArticles.map((article) => (
                  <MaterialsArticleCard key={article.id} article={article} />
                ))
              ) : (
                <div className="flex flex-col items-center justify-center py-16 text-center">
                  <p className="text-sm font-medium text-muted-foreground">
                    Không tìm thấy bài viết nào
                  </p>
                  <p className="text-xs text-muted-foreground/70 mt-1">
                    Thử thay đổi bộ lọc hoặc từ khóa tìm kiếm
                  </p>
                </div>
              )}
            </div>
          </main>
        </div>
      </div>
    </div>
  );
}
