export const runtime = "nodejs";

const started = Date.now();

export function GET(): Response {
  const uptime = Math.max(0, (Date.now() - started) / 1000);
  const memory = process.memoryUsage().rss;
  const body = [
    "# HELP judgify_website_up Website process is serving requests.",
    "# TYPE judgify_website_up gauge",
    "judgify_website_up 1",
    "# HELP judgify_website_process_uptime_seconds Website process uptime.",
    "# TYPE judgify_website_process_uptime_seconds gauge",
    `judgify_website_process_uptime_seconds ${uptime}`,
    "# HELP judgify_website_process_resident_memory_bytes Website resident memory.",
    "# TYPE judgify_website_process_resident_memory_bytes gauge",
    `judgify_website_process_resident_memory_bytes ${memory}`,
    "",
  ].join("\n");
  return new Response(body, {
    headers: {
      "Content-Type": "text/plain; version=0.0.4; charset=utf-8",
      "Cache-Control": "no-store",
    },
  });
}
