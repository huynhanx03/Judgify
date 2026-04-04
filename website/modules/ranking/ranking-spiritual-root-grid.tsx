/**
 * 5-column grid for the 5 primary spiritual roots (Ngũ Hành Linh Căn).
 * Kim - Mộc - Thủy - Hỏa - Thổ displayed as equal columns.
 * Responsive: 5 cols desktop, 3 cols tablet, 2 cols mobile.
 */

import { Sparkles, User } from "lucide-react";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { TEXT } from "@/constants/text";
import type { Cultivator, SpiritualRoot } from "@/types/ranking";
import { SPIRITUAL_ROOT_MAP } from "@/types/ranking";

/** Only the 5 primary roots (Ngũ Hành) */
const PRIMARY_ROOTS: SpiritualRoot[] = ["kim", "moc", "thuy", "hoa", "tho"];

interface RankingSpiritualRootGridProps {
  data: Record<SpiritualRoot, Cultivator[]>;
}

export function RankingSpiritualRootGrid({ data }: RankingSpiritualRootGridProps) {
  return (
    <div className="space-y-6 max-w-7xl mx-auto px-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Sparkles className="h-6 w-6 text-purple-400" />
        <h2 className="text-2xl font-black tracking-tighter uppercase">{TEXT.RANKING.TAB_SPIRITUAL_ROOT}</h2>
        <div className="flex-1 h-px bg-gradient-to-r from-purple-500/30 to-transparent" />
      </div>

      {/* 5-column grid */}
      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-5 gap-4 items-stretch">
        {PRIMARY_ROOTS.map((rootKey) => {
          const rootInfo = SPIRITUAL_ROOT_MAP[rootKey];
          const cultivators = (data[rootKey] ?? []).slice(0, 10);

          return (
            <div
              key={rootKey}
              className={`rounded-2xl border ${rootInfo.borderColor} ${rootInfo.bgColor} backdrop-blur-xl overflow-hidden shadow-md hover:shadow-lg transition-all hover:scale-[1.02] duration-300 flex flex-col`}
            >
              {/* Element header */}
              <div className={`px-4 py-3 border-b ${rootInfo.borderColor} text-center`}>
                <span className="text-2xl block mb-1">{rootInfo.icon}</span>
                <span className={`font-black text-xs uppercase tracking-widest ${rootInfo.color}`}>
                  {rootInfo.name}
                </span>
              </div>

              {/* Cultivator list */}
              <div className="flex-1 py-1">
                {cultivators.length === 0 ? (
                  <p className="text-xs text-muted-foreground text-center py-6">Chưa có tu sĩ</p>
                ) : (
                  cultivators.map((user) => {
                    const isTop = user.rank <= 3;
                    return (
                      <div
                        key={user.name}
                        className="flex items-center gap-2 px-3 py-2 hover:bg-white/[0.03] transition-colors"
                      >
                        <span className={`font-mono text-[10px] font-black w-4 text-center ${isTop ? rootInfo.color : "text-zinc-600"}`}>
                          {user.rank}
                        </span>
                        <Avatar className={`h-6 w-6 shrink-0 ${isTop ? `border ${rootInfo.borderColor}` : "border border-border/20"}`}>
                          <AvatarFallback className={`text-[9px] font-bold ${isTop ? `${rootInfo.bgColor} ${rootInfo.color}` : "bg-muted/30 text-muted-foreground"}`}>
                            {user.name.charAt(0)}
                          </AvatarFallback>
                        </Avatar>
                        <p className={`text-xs truncate flex-1 ${isTop ? "font-black" : "font-medium text-muted-foreground"}`}>
                          {user.name}
                        </p>
                      </div>
                    );
                  })
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
