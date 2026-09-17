import type { Metadata, Viewport } from "next";
import "./globals.css";
import { ThemeProvider } from "@/components/theme-provider";
import { AppProviders } from "@/components/app-providers";
import { AppToaster } from "@/components/app-toaster";
import { text } from "@/i18n/text";
import { APP_LANGUAGE } from "@/i18n/locale";
import { BROWSER_THEME_COLORS } from "@/design/tokens";

export const metadata: Metadata = {
  title: {
    default: text("APP_FULL_TITLE"),
    template: `%s | ${text("APP_NAME")}`,
  },
  description: text("APP_DESCRIPTION"),
  applicationName: text("APP_NAME"),
  formatDetection: {
    email: false,
    address: false,
    telephone: false,
  },
};

export const viewport: Viewport = {
  colorScheme: "light dark",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: BROWSER_THEME_COLORS.light },
    { media: "(prefers-color-scheme: dark)", color: BROWSER_THEME_COLORS.dark },
  ],
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang={APP_LANGUAGE} suppressHydrationWarning>
      <body
        className="min-h-dvh bg-background font-sans text-[15px] antialiased sm:text-base"
        suppressHydrationWarning
      >
        <ThemeProvider>
          <AppProviders>
            {children}
            <AppToaster />
          </AppProviders>
        </ThemeProvider>
      </body>
    </html>
  );
}
