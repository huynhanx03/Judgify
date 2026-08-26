import Link from "next/link";
import { ArrowLeft, CloudOff, Loader2, RefreshCw, ShieldX } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { APP_ROUTES } from "@/constants/routes";

interface AdminAccessStateProps {
  kind: "forbidden" | "unavailable";
  onRetry?: () => void;
  fullScreen?: boolean;
}

export function AdminAccessState({
  kind,
  onRetry,
  fullScreen = false,
}: AdminAccessStateProps) {
  const unavailable = kind === "unavailable";
  const Icon = unavailable ? CloudOff : ShieldX;
  const label = unavailable
    ? ADMIN_TEXT.ACCESS.UNAVAILABLE_LABEL
    : ADMIN_TEXT.ACCESS.FORBIDDEN_CODE;
  const title = unavailable
    ? ADMIN_TEXT.ACCESS.UNAVAILABLE_TITLE
    : ADMIN_TEXT.ACCESS.FORBIDDEN_TITLE;
  const description = unavailable
    ? ADMIN_TEXT.ACCESS.UNAVAILABLE_DESCRIPTION
    : ADMIN_TEXT.ACCESS.FORBIDDEN_DESCRIPTION;

  return (
    <div
      className={
        fullScreen
          ? "flex min-h-screen items-center justify-center bg-background p-6"
          : "flex min-h-[calc(100vh-8rem)] items-center justify-center p-4"
      }
      role="alert"
      aria-live="polite"
    >
      <Card className="relative w-full max-w-lg overflow-hidden border-border/80 bg-card/90 shadow-xl">
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/70 to-transparent" />
        <CardContent className="flex flex-col items-center px-6 py-12 text-center sm:px-12">
          <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl border border-primary/15 bg-primary/10 text-primary ring-8 ring-primary/5">
            <Icon className="h-7 w-7" aria-hidden="true" />
          </div>
          <p className="mb-3 text-xs font-semibold uppercase tracking-[0.2em] text-primary">
            {label}
          </p>
          <h1 className="text-balance text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            {title}
          </h1>
          <p className="mt-3 max-w-md text-pretty text-sm leading-6 text-muted-foreground">
            {description}
          </p>
          <div className="mt-8 flex flex-col-reverse gap-3 sm:flex-row">
            <Link href={APP_ROUTES.ARENA} className={buttonVariants({ variant: "outline" })}>
              <ArrowLeft className="h-4 w-4" />
              {ADMIN_TEXT.ACCESS.BACK_TO_PRODUCT}
            </Link>
            {unavailable && onRetry ? (
              <Button onClick={onRetry}>
                <RefreshCw className="h-4 w-4" />
                {ADMIN_TEXT.ACCESS.RETRY}
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export function AdminAccessLoading({ fullScreen = false }: { fullScreen?: boolean }) {
  return (
    <div
      className={
        fullScreen
          ? "flex min-h-screen items-center justify-center bg-background"
          : "flex min-h-[calc(100vh-8rem)] items-center justify-center"
      }
      aria-live="polite"
    >
      <div className="flex items-center gap-3 rounded-full border border-border bg-card px-5 py-3 text-sm text-muted-foreground shadow-sm">
        <Loader2 className="h-4 w-4 animate-spin text-primary motion-reduce:animate-none" aria-hidden="true" />
        <span>{ADMIN_TEXT.ACCESS.LOADING}</span>
      </div>
    </div>
  );
}
