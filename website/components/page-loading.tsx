import { cn } from "@/lib/utils";
import { text, type TextKey } from "@/i18n/text";

type PageLoadingVariant = "auth" | "content" | "workspace" | "admin";

const COPY_BY_VARIANT = {
  auth: "COMMON.AUTH_LOADING",
  content: "COMMON.PAGE_LOADING",
  workspace: "COMMON.WORKSPACE_LOADING",
  admin: "COMMON.ADMIN_LOADING",
} as const satisfies Record<PageLoadingVariant, TextKey>;

function SkeletonBlock({ className }: { className: string }) {
  return (
    <div
      aria-hidden="true"
      className={cn(
        "animate-pulse rounded-xl bg-muted/70 motion-reduce:animate-none",
        className,
      )}
    />
  );
}

/** Layout-preserving route fallback shared by App Router loading boundaries. */
export function PageLoading({
  variant = "content",
}: {
  variant?: PageLoadingVariant;
}) {
  const label = text(COPY_BY_VARIANT[variant]);

  if (variant === "auth") {
    return (
      <main
        id="main-content"
        className="flex min-h-screen items-center justify-center bg-background px-4 py-8"
        aria-busy="true"
        aria-label={label}
      >
        <div className="w-full max-w-md rounded-2xl border border-border bg-card p-6 shadow-xl sm:p-8">
          <span className="sr-only" role="status" aria-live="polite">
            {label}
          </span>
          <SkeletonBlock className="mx-auto size-14" />
          <SkeletonBlock className="mx-auto mt-5 h-7 w-48" />
          <SkeletonBlock className="mx-auto mt-3 h-4 w-64 max-w-full" />
          <div className="mt-8 space-y-5">
            <SkeletonBlock className="h-11 w-full" />
            <SkeletonBlock className="h-11 w-full" />
            <SkeletonBlock className="h-12 w-full" />
          </div>
        </div>
      </main>
    );
  }

  const admin = variant === "admin";
  const workspace = variant === "workspace";

  return (
    <div
      className={cn(
        "mx-auto w-full space-y-6",
        admin ? "max-w-[1600px] p-4 sm:p-6 lg:p-8" : "max-w-[1400px] p-4 py-8 sm:p-6 lg:p-8",
      )}
      aria-busy="true"
      aria-label={label}
    >
      <span className="sr-only" role="status" aria-live="polite">
        {label}
      </span>
      <div className="flex items-center gap-4">
        <SkeletonBlock className="size-11 shrink-0" />
        <div className="min-w-0 flex-1 space-y-2">
          <SkeletonBlock className="h-7 w-56 max-w-[70%]" />
          <SkeletonBlock className="h-4 w-96 max-w-full" />
        </div>
      </div>
      <div
        className={cn(
          "grid gap-4",
          workspace
            ? "min-h-[32rem] lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]"
            : admin
              ? "sm:grid-cols-2 xl:grid-cols-4"
              : "md:grid-cols-2 xl:grid-cols-3",
        )}
      >
        {workspace ? (
          <>
            <SkeletonBlock className="min-h-[24rem]" />
            <SkeletonBlock className="min-h-[24rem]" />
          </>
        ) : (
          Array.from({ length: admin ? 8 : 6 }, (_, index) => (
            <SkeletonBlock key={index} className="h-36" />
          ))
        )}
      </div>
    </div>
  );
}
