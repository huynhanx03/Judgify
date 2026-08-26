import type {
  NotificationCategory,
  NotificationContentKind,
} from "@/constants/notification";
import type { Cursor, EntityID, ISODateTime } from "@/types/api";

export type NotificationParameter = string | number | boolean | null;
export type NotificationParameters = Readonly<
  Record<string, NotificationParameter>
>;

export interface Notification {
  id: EntityID;
  type: string;
  category: NotificationCategory;
  content_kind: NotificationContentKind;
  title_key?: string;
  body_key?: string;
  params?: NotificationParameters;
  campaign_revision_id?: EntityID;
  campaign_title?: string;
  action_path?: string;
  version: number;
  created_at: ISODateTime;
  read_at?: ISODateTime;
  archived_at?: ISODateTime;
  expires_at?: ISODateTime;
}

export interface NotificationCampaignContent {
  notification_id: EntityID;
  revision_id: EntityID;
  title: string;
  sanitized_html: string;
  action_path?: string;
}

export interface NotificationListQuery {
  cursor?: string;
  limit?: number;
  includeArchived?: boolean;
}

export interface NotificationListResponse {
  items: Notification[];
  next_cursor?: Cursor;
  unread_count: number;
  inbox_version: number;
}

export interface NotificationUnreadResponse {
  count: number;
  version: number;
}

export interface NotificationMutationResponse {
  notification?: Notification;
  unread_count: number;
  inbox_version: number;
  changed: boolean;
}

export interface NotificationPreference {
  category: NotificationCategory;
  in_app: boolean;
  required: boolean;
  version: number;
}

export interface UpdateNotificationPreferenceRequest {
  in_app: boolean;
  expected_version: number;
}

export interface NotificationLivePayload {
  user_id: EntityID;
  notification?: Notification;
  unread_count: number;
  inbox_version: number;
}
