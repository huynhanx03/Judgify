"use client";

/**
 * Activity Heatmap — GitHub-style contribution grid showing full year activity.
 * Supports year selection with clickable year tabs.
 */

import { useState, useMemo } from "react";

interface ActivityHeatmapProps {
  /** Map of date string (YYYY-MM-DD) to submission count */
  data: Record<string, number>;
  /** Available years to display, defaults to current + previous year */
  years?: number[];
}

const DAYS = 7;
const DAY_LABELS = ["T2", "", "T4", "", "T6", "", "CN"];
const CELL_SIZE = 12;
const GAP = 2;
const MONTH_NAMES = ["Th1", "Th2", "Th3", "Th4", "Th5", "Th6", "Th7", "Th8", "Th9", "Th10", "Th11", "Th12"];

function getCellColor(count: number): string {
  if (count === 0) return "bg-muted/40";
  if (count <= 2) return "bg-emerald-500/30";
  if (count <= 5) return "bg-emerald-500/55";
  if (count <= 10) return "bg-emerald-500/80";
  return "bg-emerald-500";
}

function buildYearGrid(year: number, data: Record<string, number>) {
  const jan1 = new Date(year, 0, 1);
  const dec31 = new Date(year, 11, 31);
  const startDay = (jan1.getDay() + 6) % 7; // Mon=0

  const cells: { date: string; count: number; col: number; row: number }[] = [];
  const monthLabels: { label: string; col: number }[] = [];
  let totalSubmissions = 0;

  let col = 0;
  let lastMonth = -1;

  // Fill from Jan 1 to Dec 31
  const current = new Date(jan1);
  while (current <= dec31) {
    const dayOfWeek = (current.getDay() + 6) % 7; // Mon=0
    const currentCol = Math.floor(
      ((current.getTime() - jan1.getTime()) / 86400000 + startDay) / 7
    );
    const key = current.toISOString().slice(0, 10);
    const count = data[key] ?? 0;
    totalSubmissions += count;

    cells.push({ date: key, count, col: currentCol, row: dayOfWeek });

    // Track month labels
    const month = current.getMonth();
    if (month !== lastMonth) {
      monthLabels.push({ label: MONTH_NAMES[month], col: currentCol });
      lastMonth = month;
    }

    current.setDate(current.getDate() + 1);
  }

  const totalWeeks = cells.length > 0 ? cells[cells.length - 1].col + 1 : 52;

  return { cells, monthLabels, totalWeeks, totalSubmissions };
}

export function ActivityHeatmap({ data, years }: ActivityHeatmapProps) {
  const currentYear = new Date().getFullYear();
  const availableYears = years ?? [currentYear - 1, currentYear];
  const [selectedYear, setSelectedYear] = useState(currentYear);

  const { cells, monthLabels, totalWeeks, totalSubmissions } = useMemo(
    () => buildYearGrid(selectedYear, data),
    [selectedYear, data]
  );

  const gridWidth = totalWeeks * (CELL_SIZE + GAP) - GAP;

  return (
    <div className="space-y-3">
      {/* Year tabs + total */}
      <div className="flex items-center justify-between">
        <div className="flex gap-1.5">
          {availableYears.map((year) => (
            <button
              key={year}
              onClick={() => setSelectedYear(year)}
              className={`px-3 py-1 rounded-md text-xs font-bold transition-colors cursor-pointer ${
                selectedYear === year
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted/40 text-muted-foreground hover:bg-muted/60"
              }`}
            >
              {year}
            </button>
          ))}
        </div>
        <span className="text-xs text-muted-foreground">
          <span className="font-bold text-foreground">{totalSubmissions}</span> bài nộp trong {selectedYear}
        </span>
      </div>

      {/* Heatmap */}
      <div className="overflow-x-auto">
        <div style={{ minWidth: gridWidth + 28 }}>
          {/* Month labels */}
          <div className="relative h-4 ml-7 mb-1">
            {monthLabels.map((m, i) => (
              <span
                key={i}
                className="absolute text-[10px] text-muted-foreground"
                style={{ left: m.col * (CELL_SIZE + GAP) }}
              >
                {m.label}
              </span>
            ))}
          </div>

          <div className="flex gap-1">
            {/* Day labels */}
            <div
              className="flex flex-col justify-between pr-1 shrink-0"
              style={{ height: DAYS * (CELL_SIZE + GAP) - GAP }}
            >
              {DAY_LABELS.map((label, i) => (
                <span
                  key={i}
                  className="text-[10px] text-muted-foreground leading-none flex items-center"
                  style={{ height: CELL_SIZE }}
                >
                  {label}
                </span>
              ))}
            </div>

            {/* Grid */}
            <div
              className="relative"
              style={{ width: gridWidth, height: DAYS * (CELL_SIZE + GAP) - GAP }}
            >
              {cells.map((cell) => (
                <div
                  key={cell.date}
                  className={`absolute rounded-sm ${getCellColor(cell.count)} transition-colors`}
                  style={{
                    width: CELL_SIZE,
                    height: CELL_SIZE,
                    left: cell.col * (CELL_SIZE + GAP),
                    top: cell.row * (CELL_SIZE + GAP),
                  }}
                  title={`${cell.date}: ${cell.count} bài nộp`}
                />
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-1.5 justify-end text-[10px] text-muted-foreground">
        <span>Ít</span>
        <div className="h-3 w-3 rounded-sm bg-muted/40" />
        <div className="h-3 w-3 rounded-sm bg-emerald-500/30" />
        <div className="h-3 w-3 rounded-sm bg-emerald-500/55" />
        <div className="h-3 w-3 rounded-sm bg-emerald-500/80" />
        <div className="h-3 w-3 rounded-sm bg-emerald-500" />
        <span>Nhiều</span>
      </div>
    </div>
  );
}
