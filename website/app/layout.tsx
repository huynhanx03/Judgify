import type { Metadata } from "next";
import { Inter, Russo_One, Playfair_Display } from "next/font/google";
import "./globals.css";
import { ThemeProvider } from "@/components/theme-provider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppProviders } from "@/components/app-providers";
import { AppToaster } from "@/components/app-toaster";
import { TEXT } from "@/constants/text";

const inter = Inter({
  variable: "--font-sans",
  subsets: ["latin", "vietnamese"],
});

const playfair = Playfair_Display({
  variable: "--font-playfair",
  weight: ["400", "500", "600", "700"],
  style: ["normal", "italic"],
  subsets: ["latin", "vietnamese"],
});

const russoOne = Russo_One({
  weight: "400",
  variable: "--font-heading",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: {
    default: TEXT.APP_FULL_TITLE,
    template: `%s | ${TEXT.APP_NAME}`,
  },
  description: TEXT.APP_DESCRIPTION,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="vi" suppressHydrationWarning>
      <body className={`${inter.variable} ${russoOne.variable} ${playfair.variable} font-sans antialiased text-[15px] sm:text-base`} suppressHydrationWarning>
        <ThemeProvider
          attribute="class"
          defaultTheme="dark"
          forcedTheme="dark"
          enableSystem={false}
          disableTransitionOnChange
        >
          <AppProviders>
            <TooltipProvider>{children}</TooltipProvider>
            <AppToaster />
          </AppProviders>
        </ThemeProvider>
      </body>
    </html>
  );
}
