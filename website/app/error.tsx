"use client";

import { RouteFeedback } from "@/modules/shared/route-feedback";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";

export default function ApplicationError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <RouteFeedback
      kind="error"
      eyebrow={TEXT.ROUTE_ERROR.EYEBROW}
      title={TEXT.ROUTE_ERROR.TITLE}
      description={TEXT.ROUTE_ERROR.DESCRIPTION}
      retryLabel={TEXT.ROUTE_ERROR.RETRY}
      onRetry={reset}
      homeHref={APP_ROUTES.ARENA}
      homeLabel={TEXT.ROUTE_ERROR.BACK_HOME}
    />
  );
}
