/**
 * Small reusable sub-components used in the Profile page.
 * QuickStat: single stat row with icon, label, value.
 * StatCard: summary stat card (2x2 grid).
 * DifficultyCard: difficulty breakdown card (easy/medium/hard).
 */

import { Card } from "@/components/ui/card";

export function QuickStat({ icon, label, value }: { icon: React.ReactNode; label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 text-sm">
      {icon}
      <span className="text-muted-foreground">{label}</span>
      <span className="ml-auto">{value}</span>
    </div>
  );
}

export function StatCard({ value, label, color }: { value: string | number; label: string; color: string }) {
  return (
    <Card className="glass-card border-border/40 p-4 text-center hover:border-primary/20 transition-colors cursor-default">
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
      <p className="text-xs text-muted-foreground mt-1">{label}</p>
    </Card>
  );
}

const DIFFICULTY_COLORS: Record<string, { text: string; bg: string; border: string }> = {
  emerald: { text: "text-emerald-500", bg: "bg-emerald-500/10", border: "border-emerald-500/20" },
  amber: { text: "text-amber-500", bg: "bg-amber-500/10", border: "border-amber-500/20" },
  rose: { text: "text-rose-500", bg: "bg-rose-500/10", border: "border-rose-500/20" },
};

export function DifficultyCard({ label, count, total, color }: { label: string; count: number; total: number; color: string }) {
  const c = DIFFICULTY_COLORS[color];
  const pct = total > 0 ? Math.round((count / total) * 100) : 0;
  return (
    <div className={`rounded-xl border p-4 text-center ${c.bg} ${c.border}`}>
      <p className={`text-3xl font-bold ${c.text}`}>{count}</p>
      <p className="text-xs mt-1 text-muted-foreground">{label}</p>
      <p className={`text-xs mt-0.5 font-semibold ${c.text} opacity-70`}>{pct}%</p>
    </div>
  );
}
