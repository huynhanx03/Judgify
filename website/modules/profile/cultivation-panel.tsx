"use client";

/**
 * Unified Cultivation Panel — two-column layout.
 * Left: Rating/Realm + EXP/Rank progress bars stacked vertically.
 * Right: Radar chart with hover tooltip at cursor position.
 */

import { useState } from "react";
import { Sparkles, Flame, Crown, Star, Sprout, TreePine, Gem } from "lucide-react";
import { getRealmByRating, getRankByExp } from "@/mock/profile-stats";
import type { SpiritualRoot } from "@/mock/profile-stats";

interface CultivationPanelProps {
  rating: number;
  exp: number;
  roots: SpiritualRoot[];
}

const RANK_ICONS: Record<string, React.ReactNode> = {
  seedling: <TreePine className="h-4 w-4" />,
  sprout: <Sprout className="h-4 w-4" />,
  flame: <Flame className="h-4 w-4" />,
  star: <Star className="h-4 w-4" />,
  crown: <Crown className="h-4 w-4" />,
  gem: <Gem className="h-4 w-4" />,
};

/* ======== Radar Chart ======== */
const SIZE = 220;
const CENTER = SIZE / 2;
const RADIUS = 80;
const LEVELS = 4;

function vertex(index: number, total: number, radius: number): [number, number] {
  const angle = (Math.PI * 2 * index) / total - Math.PI / 2;
  return [CENTER + radius * Math.cos(angle), CENTER + radius * Math.sin(angle)];
}

function polygonPoints(count: number, radius: number): string {
  return Array.from({ length: count }, (_, i) => vertex(i, count, radius).join(",")).join(" ");
}

