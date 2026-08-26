"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import {
  DEFAULT_THEME_PREFERENCE,
  THEME_PREFERENCES,
  type ThemePreference,
} from "@/design/tokens";
import { text, type TextKey } from "@/i18n/text";

const THEME_OPTIONS: ReadonlyArray<{
  value: ThemePreference;
  label: TextKey;
  icon: typeof Sun;
}> = [
  { value: "light", label: "THEME.LIGHT", icon: Sun },
  { value: "dark", label: "THEME.DARK", icon: Moon },
  { value: "system", label: "THEME.SYSTEM", icon: Monitor },
];

function isThemePreference(value: string): value is ThemePreference {
  return THEME_PREFERENCES.some((preference) => preference === value);
}

interface ThemeTogglePanelProps {
  theme: string | undefined;
  onValueChange: (value: string) => void;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export function ThemeTogglePanel({
  theme,
  onValueChange,
  defaultOpen,
  onOpenChange,
}: ThemeTogglePanelProps) {
  const preference =
    (theme as ThemePreference | undefined) ?? DEFAULT_THEME_PREFERENCE;
  const ActiveIcon =
    THEME_OPTIONS.find((option) => option.value === preference)?.icon ?? Monitor;

  return (
    <DropdownMenu defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={text("THEME.SELECT")}
            title={text("THEME.SELECT")}
          />
        }
      >
        <ActiveIcon className="size-5" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuLabel>{text("THEME.LABEL")}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={preference}
          onValueChange={(value) => {
            if (isThemePreference(value)) onValueChange(value);
          }}
        >
          {THEME_OPTIONS.map((option) => {
            const Icon = option.icon;
            return (
              <DropdownMenuRadioItem key={option.value} value={option.value}>
                <Icon className="size-4" aria-hidden="true" />
                {text(option.label)}
              </DropdownMenuRadioItem>
            );
          })}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
