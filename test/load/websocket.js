"use strict";

const crypto = require("crypto");
const fs = require("fs");
const net = require("net");
const tls = require("tls");

const {
  positiveInteger,
  requestHeaders,
  required,
  testBaseURL,
  writeEvidence,
} = require("./lib");

const MAXIMUM_UPGRADE_HEADER_BYTES = 16 * 1024;
const UPGRADE_TIMEOUT_MS = 8000;

async function issueTicket(base) {
  const response = await fetch(new URL("/realtime/tickets", base), {
    method: "POST",
    headers: requestHeaders({ "Content-Type": "application/json" }),
    body: "{}",
    signal: AbortSignal.timeout(5000),
  });
  if (!response.ok) throw new Error(`ticket endpoint returned ${response.status}`);
  const payload = await response.json();
  const ticket = payload?.data?.ticket ?? payload?.ticket;
  if (typeof ticket !== "string" || !/^[A-Za-z0-9_-]{43}$/.test(ticket)) {
    throw new Error("ticket response is invalid");
  }
  return ticket;
}

function websocketURL(base, ticket) {
  const url = new URL("/ws", base);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("ticket", ticket);
  return url;
}

function completedResult(opened, clean, code, unread) {
  return { opened, clean, code, unread };
}

async function startResponsiveConnection(base, lifetimeMs) {
  const ticket = await issueTicket(base);
  const url = websocketURL(base, ticket);

  return await new Promise((resolve) => {
    const socket = new WebSocket(url);
    let settled = false;
    let finish;
    const completed = new Promise((complete) => {
      finish = complete;
    });
    const upgradeTimeout = setTimeout(() => {
      socket.close();
      if (!settled) {
        settled = true;
        finish(completedResult(false, false, 0, false));
        resolve({ opened: false, completed });
      }
    }, UPGRADE_TIMEOUT_MS);

    socket.addEventListener("open", () => {
      clearTimeout(upgradeTimeout);
      if (settled) return;
      settled = true;
      socket.send(JSON.stringify({
        v: 1,
        op: "ping",
        id: `load-${crypto.randomUUID()}`,
        type: "protocol.ping.v1",
      }));
      setTimeout(() => socket.close(1000, "load-complete"), lifetimeMs);
      resolve({ opened: true, completed });
    });
    socket.addEventListener("close", (event) => {
      clearTimeout(upgradeTimeout);
      const opened = settled;
      finish(completedResult(opened, event.wasClean, event.code, false));
      if (!settled) {
        settled = true;
        resolve({ opened: false, completed });
      }
    });
    socket.addEventListener("error", () => {
      clearTimeout(upgradeTimeout);
      if (!settled) {
        settled = true;
        finish(completedResult(false, false, 0, false));
        resolve({ opened: false, completed });
      }
    });
  });
}

function maskedTextFrame(value) {
  const payload = Buffer.from(value, "utf8");
  if (payload.length === 0 || payload.length > 65535) {
    throw new Error("unread-client subscription frame exceeds the bounded control-frame size");
  }
  const mask = crypto.randomBytes(4);
  const extended = payload.length > 125;
  const headerBytes = extended ? 4 : 2;
  const frame = Buffer.allocUnsafe(headerBytes + mask.length + payload.length);
  frame[0] = 0x81;
  frame[1] = 0x80 | (extended ? 126 : payload.length);
  if (extended) frame.writeUInt16BE(payload.length, 2);
  mask.copy(frame, headerBytes);
  for (let index = 0; index < payload.length; index += 1) {
    frame[headerBytes + mask.length + index] =
      payload[index] ^ mask[index % mask.length];
  }
  return frame;
}

