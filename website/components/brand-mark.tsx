import Image from "next/image";
import { cn } from "@/lib/utils";

interface BrandMarkProps {
  className?: string;
  priority?: boolean;
}

/**
 * The sole visual treatment for Judgify's product mark. The mark intentionally
 * has no enclosing badge: clear space, not a container, gives it recognition.
 */
export function BrandMark({ className, priority = false }: BrandMarkProps) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        className,
      )}
    >
      <Image
        src="/images/judgify-mark.png"
        alt=""
        width={40}
        height={40}
        priority={priority}
        aria-hidden="true"
        className="size-full object-contain"
      />
    </span>
  );
}
