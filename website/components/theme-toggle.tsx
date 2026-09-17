"use client";

import { useCallback, useState, useSyncExternalStore } from "react";
import { useTheme } from "next-themes";
import dynamic from "next/dynamic";
import { Monitor, Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DEFAULT_THEME_PREFERENCE,
  THEME_PREFERENCES,
  type ThemePreference,
} from "@/design/tokens";
import { text } from "@/i18n/text";

const ThemeTogglePanel = dynamic(
  () =>
    import("@/components/theme-toggle-panel").then((m) => m.ThemeTogglePanel),
  { ssr: false },
);

function preloadThemeTogglePanel() {
  void import("@/components/theme-toggle-panel");
}

const subscribeToHydration = () => () => undefined;

const THEME_ICONS = {
  light: Sun,
  dark: Moon,
  system: Monitor,
} as const satisfies Record<ThemePreference, typeof Sun>;

function isThemePreference(value: string): value is ThemePreference {
  return THEME_PREFERENCES.some((preference) => preference === value);
}

/** Hydration-safe, explicit light/dark/system preference selector. */
export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const [panelOpen, setPanelOpen] = useState(false);

  const mounted = useSyncExternalStore(
    subscribeToHydration,
    () => true,
    () => false,
  );

  const handleClick = useCallback(() => {
    preloadThemeTogglePanel();
    setPanelOpen(true);
  }, []);

  const handlePreload = useCallback(() => {
    preloadThemeTogglePanel();
  }, []);

  const handleValueChange = useCallback(
    (value: string) => {
      if (isThemePreference(value)) setTheme(value);
    },
    [setTheme],
  );

  const handlePanelOpenChange = useCallback((open: boolean) => {
    if (!open) setPanelOpen(false);
  }, []);

  if (!mounted) {
    return <div className="size-11" aria-hidden="true" />;
  }

  const preference =
    (theme as ThemePreference | undefined) ?? DEFAULT_THEME_PREFERENCE;
  const ActiveIcon = THEME_ICONS[preference] ?? Monitor;

  if (panelOpen) {
    return (
      <ThemeTogglePanel
        theme={theme}
        onValueChange={handleValueChange}
        defaultOpen
        onOpenChange={handlePanelOpenChange}
      />
    );
  }

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={text("THEME.SELECT")}
      title={text("THEME.SELECT")}
      onClick={handleClick}
      onPointerEnter={handlePreload}
      onFocus={handlePreload}
    >
      <ActiveIcon className="size-5" aria-hidden="true" />
    </Button>
  );
}
