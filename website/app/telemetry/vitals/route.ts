import { context, trace } from "@opentelemetry/api";
import { parseVitalSignal } from "@/lib/observability/vitals";

export const runtime = "nodejs";

const MAX_BODY_BYTES = 1024;
const MAX_EVENTS_PER_MINUTE = 60;
let windowStarted = Date.now();
let admitted = 0;

function reject(status: number): Response {
  return new Response(null, { status, headers: { "Cache-Control": "no-store" } });
}

async function readBoundedBody(request: Request): Promise<string | null> {
  if (!request.body) return null;
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      size += part.value.byteLength;
      if (size > MAX_BODY_BYTES) {
        await reader.cancel();
        return null;
      }
      chunks.push(part.value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return null;
  }
}

export async function POST(request: Request): Promise<Response> {
  const origin = request.headers.get("origin");
  if (origin && origin !== new URL(request.url).origin) return reject(403);
  if (request.headers.get("content-type")?.split(";", 1)[0] !== "application/json") return reject(415);
  const declared = request.headers.get("content-length");
  if (declared && (!/^\d+$/.test(declared) || Number(declared) > MAX_BODY_BYTES)) return reject(413);
  const body = await readBoundedBody(request);
  if (body === null) return reject(413);

  let parsed: unknown;
  try { parsed = JSON.parse(body); } catch { return reject(400); }
  const signal = parseVitalSignal(parsed);
  if (!signal) return reject(400);

  const now = Date.now();
  if (now - windowStarted >= 60_000) {
    windowStarted = now;
    admitted = 0;
  }
  if (admitted >= MAX_EVENTS_PER_MINUTE) return reject(429);
  admitted++;

  const span = trace.getSpan(context.active())?.spanContext();
  console.info(JSON.stringify({
    timestamp: new Date(now).toISOString(),
    level: "info",
    message: "web.vital",
    ...(span && trace.isSpanContextValid(span)
      ? { trace_id: span.traceId, span_id: span.spanId }
      : {}),
    metadata: signal,
  }));
  return reject(204);
}
