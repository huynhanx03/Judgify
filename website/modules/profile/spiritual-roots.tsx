"use client";

/**
 * Spiritual Roots (Linh Căn) visualization component.
 * Displays a pentagon radar chart + individual element bars.
 * Five elements: Kim (Metal), Mộc (Wood), Thủy (Water), Hỏa (Fire), Thổ (Earth).
 */

import type { SpiritualRoot } from "@/mock/profile-stats";

interface SpiritualRootsProps {
  roots: SpiritualRoot[];
}

/** SVG pentagon radar chart dimensions. */
const SIZE = 240;
const CENTER = SIZE / 2;
const RADIUS = 90;
const LEVELS = 4;

/** Compute pentagon vertex at given angle and distance from center. */
function vertex(index: number, total: number, radius: number): [number, number] {
  const angle = (Math.PI * 2 * index) / total - Math.PI / 2;
  return [CENTER + radius * Math.cos(angle), CENTER + radius * Math.sin(angle)];
}

/** Build SVG polygon points string. */
function polygonPoints(count: number, radius: number): string {
  return Array.from({ length: count }, (_, i) => vertex(i, count, radius).join(",")).join(" ");
}

export function SpiritualRoots({ roots }: SpiritualRootsProps) {
  const count = roots.length;

  /** Data polygon points based on each root's value. */
  const dataPoints = roots
    .map((r, i) => vertex(i, count, (r.value / 100) * RADIUS).join(","))
    .join(" ");

  return (
    <div className="space-y-6">
      {/* Radar Chart */}
      <div className="flex justify-center">
        <svg viewBox={`0 0 ${SIZE} ${SIZE}`} className="w-full max-w-[280px] h-auto">
          <defs>
            <radialGradient id="radarGlow" cx="50%" cy="50%" r="50%">
              <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.15" />
              <stop offset="100%" stopColor="rgb(245,158,11)" stopOpacity="0" />
            </radialGradient>
            <linearGradient id="dataFill" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.25" />
              <stop offset="50%" stopColor="rgb(139,92,246)" stopOpacity="0.2" />
              <stop offset="100%" stopColor="rgb(59,130,246)" stopOpacity="0.25" />
            </linearGradient>
            <filter id="radarBlur">
              <feGaussianBlur stdDeviation="2" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          {/* Background glow */}
          <circle cx={CENTER} cy={CENTER} r={RADIUS + 10} fill="url(#radarGlow)" />

          {/* Grid levels */}
          {Array.from({ length: LEVELS }, (_, i) => {
            const r = ((i + 1) / LEVELS) * RADIUS;
            return (
              <polygon
                key={i}
                points={polygonPoints(count, r)}
                fill="none"
                stroke="currentColor"
                strokeWidth="0.5"
                opacity="0.1"
              />
            );
          })}

          {/* Axis lines */}
          {roots.map((_, i) => {
            const [vx, vy] = vertex(i, count, RADIUS);
            return (
              <line
                key={i}
                x1={CENTER}
                y1={CENTER}
                x2={vx}
                y2={vy}
                stroke="currentColor"
                strokeWidth="0.5"
                opacity="0.1"
              />
            );
          })}

          {/* Data polygon */}
          <polygon
            points={dataPoints}
            fill="url(#dataFill)"
            stroke="rgb(245,158,11)"
            strokeWidth="1.5"
            strokeLinejoin="round"
            filter="url(#radarBlur)"
          />

          {/* Data points & labels */}
          {roots.map((root, i) => {
            const [dx, dy] = vertex(i, count, (root.value / 100) * RADIUS);
            const [lx, ly] = vertex(i, count, RADIUS + 22);
            return (
              <g key={root.element}>
                {/* Dot on data polygon */}
                <circle cx={dx} cy={dy} r="4" fill={root.color} stroke="rgb(9,9,11)" strokeWidth="2" />
                {/* Element label */}
                <text
                  x={lx}
                  y={ly}
                  textAnchor="middle"
                  dominantBaseline="central"
                  fill={root.color}
                  fontSize="11"
                  fontWeight="bold"
                >
                  {root.element}
                </text>
              </g>
            );
          })}
        </svg>
      </div>

      {/* Element Bars */}
      <div className="space-y-3">
        {roots.map((root) => (
          <div key={root.element} className="space-y-1.5">
            <div className="flex items-center justify-between text-sm">
              <div className="flex items-center gap-2">
                <span
                  className="h-2.5 w-2.5 rounded-full shadow-sm"
                  style={{ backgroundColor: root.color, boxShadow: `0 0 6px ${root.bgGlow}` }}
                />
                <span className="font-medium text-xs">{root.name}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">{root.solved} bài</span>
                <span className="text-xs font-bold" style={{ color: root.color }}>
                  {root.value}%
                </span>
              </div>
            </div>
            <div className="h-2 rounded-full bg-muted/50 overflow-hidden">
              <div
                className={`h-full rounded-full bg-gradient-to-r ${root.gradient} transition-all duration-1000 relative`}
                style={{ width: `${root.value}%` }}
              >
                <div className="absolute inset-0 bg-white/10 rounded-full" />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
