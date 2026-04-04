/** Shared date/time formatting utilities. */

const pad = (n: number) => String(n).padStart(2, "0");

/** Format to "DD/MM HH:MM" — compact timestamp for table cells. */
export function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Format to "DD/MM/YYYY HH:MM:SS" — full timestamp for detail views. */
export function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
