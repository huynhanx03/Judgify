"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";

/**
 * Last-resort boundary for failures in the root layout itself. It deliberately
 * avoids application providers so it remains renderable when those providers
 * are the source of the failure.
 */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Unrecoverable application error", error);
  }, [error]);

  return (
    <html lang="vi">
      <body className="flex min-h-dvh items-center justify-center bg-background px-6 text-foreground">
        <main className="max-w-md space-y-4 text-center">
          <p className="text-sm font-medium text-primary">
            {TEXT.ROUTE_ERROR.EYEBROW}
          </p>
          <h1 className="text-2xl font-semibold">{TEXT.ROUTE_ERROR.TITLE}</h1>
          <p className="text-muted-foreground">{TEXT.ROUTE_ERROR.DESCRIPTION}</p>
          <Button type="button" onClick={reset}>
            {TEXT.ROUTE_ERROR.RETRY}
          </Button>
        </main>
      </body>
    </html>
  );
}
