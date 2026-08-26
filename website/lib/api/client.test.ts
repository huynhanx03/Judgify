import assert from "node:assert/strict";
import { test } from "vitest";

import { createApiClient } from "@/lib/api/client";
import { createStaticCsrfTokenSource } from "@/lib/api/csrf";
import { entityIDSchema } from "@/lib/api/contracts";

const CID = "019f6abb-8dd5-7581-8449-2b9ad77873e5";
const ENTITY_ID = "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17";
const IDEMPOTENCY_KEY = "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a";

function success(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
      "X-Correlation-ID": CID,
    },
  });
}

function requestFromFetchArguments(
  input: URL | RequestInfo,
  init?: RequestInit,
): Request {
  return input instanceof Request ? input : new Request(input, init);
}

test("unsafe same-origin requests include cookies, CSRF, idempotency and expected revision", async () => {
  const requests: Request[] = [];
  const client = createApiClient({
    baseUrl: window.location.origin,
    csrf: createStaticCsrfTokenSource("signed-csrf-value"),
    fetcher: async (input, init) => {
      requests.push(requestFromFetchArguments(input, init));
      return success(
        {
          code: 20000,
          message: "success",
          data: { id: ENTITY_ID },
        },
        202,
      );
    },
  });

  const result = await client.api(
    "/submissions",
    {
      method: "POST",
      body: { problem_id: ENTITY_ID },
      idempotencyKey: IDEMPOTENCY_KEY,
      expectedVersion: 7,
      expectedStatus: 202,
      schema: {
        parse(value: unknown) {
          return {
            id: entityIDSchema.parse((value as { id?: unknown }).id),
          };
        },
      },
    },
  );

  assert.deepEqual(result, { id: ENTITY_ID });
  assert.equal(requests.length, 1);
  assert.equal(requests[0].credentials, "include");
  assert.equal(requests[0].headers.get("X-CSRF-Token"), "signed-csrf-value");
  assert.equal(
    requests[0].headers.get("Idempotency-Key"),
    IDEMPOTENCY_KEY,
  );
  assert.equal(requests[0].headers.get("If-Match"), '"7"');
  assert.equal(requests[0].headers.get("Authorization"), null);
  assert.equal(requests[0].headers.get("Content-Type"), "application/json");
});

test("safe requests omit CSRF and 204 returns undefined", async () => {
  const requests: Request[] = [];
  const client = createApiClient({
    baseUrl: window.location.origin,
    csrf: createStaticCsrfTokenSource("signed-csrf-value"),
    fetcher: async (input, init) => {
      requests.push(requestFromFetchArguments(input, init));
      return new Response(null, {
        status: 204,
        headers: { "X-Correlation-ID": CID },
      });
    },
  });

  const result = await client.api<void>("/auth/logout", { method: "GET" });

  assert.equal(result, undefined);
  assert.equal(requests[0].headers.get("X-CSRF-Token"), null);
  assert.equal(requests[0].headers.get("Content-Type"), null);
});

test("a data-bearing 200 response cannot silently become undefined", async () => {
  const client = createApiClient({
    baseUrl: window.location.origin,
    fetcher: async () =>
      new Response(null, {
        status: 200,
        headers: {
          "Content-Length": "0",
          "X-Correlation-ID": CID,
        },
      }),
  });

  await assert.rejects(
    () => client.api("/session/me"),
    (error: unknown) =>
      error instanceof Error &&
      "code" in error &&
      error.code === "invalid_response",
  );
});

test("commands can send a domain-specific bounded If-Match entity tag", async () => {
  const requests: Request[] = [];
  const client = createApiClient({
    baseUrl: window.location.origin,
    csrf: createStaticCsrfTokenSource("signed-csrf-value"),
    fetcher: async (input, init) => {
      requests.push(requestFromFetchArguments(input, init));
      return success({
        code: 20000,
        message: "success",
        data: { id: ENTITY_ID },
      });
    },
  });

  await client.api("/contests/example/publish", {
    method: "POST",
    body: { reason: "release approved" },
    idempotencyKey: IDEMPOTENCY_KEY,
    ifMatch: '"contest-v4"',
  });

  assert.equal(requests[0].headers.get("If-Match"), '"contest-v4"');
});

test("unsafe requests are never retried without their own idempotency key", async () => {
  let calls = 0;
  const client = createApiClient({
    baseUrl: window.location.origin,
    csrf: createStaticCsrfTokenSource("signed-csrf-value"),
    fetcher: async () => {
      calls += 1;
      return success(
        {
          error: { code: "authentication_expired" },
          meta: { cid: CID },
        },
        401,
      );
    },
  });
  client.setUnauthorizedHandler(async () => true);

  await assert.rejects(
    () =>
      client.api("/commands", {
        method: "POST",
        body: {},
      }),
    /authentication_expired/,
  );
  assert.equal(calls, 1);
});

test("external cancellation reaches fetch without being replaced by timeout state", async () => {
  const controller = new AbortController();
  const client = createApiClient({
    baseUrl: window.location.origin,
    csrf: createStaticCsrfTokenSource("signed-csrf-value"),
    fetcher: async (input, init) =>
      new Promise<Response>((_resolve, reject) => {
        const request = requestFromFetchArguments(input, init);
        request.signal.addEventListener(
          "abort",
          () => reject(request.signal.reason),
          { once: true },
        );
      }),
  });

  const pending = client.api("/slow", {
    signal: controller.signal,
    timeoutMs: 60_000,
  });
  controller.abort(new DOMException("cancelled", "AbortError"));

  await assert.rejects(pending, /cancelled/);
});

