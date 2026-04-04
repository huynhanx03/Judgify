"use client"

import { useState } from "react"
import { Sparkles, Star } from "lucide-react"
import type { CultivationInfo, ElementExp } from "@/types/user"
import { ELEMENT_DISPLAY, getTierColors } from "@/types/cultivation"
import { TEXT } from "@/constants/text"

interface CultivationPanelProps {
  cultivation: CultivationInfo
}

// ── Radar chart constants ──────────────────────────────────────────────────
const SVG_SIZE = 220
const CENTER = SVG_SIZE / 2
const RADIUS = 80
const GRID_LEVELS = 4

function vertex(index: number, total: number, radius: number): [number, number] {
  const angle = (Math.PI * 2 * index) / total - Math.PI / 2
  return [CENTER + radius * Math.cos(angle), CENTER + radius * Math.sin(angle)]
}

function polygonPoints(count: number, radius: number): string {
  return Array.from({ length: count }, (_, i) => vertex(i, count, radius).join(",")).join(" ")
}

function normalizeElements(elements: ElementExp[]): Array<ElementExp & { value: number }> {
  const max = Math.max(...elements.map(e => Number(e.exp)), 1)
  return elements.map(e => ({ ...e, value: Math.round((Number(e.exp) / max) * 100) }))
}


// ── Tier progress row ──────────────────────────────────────────────────────
interface TierRowProps {
  label: string
  icon: React.ReactNode
  statValue: string | number
  statColorClass: string
  current: string
  next?: string
  nextBadgeClass?: string
  progress: number
  hint?: React.ReactNode
  badgeClass: string
  barClass: string
}

