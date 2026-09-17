import { RouteFeedback } from "@/modules/shared/route-feedback";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";

export default function NotFoundPage() {
  return (
    <RouteFeedback
      kind="not-found"
      eyebrow={TEXT.NOT_FOUND.EYEBROW}
      title={TEXT.NOT_FOUND.TITLE}
      description={TEXT.NOT_FOUND.DESCRIPTION}
      homeHref={APP_ROUTES.ARENA}
      homeLabel={TEXT.NOT_FOUND.BACK_HOME}
    />
  );
}