test("timeout remains active while the response body is being consumed", async () => {
  const client = createApiClient({
    baseUrl: window.location.origin,
    fetcher: async (input, init) => {
      const request = requestFromFetchArguments(input, init);
      const body = new ReadableStream<Uint8Array>({
        start(controller) {
          request.signal.addEventListener(
            "abort",
            () => controller.error(request.signal.reason),
            { once: true },
          );
        },
      });
      return new Response(body, {
        status: 200,
        headers: {
          "Content-Type": "application/json",
          "X-Correlation-ID": CID,
        },
      });
    },
  });

  await assert.rejects(
    () => client.api("/slow-body", { timeoutMs: 10 }),
    (error: unknown) =>
      error instanceof Error &&
      "code" in error &&
      error.code === "request_timeout",
  );
});

test("streaming response size is rejected before unbounded buffering", async () => {
  const client = createApiClient({
    baseUrl: window.location.origin,
    maximumResponseCharacters: 32,
    fetcher: async () =>
      success({
        code: 20000,
        message: "success",
        data: { value: "x".repeat(128) },
      }),
  });

  await assert.rejects(
    () => client.api("/oversized"),
    (error: unknown) =>
      error instanceof Error &&
      "code" in error &&
      error.code === "invalid_response",
  );
});

test("transport rejects malformed local authority, unsafe paths, and owned headers", async () => {
  await assert.rejects(
    () => createApiClient({ baseUrl: "//evil.example" }).api("/safe"),
    /base URL/,
  );
  await assert.rejects(
    () => createApiClient({ baseUrl: "https://user@example.com" }).api("/safe"),
    /base URL/,
  );
  const client = createApiClient({ baseUrl: window.location.origin, fetcher: fetch });
  await assert.rejects(() => client.api("relative"), /same-origin absolute path/);
  await assert.rejects(() => client.api("//evil.example"), /same-origin absolute path/);
  await assert.rejects(
    () => client.api("/safe", { headers: { Authorization: "Bearer no" } }),
    /owned by the API transport/,
  );
});

test("safe reads retry once after canonical unauthorized recovery", async () => {
  let calls = 0;
  const client = createApiClient({
    baseUrl: window.location.origin,
    fetcher: async () => {
      calls += 1;
      return calls === 1
        ? success({ error: { code: "authentication_expired" }, meta: { cid: CID } }, 401)
        : success({ code: 20000, message: "success", data: { id: ENTITY_ID } });
    },
  });
  client.setUnauthorizedHandler(async () => true);

  assert.deepEqual(await client.api("/profile"), { id: ENTITY_ID });
  assert.equal(calls, 2);
});

test("response configuration boundaries are rejected before a request is made", () => {
  assert.throws(() => createApiClient({ defaultTimeoutMs: 0 }), /request timeout/);
  assert.throws(() => createApiClient({ defaultTimeoutMs: 120_001 }), /request timeout/);
  assert.throws(() => createApiClient({ maximumResponseCharacters: 0 }), /response size/);
  assert.throws(() => createApiClient({ maximumResponseCharacters: 64 * 1024 * 1024 + 1 }), /response size/);
});

test("request contract rejects unsafe method, version, and entity-tag combinations", async () => {
  const client = createApiClient({ baseUrl: window.location.origin, csrf: createStaticCsrfTokenSource("signed-csrf-value"), fetcher: async () => success({ code: 20000, message: "success", data: {} }) });
  await assert.rejects(() => client.api("/safe", { body: {} }), /cannot carry a body/);
  await assert.rejects(() => client.api("/safe", { expectedStatus: 199 }), /expected HTTP status/);
  await assert.rejects(() => client.api("/safe", { expectedVersion: 1, ifMatch: '"v1"' }), /mutually exclusive/);
  await assert.rejects(() => client.api("/safe", { expectedVersion: 0 }), /expected version/);
  await assert.rejects(() => client.api("/safe", { ifMatch: "unquoted" }), /valid bounded entity tag/);
});

test("successful response contracts require the declared status, JSON media type, and CID", async () => {
  const mismatched = createApiClient({ baseUrl: window.location.origin, fetcher: async () => success({ code: 20000, message: "success", data: {} }, 200) });
  await assert.rejects(() => mismatched.api("/safe", { expectedStatus: 201 }), /invalid_response/);
  const nonJson = createApiClient({ baseUrl: window.location.origin, fetcher: async () => new Response("ok", { status: 200, headers: { "X-Correlation-ID": CID } }) });
  await assert.rejects(() => nonJson.api("/safe"), /invalid_response/);
  const emptyMissingCID = createApiClient({ baseUrl: window.location.origin, fetcher: async () => new Response(null, { status: 204 }) });
  await assert.rejects(() => emptyMissingCID.api("/safe"), /invalid_response/);
});

test("an explicitly public request never invokes session recovery", async () => {
  let recoveries = 0;
  const client = createApiClient({
    baseUrl: window.location.origin,
    fetcher: async () => success({ error: { code: "authentication_expired" }, meta: { cid: CID } }, 401),
  });
  client.setUnauthorizedHandler(async () => { recoveries += 1; return true; });
  await assert.rejects(() => client.api("/public", { auth: "none" }), /authentication_expired/);
  assert.equal(recoveries, 0);
});

test("unsafe commands require CSRF unless the typed contract explicitly omits it", async () => {
  const withoutCsrf = createApiClient({
    baseUrl: window.location.origin,
    csrf: { read: () => null },
    fetcher: async () => success({ code: 20000, message: "success", data: {} }),
  });
  await assert.rejects(
    () => withoutCsrf.api("/webhook-callback", { method: "POST", body: {} }),
    /CSRF bootstrap/,
  );
  assert.deepEqual(
    await withoutCsrf.api("/webhook-callback", { method: "POST", body: {}, csrf: "omit" }),
    {},
  );
});
