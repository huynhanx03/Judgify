"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { textForLocale } from "@/i18n/text";
import type { TextArguments, TextKey } from "@/i18n/types";

export type AppLanguage = "vi" | "en";
const STORAGE_KEY = "judgify-language";
type LocaleContextValue = {
  language: AppLanguage;
  setLanguage: (language: AppLanguage) => void;
  t: <Key extends TextKey>(key: Key, ...args: TextArguments<Key>) => string;
};
const LocaleContext = createContext<LocaleContextValue | null>(null);

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<AppLanguage>("vi");
  useEffect(() => {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved === "en" || saved === "vi") setLanguage(saved);
  }, []);
  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, language);
    document.documentElement.lang = language;
  }, [language]);
  const t = useCallback(<Key extends TextKey>(key: Key, ...args: TextArguments<Key>) =>
    textForLocale(language, key, ...args), [language]);
  const value = useMemo(() => ({ language, setLanguage, t }), [language, t]);
  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

export function useLocale() {
  const value = useContext(LocaleContext);
  if (!value) throw new Error("useLocale must be used inside LocaleProvider");
  return value;
}