export function CultivationPanel({ rating, exp, roots }: CultivationPanelProps) {
  const realm = getRealmByRating(rating);
  const rank = getRankByExp(exp);
  const [hovered, setHovered] = useState<number | null>(null);

  const realmProgress = realm.next
    ? ((rating - realm.current.minRating) / (realm.next.minRating - realm.current.minRating)) * 100
    : 100;

  const rankProgress = rank.next
    ? ((exp - rank.current.minExp) / (rank.next.minExp - rank.current.minExp)) * 100
    : 100;

  const count = roots.length;
  const dataPoints = roots
    .map((r, i) => vertex(i, count, (r.value / 100) * RADIUS).join(","))
    .join(" ");

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 items-center">
      {/* ====== LEFT: Progress Bars ====== */}
      <div className="space-y-5">
        {/* Rating / Realm */}
        <div className="space-y-2.5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Sparkles className="h-4 w-4" style={{ color: realm.current.color }} />
              <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Cảnh Giới</span>
            </div>
            <span className="text-sm font-bold" style={{ color: realm.current.color }}>{rating}</span>
          </div>
          <div className="flex items-center gap-2">
            <span
              className={`text-[10px] font-bold text-white px-2 py-0.5 rounded-md shrink-0 bg-gradient-to-r ${realm.current.gradient}`}
            >
              {realm.current.name}
            </span>
            <div className="flex-1 h-2.5 rounded-full bg-muted/50 overflow-hidden">
              <div
                className={`h-full rounded-full bg-gradient-to-r ${realm.current.gradient} transition-all duration-1000`}
                style={{ width: `${Math.min(realmProgress, 100)}%` }}
              />
            </div>
            {realm.next && (
              <span className="text-[10px] text-muted-foreground shrink-0">{realm.next.name}</span>
            )}
          </div>
          {realm.next && (
            <p className="text-[11px] text-muted-foreground">
              Cần thêm <span className="font-bold text-foreground">{realm.next.minRating - rating}</span> để đột phá
            </p>
          )}
        </div>

        <div className="h-px bg-border/30" />

        {/* EXP / Rank */}
        <div className="space-y-2.5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {RANK_ICONS[rank.current.icon] ?? <Star className="h-4 w-4" />}
              <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Cấp Bậc</span>
            </div>
            <span className="text-sm font-bold text-secondary">{exp.toLocaleString()} EXP</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-[10px] font-bold text-primary px-2 py-0.5 rounded-md shrink-0 bg-primary/10 border border-primary/20">
              Lv.{rank.level}
            </span>
            <div className="flex-1 h-2.5 rounded-full bg-muted/50 overflow-hidden">
              <div
                className="h-full rounded-full bg-gradient-to-r from-primary to-secondary transition-all duration-1000"
                style={{ width: `${Math.min(rankProgress, 100)}%` }}
              />
            </div>
            {rank.next && (
              <span className="text-[10px] text-muted-foreground shrink-0 whitespace-nowrap">{rank.next.name}</span>
            )}
          </div>
          <p className="text-[11px] text-muted-foreground">
            {rank.current.name}
            {rank.next && (
              <> — cần <span className="font-bold text-foreground">{(rank.next.minExp - exp).toLocaleString()}</span> EXP</>
            )}
          </p>
        </div>
      </div>

      {/* ====== RIGHT: Radar Chart ====== */}
      <div className="flex justify-center">
        <svg viewBox={`0 0 ${SIZE} ${SIZE}`} className="w-full max-w-[240px] h-auto">
          <defs>
            <radialGradient id="cpGlow" cx="50%" cy="50%" r="50%">
              <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.1" />
              <stop offset="100%" stopColor="rgb(245,158,11)" stopOpacity="0" />
            </radialGradient>
            <linearGradient id="cpFill" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.2" />
              <stop offset="50%" stopColor="rgb(139,92,246)" stopOpacity="0.15" />
              <stop offset="100%" stopColor="rgb(59,130,246)" stopOpacity="0.2" />
            </linearGradient>
            <filter id="cpBlur">
              <feGaussianBlur stdDeviation="2" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          <circle cx={CENTER} cy={CENTER} r={RADIUS + 10} fill="url(#cpGlow)" />

          {/* Grid */}
          {Array.from({ length: LEVELS }, (_, i) => (
            <polygon
              key={i}
              points={polygonPoints(count, ((i + 1) / LEVELS) * RADIUS)}
              fill="none"
              stroke="currentColor"
              strokeWidth="0.5"
              opacity="0.08"
            />
          ))}

          {/* Axes */}
          {roots.map((_, i) => {
            const [vx, vy] = vertex(i, count, RADIUS);
            return <line key={i} x1={CENTER} y1={CENTER} x2={vx} y2={vy} stroke="currentColor" strokeWidth="0.5" opacity="0.08" />;
          })}

          {/* Data polygon */}
          <polygon
            points={dataPoints}
            fill="url(#cpFill)"
            stroke="rgb(245,158,11)"
            strokeWidth="1.5"
            strokeLinejoin="round"
            filter="url(#cpBlur)"
          />

          {/* Dots + Labels + Hover Tooltips */}
          {roots.map((root, i) => {
            const [dx, dy] = vertex(i, count, (root.value / 100) * RADIUS);
            const [lx, ly] = vertex(i, count, RADIUS + 20);
            const isHovered = hovered === i;

            /* Tooltip position: offset from dot toward outside */
            const [tx, ty] = vertex(i, count, (root.value / 100) * RADIUS + 18);

            return (
              <g
                key={root.element}
                onMouseEnter={() => setHovered(i)}
                onMouseLeave={() => setHovered(null)}
                className="cursor-pointer"
              >
                {/* Hit area */}
                <circle cx={dx} cy={dy} r="16" fill="transparent" />

                {/* Glow on hover */}
                {isHovered && <circle cx={dx} cy={dy} r="10" fill={root.color} opacity="0.15" />}

                {/* Dot */}
                <circle
                  cx={dx} cy={dy}
                  r={isHovered ? 6 : 4}
                  fill={root.color}
                  stroke="rgb(9,9,11)"
                  strokeWidth="2"
                />

                {/* Element label */}
                <text
                  x={lx} y={ly}
                  textAnchor="middle"
                  dominantBaseline="central"
                  fill={isHovered ? root.color : "currentColor"}
                  fontSize="11"
                  fontWeight={isHovered ? "800" : "600"}
                  opacity={isHovered ? 1 : 0.5}
                >
                  {root.element}
                </text>

                {/* Tooltip at cursor — small box near the dot */}
                {isHovered && (
                  <g>
                    <rect
                      x={tx - 42}
                      y={ty - 16}
                      width="84"
                      height="32"
                      rx="6"
                      fill="rgb(24,24,27)"
                      stroke={root.color}
                      strokeWidth="1"
                      opacity="0.95"
                    />
                    <text x={tx} y={ty - 4} textAnchor="middle" fill={root.color} fontSize="10" fontWeight="bold">
                      {root.name}
                    </text>
                    <text x={tx} y={ty + 9} textAnchor="middle" fill="#a1a1aa" fontSize="9">
                      {root.value}% · {root.solved} bài
                    </text>
                  </g>
                )}
              </g>
            );
          })}
        </svg>
      </div>
    </div>
  );
}