function TierRow({
  label, icon, statValue, statColorClass,
  current, next, nextBadgeClass, progress, hint,
  badgeClass, barClass,
}: TierRowProps) {
  const pct = Math.min(Math.max(progress, 0), 100)

  return (
    <div className="space-y-2.5">
      {/* Header: icon + label | stat value */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {icon}
          <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
        </div>
        <span className={`text-sm font-bold ${statColorClass}`}>
          {typeof statValue === "number" ? statValue.toLocaleString() : statValue}
        </span>
      </div>

      {/* Middle: badge | progress bar | next name */}
      <div className="flex items-center gap-2">
        <span className={`text-[10px] font-bold px-2 py-0.5 rounded-md shrink-0 ${badgeClass}`}>
          {current || "—"}
        </span>
        <div className="flex-1 h-2.5 rounded-full bg-muted/50 overflow-hidden">
          <div
            className={`h-full rounded-full transition-all duration-1000 bg-gradient-to-r ${barClass}`}
            style={{ width: `${pct}%` }}
          />
        </div>
        {next ? (
          <span className={`text-[10px] font-bold px-2 py-0.5 rounded-md shrink-0 opacity-60 ${nextBadgeClass}`}>
            {next}
          </span>
        ) : (
          <span className="text-[10px] text-amber-400/80 font-semibold shrink-0">
            {TEXT.PROFILE.CULTIVATION_MAX_RANK}
          </span>
        )}
      </div>

      {hint && <p className="text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  )
}

// ── Element radar chart ────────────────────────────────────────────────────
function ElementRadar({ elements }: { elements: ElementExp[] }) {
  const [hovered, setHovered] = useState<number | null>(null)
  const normalized = normalizeElements(elements)
  const count = normalized.length

  const dataPoints = normalized
    .map((el, i) => vertex(i, count, (el.value / 100) * RADIUS).join(","))
    .join(" ")

  return (
    <svg viewBox={`0 0 ${SVG_SIZE} ${SVG_SIZE}`} className="w-full max-w-[200px] h-auto mx-auto">
      <defs>
        <radialGradient id="radarGlow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.08" />
          <stop offset="100%" stopColor="rgb(245,158,11)" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="radarFill" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.25" />
          <stop offset="100%" stopColor="rgb(99,102,241)" stopOpacity="0.25" />
        </linearGradient>
        <filter id="dotGlow">
          <feGaussianBlur stdDeviation="2" result="blur" />
          <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
      </defs>

      <circle cx={CENTER} cy={CENTER} r={RADIUS + 14} fill="url(#radarGlow)" />

      {Array.from({ length: GRID_LEVELS }, (_, i) => (
        <polygon key={i}
          points={polygonPoints(count, ((i + 1) / GRID_LEVELS) * RADIUS)}
          fill="none" stroke="currentColor" strokeWidth="0.5"
          opacity={i === GRID_LEVELS - 1 ? 0.12 : 0.06}
        />
      ))}

      {normalized.map((_, i) => {
        const [vx, vy] = vertex(i, count, RADIUS)
        return <line key={i} x1={CENTER} y1={CENTER} x2={vx} y2={vy} stroke="currentColor" strokeWidth="0.5" opacity="0.08" />
      })}

      <polygon points={dataPoints} fill="url(#radarFill)" stroke="rgb(245,158,11)" strokeWidth="1.5" strokeLinejoin="round" opacity="0.9" />

      {normalized.map((el, i) => {
        const display = ELEMENT_DISPLAY[el.code]
        const hex = display?.hex ?? "#d97706"
        const [dx, dy] = vertex(i, count, (el.value / 100) * RADIUS)
        const [lx, ly] = vertex(i, count, RADIUS + 22)
        const isHovered = hovered === i

        return (
          <g key={el.code} onMouseEnter={() => setHovered(i)} onMouseLeave={() => setHovered(null)} className="cursor-pointer">
            <circle cx={dx} cy={dy} r="14" fill="transparent" />
            {isHovered && <circle cx={dx} cy={dy} r="10" fill={hex} opacity="0.15" filter="url(#dotGlow)" />}
            <circle cx={dx} cy={dy} r={isHovered ? 5.5 : 3.5} fill={hex} stroke="rgb(9,9,11)" strokeWidth="1.5"
              filter={isHovered ? "url(#dotGlow)" : undefined} />
            <text x={lx} y={ly} textAnchor="middle" dominantBaseline="central"
              fill={isHovered ? hex : "currentColor"} fontSize="12"
              fontWeight={isHovered ? "800" : "500"} opacity={isHovered ? 1 : 0.45}>
              {display?.icon ?? el.code}
            </text>
            {isHovered && (
              <g>
                <rect x={dx - 42} y={dy - 18} width="84" height="36" rx="7"
                  fill="rgb(24,24,27)" stroke={hex} strokeWidth="1" opacity="0.97" />
                <text x={dx} y={dy - 5} textAnchor="middle" fill={hex} fontSize="9.5" fontWeight="bold">{el.name}</text>
                <text x={dx} y={dy + 9} textAnchor="middle" fill="#71717a" fontSize="8.5">{Number(el.exp).toLocaleString()} EXP</text>
              </g>
            )}
          </g>
        )
      })}
    </svg>
  )
}

// ── Main panel ─────────────────────────────────────────────────────────────
export function CultivationPanel({ cultivation }: CultivationPanelProps) {
  // Cảnh Giới = rank (rating-based), Cấp Bậc = level (exp-based)
  const { level, rank, elements, total_exp, rating } = cultivation

  const rankColors = getTierColors(rank.tier_index)
  const levelColors = getTierColors(level.tier_index)
  const nextRankColors = rank.next_tier_index != null ? getTierColors(rank.next_tier_index) : null
  const nextLevelColors = level.next_tier_index != null ? getTierColors(level.next_tier_index) : null

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 items-start">
      {/* Left: tier progress */}
      <div className="space-y-5">
        {/* Cảnh Giới — icon & stat fixed amber */}
        <TierRow
          label={TEXT.PROFILE.CULTIVATION_REALM}
          icon={<Sparkles className="h-4 w-4 text-amber-400" />}
          statValue={rating}
          statColorClass="text-amber-400"
          current={rank.name}
          next={rank.next_name}
          nextBadgeClass={nextRankColors?.badge}
          progress={rank.progress}
          badgeClass={rankColors.badge}
          barClass={rankColors.bar}
          hint={rank.rating_to_next > 0 ? (
            <>Cần thêm <span className="font-bold text-foreground">{rank.rating_to_next}</span> {TEXT.PROFILE.CULTIVATION_RATING_TO_NEXT}</>
          ) : undefined}
        />

        <div className="h-px bg-border/20" />

        {/* Cấp Bậc — icon & stat fixed primary */}
        <TierRow
          label={TEXT.PROFILE.CULTIVATION_RANK}
          icon={<Star className="h-4 w-4 text-primary" />}
          statValue={total_exp}
          statColorClass="text-primary"
          current={level.name}
          next={level.next_name}
          nextBadgeClass={nextLevelColors?.badge}
          progress={level.progress}
          badgeClass={levelColors.badge}
          barClass={levelColors.bar}
          hint={level.exp_to_next > 0 ? (
            <>Cần thêm <span className="font-bold text-foreground">{level.exp_to_next.toLocaleString()}</span> {TEXT.PROFILE.CULTIVATION_EXP_TO_BREAK}</>
          ) : undefined}
        />
      </div>

      {/* Right: element radar */}
      {elements.length > 0 ? (
        <div className="flex justify-center">
          <ElementRadar elements={elements} />
        </div>
      ) : (
        <div className="flex items-center justify-center h-36 rounded-xl border border-border/20 text-muted-foreground text-sm">
          {TEXT.PROFILE.CULTIVATION_NO_ELEMENTS}
        </div>
      )}
    </div>
  )
}