function rawSocket(url) {
  const port = Number(url.port || (url.protocol === "wss:" ? 443 : 80));
  if (!Number.isSafeInteger(port) || port < 1 || port > 65535) {
    throw new Error("websocket target port is invalid");
  }
  if (url.protocol === "wss:") {
    return tls.connect({
      host: url.hostname,
      port,
      servername: net.isIP(url.hostname) ? undefined : url.hostname,
      rejectUnauthorized: true,
    });
  }
  if (url.protocol !== "ws:") throw new Error("unsupported websocket protocol");
  return net.connect({ host: url.hostname, port });
}

function parseUpgradeResponse(header, expectedAccept) {
  const lines = header.split("\r\n");
  if (!/^HTTP\/1\.[01] 101(?: |$)/.test(lines[0] ?? "")) {
    throw new Error("raw unread client did not receive HTTP 101");
  }
  const headers = new Map();
  for (const line of lines.slice(1)) {
    const separator = line.indexOf(":");
    if (separator <= 0) continue;
    headers.set(
      line.slice(0, separator).trim().toLowerCase(),
      line.slice(separator + 1).trim(),
    );
  }
  if ((headers.get("upgrade") ?? "").toLowerCase() !== "websocket" ||
      headers.get("sec-websocket-accept") !== expectedAccept) {
    throw new Error("raw unread client received an invalid upgrade proof");
  }
}

async function startUnreadConnection(base, lifetimeMs, topic) {
  const ticket = await issueTicket(base);
  const url = websocketURL(base, ticket);
  const key = crypto.randomBytes(16).toString("base64");
  const expectedAccept = crypto
    .createHash("sha1")
    .update(`${key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`, "ascii")
    .digest("base64");
  const host = url.port ? `${url.hostname}:${url.port}` : url.hostname;
  const request = [
    `GET ${url.pathname}${url.search} HTTP/1.1`,
    `Host: ${host}`,
    `Origin: ${base.origin}`,
    "Upgrade: websocket",
    "Connection: Upgrade",
    `Sec-WebSocket-Key: ${key}`,
    "Sec-WebSocket-Version: 13",
    "",
    "",
  ].join("\r\n");

  return await new Promise((resolve) => {
    const socket = rawSocket(url);
    let upgraded = false;
    let settled = false;
    let response = Buffer.alloc(0);
    let finish;
    const completed = new Promise((complete) => {
      finish = complete;
    });
    const upgradeTimeout = setTimeout(() => {
      socket.destroy();
      if (!settled) {
        settled = true;
        finish(completedResult(false, false, 0, true));
        resolve({ opened: false, completed });
      }
    }, UPGRADE_TIMEOUT_MS);

    socket.once(url.protocol === "wss:" ? "secureConnect" : "connect", () => {
      socket.write(request, "ascii");
    });
    socket.on("data", (chunk) => {
      if (upgraded || settled) return;
      response = Buffer.concat([response, chunk]);
      if (response.length > MAXIMUM_UPGRADE_HEADER_BYTES) {
        socket.destroy(new Error("websocket upgrade headers exceed the safety bound"));
        return;
      }
      const end = response.indexOf("\r\n\r\n");
      if (end < 0) return;
      try {
        parseUpgradeResponse(response.subarray(0, end).toString("latin1"), expectedAccept);
        const subscription = JSON.stringify({
          v: 1,
          op: "subscribe",
          id: `unread-${crypto.randomUUID()}`,
          topic,
          type: "protocol.subscription.v1",
        });
        socket.write(maskedTextFrame(subscription));
        // This is the key property that browser WebSocket cannot prove: stop
        // reading the TCP stream so the peer receive window and then the
        // server's bounded mailbox/write deadline must absorb the pressure.
        socket.pause();
        upgraded = true;
        settled = true;
        clearTimeout(upgradeTimeout);
        setTimeout(() => socket.destroy(), lifetimeMs);
        resolve({ opened: true, completed });
      } catch (error) {
        socket.destroy(error);
      }
    });
    socket.once("close", () => {
      clearTimeout(upgradeTimeout);
      finish(completedResult(upgraded, false, upgraded ? 1006 : 0, true));
      if (!settled) {
        settled = true;
        resolve({ opened: false, completed });
      }
    });
    socket.once("error", () => {
      clearTimeout(upgradeTimeout);
      if (!settled) {
        settled = true;
        finish(completedResult(false, false, 0, true));
        resolve({ opened: false, completed });
      }
    });
  });
}

