import {
  CAMPAIGN_AUDIENCE,
  CAMPAIGN_RECIPIENT_STATUS,
} from "@/constants/notification-campaign";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import type {
  CampaignAudience,
  CampaignRecipientCounts,
  CampaignRecipientStatus,
} from "@/types/notification-campaign";

export function campaignAudienceKindLabel(
  kind: CampaignAudience["kind"],
): string {
  switch (kind) {
    case CAMPAIGN_AUDIENCE.EXPLICIT_USERS:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.EXPLICIT_USERS;
    case CAMPAIGN_AUDIENCE.ROLE:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.ROLE;
    case CAMPAIGN_AUDIENCE.ALL_ACTIVE:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.ALL_ACTIVE;
  }
}

export function campaignAudienceLabel(audience: CampaignAudience): string {
  switch (audience.kind) {
    case CAMPAIGN_AUDIENCE.EXPLICIT_USERS:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.EXPLICIT_COUNT(
        audience.explicit_users?.length ?? 0,
      );
    case CAMPAIGN_AUDIENCE.ROLE:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.ROLE_ID(
        audience.role_id ?? TEXT.COMMON.UNKNOWN,
      );
    case CAMPAIGN_AUDIENCE.ALL_ACTIVE:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.ALL_ACTIVE;
  }
}

export function campaignRecipientStatusLabel(
  status: CampaignRecipientStatus,
): string {
  switch (status) {
    case CAMPAIGN_RECIPIENT_STATUS.PENDING:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.PENDING;
    case CAMPAIGN_RECIPIENT_STATUS.CREATED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.CREATED;
    case CAMPAIGN_RECIPIENT_STATUS.SKIPPED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.SKIPPED;
    case CAMPAIGN_RECIPIENT_STATUS.FAILED:
      return ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.RECIPIENT.STATUS.FAILED;
  }
}

export function campaignProcessedCount(
  counts: CampaignRecipientCounts,
): number {
  return counts.created + counts.skipped + counts.failed;
}
