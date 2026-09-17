/** Stable theme preferences persisted by next-themes. */
export const THEME_PREFERENCES = ["light", "dark", "system"] as const;

export type ThemePreference = (typeof THEME_PREFERENCES)[number];

export const DEFAULT_THEME_PREFERENCE: ThemePreference = "system";
export const THEME_STORAGE_KEY = "judgify-theme";
export const BROWSER_THEME_COLORS = {
  light: "#f8fafc",
  dark: "#08111f",
} as const;

/**
 * Public token contract for component authors and architecture tests.
 * Values live in globals.css so Tailwind and non-React surfaces share them.
 */
export const SEMANTIC_COLOR_TOKENS = [
  "background",
  "foreground",
  "surface",
  "surface-raised",
  "surface-sunken",
  "border",
  "border-strong",
  "muted",
  "muted-foreground",
  "accent",
  "accent-foreground",
  "focus-ring",
  "success",
  "success-foreground",
  "warning",
  "warning-foreground",
  "danger",
  "danger-foreground",
  "info",
  "info-foreground",
  "cultivation",
  "cultivation-foreground",
  "code-background",
  "editor-gutter",
] as const;

export type SemanticColorToken = (typeof SEMANTIC_COLOR_TOKENS)[number];

export const COMPONENT_TOKENS = [
  "control-height",
  "control-radius",
  "button-background",
  "button-background-hover",
  "button-foreground",
  "input-background",
  "input-border",
  "card-background",
  "card-border",
  "dialog-background",
  "dialog-overlay",
  "table-row-hover",
  "tooltip-background",
  "tooltip-foreground",
] as const;

export type ComponentToken = (typeof COMPONENT_TOKENS)[number];
