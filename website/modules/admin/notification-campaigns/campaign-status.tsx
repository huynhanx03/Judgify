import {
  Ban,
  CheckCircle2,
  CircleDashed,
  Clock3,
  LoaderCircle,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { CAMPAIGN_STATUS } from "@/constants/notification-campaign";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { cn } from "@/lib/utils";
import type { CampaignStatus } from "@/types/notification-campaign";

const presentation: Record<
  CampaignStatus,
  { label: string; icon: LucideIcon; className: string }
> = {
  [CAMPAIGN_STATUS.DRAFT]: {
    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.DRAFT,
    icon: CircleDashed,
    className: "border-border bg-muted text-muted-foreground",
  },
  [CAMPAIGN_STATUS.SCHEDULED]: {
    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.SCHEDULED,
    icon: Clock3,
    className: "border-info/30 bg-info/10 text-info",
  },
  [CAMPAIGN_STATUS.SENDING]: {
    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.SENDING,
    icon: LoaderCircle,
    className: "border-warning/30 bg-warning/10 text-warning",
  },
  [CAMPAIGN_STATUS.COMPLETED]: {
    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.COMPLETED,
    icon: CheckCircle2,
    className: "border-success/30 bg-success/10 text-success",
  },
  [CAMPAIGN_STATUS.FAILED]: {
    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.FAILED,
    icon: TriangleAlert,
    className: "border-destructive/35 bg-destructive/10 text-destructive",
  },
  [CAMPAIGN_STATUS.CANCELLED]: {
    label: ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.STATUS.CANCELLED,
    icon: Ban,
    className: "border-border bg-muted text-muted-foreground",
  },
};

export function CampaignStatusBadge({
  status,
  className,
}: {
  status: CampaignStatus;
  className?: string;
}) {
  const current = presentation[status];
  const Icon = current.icon;
  return (
    <Badge
      variant="outline"
      className={cn("gap-1.5 whitespace-nowrap", current.className, className)}
    >
      <Icon
        className={cn(
          "size-3.5",
          status === CAMPAIGN_STATUS.SENDING &&
            "animate-spin motion-reduce:animate-none",
        )}
        aria-hidden="true"
      />
      {current.label}
    </Badge>
  );
}
