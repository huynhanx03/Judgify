import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import { NotificationCenter } from "@/modules/notifications/notification-center";

export const metadata: Metadata = {
  title: `${TEXT.NOTIFICATION.TITLE} | ${TEXT.APP_NAME}`,
  description: TEXT.NOTIFICATION.DESCRIPTION,
};

export default function NotificationsPage() {
  return <NotificationCenter />;
}
