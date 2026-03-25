"use client";

/**
 * Rating history chart rendered with pure SVG.
 * Shows rating progression over time with realm threshold lines.
 */

import { CULTIVATION_REALMS } from "@/mock/profile-stats";

interface RatingDataPoint {
  date: string;
  rating: number;
}

interface RatingChartProps {
  data: RatingDataPoint[];
  currentRating: number;
}

const CHART_W = 600;
const CHART_H = 240;
const PAD = { top: 20, right: 20, bottom: 32, left: 48 };
const INNER_W = CHART_W - PAD.left - PAD.right;
const INNER_H = CHART_H - PAD.top - PAD.bottom;

export function RatingChart({ data, currentRating }: RatingChartProps) {
  if (data.length < 2) return null;

  const maxRating = Math.max(...data.map((d) => d.rating), currentRating) + 200;
  const minRating = 0;
  const range = maxRating - minRating;

  function x(i: number) {
    return PAD.left + (i / (data.length - 1)) * INNER_W;
  }

  function y(rating: number) {
    return PAD.top + INNER_H - ((rating - minRating) / range) * INNER_H;
  }

  /** Build SVG path from data points. */
  const linePath = data
    .map((d, i) => `${i === 0 ? "M" : "L"} ${x(i).toFixed(1)} ${y(d.rating).toFixed(1)}`)
    .join(" ");

  /** Area fill path (line + bottom edge). */
  const areaPath = `${linePath} L ${x(data.length - 1).toFixed(1)} ${y(0).toFixed(1)} L ${x(0).toFixed(1)} ${y(0).toFixed(1)} Z`;

  /** Visible realm thresholds within chart range. */
  const visibleRealms = CULTIVATION_REALMS.filter(
    (r) => r.minRating > 0 && r.minRating < maxRating
  );

  return (
    <svg viewBox={`0 0 ${CHART_W} ${CHART_H}`} className="w-full h-auto" preserveAspectRatio="xMidYMid meet">
      <defs>
        <linearGradient id="ratingGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.3" />
          <stop offset="100%" stopColor="rgb(245,158,11)" stopOpacity="0.02" />
        </linearGradient>
        <linearGradient id="lineGrad" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="rgb(245,158,11)" stopOpacity="0.6" />
          <stop offset="100%" stopColor="rgb(245,158,11)" stopOpacity="1" />
        </linearGradient>
        <filter id="glow">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Realm threshold lines */}
      {visibleRealms.map((r) => (
        <g key={r.name}>
          <line
            x1={PAD.left}
            y1={y(r.minRating)}
            x2={CHART_W - PAD.right}
            y2={y(r.minRating)}
            stroke={r.color}
            strokeWidth="1"
            strokeDasharray="4 4"
            opacity="0.3"
          />
          <text
            x={CHART_W - PAD.right - 4}
            y={y(r.minRating) - 4}
            textAnchor="end"
            fill={r.color}
            fontSize="9"
            fontWeight="bold"
            opacity="0.6"
          >
            {r.name}
          </text>
        </g>
      ))}

      {/* Area fill */}
      <path d={areaPath} fill="url(#ratingGrad)" />

      {/* Line */}
      <path d={linePath} fill="none" stroke="url(#lineGrad)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" filter="url(#glow)" />

      {/* Data points */}
      {data.map((d, i) => (
        <g key={d.date}>
          <circle cx={x(i)} cy={y(d.rating)} r="4" fill="rgb(245,158,11)" stroke="rgb(9,9,11)" strokeWidth="2" />
          {/* Date label on x-axis */}
          <text
            x={x(i)}
            y={CHART_H - 8}
            textAnchor="middle"
            fill="currentColor"
            fontSize="9"
            opacity="0.4"
          >
            {d.date.split("-")[1]}/{d.date.split("-")[0].slice(2)}
          </text>
        </g>
      ))}

      {/* Current rating dot — larger, glowing */}
      {data.length > 0 && (
        <g>
          <circle
            cx={x(data.length - 1)}
            cy={y(data[data.length - 1].rating)}
            r="6"
            fill="rgb(245,158,11)"
            stroke="rgb(245,158,11)"
            strokeWidth="3"
            strokeOpacity="0.3"
            filter="url(#glow)"
          />
          <text
            x={x(data.length - 1)}
            y={y(data[data.length - 1].rating) - 14}
            textAnchor="middle"
            fill="rgb(245,158,11)"
            fontSize="12"
            fontWeight="bold"
          >
            {data[data.length - 1].rating}
          </text>
        </g>
      )}

      {/* Y-axis rating labels */}
      {[0, Math.round(maxRating / 4), Math.round(maxRating / 2), Math.round((maxRating * 3) / 4), maxRating].map((val) => (
        <text
          key={val}
          x={PAD.left - 8}
          y={y(val) + 3}
          textAnchor="end"
          fill="currentColor"
          fontSize="9"
          opacity="0.35"
        >
          {val}
        </text>
      ))}
    </svg>
  );
}
