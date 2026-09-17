import { text } from "@/i18n/text";
import { cn } from "@/lib/utils";

export function SkipLink({ className }: { className?: string }) {
  return (
    <a
      href="#main-content"
      className={cn(
        "fixed left-4 top-4 z-[100] -translate-y-24 rounded-md bg-primary px-4 py-3 font-semibold text-primary-foreground shadow-lg transition-transform duration-150 focus:translate-y-0 motion-reduce:transition-none",
        className,
      )}
    >
      {text("NAV.SKIP_TO_CONTENT")}
    </a>
  );
}
