import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

const REALTIME_PROTOCOL = Object.freeze({
  VERSION: 1,
  OPERATION_SUBSCRIBE: "subscribe",
  OPERATION_UNSUBSCRIBE: "unsubscribe",
  OPERATION_EVENT: "event",
  OPERATION_PING: "ping",
  OPERATION_PONG: "pong",
  OPERATION_ACK: "ack",
  OPERATION_ERROR: "error",
  OPERATION_SUBSCRIBED: "subscribed",
  OPERATION_UNSUBSCRIBED: "unsubscribed",
  OPERATION_RESYNC_REQUIRED: "resync_required",
  TYPE_SUBSCRIPTION: "protocol.subscription.v1",
  TYPE_PING: "protocol.ping.v1",
  TYPE_PONG: "protocol.pong.v1",
  TYPE_ERROR: "protocol.error.v1",
  TYPE_RESYNC_REQUIRED: "protocol.resync-required.v1",
  TYPE_REPLAY_REQUEST: "realtime.replay.request.v1",
  TYPE_REPLAY_COMPLETE: "realtime.replay.complete.v1",
  REQUEST_ID_PREFIX: "realtime-",
});

const REALTIME_CONNECTION_MODE = Object.freeze({
  ANONYMOUS: "anonymous",
  TICKET: "ticket",
});

const REALTIME_POLICY = Object.freeze({
  REPLAY_PAGE_SIZE: 128,
  MAXIMUM_DEDUPLICATION_ENTRIES: 4_096,
});

class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances = [];

  constructor(url) {
    this.url = url;
    this.readyState = FakeWebSocket.CONNECTING;
    this.listeners = new Map();
    this.sent = [];
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type, handler) {
    const handlers = this.listeners.get(type) ?? [];
    handlers.push(handler);
    this.listeners.set(type, handlers);
  }

  send(message) {
    this.sent.push(message);
  }

  open() {
    this.readyState = FakeWebSocket.OPEN;
    this.emit("open", {});
  }

  close() {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    this.emit("close", {});
  }

  emit(type, event) {
    for (const handler of this.listeners.get(type) ?? []) handler(event);
  }

  message(envelope) {
    this.emit("message", { data: JSON.stringify(envelope) });
  }

  static reset() {
    FakeWebSocket.instances = [];
  }
}

async function loadClient() {
  const source = await readFile(
    path.join(ROOT, "lib/realtime/websocket-client.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const evaluatedModule = { exports: {} };
  const require = (specifier) => {
    if (specifier === "@/constants/realtime") {
      return { REALTIME_CONNECTION_MODE, REALTIME_POLICY, REALTIME_PROTOCOL };
    }
    if (specifier === "@/lib/api/contracts") {
      return {
        isoDateTimeSchema: {
          parse(value) {
            if (typeof value !== "string" || Number.isNaN(Date.parse(value))) {
              throw new TypeError("invalid timestamp");
            }
            return value;
          },
        },
      };
    }
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    evaluatedModule,
    evaluatedModule.exports,
  );
  return evaluatedModule.exports.WebSocketClient;
}

const reconnect = Object.freeze({
  initialDelayMs: 0,
  maximumDelayMs: 0,
  multiplier: 1,
});

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

test.beforeEach(() => {
  FakeWebSocket.reset();
  globalThis.WebSocket = FakeWebSocket;
});

test.afterEach(() => {
  delete globalThis.WebSocket;
});

test("anonymous mode upgrades without issuing a private ticket", async (t) => {
  const WebSocketClient = await loadClient();
  const client = new WebSocketClient({
    connection: {
      mode: REALTIME_CONNECTION_MODE.ANONYMOUS,
      url: () => "ws://judgify.test/ws",
    },
    reconnect,
  });
  t.after(() => client.close());

  client.connect();
  await flush();

  assert.equal(FakeWebSocket.instances.length, 1);
  assert.equal(FakeWebSocket.instances[0].url, "ws://judgify.test/ws");
});

test("ticket mode issues a fresh single-use ticket for every upgrade", async (t) => {
  const WebSocketClient = await loadClient();
  const issuedTickets = [];
  const client = new WebSocketClient({
    connection: {
      mode: REALTIME_CONNECTION_MODE.TICKET,
      issueTicket: async () => {
        const ticket = `ticket-${issuedTickets.length + 1}`;
        issuedTickets.push(ticket);
        return ticket;
      },
      url: (ticket) => `ws://judgify.test/ws?ticket=${ticket}`,
    },
    reconnect,
  });
  t.after(() => client.close());

  client.connect();
  await flush();
  FakeWebSocket.instances[0].open();
  FakeWebSocket.instances[0].close();
  await flush();
  await flush();

  assert.deepEqual(issuedTickets, ["ticket-1", "ticket-2"]);
  assert.equal(
    FakeWebSocket.instances[1].url,
    "ws://judgify.test/ws?ticket=ticket-2",
  );
});

test("topic readiness requires a subscription ACK and resets on close", async (t) => {
  const WebSocketClient = await loadClient();
  const client = new WebSocketClient({
    connection: {
      mode: REALTIME_CONNECTION_MODE.ANONYMOUS,
      url: () => "ws://judgify.test/ws",
    },
    reconnect,
  });
  t.after(() => client.close());
  const states = [];
  client.subscribeToTopic("contest:00000000-0000-0000-0000-000000000001:standings");
  client.onTopicStateChange(
    "contest:00000000-0000-0000-0000-000000000001:standings",
    (state) => states.push(state),
  );

  client.connect();
  await flush();
  const socket = FakeWebSocket.instances[0];
  socket.open();
  client.resubscribe();
  const request = JSON.parse(socket.sent[0]);

  assert.notEqual(states.at(-1), "subscribed");
  socket.message({
    v: REALTIME_PROTOCOL.VERSION,
    op: REALTIME_PROTOCOL.OPERATION_SUBSCRIBED,
    id: request.id,
    topic: request.topic,
    type: REALTIME_PROTOCOL.TYPE_SUBSCRIPTION,
  });
  assert.equal(states.at(-1), "subscribed");

  socket.close();
  assert.equal(states.at(-1), "pending");
});

test("subscription protocol errors mark the correlated topic unhealthy", async (t) => {
  const WebSocketClient = await loadClient();
  const client = new WebSocketClient({
    connection: {
      mode: REALTIME_CONNECTION_MODE.ANONYMOUS,
      url: () => "ws://judgify.test/ws",
    },
    reconnect,
  });
  t.after(() => client.close());
  const topic = "contest:00000000-0000-0000-0000-000000000001:standings";
  const states = [];
  const errors = [];
  client.subscribeToTopic(topic);
  client.onTopicStateChange(topic, (state) => states.push(state));
  client.onProtocolError((error) => errors.push(error));

  client.connect();
  await flush();
  const socket = FakeWebSocket.instances[0];
  socket.open();
  client.resubscribe();
  const request = JSON.parse(socket.sent[0]);
  socket.message({
    v: REALTIME_PROTOCOL.VERSION,
    op: REALTIME_PROTOCOL.OPERATION_ERROR,
    id: request.id,
    type: "protocol.error.v1",
    data: { code: "forbidden_topic", retryable: false },
  });

  assert.equal(states.at(-1), "error");
  assert.deepEqual(errors.at(-1), {
    code: "forbidden_topic",
    retryable: false,
  });
});
