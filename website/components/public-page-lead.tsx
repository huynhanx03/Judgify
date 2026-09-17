import Image from "next/image";
import type { ComponentType, ReactNode } from "react";
import { PageHeader } from "@/components/page-header";
import { cn } from "@/lib/utils";

interface PublicPageLeadProps {
  title: string;
  art?: PublicPageLeadArt;
  eyebrow?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  icon?: ComponentType<{ className?: string; "aria-hidden"?: boolean }>;
  className?: string;
}

export type PublicPageLeadArt =
  | "arena"
  | "materials"
  | "contest"
  | "ranking"
  | "about";

const PAGE_LEAD_ART: Record<PublicPageLeadArt, string> = {
  arena: "/images/arena-page-lead-v1.png",
  materials: "/images/materials-page-lead-v1.png",
  contest: "/images/contest-page-lead-v1.png",
  ranking: "/images/ranking-page-lead-v1.png",
  about: "/images/about-page-lead-v1.png",
};

/**
 * Shared, restrained introduction for public product surfaces.
 * The illustration is intentionally decorative: page content and headings
 * remain the first visual anchor, while every discovery route keeps one shape.
 */
export function PublicPageLead({
  art = "materials",
  className,
  title,
  eyebrow,
  description,
  actions,
  icon,
}: PublicPageLeadProps) {
  return (
    <div
      className={cn(
        "relative isolate overflow-hidden",
        className,
      )}
    >
      <div className="pointer-events-none absolute inset-y-0 right-0 -z-10 w-[min(44vw,34rem)] max-sm:hidden">
        <Image
          src={PAGE_LEAD_ART[art]}
          alt=""
          fill
          priority
          sizes="(max-width: 640px) 0px, (max-width: 1400px) 44vw, 544px"
          className="object-cover object-right opacity-45 select-none"
        />
        <div
          className="absolute inset-0 bg-gradient-to-r from-background via-background/55 to-transparent"
          aria-hidden="true"
        />
      </div>
      <PageHeader
        title={title}
        eyebrow={eyebrow}
        description={description}
        actions={actions}
        icon={icon}
        className="relative w-full"
      />
    </div>
  );
}