function publishReady(path, opened, unreadOpened) {
  if (!path) return;
  fs.writeFileSync(path, `${JSON.stringify({
    schema_version: 1,
    opened_connections: opened,
    unread_opened_connections: unreadOpened,
    ready_at: new Date().toISOString(),
  })}\n`, { encoding: "utf8", mode: 0o600, flag: "wx" });
}

async function main() {
  if (typeof WebSocket !== "function") {
    throw new Error("Node.js with the built-in WebSocket client is required");
  }
  const base = testBaseURL();
  const connections = positiveInteger("JUDGIFY_WS_CONNECTIONS", 1000, 5000);
  const concurrency = positiveInteger("JUDGIFY_WS_OPEN_CONCURRENCY", 64, 512);
  const lifetimeMs = positiveInteger("JUDGIFY_WS_LIFETIME_MS", 30000, 300000);
  const unreadConnections = positiveInteger("JUDGIFY_WS_UNREAD_CONNECTIONS", 50, 500);
  if (unreadConnections >= connections) {
    throw new Error("JUDGIFY_WS_UNREAD_CONNECTIONS must be smaller than JUDGIFY_WS_CONNECTIONS");
  }
  const unreadTopic = required("JUDGIFY_WS_UNREAD_TOPIC");
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$/.test(unreadTopic)) {
    throw new Error("JUDGIFY_WS_UNREAD_TOPIC is invalid");
  }

  let next = 0;
  const controllers = [];
  async function worker() {
    for (;;) {
      const index = next++;
      if (index >= connections) return;
      const unread = index < unreadConnections;
      try {
        controllers[index] = unread
          ? await startUnreadConnection(base, lifetimeMs, unreadTopic)
          : await startResponsiveConnection(base, lifetimeMs);
      } catch {
        controllers[index] = {
          opened: false,
          completed: Promise.resolve(completedResult(false, false, 0, unread)),
        };
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, connections) }, worker));

  const opened = controllers.filter((controller) => controller.opened).length;
  const unreadOpened = controllers
    .slice(0, unreadConnections)
    .filter((controller) => controller.opened).length;
  publishReady(process.env.JUDGIFY_WS_READY_FILE, opened, unreadOpened);

  const results = await Promise.all(controllers.map((controller) => controller.completed));
  const responsive = results.filter((result) => !result.unread);
  const responsiveOpened = responsive.filter((result) => result.opened).length;
  const abnormal = responsive.filter((result) => ![1000, 1001].includes(result.code)).length;
  const openRatio = opened / connections;
  const unreadOpenRatio = unreadOpened / unreadConnections;
  const abnormalRatio = abnormal / responsive.length;
  const passed = openRatio >= 0.995 && unreadOpenRatio >= 0.99 && abnormalRatio <= 0.01;
  writeEvidence("websocket", {
    requested_connections: connections,
    opened_connections: opened,
    open_ratio: openRatio,
    responsive_connections: responsive.length,
    responsive_opened_connections: responsiveOpened,
    abnormal_closes: abnormal,
    abnormal_ratio: abnormalRatio,
    unread_connections: unreadConnections,
    unread_opened_connections: unreadOpened,
    unread_open_ratio: unreadOpenRatio,
    unread_transport: "raw_tcp_paused_after_subscription",
    pressure_verification: "external_domain_driver",
    passed,
  });
  if (!passed) process.exitCode = 1;
}

main().catch((error) => {
  process.stderr.write(`websocket load failed: ${error.message}\n`);
  process.exitCode = 1;
});
