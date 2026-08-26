/**
 * Sidebar navigation menu items configuration.
 * Icons are referenced by name from lucide-react.
 */

import {
  Swords,
  BookOpen,
  Trophy,
  Crown,
  User,
  Info,
  type LucideIcon,
} from "lucide-react";
import { APP_ROUTES } from "./routes";
import { text, type TextKey } from "@/i18n/text";

/** Single navigation menu item. */
export interface NavItem {
  labelKey: TextKey;
  label: string;
  href: string;
  icon: LucideIcon;
}

/** Main navigation items displayed in the sidebar. */
export const MAIN_NAV_ITEMS: NavItem[] = [
  {
    labelKey: "NAV.ARENA",
    label: text("NAV.ARENA"),
    href: APP_ROUTES.ARENA,
    icon: Swords,
  },
  {
    labelKey: "NAV.MATERIALS",
    label: text("NAV.MATERIALS"),
    href: APP_ROUTES.MATERIALS,
    icon: BookOpen,
  },
  {
    labelKey: "NAV.CONTEST",
    label: text("NAV.CONTEST"),
    href: APP_ROUTES.CONTEST,
    icon: Trophy,
  },
  {
    labelKey: "NAV.RANKING",
    label: text("NAV.RANKING"),
    href: APP_ROUTES.RANKING,
    icon: Crown,
  },
  {
    labelKey: "NAV.ABOUT",
    label: text("NAV.ABOUT"),
    href: APP_ROUTES.ABOUT,
    icon: Info,
  },
];

/** Secondary navigation items (bottom of sidebar). */
export const SECONDARY_NAV_ITEMS: NavItem[] = [
  {
    labelKey: "NAV.PROFILE",
    label: text("NAV.PROFILE"),
    href: APP_ROUTES.PROFILE,
    icon: User,
  },
];
