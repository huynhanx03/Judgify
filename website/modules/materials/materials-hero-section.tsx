/**
 * Compact hero banner for the Knowledge Base (Tàng Kinh Các) page.
 */

import { TEXT } from "@/constants/text";
import { BookOpen } from "lucide-react";

export function MaterialsHeroSection() {
  return (
    <div className="border-b border-border bg-card/50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
        <div className="flex items-center gap-3 mb-3">
          <div className="p-2.5 rounded-xl bg-primary/10 text-primary">
            <BookOpen className="h-6 w-6" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-foreground">
            {TEXT.NAV.MATERIALS}
          </h1>
        </div>
        <p className="text-muted-foreground text-sm sm:text-base max-w-2xl leading-relaxed">
          Kho tài liệu học thuật từ cơ bản đến nâng cao, bao gồm cấu trúc dữ liệu,
          giải thuật, ngôn ngữ lập trình và thiết kế hệ thống.
        </p>
      </div>
    </div>
  );
}
