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
  type LucideIcon,
} from "lucide-react";
import { TEXT } from "./text";

/** Single navigation menu item. */
export interface NavItem {
  label: string;
  href: string;
  icon: LucideIcon;
}

/** Main navigation items displayed in the sidebar. */
export const MAIN_NAV_ITEMS: NavItem[] = [
  {
    label: TEXT.NAV.ARENA,
    href: "/arena",
    icon: Swords,
  },
  {
    label: TEXT.NAV.MATERIALS,
    href: "/materials",
    icon: BookOpen,
  },
  {
    label: TEXT.NAV.CONTEST,
    href: "/contest",
    icon: Trophy,
  },
  {
    label: TEXT.NAV.RANKING,
    href: "/ranking",
    icon: Crown,
  },
];

/** Secondary navigation items (bottom of sidebar). */
export const SECONDARY_NAV_ITEMS: NavItem[] = [
  {
    label: TEXT.NAV.PROFILE,
    href: "/profile",
    icon: User,
  },
];
