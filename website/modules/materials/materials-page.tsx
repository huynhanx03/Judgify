"use client";

/** Public materials library with abortable, validated catalog and list data. */

import { useMemo, useState } from "react";
import { Loader2, Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { useCursorFeed } from "@/hooks/use-cursor-feed";
import { materialService } from "@/services/material.service";
import { difficultyService } from "@/services/difficulty.service";
import type {
  MaterialCategory,
  MaterialArticle,
  MaterialSearchRequest,
} from "@/types/material";
import type { DifficultyResponse } from "@/types/difficulty";
import type { EntityID } from "@/types/api";
import { MaterialsHeroSection } from "@/modules/materials/materials-hero-section";
import { MaterialsCategorySidebar } from "@/modules/materials/materials-category-sidebar";
import { MaterialsSearchBar } from "@/modules/materials/materials-search-bar";
import { MaterialsDifficultyFilter } from "@/modules/materials/materials-difficulty-filter";
import { MaterialsArticleCard } from "@/modules/materials/materials-article-card";
import {
  MaterialsLoadFeedback,
  MaterialsLoadingFeedback,
} from "@/modules/materials/materials-resource-feedback";
import { TEXT } from "@/constants/text";
import { MATERIAL_DISCOVERY } from "@/constants/material";
import { useDebouncedValue } from "@/hooks/use-debounced-value";

export default function MaterialsPage() {
  const [activeCategory, setActiveCategory] = useState<EntityID | null>(null);
  const [activeDifficulty, setActiveDifficulty] = useState<EntityID | null>(
    null,
  );
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearch = useDebouncedValue(
    searchQuery.trim(),
    MATERIAL_DISCOVERY.SEARCH_DEBOUNCE_MS,
  );
  const [isCategoryMenuOpen, setIsCategoryMenuOpen] = useState(false);

  const categoryResource = useRetryableResource<MaterialCategory[]>({
    resetKey: "materials-categories",
    initialData: [],
    keepPreviousData: true,
    load: (signal) => materialService.getAllCategories(signal),
    onSuccess: (categories) => {
      setActiveCategory((current) =>
        current && !categories.some((category) => category.id === current)
          ? null
          : current,
      );
    },
  });
  const difficultyResource = useRetryableResource<DifficultyResponse[]>({
    resetKey: "materials-difficulties",
    initialData: [],
    keepPreviousData: true,
    load: (signal) => difficultyService.getAll(signal),
    onSuccess: (difficulties) => {
      setActiveDifficulty((current) =>
        current &&
        !difficulties.some((difficulty) => difficulty.id === current)
          ? null
          : current,
      );
    },
  });

  const query = useMemo<MaterialSearchRequest>(() => {
    return {
      ...(activeCategory ? { category_id: activeCategory } : {}),
      ...(activeDifficulty ? { difficulty_id: activeDifficulty } : {}),
      ...(debouncedSearch ? { text: debouncedSearch } : {}),
      sort: debouncedSearch ? "relevance" : "published",
      limit: MATERIAL_DISCOVERY.PAGE_SIZE,
    };
  }, [activeCategory, activeDifficulty, debouncedSearch]);
  const queryKey = useMemo(() => JSON.stringify(query), [query]);
  const articlesFeed = useCursorFeed<MaterialArticle>({
    resetKey: queryKey,
    load: async (cursor, signal) => {
      const page = await materialService.search(
        {
          ...query,
          ...(cursor ? { cursor } : {}),
        },
        signal,
      );
      return {
        items: page.records,
        next_cursor: page.next_cursor,
      };
    },
    keyOf: (article) => article.id,
  });

  const categories = categoryResource.data;
  const difficulties = difficultyResource.data;
  const isCurrentFeed = articlesFeed.dataKey === queryKey;
  const articles = isCurrentFeed ? articlesFeed.items : [];
  const feedStatus = isCurrentFeed ? articlesFeed.status : "loading";
  const totalItems = articles.length;
  const isRefreshing = feedStatus === "refreshing";
  const isLoadingMore = feedStatus === "loading_more";
  const hasMore = isCurrentFeed && articlesFeed.hasMore;
  const isSearchPending = searchQuery.trim() !== debouncedSearch;
  const activeCategoryName = activeCategory
    ? categories.find((category) => category.id === activeCategory)?.name
    : null;

  const selectCategory = (id: EntityID | null) => {
    setActiveCategory(id);
    setIsCategoryMenuOpen(false);
  };
  const selectDifficulty = (id: EntityID | null) => setActiveDifficulty(id);

  return (
    <div className="min-h-screen">
      <MaterialsHeroSection />

      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
        <div className="flex gap-8">
          <aside className="hidden w-56 shrink-0 lg:block">
            <MaterialsCategorySidebar
              categories={categories}
              activeCategory={activeCategory}
              onSelectCategory={selectCategory}
              loading={categoryResource.status === "loading"}
            />
          </aside>

          <section
            className="min-w-0 flex-1 space-y-5"
            aria-labelledby="materials-results-heading"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="space-y-1">
                <h2 id="materials-results-heading" className="text-lg font-bold text-foreground">
                  {activeCategoryName ?? TEXT.MATERIALS.ALL_ARTICLES}
                </h2>
                {activeCategoryName ? (
                  <p className="text-xs text-muted-foreground">
                    {
                      categories.find((category) => category.id === activeCategory)
                        ?.description
                    }
                  </p>
                ) : null}
              </div>

              <Sheet
                open={isCategoryMenuOpen}
                onOpenChange={setIsCategoryMenuOpen}
              >
                <SheetTrigger
                  render={
                    <button
                      type="button"
                      className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-border px-3 text-sm font-medium text-foreground transition-colors hover:bg-muted lg:hidden"
                      aria-label={TEXT.MATERIALS.TOGGLE_CATEGORIES}
                    />
                  }
                >
                  <Menu className="size-4" aria-hidden="true" />
                  <span>{TEXT.MATERIALS.CATEGORY_TITLE}</span>
                </SheetTrigger>
                <SheetContent
                  side="left"
                  className="w-[86vw] max-w-sm gap-0 border-border bg-background p-0"
                >
                  <SheetHeader className="border-b border-border p-5 pr-14">
                    <SheetTitle>{TEXT.MATERIALS.CATEGORY_MENU}</SheetTitle>
                    <SheetDescription className="sr-only">
                      {TEXT.MATERIALS.TOGGLE_CATEGORIES}
                    </SheetDescription>
                  </SheetHeader>
                  <div className="flex-1 overflow-y-auto p-4">
                    <MaterialsCategorySidebar
                      categories={categories}
                      activeCategory={activeCategory}
                      onSelectCategory={selectCategory}
                      loading={categoryResource.status === "loading"}
                    />
                  </div>
                </SheetContent>
              </Sheet>
            </div>

            <div className="space-y-3">
              <MaterialsSearchBar
                value={searchQuery}
                onChange={(value) =>
                  setSearchQuery(
                    value.slice(
                      0,
                      MATERIAL_DISCOVERY.MAXIMUM_SEARCH_CHARACTERS,
                    ),
                  )
                }
                resultCount={totalItems}
              />
              <MaterialsDifficultyFilter
                difficulties={difficulties}
                active={activeDifficulty}
                onChange={selectDifficulty}
                loading={difficultyResource.status === "loading"}
              />
            </div>

            {categoryResource.status === "error" ||
            difficultyResource.status === "error" ? (
              <div className="grid gap-3 md:grid-cols-2">
                {categoryResource.status === "error" ? (
                  <MaterialsLoadFeedback
                    compact
                    title={TEXT.MATERIALS.CATEGORIES_LOAD_ERROR_TITLE}
                    description={
                      TEXT.MATERIALS.CATEGORIES_LOAD_ERROR_DESCRIPTION
                    }
                    retryLabel={TEXT.MATERIALS.RETRY}
                    onRetry={categoryResource.retry}
                  />
                ) : null}
                {difficultyResource.status === "error" ? (
                  <MaterialsLoadFeedback
                    compact
                    title={TEXT.MATERIALS.DIFFICULTIES_LOAD_ERROR_TITLE}
                    description={
                      TEXT.MATERIALS.DIFFICULTIES_LOAD_ERROR_DESCRIPTION
                    }
                    retryLabel={TEXT.MATERIALS.RETRY}
                    onRetry={difficultyResource.retry}
                  />
                ) : null}
              </div>
            ) : null}

            <section
              className="space-y-3"
              aria-busy={
                feedStatus === "loading" ||
                feedStatus === "refreshing" ||
                feedStatus === "loading_more"
              }
              aria-labelledby="materials-results-heading"
            >
              {isRefreshing || isSearchPending ? (
                <p
                  className="flex items-center gap-2 text-xs text-muted-foreground"
                  role="status"
                  aria-live="polite"
                >
                  <Loader2
                    className="size-3.5 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                  {isSearchPending
                    ? TEXT.MATERIALS.SEARCH_UPDATING
                    : TEXT.MATERIALS.ARTICLES_REFRESHING}
                </p>
              ) : null}

              {feedStatus === "loading" ? (
                <MaterialsLoadingFeedback label={TEXT.MATERIALS.ARTICLES_LOADING} />
              ) : feedStatus === "error" && articles.length === 0 ? (
                <MaterialsLoadFeedback
                  title={TEXT.MATERIALS.ARTICLES_LOAD_ERROR_TITLE}
                  description={TEXT.MATERIALS.ARTICLES_LOAD_ERROR_DESCRIPTION}
                  retryLabel={TEXT.MATERIALS.RETRY}
                  onRetry={articlesFeed.reload}
                />
              ) : (
                <>
                  {feedStatus === "error" ? (
                    <MaterialsLoadFeedback
                      compact
                      title={TEXT.MATERIALS.ARTICLES_LOAD_ERROR_TITLE}
                      description={TEXT.MATERIALS.ARTICLES_LOAD_ERROR_DESCRIPTION}
                      retryLabel={TEXT.MATERIALS.RETRY}
                      onRetry={
                        hasMore
                          ? () => void articlesFeed.loadMore()
                          : articlesFeed.reload
                      }
                    />
                  ) : null}

                  {articles.length > 0 ? (
                    articles.map((article) => (
                      <MaterialsArticleCard key={article.id} article={article} />
                    ))
                  ) : (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                      <p className="text-sm font-medium text-muted-foreground">
                        {TEXT.MATERIALS.EMPTY}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground/70">
                        {TEXT.MATERIALS.EMPTY_DESCRIPTION}
                      </p>
                    </div>
                  )}

                  {articles.length > 0 && (hasMore || isLoadingMore) ? (
                    <div className="flex justify-center pt-3">
                      <Button
                        type="button"
                        variant="outline"
                        disabled={isLoadingMore}
                        onClick={() => void articlesFeed.loadMore()}
                      >
                        {isLoadingMore ? (
                          <Loader2
                            className="size-4 animate-spin motion-reduce:animate-none"
                            aria-hidden="true"
                          />
                        ) : null}
                        {isLoadingMore
                          ? TEXT.MATERIALS.ARTICLES_LOADING_MORE
                          : TEXT.MATERIALS.ARTICLES_LOAD_MORE}
                      </Button>
                    </div>
                  ) : articles.length > 0 ? (
                    <p className="pt-3 text-center text-xs text-muted-foreground">
                      {TEXT.MATERIALS.ARTICLES_END}
                    </p>
                  ) : null}
                </>
              )}
            </section>
          </section>
        </div>
      </div>
    </div>
  );
}
