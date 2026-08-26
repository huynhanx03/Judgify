/**
 * Admin sidebar navigation configuration.
 */

import {
  LayoutDashboard,
  Users,
  FileCode2,
  Tags,
  ShieldCheck,
  Flame,
  Sparkles,
  Layers,
  Trophy,
  BarChart3,
  Gem,
  Swords,
  BookOpenText,
  FolderTree,
  Activity,
  ScrollText,
  ServerCog,
  Workflow,
  Megaphone,
  Bug,
  type LucideIcon,
} from "lucide-react";
import { adminText, type AdminTextKey } from "@/i18n/admin-text";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "./authorization";
import type { CapabilityRequirement } from "@/types/auth";
import { APP_ROUTES } from "./routes";

export interface AdminNavItem {
  labelKey: AdminTextKey;
  label: string;
  href: string;
  icon: LucideIcon;
  requirements?: readonly CapabilityRequirement[];
  requiresAnyCapability?: boolean;
}

export interface AdminNavSection {
  titleKey: AdminTextKey;
  title: string;
  items: AdminNavItem[];
}

const read = (resource: string): readonly CapabilityRequirement[] => [
  { resource, action: AUTHORIZATION_ACTION.READ },
];

export const ADMIN_NAV_SECTIONS: AdminNavSection[] = [
  {
    titleKey: "NAV.OVERVIEW",
    title: adminText("NAV.OVERVIEW"),
    items: [
      {
        labelKey: "DASHBOARD_TITLE",
        label: adminText("DASHBOARD_TITLE"),
        href: APP_ROUTES.ADMIN,
        icon: LayoutDashboard,
        requiresAnyCapability: true,
      },
    ],
  },
  {
    titleKey: "NAV.CONTENT",
    title: adminText("NAV.CONTENT"),
    items: [
      {
        labelKey: "PROBLEMS.TITLE",
        label: adminText("PROBLEMS.TITLE"),
        href: APP_ROUTES.ADMIN_PROBLEMS,
        icon: FileCode2,
        requirements: read(AUTHORIZATION_RESOURCE.PROBLEM),
      },
      {
        labelKey: "TAGS.TITLE",
        label: adminText("TAGS.TITLE"),
        href: APP_ROUTES.ADMIN_TAGS,
        icon: Tags,
        requirements: read(AUTHORIZATION_RESOURCE.TAG),
      },
      {
        labelKey: "DIFFICULTIES.TITLE",
        label: adminText("DIFFICULTIES.TITLE"),
        href: APP_ROUTES.ADMIN_DIFFICULTIES,
        icon: BarChart3,
        requirements: read(AUTHORIZATION_RESOURCE.DIFFICULTY),
      },
      {
        labelKey: "CONTESTS.TITLE",
        label: adminText("CONTESTS.TITLE"),
        href: APP_ROUTES.ADMIN_CONTESTS,
        icon: Swords,
        requirements: read(AUTHORIZATION_RESOURCE.CONTEST),
      },
      {
        labelKey: "MATERIALS.TITLE",
        label: adminText("MATERIALS.TITLE"),
        href: APP_ROUTES.ADMIN_MATERIALS,
        icon: BookOpenText,
        requirements: read(AUTHORIZATION_RESOURCE.MATERIAL),
      },
      {
        labelKey: "MATERIAL_CATEGORIES.TITLE",
        label: adminText("MATERIAL_CATEGORIES.TITLE"),
        href: APP_ROUTES.ADMIN_MATERIAL_CATEGORIES,
        icon: FolderTree,
        requirements: read(AUTHORIZATION_RESOURCE.MATERIAL_CATEGORY),
      },
    ],
  },
  {
    titleKey: "NAV.OPERATIONS",
    title: adminText("NAV.OPERATIONS"),
    items: [
      {
        labelKey: "AUDIT.TITLE",
        label: adminText("AUDIT.TITLE"),
        href: APP_ROUTES.ADMIN_AUDIT,
        icon: ScrollText,
        requirements: read(AUTHORIZATION_RESOURCE.AUDIT),
      },
      {
        labelKey: "SUBMISSIONS.TITLE",
        label: adminText("SUBMISSIONS.TITLE"),
        href: APP_ROUTES.ADMIN_SUBMISSIONS,
        icon: Activity,
        requirements: read(AUTHORIZATION_RESOURCE.SUBMISSION),
      },
      {
        labelKey: "JUDGE.TITLE",
        label: adminText("JUDGE.TITLE"),
        href: APP_ROUTES.ADMIN_JUDGE,
        icon: ServerCog,
        requirements: read(AUTHORIZATION_RESOURCE.JUDGE),
      },
      {
        labelKey: "OPERATIONS.TITLE",
        label: adminText("OPERATIONS.TITLE"),
        href: APP_ROUTES.ADMIN_OPERATIONS,
        icon: Workflow,
        requirements: read(AUTHORIZATION_RESOURCE.OPERATION),
      },
      {
        labelKey: "OBSERVABILITY.TITLE",
        label: adminText("OBSERVABILITY.TITLE"),
        href: APP_ROUTES.ADMIN_OBSERVABILITY,
        icon: Bug,
        requirements: read(AUTHORIZATION_RESOURCE.OBSERVABILITY),
      },
      {
        labelKey: "NOTIFICATION_CAMPAIGNS.TITLE",
        label: adminText("NOTIFICATION_CAMPAIGNS.TITLE"),
        href: APP_ROUTES.ADMIN_NOTIFICATION_CAMPAIGNS,
        icon: Megaphone,
        requirements: [
          {
            resource: AUTHORIZATION_RESOURCE.NOTIFICATION,
            action: AUTHORIZATION_ACTION.COMPOSE,
          },
        ],
      },
    ],
  },
  {
    titleKey: "NAV.CULTIVATION",
    title: adminText("NAV.CULTIVATION"),
    items: [
      {
        labelKey: "ELEMENTS.TITLE",
        label: adminText("ELEMENTS.TITLE"),
        href: APP_ROUTES.ADMIN_ELEMENTS,
        icon: Flame,
        requirements: read(AUTHORIZATION_RESOURCE.ELEMENT),
      },
      {
        labelKey: "RARITIES.TITLE",
        label: adminText("RARITIES.TITLE"),
        href: APP_ROUTES.ADMIN_RARITIES,
        icon: Gem,
        requirements: read(AUTHORIZATION_RESOURCE.RARITY),
      },
      {
        labelKey: "TRAITS.TITLE",
        label: adminText("TRAITS.TITLE"),
        href: APP_ROUTES.ADMIN_TRAITS,
        icon: Sparkles,
        requirements: read(AUTHORIZATION_RESOURCE.TRAIT),
      },
      {
        labelKey: "LEVELS.TITLE",
        label: adminText("LEVELS.TITLE"),
        href: APP_ROUTES.ADMIN_LEVELS,
        icon: Layers,
        requirements: read(AUTHORIZATION_RESOURCE.LEVEL),
      },
      {
        labelKey: "RANKS.TITLE",
        label: adminText("RANKS.TITLE"),
        href: APP_ROUTES.ADMIN_RANKS,
        icon: Trophy,
        requirements: read(AUTHORIZATION_RESOURCE.RANK),
      },
      {
        labelKey: "USER_TRAITS.TITLE",
        label: adminText("USER_TRAITS.TITLE"),
        href: APP_ROUTES.ADMIN_USER_TRAITS,
        icon: Sparkles,
        requirements: [
          { resource: AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE, action: AUTHORIZATION_ACTION.READ },
          { resource: AUTHORIZATION_RESOURCE.USER, action: AUTHORIZATION_ACTION.READ },
        ],
      },
      {
		labelKey: "PROGRESSION.TITLE",
		label: adminText("PROGRESSION.TITLE"),
		href: APP_ROUTES.ADMIN_PROGRESSION,
		icon: ScrollText,
		requirements: [
		  { resource: AUTHORIZATION_RESOURCE.CULTIVATION_LEDGER, action: AUTHORIZATION_ACTION.READ },
		  { resource: AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE, action: AUTHORIZATION_ACTION.READ },
		],
		requiresAnyCapability: true,
	  },
    ],
  },
  {
    titleKey: "NAV.SYSTEM",
    title: adminText("NAV.SYSTEM"),
    items: [
      {
        labelKey: "USERS_TITLE",
        label: adminText("USERS_TITLE"),
        href: APP_ROUTES.ADMIN_USERS,
        icon: Users,
        requirements: read(AUTHORIZATION_RESOURCE.USER),
      },
      {
        labelKey: "ROLES_TITLE",
        label: adminText("ROLES_TITLE"),
        href: APP_ROUTES.ADMIN_ROLES,
        icon: ShieldCheck,
        requirements: [
          {
            resource: AUTHORIZATION_RESOURCE.ROLE,
            action: AUTHORIZATION_ACTION.READ,
          },
          {
            resource: AUTHORIZATION_RESOURCE.AUTHORIZATION,
            action: AUTHORIZATION_ACTION.MANAGE,
          },
        ],
      },
      {
        labelKey: "ATTRIBUTE_DEFINITIONS.TITLE",
        label: adminText("ATTRIBUTE_DEFINITIONS.TITLE"),
        href: APP_ROUTES.ADMIN_ATTRIBUTE_DEFINITIONS,
        icon: Tags,
        requirements: read(AUTHORIZATION_RESOURCE.ATTRIBUTE_DEFINITION),
      },
    ],
  },
];
