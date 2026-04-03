"use client"

import { TEXT } from "@/constants/text"
import type { ProblemStats, DiffStat } from "@/types/user"

interface StatsPanelProps {
  stats: ProblemStats
  /** All available difficulties (merged with solved counts, 0 if not solved) */
  difficulties: DiffStat[]
}

const DIFF_COLORS: Record<number, { stroke: string; text: string; bg: string; border: string }> = {
  1: { stroke: "#10b981", text: "text-emerald-500", bg: "bg-emerald-500/10", border: "border-emerald-500/20" },
  2: { stroke: "#f59e0b", text: "text-amber-500",   bg: "bg-amber-500/10",   border: "border-amber-500/20"   },
  3: { stroke: "#f43f5e", text: "text-rose-500",    bg: "bg-rose-500/10",    border: "border-rose-500/20"    },
  4: { stroke: "#a855f7", text: "text-purple-500",  bg: "bg-purple-500/10",  border: "border-purple-500/20"  },
  5: { stroke: "#06b6d4", text: "text-cyan-500",    bg: "bg-cyan-500/10",    border: "border-cyan-500/20"    },
}
const FALLBACK_COLOR = { stroke: "#6b7280", text: "text-slate-400", bg: "bg-slate-400/10", border: "border-slate-400/20" }
function diffColor(level: number) { return DIFF_COLORS[level] ?? FALLBACK_COLOR }

// circumference = 2π × r = 2π × 32 ≈ 201.06
const R = 32
const C = 2 * Math.PI * R

function DonutChart({ diffs, total }: { diffs: DiffStat[]; total: number }) {
  let offset = 0
  return (
    <div className="relative shrink-0">
      <svg width="80" height="80" viewBox="0 0 80 80">
        <circle cx="40" cy="40" r={R} fill="none" stroke="currentColor" strokeWidth="6" opacity="0.08" />
        {total === 0 ? null : diffs.map((d) => {
          const len = (d.solved_count / total) * C
          const off = offset
          offset += len
          const c = diffColor(d.level)
          return (
            <circle key={d.level} cx="40" cy="40" r={R}
              fill="none" stroke={c.stroke} strokeWidth="6"
              strokeDasharray={`${len} ${C}`} strokeDashoffset={-off}
              transform="rotate(-90 40 40)" strokeLinecap="round"
            />
          )
        })}
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-xl font-black text-primary leading-none">{total}</span>
        <span className="text-[9px] text-muted-foreground">{TEXT.STATS.BAI}</span>
      </div>
    </div>
  )
}

export function StatsPanel({ stats, difficulties }: StatsPanelProps) {
  const totalSolved = difficulties.reduce((s, d) => s + d.solved_count, 0)
  const acRate = stats.total_submissions > 0
    ? ((stats.accepted_count / stats.total_submissions) * 100).toFixed(1)
    : "0.0"

  return (
    <div className="space-y-5">
      {/* Donut + difficulty tiles */}
      <div className="flex items-center gap-4">
        <DonutChart diffs={difficulties} total={totalSolved} />
        <div className="flex-1 grid gap-2" style={{ gridTemplateColumns: `repeat(${difficulties.length || 3}, 1fr)` }}>
          {difficulties.map((d) => {
            const c = diffColor(d.level)
            const pct = totalSolved > 0 ? Math.round((d.solved_count / totalSolved) * 100) : 0
            return (
              <div key={d.level} className={`rounded-xl border p-3 text-center ${c.bg} ${c.border}`}>
                <p className={`text-2xl font-bold ${c.text}`}>{d.solved_count}</p>
                <p className="text-xs mt-1 text-muted-foreground">{d.name}</p>
                <p className={`text-xs mt-0.5 font-semibold ${c.text} opacity-70`}>{pct}%</p>
              </div>
            )
          })}
        </div>
      </div>

      {/* Submission counters */}
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg bg-muted/30 px-3 py-2 text-center">
          <span className="text-lg font-bold text-foreground">{stats.total_submissions}</span>
          <p className="text-[10px] text-muted-foreground">{TEXT.STATS.TOTAL_SUBMISSIONS}</p>
        </div>
        <div className="rounded-lg bg-muted/30 px-3 py-2 text-center">
          <span className="text-lg font-bold text-primary">{acRate}%</span>
          <p className="text-[10px] text-muted-foreground">{TEXT.STATS.AC_RATE}</p>
        </div>
      </div>
    </div>
  )
}
