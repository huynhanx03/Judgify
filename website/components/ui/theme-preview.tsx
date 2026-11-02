"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { useSyncExternalStore } from "react";

import { text } from "@/i18n/text";
import {
  DEFAULT_THEME_PREFERENCE,
  THEME_PREFERENCES,
  type ThemePreference,
} from "@/design/tokens";
import { cn } from "@/lib/utils";

const subscribeToHydration = () => () => undefined;

const THEME_META = {
  light: { label: "THEME.LIGHT", icon: Sun },
  dark: { label: "THEME.DARK", icon: Moon },
  system: { label: "THEME.SYSTEM", icon: Monitor },
} as const;

export function ThemePreview({ className }: { className?: string }) {
  const { theme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(
    subscribeToHydration,
    () => true,
    () => false,
  );
  const preference = mounted
    ? (theme as ThemePreference | undefined) ?? DEFAULT_THEME_PREFERENCE
    : DEFAULT_THEME_PREFERENCE;

  return (
    <fieldset className={cn("space-y-3", className)} disabled={!mounted}>
      <legend className="text-sm font-semibold text-foreground">
        {text("THEME.LABEL")}
      </legend>
      <div className="grid gap-2 sm:grid-cols-3">
        {THEME_PREFERENCES.map((choice) => {
          const Icon = THEME_META[choice].icon;
          const selected = preference === choice;

          return (
            <button
              key={choice}
              type="button"
              className={cn(
                "flex min-h-11 items-center gap-3 rounded-lg border bg-surface px-3 text-left text-sm transition-colors duration-200",
                "hover:border-border-strong hover:bg-accent focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                selected && "border-primary bg-primary/10 text-primary",
              )}
              aria-pressed={selected}
              onClick={() => setTheme(choice)}
            >
              <Icon className="size-4" aria-hidden="true" />
              <span>{text(THEME_META[choice].label)}</span>
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}
