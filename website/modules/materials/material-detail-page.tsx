"use client";

/** Public material detail with an abortable, validated resource boundary. */

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, BookOpen, Clock, Eye } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { LoadingSpinner } from "@/components/loading-spinner";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { materialService } from "@/services/material.service";
import type { MaterialArticle, MaterialSlugResponse } from "@/types/material";
import { DIFFICULTY_SLUG } from "@/types/difficulty";
import { MaterialsArticleCard } from "@/modules/materials/materials-article-card";
import {
  MaterialsLoadFeedback,
  MaterialsLoadingFeedback,
} from "@/modules/materials/materials-resource-feedback";
import { TEXT } from "@/constants/text";
import { APP_ROUTES } from "@/constants/routes";
import { ApiError } from "@/lib/api/error";

function trustedSanitizedHTML(html: string) {
  return { __html: html };
}

const DIFFICULTY_STYLES: Record<string, string> = {
  easy: "border-difficulty-1/25 bg-difficulty-1/10 text-difficulty-1",
  medium: "border-difficulty-2/25 bg-difficulty-2/10 text-difficulty-2",
  hard: "border-difficulty-3/25 bg-difficulty-3/10 text-difficulty-3",
};

export default function MaterialDetailPage() {
  const params = useParams();
  const router = useRouter();
  const requestedSlug = typeof params.id === "string" ? params.id : "";
  const articleResource = useRetryableResource<MaterialSlugResponse | null>({
    resetKey: requestedSlug || "invalid-material-slug",
    enabled: requestedSlug.length > 0,
    initialData: null,
    load: (signal) =>
      requestedSlug
        ? materialService.getBySlug(requestedSlug, signal)
        : Promise.resolve(null),
  });
  const article = articleResource.data?.material ?? null;
  const canonicalSlug =
    articleResource.data?.canonical_slug ?? requestedSlug;
  const relatedResource = useRetryableResource<MaterialArticle[]>({
    resetKey: article?.id ?? "unresolved-material",
    enabled: article !== null,
    initialData: [],
    load: (signal) => materialService.related(canonicalSlug, signal),
  });

  useEffect(() => {
    const resolution = articleResource.data;
    if (!resolution) return;
    if (resolution.redirect && resolution.canonical_slug !== requestedSlug) {
      router.replace(APP_ROUTES.MATERIAL_DETAIL(resolution.canonical_slug));
      return;
    }
    const controller = new AbortController();
    void materialService
      .recordView(resolution.material.id, controller.signal)
      .catch(() => undefined);
    return () => controller.abort();
  }, [articleResource.data, requestedSlug, router]);

  if (!requestedSlug) {
    return <MaterialNotFound />;
  }

  if (articleResource.status === "loading") {
    return <LoadingSpinner label={TEXT.MATERIALS.ARTICLE_LOADING} />;
  }

  const articleNotFound =
    articleResource.error instanceof ApiError &&
    articleResource.error.status === 404;

  if (articleResource.status === "error" && !articleNotFound) {
    return (
      <div className="mx-auto flex min-h-[60vh] max-w-2xl items-center px-4 sm:px-6">
        <MaterialsLoadFeedback
          title={TEXT.MATERIALS.ARTICLE_LOAD_ERROR_TITLE}
          description={TEXT.MATERIALS.ARTICLE_LOAD_ERROR_DESCRIPTION}
          retryLabel={TEXT.MATERIALS.RETRY}
          onRetry={articleResource.retry}
        />
      </div>
    );
  }

  if (articleNotFound || !article) return <MaterialNotFound />;

  const slug = DIFFICULTY_SLUG[article.difficulty.level] ?? "medium";

  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6">
      <Link
        href={APP_ROUTES.MATERIALS}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-primary"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        {TEXT.MATERIALS.LIBRARY}
      </Link>

      <Card className="glass-card border-border/40">
        <CardContent className="space-y-6 p-6 sm:p-8">
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <BookOpen className="h-4 w-4" aria-hidden="true" />
              <span>{TEXT.MATERIALS.ARTICLE}</span>
              <span className="text-muted-foreground/50">·</span>
              <span>{article.category.name}</span>
              {article.estimated_read_time > 0 ? (
                <>
                  <span className="text-muted-foreground/50">·</span>
                  <span className="inline-flex items-center gap-1">
                    <Clock className="h-3 w-3" aria-hidden="true" />
                    {TEXT.MATERIALS.READ_MINUTES(article.estimated_read_time)}
                  </span>
                </>
              ) : null}
              <span className="text-muted-foreground/50">·</span>
              <span className="inline-flex items-center gap-1">
                <Eye className="h-3 w-3" aria-hidden="true" />
                {TEXT.MATERIALS.VIEW_COUNT(article.view_count)}
              </span>
            </div>

            <h1 className="text-2xl font-bold text-foreground">{article.title}</h1>

            <div className="flex flex-wrap items-center gap-2">
              <Badge
                variant="outline"
                className={`border px-2.5 py-0.5 text-xs font-medium ${DIFFICULTY_STYLES[slug]}`}
              >
                {article.difficulty.name}
              </Badge>
              {article.tags.map((tag) => (
                <span
                  key={tag.id}
                  className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
                >
                  {tag.name}
                </span>
              ))}
            </div>

            <p className="text-sm text-muted-foreground">{article.description}</p>
          </div>

          <div className="h-px bg-border/40" />
          <article
            className="prose prose-sm max-w-none text-foreground dark:prose-invert sm:prose-base"
            dangerouslySetInnerHTML={trustedSanitizedHTML(
              article.sanitized_html ?? "",
            )}
          />
        </CardContent>
      </Card>

      <section
        className="space-y-3"
        aria-busy={relatedResource.status === "loading"}
        aria-labelledby="related-materials-heading"
      >
        <h2 id="related-materials-heading" className="text-lg font-semibold">
          {TEXT.MATERIALS.RELATED_TITLE}
        </h2>
        {relatedResource.status === "loading" ? (
          <MaterialsLoadingFeedback
            label={TEXT.MATERIALS.RELATED_LOADING}
          />
        ) : relatedResource.status === "error" ? (
          <MaterialsLoadFeedback
            compact
            title={TEXT.MATERIALS.RELATED_LOAD_ERROR_TITLE}
            description={TEXT.MATERIALS.RELATED_LOAD_ERROR_DESCRIPTION}
            retryLabel={TEXT.MATERIALS.RETRY}
            onRetry={relatedResource.retry}
          />
        ) : relatedResource.data.length > 0 ? (
          <div className="grid gap-3 md:grid-cols-2">
            {relatedResource.data.map((item) => (
              <MaterialsArticleCard key={item.id} article={item} />
            ))}
          </div>
        ) : (
          <p className="rounded-xl border border-border bg-muted/20 px-4 py-6 text-center text-sm text-muted-foreground">
            {TEXT.MATERIALS.RELATED_EMPTY}
          </p>
        )}
      </section>
    </div>
  );
}

function MaterialNotFound() {
  return (
    <div className="flex h-[60vh] flex-col items-center justify-center gap-4 px-4 text-center">
      <p className="text-muted-foreground">{TEXT.MATERIALS.NOT_FOUND}</p>
      <Link href={APP_ROUTES.MATERIALS} className="text-sm text-primary hover:underline">
        {TEXT.MATERIALS.BACK_TO_LIBRARY}
      </Link>
    </div>
  );
}
