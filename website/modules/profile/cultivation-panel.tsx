"use client"

import { useId, useState } from "react"
import { Sparkles, Star } from "lucide-react"
import type { CultivationInfo, ElementExp } from "@/types/user"
import {
  getElementPresentation,
  getTierColors,
} from "@/constants/cultivation-presentation"
import { TEXT } from "@/constants/text"
import { formatNumber } from "@/lib/format"

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
          {typeof statValue === "number" ? formatNumber(statValue) : statValue}
        </span>
      </div>

      {/* Middle: badge | progress bar | next name */}
      <div className="flex items-center gap-2">
        <span className={`text-[10px] font-bold px-2 py-0.5 rounded-md shrink-0 ${badgeClass}`}>
          {current || TEXT.COMMON.NOT_AVAILABLE}
        </span>
        <div
          className="h-2.5 flex-1 overflow-hidden rounded-full bg-muted/50"
          role="progressbar"
          aria-label={label}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(pct)}
        >
          <div
            className={`h-full origin-left rounded-full bg-gradient-to-r transition-transform duration-300 motion-reduce:transition-none ${barClass}`}
            style={{ transform: `scaleX(${pct / 100})` }}
          />
        </div>
        {next ? (
          <span className={`text-[10px] font-bold px-2 py-0.5 rounded-md shrink-0 opacity-60 ${nextBadgeClass}`}>
            {next}
          </span>
        ) : (
          <span className="shrink-0 text-[10px] font-semibold text-cultivation/80">
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
  const titleID = useId()
  const descriptionID = useId()
  const normalized = normalizeElements(elements)
  const count = normalized.length

  const dataPoints = normalized
    .map((el, i) => vertex(i, count, (el.value / 100) * RADIUS).join(","))
    .join(" ")

  return (
    <figure className="w-full">
    <svg
      viewBox={`0 0 ${SVG_SIZE} ${SVG_SIZE}`}
      className="mx-auto h-auto w-full max-w-[200px]"
      role="img"
      aria-labelledby={`${titleID} ${descriptionID}`}
    >
      <title id={titleID}>{TEXT.PROFILE.CULTIVATION_ELEMENTS_TITLE}</title>
      <desc id={descriptionID}>{TEXT.PROFILE.CULTIVATION_ELEMENTS_DESCRIPTION}</desc>
      <defs>
        <radialGradient id="radarGlow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="var(--primary)" stopOpacity="0.08" />
          <stop offset="100%" stopColor="var(--primary)" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="radarFill" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="var(--primary)" stopOpacity="0.25" />
          <stop offset="100%" stopColor="var(--secondary)" stopOpacity="0.25" />
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

      <polygon points={dataPoints} fill="url(#radarFill)" stroke="var(--primary)" strokeWidth="1.5" strokeLinejoin="round" opacity="0.9" />

      {normalized.map((el, i) => {
        const display = getElementPresentation(el.code)
        const chartColor = display.chartColor
        const [dx, dy] = vertex(i, count, (el.value / 100) * RADIUS)
        const [lx, ly] = vertex(i, count, RADIUS + 22)
        const isHovered = hovered === i

        return (
          <g key={el.code} onMouseEnter={() => setHovered(i)} onMouseLeave={() => setHovered(null)} className="cursor-pointer">
            <circle cx={dx} cy={dy} r="14" fill="transparent" />
            {isHovered && <circle cx={dx} cy={dy} r="10" fill={chartColor} opacity="0.15" filter="url(#dotGlow)" />}
            <circle cx={dx} cy={dy} r={isHovered ? 5.5 : 3.5} fill={chartColor} stroke="var(--background)" strokeWidth="1.5"
              filter={isHovered ? "url(#dotGlow)" : undefined} />
            <text x={lx} y={ly} textAnchor="middle" dominantBaseline="central"
              fill={isHovered ? chartColor : "currentColor"} fontSize="12"
              fontWeight={isHovered ? "800" : "500"} opacity={isHovered ? 1 : 0.45}>
              {display.shortLabel}
            </text>
            {isHovered && (
              <g>
                <rect x={dx - 42} y={dy - 18} width="84" height="36" rx="7"
                  fill="var(--popover)" stroke={chartColor} strokeWidth="1" opacity="0.97" />
                <text x={dx} y={dy - 5} textAnchor="middle" fill={chartColor} fontSize="9.5" fontWeight="bold">{el.name}</text>
                <text x={dx} y={dy + 9} textAnchor="middle" fill="var(--muted-foreground)" fontSize="8.5">{formatNumber(Number(el.exp))} {TEXT.CULTIVATION.EXP}</text>
              </g>
            )}
          </g>
        )
      })}
    </svg>
      <figcaption className="mt-2 grid grid-cols-2 gap-1.5 text-xs">
        {normalized.map((element) => {
          const display = getElementPresentation(element.code)
          return (
            <span
              key={element.code}
              className="flex min-w-0 items-center justify-between gap-2 rounded-md bg-muted/40 px-2 py-1 text-muted-foreground"
            >
              <span className="truncate">{display.shortLabel}</span>
              <span className="shrink-0 tabular-nums text-foreground">
                {formatNumber(element.exp)} {TEXT.CULTIVATION.EXP}
              </span>
            </span>
          )
        })}
      </figcaption>
    </figure>
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
          icon={<Sparkles className="h-4 w-4 text-cultivation" aria-hidden="true" />}
          statValue={rating}
          statColorClass="text-cultivation"
          current={rank.name}
          next={rank.next_name}
          nextBadgeClass={nextRankColors?.badge}
          progress={rank.progress}
          badgeClass={rankColors.badge}
          barClass={rankColors.bar}
          hint={rank.rating_to_next > 0 ? (
            <>{TEXT.CULTIVATION.NEED_MORE} <span className="font-bold text-foreground">{rank.rating_to_next}</span> {TEXT.PROFILE.CULTIVATION_RATING_TO_NEXT}</>
          ) : undefined}
        />

        <div className="h-px bg-border/20" />

        {/* Cấp Bậc — icon & stat fixed primary */}
        <TierRow
          label={TEXT.PROFILE.CULTIVATION_RANK}
          icon={<Star className="h-4 w-4 text-primary" aria-hidden="true" />}
          statValue={total_exp}
          statColorClass="text-primary"
          current={level.name}
          next={level.next_name}
          nextBadgeClass={nextLevelColors?.badge}
          progress={level.progress}
          badgeClass={levelColors.badge}
          barClass={levelColors.bar}
          hint={level.exp_to_next > 0 ? (
            <>{TEXT.CULTIVATION.NEED_MORE} <span className="font-bold text-foreground">{formatNumber(level.exp_to_next)}</span> {TEXT.PROFILE.CULTIVATION_EXP_TO_BREAK}</>
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
