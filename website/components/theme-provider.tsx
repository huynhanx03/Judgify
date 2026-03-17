"use client";

/**
 * Theme provider component wrapping next-themes.
 * Enables dark/light mode switching across the entire app.
 */

import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ComponentProps } from "react";

/** Wraps the app with next-themes ThemeProvider for dark/light mode support. */
export function ThemeProvider({
  children,
  ...props
}: ComponentProps<typeof NextThemesProvider>) {
  return <NextThemesProvider {...props}>{children}</NextThemesProvider>;
}
