# Go-common Production Library Design

**Date:** 2026-07-16  
**Status:** Draft for written review; implementation will be governed by a separate execution plan  
**Module:** `github.com/huynhanx03/go-common`  
**Primary consumer:** Judgify, without embedding Judgify domain concepts in the library

## 1. Purpose

`go-common` becomes the production-grade shared library for current and future Go services. It remains one repository and one Go module, and it retains the existing package organization as the base. The work is a logic, safety, lifecycle, security, performance, testing, and documentation upgrade rather than a cosmetic mass rename or a framework rewrite.

Consumers continue importing the subpackages they need:

```go
import (
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/database/ent"
	"github.com/huynhanx03/go-common/pkg/logger"
)
```

There is no root god package that re-exports every feature. A single module gives unified versioning; focused subpackages preserve dependency clarity, compile-time isolation, and maintainability.

## 2. Scope and non-goals

### In scope

- Preserve and harden the current `pkg/common`, `pkg/database`, `pkg/mq`, `pkg/logger`, `pkg/settings`, `pkg/security`, `pkg/hash`, and supporting package structure.
- Add reusable production primitives for authentication, authorization, correlation, lifecycle, health, gRPC, and WebSocket.
- Correct unsafe or ambiguous HTTP, cache, database, MQ, retry, pagination, concurrency, and shutdown behavior.
- Define explicit durability, ownership, cancellation, backpressure, and error semantics.
- Add compatibility wrappers so existing services can migrate incrementally.
- Establish stable, deprecated, and experimental maturity levels with enforceable release gates.
- Make Judgify the first production consumer and contract proving ground.

### Out of scope

- Splitting `go-common` into multiple Go modules.
- Replacing the existing package tree with a new framework taxonomy.
- A root package that imports Gin, Ent, Casbin, Kafka, Redis, and WebSocket together.
- Judgify entities, policies, job schemas, scoring rules, testsets, submission state machines, or other application behavior.
- A distributed durable workflow engine.
- Microservice infrastructure or multi-node coordination that is not required by a real consumer.
- Hiding vendor types inside packages explicitly named after that vendor, such as `database/ent` or `security/authorization/casbin`. Vendor-neutral core packages must remain vendor-neutral.

## 3. Design principles

### 3.1 Generic core, adapter, domain enrichment

Every capability follows this dependency direction:

```text
generic core
    -> transport/infrastructure adapter
        -> service/domain integration
```

Examples:

- `pkg/correlation` owns a correlation identifier in `context.Context`.
- HTTP, gRPC, Kafka, and WebSocket adapters propagate it.
- Judgify attaches `submission_id`, `contest_id`, or `problem_id` when those fields are relevant.

`go-common` must never import a Judgify package or hardcode a Judgify field. Generic identifiers such as `job_id`, `event_id`, `causation_id`, and `correlation_id` may be supported by generic job or event adapters, but a domain identifier remains service-owned.

### 3.2 Explicit semantics over convenience

Public APIs state:

- whether data is memory-only or durable;
- when an operation is acknowledged;
- whether the implementation copies caller-owned buffers;
- whether ordering is guaranteed and at what scope;
- how cancellation and deadlines behave;
- who owns background goroutines;
- how shutdown drains or rejects work;
- what happens under overload;
- whether an error is retryable.

An API must not return a successful-looking zero value when the operation did not succeed. Operations that require a result cannot return `(nil, nil)`.

### 3.3 Bounded by default

Queues, buffers, caches, retries, fan-out, goroutines, payload sizes, request bodies, log cardinality, and shutdown waits have finite defaults. Unlimited behavior must require an explicit option and must be rejected where it can break process safety.

### 3.4 Context and lifecycle ownership

Every potentially blocking operation accepts `context.Context`. Every goroutine has an owner and a termination path. Libraries never capture OS signals, call `log.Fatal`, or leave unbounded background work behind. `Close` or `Shutdown` is idempotent and has documented drain semantics.

### 3.5 Safe defaults, configurable production behavior

Security and durability defaults fail closed. Timeouts and TLS modes are configurable, validated, and never silently disabled. Configuration uses `time.Duration` rather than ambiguous integers.

### 3.6 Compatibility without preserving unsafe semantics forever

Safe internal corrections that preserve the intended contract may land behind an existing API. When an old API is inherently ambiguous or unsafe, a new explicit API is introduced and the old API delegates where possible with a `Deprecated:` notice. Unsafe legacy behavior is not promoted as stable merely to avoid a migration.

## 4. Target package taxonomy

The existing tree remains the foundation. The following packages are added or reorganized deliberately:

```text
pkg/
├── common/
│   ├── http/
│   │   ├── handler/
│   │   ├── middlewares/
│   │   ├── request/
│   │   ├── response/
│   │   └── validation/
│   ├── grpc/
│   │   └── interceptors/
│   ├── websocket/
│   ├── lifecycle/
│   ├── health/
│   ├── cache/
│   ├── apperr/
│   └── tx/
├── security/
│   ├── authentication/
│   │   └── jwt/
│   ├── authorization/
│   │   └── casbin/
│   └── password/
├── correlation/
├── logger/
├── database/
├── mq/
├── settings/
├── hash/
└── ... existing focused packages
```

The design intentionally does not add:

- `pkg/common/observability`: logging remains in `pkg/logger`; correlation remains focused; metrics hooks live with the component they describe until a real shared abstraction exists.
- `pkg/realtime/envelope`: the only approved realtime transport is WebSocket, so its envelope belongs to `pkg/common/websocket`.
- nested `http/middlewares/security` or `http/request/resilience`: the existing HTTP folders already provide the correct boundary.

## 5. Package maturity policy

Every public package is classified in a machine-readable package catalog and in its package documentation.

### Stable

A stable package has:

- documented API and semantics;
- unit and contract tests;
- race coverage for concurrent behavior;
- fuzz tests for parsers, framing, or untrusted input where applicable;
- benchmarks for performance-sensitive behavior;
- deterministic cleanup and no hidden external service startup;
- semantic versioning guarantees.

Stable packages may be used on production critical paths.

### Deprecated

A deprecated package or symbol remains functional for a documented compatibility window, delegates to the replacement where safe, and names the exact replacement in Go documentation. It receives security and correctness fixes but no new features.

### Experimental

An experimental package has no compatibility guarantee and cannot be imported by a production critical-path package. Its package documentation states its limits. Forge, unsafe runtime helpers, custom synchronization primitives, and unproven generic repositories remain experimental until they meet the stable gates.

### Internal implementation detail

Private implementation packages are free to change. Stable public core contracts must not expose `coder/websocket`, Gin, Zap, Casbin, Ristretto, or another vendor type unless the package itself is explicitly a vendor adapter.

## 6. Security packages

### 6.1 `pkg/security/password`

The current password helpers move from the broad `pkg/security` package into the focused `pkg/security/password` package.

The package provides:

```go
type Hasher struct { /* immutable validated policy and bounded work gate */ }

type Verification struct {
	NeedsRehash bool
	Replacement string // populated only after successful VerifyAndUpgrade
}

func NewHasher(policy Policy) (*Hasher, error)
func (h *Hasher) Hash(ctx context.Context, plain string) (string, error)
func (h *Hasher) Verify(ctx context.Context, encoded, plain string) error
func (h *Hasher) VerifyAndUpgrade(ctx context.Context, encoded, plain string) (Verification, error)
func (h *Hasher) NeedsRehash(encoded string) (bool, error)

// Default-policy conveniences for callers that cannot yet inject a Hasher.
func Hash(plain string) (string, error)
func Verify(encoded, plain string) error
func NeedsRehash(encoded string, policy Policy) (bool, error)
```

The stable default is a versioned Argon2id policy implemented with `golang.org/x/crypto/argon2`. New hashes use the standard PHC representation `$argon2id$v=19$m=...,t=...,p=...$salt$hash`; output is self-describing and contains no vendor-private wrapper. The policy version pins the exact memory, iterations, parallelism, salt length, derived-key length, input limit, and admitted-concurrency limit. Before a release is tagged, a checked-in benchmark artifact from the production machine class selects and records those exact parameters and latency/RSS budgets. An unmeasured placeholder must not silently become the stable default, and any later parameter change creates a new policy version while old supported hashes remain verifiable.

Salt bytes come from `crypto/rand` and generation failure fails hashing. PHC parsing first bounds the encoded string and segment/base64 lengths, then validates algorithm, version, duplicate/unknown parameters, integer overflow, and configured minimum/maximum memory, iteration, parallelism, salt, and derived-key limits **before** invoking Argon2 or allocating attacker-declared work buffers. The derived key is compared with `crypto/subtle.ConstantTimeCompare`. Mutable password/derived-key/salt copies are zeroed as soon as practical; immutable Go strings cannot be promised erased and are never retained, logged, placed in errors, or attached to metrics/traces.

Password semantics are exact input bytes. The package never trims, case-folds, Unicode-normalizes, or otherwise rewrites a password; limits are measured in bytes and checked before expensive work. Typed errors distinguish malformed/unsupported stored hashes, credential mismatch, input-too-large, work-gate saturation/cancellation, and entropy failure without returning secret data to callers.

The migration verifier recognizes only explicitly supported legacy bcrypt modular-crypt prefixes and a configured bounded set/range of costs discovered by a pre-migration hash audit, using `golang.org/x/crypto/bcrypt`. Cost `12` is the expected current case, but a previously issued hash at another observed safe cost is not allowed to lock out its user. The configured costs are validated against benchmarked CPU limits; unobserved, unsafe, or malformed encodings fail before expensive work. A successful legacy verification always reports `NeedsRehash`. `VerifyAndUpgrade` then creates an Argon2id replacement under the current policy and returns it to the authentication service, which persists it with a compare-and-swap on the old hash after the login transaction has otherwise succeeded. A mismatch never hashes an upgrade and never changes storage. Legacy bcrypt generation is not exposed.

Every `Hasher` owns a bounded semaphore for both Argon2id and bcrypt work. Context-aware methods acquire it before hashing and return on cancellation or the configured bounded wait; they never create a goroutine per queued request. The process-wide default conveniences share one bounded default hasher rather than creating new semaphores. Benchmarks and load tests prove the selected concurrency cap stays within the service memory budget because Argon2 memory cost multiplied by concurrent hashes is part of capacity planning, not merely a per-call setting.

The existing `security.HashPassword` and `security.ComparePassword` functions become compatibility wrappers. They are not removed until all known consumers have migrated through at least one compatibility release.

### 6.2 `pkg/hash`

`pkg/hash` is explicitly non-cryptographic and must not be used for passwords, tokens, signatures, reset secrets, or credential storage. Stable hashing uses a documented algorithm such as xxhash when that behavior is part of the contract. The package removes dependence on private Go runtime symbols and `go:linkname`.

Public naming communicates the contract, for example stable versus seeded process-local hashes. A process-random or runtime-dependent hash is never presented as persisted/stable output.

### 6.3 `pkg/security/authentication`

Authentication logic is transport-neutral. It verifies a credential and returns a typed principal; it does not render HTTP responses, access a Gin context, or own a service's session database.

Core concepts:

```go
type TokenPurpose string

const (
	PurposeAccess  TokenPurpose = "access"
	PurposeRefresh TokenPurpose = "refresh"
)

type Principal struct {
	Authenticated bool
	Subject   string
	UserID    string
	SessionID string
	Username  string
	TokenID   string
	AuthenticatedUntil time.Time
}

type Verifier interface {
	Verify(ctx context.Context, raw string, expected TokenPurpose) (Principal, error)
}

type Issuer interface {
	Issue(ctx context.Context, request IssueRequest) (IssuedToken, error)
}
```

`Principal` holds generic identity facts. `Authenticated=false` is the explicit anonymous value: subject/user/session/token fields and `AuthenticatedUntil` must be empty. `Authenticated=true` requires a non-empty stable subject and applies the normal expiry contract. A successful credential `Verifier` always returns authenticated; anonymous is constructed explicitly by an optional transport resolver only when credentials are absent and its service policy permits it. A presented malformed, expired, wrong-purpose, or otherwise invalid credential remains an error and can never be downgraded to anonymous. Services may attach additional facts through their own types rather than extending common code for each domain.

The JWT adapter in `pkg/security/authentication/jwt`:

- validates the exact allowed signing algorithm;
- requires token purpose and rejects refresh tokens at access-token boundaries;
- validates issuer, audience, subject, expiry, issued-at, and not-before;
- supports bounded clock skew;
- requires and resolves `kid` for key rotation when multiple keys are configured;
- supports explicit active/retired key sets;
- returns typed errors for malformed, expired, not-yet-valid, wrong-purpose, wrong-issuer/audience, and unknown-key cases;
- never logs a raw token or claims containing secrets.

The generic library does not own refresh-session persistence, rotation, reuse detection, or user status. Those are application workflows because their storage and security policy belong to the service. The library provides the token primitives needed to implement them safely.

### 6.4 `pkg/security/authorization`

Authorization is also transport-neutral:

```go
type Request struct {
	Principal authentication.Principal
	Resource  string
	Action    string
}

type Decision struct {
	Allowed  bool
	Reason   string
	Revision int64
}

type Authorizer interface {
	Authorize(ctx context.Context, request Request) (Decision, error)
}
```

`Decision.Reason` is a bounded machine code suitable for metrics and safe client mapping, never a raw Casbin, policy, or database error. The initial stable contract intentionally carries no open-ended attribute map. Services perform object and state checks in their application layer; a future typed policy-facts contract requires its own versioned design. The package fails closed: evaluation errors never become an allow decision.

The Casbin adapter in `pkg/security/authorization/casbin` supplies a reusable runtime:

- builds an enforcer from a complete validated policy snapshot;
- publishes a new snapshot by atomic swap;
- exposes applied policy revision;
- permits concurrent authorization while a candidate snapshot is prepared;
- guarantees a request sees the complete old or complete new policy;
- reports desired/applied revision mismatch as an unhealthy state;
- does not expose the underlying enforcer through the core `Authorizer` contract.

Policy persistence, role catalogs, resource/action registries, self-lockout rules, superadmin invariants, audit records, and transactional admin commands are service concerns. Judgify implements those in its Identity module using Ent, Casbin, and the Ent adapter. The common Casbin runtime consumes a complete snapshot; it does not mutate application policy row by row.

### 6.5 `pkg/oauth`

The existing root package remains a provider-adapter package; it does not become a second authentication/session system. Application code owns browser state, PKCE-verifier storage, nonce/pre-session binding, completion tickets, account linking, sessions, and audit. The reusable provider contract only builds a bounded authorization request and exchanges a returned code for a normalized provider subject/profile.

Stable constructors return `(*Provider, error)` and validate client ID presence, secret handling, exact redirect URI, scopes, endpoint policy, injected HTTP client, and timeouts. Authorization URL creation requires nonempty bounded state plus PKCE S256 challenge; code exchange requires the matching verifier and exact redirect URI. Development HTTP redirects/endpoints require an explicit development option; production accepts HTTPS provider endpoints and registered same-origin callback configuration only.

Provider HTTP calls preserve the caller context, use an injected client/transport with a total deadline and connection limits, check status/content type, cap response bytes before decode, and close/drain bodies safely. GitHub email lookup does not call a context-free `client.Get`, silently swallow a transport/parse failure as a verified identity, or select an unverified email. Google/GitHub immutable provider subject—not email—is the normalized `ExternalID`. The normalized result has bounded typed fields for subject, email/verification, display name, login, and avatar URL rather than an unbounded `map[string]any`; provider access/refresh tokens and raw responses are never returned, retained, or logged.

The adapter exposes typed error classes for state/PKCE input, exchange rejection, provider unavailability/rate limiting, malformed/oversized response, and missing immutable subject. Application handlers map them without leaking code, token, client secret, or provider body. Contract tests use a local TLS test server and cover timeout, cancellation, redirect mismatch, PKCE, status/content-type/body limits, duplicate/unknown fields, subject/email semantics, and redaction. Existing `AuthCodeURL(state)` and metadata-map results remain deprecated wrappers only during the consumer migration.

## 7. Correlation and context propagation

### 7.1 Core package

`pkg/correlation` replaces the overloaded role of `pkg/cid` while preserving its UUIDv7-based intent:

```go
func New() string
func WithContext(ctx context.Context, id string) context.Context
func FromContext(ctx context.Context) string
func EnsureContext(ctx context.Context) context.Context
func Validate(id string) error
```

Incoming identifiers are bounded and validated against an explicit character/length policy. Invalid external values are not echoed or trusted; the inbound adapter creates a new identifier. The correlation ID can span HTTP, gRPC, WebSocket, Kafka, local jobs, and other asynchronous work, so the package is not named `requestid`.

`pkg/cid` becomes a deprecated compatibility facade that re-exports or delegates to `pkg/correlation` without maintaining a second context key.

### 7.2 Transport ownership

Propagation belongs to the transport package:

- HTTP inbound: `pkg/common/http/middlewares/correlation.go`
- HTTP outbound: `pkg/common/http/request/correlation.go`
- gRPC: `pkg/common/grpc/interceptors/correlation.go`
- Kafka: `pkg/mq/kafka/correlation.go`
- WebSocket: connection context in `pkg/common/websocket`

Async envelopes may carry:

- `correlation_id`: the originating flow;
- `causation_id`: the command/event that directly caused this work;
- an adapter-owned generic `event_id` or `job_id` when such a concept exists.

A worker restores these values into context before invoking the handler. It then enriches the logger with service/domain identifiers. The common library does not define `submission_id` or any other Judgify-specific field.

## 8. Logging and traceability

`pkg/logger` remains the logging home. No umbrella observability package is introduced.

### 8.1 Structured contract

Every record contains, when configured or available:

```text
timestamp, level, service, version, environment, component, message, cid
```

Callers add bounded fields such as `user_id`, `job_id`, `event_id`, `order_id`, or `submission_id`. Only `cid` is automatically understood by the generic context logger. Generic adapters may attach their own generic identifiers; domain fields are supplied by the service.

The preferred usage is context-first:

```go
ctx = logger.WithFields(ctx,
	logger.String("submission_id", submissionID),
	logger.String("job_id", jobID),
)

logger.FromContext(ctx).Info("judge job claimed")
```

The exact field implementation may use Zap internally. New vendor-neutral helpers avoid requiring every service layer to import Zap. Existing Zap-returning APIs remain as compatibility surface while consumers migrate.

### 8.2 Level semantics

- `DEBUG`: detailed internal decisions, state transition detail, cache behavior, retry decisions, scheduler selection, and diagnostic timings.
- `INFO`: lifecycle events, request summaries, accepted commands, durable state transitions, policy revision changes, worker/job milestones, and WebSocket connect/disconnect.
- `WARN`: rate limiting, transient retry, lease expiry, outbox lag, slow clients, degraded dependency, and rejected malformed credentials.
- `ERROR`: an operation failed, a dependency remained unavailable, a job became dead, or an invariant-preserving workflow could not complete.
- `PANIC`/`DPANIC`: reserved for unrecoverable programmer invariants at a containment boundary.
- `FATAL`: allowed only in the executable entry point before lifecycle orchestration starts; never inside a library or application component.

Default thresholds:

```text
development: debug
test:        warn
staging:     info
production:  info
```

Production contains debug call sites but does not emit them at the default threshold. A service may temporarily raise a named component to debug using an authenticated, audited, time-bounded control backed by an atomic level. The raw logger level handler is not exposed publicly without application authorization.

Expensive debug fields use lazy evaluation or a level check. Hot loops, per-testcase logs, and WebSocket message logs are sampled or rate-limited.

### 8.3 Secret and cardinality policy

No level may log:

- passwords or password hashes;
- access, refresh, OAuth, reset, CSRF, API, or lease tokens;
- `Authorization` or `Cookie` headers;
- database credentials or private keys;
- submitted source code;
- hidden testcase input, expected output, or raw secret policy payloads.

HTTP query logging redacts sensitive keys. Errors are assigned stable codes and include `cid` in client-facing error metadata without exposing stack traces. Field names are normalized; unbounded maps, arbitrary headers, raw payloads, and high-cardinality content are rejected or summarized.

JSON to stdout is the production default. Process/container operations own retention, rotation, and disk quotas. Optional local file output remains explicit and must not make logger construction panic.

## 9. HTTP package hardening

The existing `pkg/common/http` layout remains. Security middleware is added directly to `middlewares`; resilience functions remain directly in `request`.

### 9.1 Authentication middleware

The current middleware becomes a thin HTTP adapter over `authentication.Verifier`:

```go
func AuthenticationWithVerifier(v authentication.Verifier, opts ...AuthenticationOption) gin.HandlerFunc
func OptionalAuthenticationWithVerifier(v authentication.Verifier, opts ...AuthenticationOption) gin.HandlerFunc
```

Behavior is explicit:

- required authentication returns `401` when credentials are missing or invalid;
- optional authentication continues anonymously only when credentials are absent;
- if a caller sends malformed, expired, or wrong-purpose credentials, optional authentication returns `401` rather than silently downgrading the request to anonymous;
- the middleware expects `PurposeAccess`;
- credential extraction supports configured bearer or cookie sources with deterministic precedence;
- conflicting credentials are rejected;
- a typed `Principal` is stored in request context;
- legacy user-ID/username context values are populated only by the compatibility layer.

The existing `Authentication(publicKey)` and `OptionalAuthentication(publicKey)` functions delegate to the JWT verifier and are deprecated compatibility-only wrappers. Their old signature cannot express complete issuer/audience/key-set policy, so Judgify and every stable critical path must use `AuthenticationWithVerifier`; a release architecture check forbids imports of the wrappers from stable application code.

### 9.2 Authorization middleware

`AuthorizationChecker` retains its current resource/action shape but depends on `authorization.Authorizer` and a typed principal.

- no principal returns `401`;
- a valid deny decision returns `403`;
- an evaluator, storage, or runtime error fails closed and returns `500` with a stable error code, not a misleading `403`;
- route middleware performs coarse permission checks;
- object ownership and domain state checks remain in application services.

The existing bitmask `PermissionChecker` remains only as a compatibility path. Casbin-backed resource/action authorization is the supported path for Judgify and new services.

### 9.3 CORS, CSRF, and headers

CORS becomes constructor-based configuration rather than a global reflection function:

- exact normalized origin allowlist;
- wildcard origin is incompatible with credentials;
- preflight methods and headers are explicit;
- `Vary: Origin` is set correctly;
- disallowed origins receive no permissive CORS headers;
- development localhost origins require explicit development configuration.

Cookie-authenticated state changes use CSRF protection with a service-selected token strategy. Middleware also provides configurable security headers, body limits, content-type enforcement, proxy trust, and request deadline support. Defaults are secure but do not hardcode a Judgify deployment origin.

### 9.4 Middleware composition and concurrency

Gin middleware ordering is represented as Gin's normal handler chain. `Compose` must not manually invoke middleware that itself calls `c.Next`, because that can duplicate or reorder execution. `ComposeHandlers` may remain as a simple ordering helper with characterization tests.

`Parallel` is removed from the stable API. A single `*gin.Context` is not safe for parallel handler execution. Parallel application work uses request-layer fan-out against `context.Context` and immutable inputs, then joins before rendering one response.

Recovery catches panics at the request boundary, logs a redacted stack with CID, and returns the stable internal-error envelope. It does not hide cancellation or write a second response after headers are committed.

### 9.5 Rate limiting and circuit breaking

Rate-limit storage is bounded and has an owned cleanup lifecycle. Configuration rejects zero or negative derived limits instead of silently selecting unrelated defaults. Client identity respects trusted-proxy configuration and is not taken from arbitrary forwarded headers.

The inbound global circuit-breaker middleware is deprecated. A sequence of unrelated handler failures must never open a circuit for the entire API. Circuit breakers belong around a named outbound dependency and combine with deadlines, bounded concurrency, and retry policy.

### 9.6 HTTP server contract

The shared HTTP server helper requires explicit or validated defaults for:

- read-header timeout;
- read timeout;
- write timeout;
- idle timeout;
- maximum header bytes;
- maximum request body size;
- graceful drain timeout.

It accepts a parent context, never owns process signals, and returns startup/serve/shutdown errors to `pkg/common/lifecycle`.

### 9.7 Outbound requests and retry

The HTTP client and `pkg/common/http/request` use one explicit resilience contract:

- every request has a context and timeout budget;
- retry count zero means no retry;
- request bodies are replayed only through `GetBody` or a copied/re-creatable body;
- non-idempotent operations are not retried unless the caller supplies an idempotency key and explicitly enables the policy;
- retryable network errors and status codes are listed, not inferred from every error;
- `Retry-After` is honored within the remaining deadline;
- exponential backoff has jitter and a maximum delay;
- the final response or final error is always returned and never `(nil, nil)`;
- canceled/deadline-exceeded contexts stop immediately;
- intermediate response bodies are drained/closed according to connection-reuse rules;
- attempts, elapsed time, and last error are observable without logging secrets.

`Fanout` uses bounded concurrency, recovers child panics into typed errors, preserves result ordering, cancels siblings for first-success/first-error modes as documented, and never starts unbounded goroutines.

## 10. gRPC package

gRPC is a transport peer of HTTP and WebSocket:

```text
pkg/common/grpc/
├── interceptors/
│   ├── correlation.go
│   ├── authentication.go
│   ├── authorization.go
│   ├── logging.go
│   └── recovery.go
├── errors.go
└── options.go
```

Only implemented capabilities are created; empty client/server framework layers are not added for appearance.

Interceptors reuse the same correlation, authentication, authorization, logger, and error concepts as HTTP. Unary and stream interceptors preserve context propagation. Stream wrappers return the enriched context. Metadata is bounded and validated. gRPC status mapping uses stable application error codes and never exposes internal error text.

The gRPC code currently under `pkg/cid` migrates to the transport package. Deprecated `pkg/cid` interceptors delegate to the new implementation during the compatibility window.

## 11. WebSocket package

WebSocket lives at `pkg/common/websocket`, parallel to `pkg/common/http` and `pkg/common/grpc`. It does not live under HTTP merely because the connection begins as an HTTP upgrade, and it does not add an unnecessary `realtime` parent package.

### 11.1 Core and dependency boundary

`github.com/coder/websocket` is the private protocol engine. No public type exposes it. The package owns:

- HTTP upgrade handler;
- connection identity and lifecycle;
- one read pump and one writer pump per connection;
- ping/pong and deadlines;
- bounded outgoing mailbox;
- handler registry;
- rooms/topics;
- local hub routing;
- backpressure policy;
- graceful shutdown;
- message envelope and protocol errors;
- a private fan-out boundary for future multi-process delivery.

Gin integration uses `gin.WrapH` or a small adapter outside the WebSocket core; the core remains `net/http` compatible.

### 11.2 Design-level API

```go
type Options struct {
	AllowedOrigins                   []string
	ReadLimit                        int64
	WriteQueueCapacity               int
	WriteQueueByteCapacity           int64
	MaxHubQueuedBytes                int64
	MaxOutboundMessageBytes          int64
	InboundMessagesPerSecond         int
	InboundBurst                     int
	MaxInFlightHandlersPerConnection int
	MaxInFlightHandlers              int
	HandlerTimeout                   time.Duration
	PingInterval                     time.Duration
	PongTimeout                      time.Duration
	WriteTimeout                     time.Duration
	ShutdownTimeout                  time.Duration
	MaxConnections                   int
	MaxConnectionsPerPrincipal       int
	MaxConnectionsPerRemoteIP        int
	UpgradeRatePerSecondPerRemoteIP  int
	UpgradeBurstPerRemoteIP          int
	MaxSubscriptions                 int
}

type PrincipalResolver func(ctx context.Context, request *http.Request) (authentication.Principal, error)
type TopicAuthorizer func(ctx context.Context, principal authentication.Principal, topic string) error

type Hub struct { /* private state */ }
type Conn struct { /* private transport */ }

type DeliveryClass uint8

const (
	DeliveryLatestState DeliveryClass = iota + 1
	DeliveryEphemeral
	DeliveryCritical
)

type Message struct {
	Envelope    Envelope
	Class       DeliveryClass
	CoalesceKey string
}

type HandlerOptions[T any] struct {
	Parallelism int
	OrderingKey func(*T) string
	Timeout     time.Duration
}

type Handler[T any] func(ctx context.Context, conn *Conn, message *T) error

type PrincipalClass uint8

const (
	PrincipalAny PrincipalClass = iota
	PrincipalAuthenticated
	PrincipalAnonymous
)

type ConnectionSelector struct {
	All            bool
	PrincipalClass PrincipalClass
	Subjects      []string
	SessionIDs    []string
	ConnectionIDs []string
	Topics        []string
}

type CloseOptions struct {
	Code   int
	Reason string
}

type CloseResult struct {
	Matched        int
	MarkedClosing  int
	AlreadyClosing int
}

func NewHub(options Options) (*Hub, error)
func (h *Hub) Handler(resolve PrincipalResolver, authorize TopicAuthorizer) http.Handler
func (h *Hub) Run(ctx context.Context) error
func (h *Hub) Shutdown(ctx context.Context) error
func (h *Hub) Broadcast(ctx context.Context, topic string, message Message) error
func (h *Hub) SendToConnection(ctx context.Context, connectionID string, message Message) error
func (h *Hub) SendToPrincipal(ctx context.Context, subject string, message Message) error
func (h *Hub) CloseConnections(ctx context.Context, selector ConnectionSelector, options CloseOptions) (CloseResult, error)
func On[T any](h *Hub, messageType string, handler Handler[T]) error
func OnWithOptions[T any](h *Hub, messageType string, options HandlerOptions[T], handler Handler[T]) error
```

`Broadcast` targets current subscribers of one authorized application topic. A valid broadcast with zero current subscribers succeeds as a normal no-op because leave races are expected. `SendToConnection` targets one connection, while `SendToPrincipal` requires a non-empty subject and targets every current **authenticated** connection whose `Principal.Subject` matches; it never treats the empty anonymous subject as a group target. The common package never assumes that a subject is a Judgify user ID. A missing targeted destination returns a typed not-found result.

These methods are the only stable server-side outbound API. They copy or encode caller-owned envelope data before asynchronous retention, preserve each destination connection's writer order, and pass through the same size, mailbox, delivery-class, and shutdown rules. Per-destination coalesce, ephemeral drop, or critical-stream close is an observed delivery-policy outcome with metrics, not a global partial-failure error. Only invalid input/topic, oversize payload, closed/shutting-down Hub, or caller context cancellation/deadline fails the call.

`Conn` exposes safe operations such as its typed authenticated/anonymous principal, connection ID, reply/send, and close. It does **not** expose a public join/subscribe mutation that could bypass `TopicAuthorizer`. Every subscription addition is protocol-internal and routes through the authorizer captured by `Hub.Handler`; unsubscribe can remove only that same connection's own membership and does not require authorization. Callers never receive concurrent access to the underlying writer. The package keeps its local fan-out implementation and private `broker` interface under an internal package. Judgify and other services neither implement nor retain a broker. A future package-owned PostgreSQL or Redis fan-out adapter is selected through package configuration or a new constructor without changing the stable Hub call sites.

`CloseConnections` is the bounded declarative control primitive for committed session/account/policy/audience changes. Selector list counts and values are bounded. Values inside one non-empty list are ORed; the principal-class and each populated list group are ANDed. `All=true` is an explicit broad-scope acknowledgement and may still be narrowed by other groups; `All=false` requires at least one identifier/topic list. A selector that violates that rule, contains contradictory principal data or duplicate/oversized values, or supplies an invalid close code/reason is rejected before mutation. No caller predicate executes while a Hub lock is held.

Under the routing lock the Hub finds the bounded match set and atomically marks each connection closing before releasing the lock. A marked connection rejects new enqueue and subscription work, leaves all topics, drops its bounded queued frames with exact item/byte charge release, and then its owned pump sends the validated close reason best-effort. Repeated calls are idempotent and report already-closing connections rather than double-releasing resources. The call returns match/transition counts, not a claim that peer browsers received the close frame. V1 deliberately does not expose in-place reauthorization: a generic Hub cannot safely preserve a service's durable replay cursor while discarding old-policy frames. Services close, reconnect, reauthorize, and replay; a future reauthorization API requires an explicit resync/fence contract.

### 11.3 Envelope

The envelope is versioned and belongs in the WebSocket package:

```go
type Envelope struct {
	Version          int             `json:"v"`
	Operation        string          `json:"op"`
	ID               string          `json:"id,omitempty"`
	Cursor           string          `json:"cursor,omitempty"`
	Topic            string          `json:"topic,omitempty"`
	Type             string          `json:"type"`
	AggregateVersion int64           `json:"aggregate_version,omitempty"`
	OccurredAt       time.Time       `json:"occurred_at,omitempty"`
	Data             json.RawMessage `json:"data,omitempty"`
}
```

Sonic may be used internally, but callers interact with Go values and package-owned errors. Inbound decode is strict, size-limited, and validates protocol version, operation, type, and payload. Unknown protocol versions receive a version error rather than being partially interpreted. Outbound envelopes are encoded and size-checked before entering any mailbox; an envelope larger than `MaxOutboundMessageBytes` is rejected with a typed error and is never partially queued.

Control operations (`subscribe`, `unsubscribe`, protocol ping) are serialized in read order for each connection. Application handlers execute serially per connection by default, so `On` cannot reorder messages or spawn unbounded goroutines. `OnWithOptions` may opt into bounded parallelism only when `Parallelism > 1` and a bounded non-empty `OrderingKey` is supplied; messages with the same key remain serial and ordered, while different keys explicitly have no relative completion-order guarantee.

Every decoded inbound envelope consumes a per-connection token from the configured rate/burst limiter before dispatch. Handler execution must acquire both per-connection and Hub-global in-flight slots and receives the smaller of its registered timeout, the Hub handler timeout, and the connection lifetime. No slot starts an untracked goroutine. The handler contract requires honoring cancellation; Go cannot forcibly terminate a callback that ignores its context. Overload returns a stable protocol error and repeated abuse closes with policy/overload code. Shutdown cancels all tracked handler contexts and waits only to its deadline; if a callback still has not returned, the Hub releases no false completion signal and returns a typed undrained-handler error/count so the process lifecycle can report degraded shutdown and exit. Slot limits bound how many such callbacks can exist. Handler panics are recovered at the dispatch boundary, logged with CID and bounded metadata, converted to a stable protocol error, and never kill the read pump or leak a slot.

### 11.4 Authentication and authorization

The upgrade uses the shared authentication verifier through a resolver. Browser connections normally use same-origin secure cookies because browser WebSocket APIs cannot set arbitrary `Authorization` headers. A service may instead issue a short-lived one-time socket ticket through the resolver contract. A service resolver may explicitly return `Principal{Authenticated:false}` when credentials are absent and that endpoint permits anonymous access; any presented invalid credential rejects the upgrade and never falls back to anonymous.

Origin validation uses the exact allowlist. Before allocating a connection, the Hub validates the principal invariant: authenticated principals have a non-empty subject and expiry, while anonymous principals have empty identity/session/token fields. Topic names are not permissions: `TopicAuthorizer` authorizes the destination principal's **subscribe** request only and must fail closed for anonymous principals unless the service explicitly recognizes a public topic. The Hub applies its global connection limit to everyone. `MaxConnectionsPerPrincipal` applies only to `Authenticated=true` with a non-empty subject; anonymous connections are never collapsed into an empty-subject bucket and are instead bounded by trusted-remote-IP connection plus upgrade rate/burst limits. Remote IP comes only from the socket peer or a shared extractor configured with an exact trusted-proxy set; arbitrary forwarding headers are ignored. The public outbound methods are trusted server-side transport primitives and cannot prove the calling actor's domain permission from their current signatures; a service must expose them only behind its own authorized application adapter and construct a previously authorized topic/target. The common package does not know Judgify topics, object ownership, or role names and never claims that a targeted send performed domain authorization.

### 11.5 Ordering, replay, and durability

The local Hub is not a durable event store. It provides per-connection writer ordering and documented topic routing. Durable event storage, replay cursor retention, and application event authorization belong to the service. The generic envelope carries the required IDs and versions.

The private fan-out boundary is an in-memory routing mechanism, not an acknowledgment or durability abstraction. A successful `Broadcast` or targeted send means the message passed validation and was offered to each current destination under its delivery-class policy; it does not mean a browser received or processed it. A future package-owned Redis or PostgreSQL-notify adapter cannot weaken that contract, and published application state remains durable elsewhere.

### 11.6 Backpressure

Each outgoing message is assigned a package-owned delivery class:

- **latest state:** coalesce by caller-provided key;
- **ephemeral:** drop under pressure and increment a metric;
- **critical notification:** if it cannot be queued, close with `1013 Try Again Later` and a resync reason.

Every connection mailbox is bounded by both item count and total encoded bytes, and the Hub is bounded by `MaxHubQueuedBytes` across all mailboxes. Admission atomically reserves both the destination mailbox cost and Hub-global logical cost before retaining the frame. Coalescing atomically releases the replaced frame's cost; drop, dequeue, close, cancellation, and shutdown release both charges exactly once. Immutable encoded frames may be shared across destination queues to avoid physical copies, but each mailbox and the Hub-global counter are charged the full logical byte cost for each destination, so physical sharing cannot hide an unsafe fan-out commitment. `DeliveryLatestState` requires a non-empty bounded coalesce key.

When a destination cannot reserve per-connection or global bytes, its delivery class applies: latest state first attempts bounded replacement, ephemeral drops, and critical delivery closes that destination for resync. Other recipients continue. Configuration rejects non-positive or inconsistent byte/rate/in-flight limits, an outbound-message limit larger than the mailbox byte capacity, a mailbox byte capacity larger than the Hub budget, or a Hub budget larger than the validated process memory allocation. The Hub never creates an unbounded per-client queue or handler set and never blocks all clients on one slow writer/handler. A service uses REST or another durable snapshot source to resynchronize critical state.

Compression is disabled by default because it consumes CPU and can amplify secret-compression risks. The default inbound and outbound message limits are each 64 KiB; both are configurable only within safe upper bounds. Shutdown stops new upgrades, rejects new subscriptions, cancels tracked handlers, drains bounded writes within the deadline, and closes remaining connections with code `1001`; a callback that violates the cancellation contract produces the typed undrained result described above rather than an impossible guarantee that the library killed its goroutine.

## 12. Lifecycle and health

### 12.1 `pkg/common/lifecycle`

Lifecycle coordinates independent components under one root context:

```go
type Component interface {
	Name() string
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}
```

The runner:

- validates unique component names;
- starts components without serially blocking unrelated siblings;
- cancels the root when a required component exits unexpectedly;
- returns the original component error with component identity;
- shuts down in reverse dependency/registration order;
- applies per-component and total shutdown deadlines;
- aggregates shutdown errors without losing the primary failure;
- makes repeated shutdown calls safe;
- contains panics at the component boundary and reports them as errors.

Components do not capture signals. Executable `main` creates the signal context and passes it to the lifecycle runner.

### 12.2 `pkg/common/health`

Health is a bounded, concurrent checker registry:

```go
type Checker interface {
	Check(ctx context.Context) error
}

type Status struct {
	Name      string
	Healthy   bool
	Critical  bool
	Duration  time.Duration
	ErrorCode string
}
```

- **Liveness** reports whether the process and essential event loop can continue.
- **Readiness** reports whether the process should receive new traffic.
- Dependency checks have individual deadlines and bounded concurrency.
- Critical dependency failure makes readiness false; optional dependency degradation is reported without necessarily removing readiness.
- Responses expose stable error codes and timing, not credentials or raw internal errors.
- HTTP and gRPC health endpoints are transport adapters over the registry.

A static `/ping` endpoint is not a substitute for readiness.

## 13. Configuration and settings

`pkg/settings` keeps its current role but adopts a uniform contract:

- typed durations and byte sizes;
- explicit environment overlays;
- startup validation with field-scoped errors;
- no silent fallback after a value was supplied but invalid;
- secrets redacted from string/error/log output;
- TLS/SSL mode configurable and production-validated;
- defaults defined once and covered by tests;
- immutable configuration after startup except explicitly dynamic controls such as log levels;
- no package-global mutable configuration.

Constructors return errors instead of panicking. Configuration errors are caught before a component starts accepting work.

## 14. Cache design

The existing `pkg/common/cache` and Ristretto adapter remain. Their contract explicitly distinguishes local memory from remote cache.

### Local cache

- capacity is defined by byte cost or an explicit bounded cost function, not only item count;
- TTL and zero/negative TTL semantics are documented;
- write admission is either asynchronous and observable or synchronously awaited by an explicitly named operation;
- ordinary `Set` does not force a global `Wait` on every write unless the caller chose synchronous visibility;
- background refresh has an owner context, timeout, panic recovery, and bounded concurrency;
- stampede protection and negative caching are opt-in policies with bounded TTL;
- cached mutable values are copied or documented as immutable;
- `Close` drains/stops refresh work.

### Remote cache

- remote errors are returned rather than silently converted to misses unless a named best-effort policy requests that behavior;
- serialization errors are distinguishable from transport errors;
- prefix invalidation uses cursor-based scan or versioned namespaces, never blocking `KEYS` in production;
- Redis operations are not serialized behind one process-wide mutex;
- pipeline/batch partial failures are reported per operation;
- cache keys are length-bounded and namespace-aware.

Cache is never a durable source of truth.

## 15. Database packages

### 15.1 Shared rules

- caller context controls every operation;
- connection, query, and transaction timeouts are explicit;
- TLS is configurable and production defaults do not hardcode `sslmode=disable`;
- clients expose deterministic `Close` behavior;
- no repository manufactures `context.Background()` for caller operations;
- pagination and sort/filter input is allowlisted by the application;
- partial batch failures are surfaced;
- logging uses CID, operation name, duration, and redacted metadata rather than raw secrets or sensitive values.

Broad generic repositories remain experimental until their semantics are actually implementable across the target database. A generic interface must not claim pagination/filter behavior that an adapter silently ignores.

### 15.2 Ent

`pkg/database/ent` remains a first-class stable adapter after hardening:

- transaction helpers preserve the caller context and return rollback/commit errors without masking the original error;
- transaction-scoped client retrieval is explicit;
- actor/audit resolution is passed by constructor or stored in a race-safe immutable registry, not changed through an unsynchronized global;
- dynamic filters and sorts require endpoint-provided field/operator allowlists;
- invalid filters return validation errors instead of being silently skipped;
- page size has a validated maximum and pagination never divides by zero;
- cursor pagination validates that cursor encoding matches the declared sort and tie-breaker;
- UUID cursor logic is not applied to arbitrary sort fields;
- slow-query logging is configurable and redacted;
- database driver options, TLS, pool sizes, and lifetimes are validated.

### 15.3 Redis

The Redis adapter removes the global operation mutex, uses the client's native concurrency safety, replaces production prefix `KEYS` with scan/versioning, and makes close/error behavior consistent. Geo and sorted-set helpers validate units, bounds, and partial pipeline failures.

### 15.4 MongoDB, wide-column, and Elasticsearch

These adapters remain available but experimental until contract tests prove their advertised repository behavior.

- MongoDB always uses caller context and validates filters/sorts.
- Wide-column queries either implement supported filters/pagination or return a clear unsupported-operation error.
- Elasticsearch does not force refresh on every index by default, calculates totals/pagination correctly, closes response bodies, and inspects every bulk response item for partial errors.

## 16. MQ and asynchronous primitives

### 16.1 Generic messaging metadata

MQ adapters support bounded metadata propagation for correlation and causation. They copy caller-owned key/value/header bytes before asynchronous retention or document a synchronous ownership transfer. Handlers receive caller-cancelable context and stable error classification.

### 16.2 Kafka

Kafka remains the distributed broker adapter. It gains:

- correlation propagation middleware;
- explicit sync/async acknowledgment semantics;
- bounded producer buffering;
- surfaced asynchronous publish errors;
- handler panic recovery;
- graceful consumer-group drain;
- retry/dead-letter hooks without hardcoded service topics;
- contract and integration tests isolated from ordinary unit tests.

### 16.3 Batcher and worker pool

Batchers and worker pools:

- accept context;
- have bounded input;
- return overload or cancellation errors;
- copy/own slices across async boundaries;
- surface consumer errors;
- define flush, drain, and close behavior;
- never hold a spinlock while executing user callbacks, disk I/O, or network I/O;
- recover user callback panics at the worker boundary.

### 16.4 Forge

`pkg/mq/forge` is retained as an embedded single-owner, single-machine commit-log/queue project, but remains **experimental** until its durability contract is proven. It is not the authoritative judge queue for Judgify. Judgify uses PostgreSQL transactional jobs/outbox so an acknowledged submission survives API or worker crashes.

Forge stabilization requires all of the following before stable classification:

- producer copies caller key/value/header slices;
- pending memory is bounded by default;
- `Send` names whether it acknowledges memory admission, append, or fsync durability;
- concurrent flush preserves ordering and cannot clear unpersisted records on append failure;
- linger errors are observable;
- producer/broker shutdown tracks and closes all owned components;
- record batches up to the configured maximum can always be read and recovered;
- recovery terminates on corruption or oversized input with a typed error rather than looping;
- index rebuild and restart preserve offsets without overwrite;
- consumer poll, commit, retention, and compaction preserve documented offset semantics;
- retention never removes data still required by a committed consumer contract;
- compaction is crash-safe through staged write, fsync, and atomic rename;
- file I/O never occurs while holding a CPU-spinning lock;
- fault-injection, crash-recovery, race, fuzz, and multi-gigabyte soak tests pass.

If Forge cannot satisfy this contract without becoming a distributed coordination system, its stable role remains a local wake-up/commit-log acceleration primitive, not a cross-process source of truth.

## 17. Unsafe runtime, pools, and data structures

Private Go runtime links and `go:linkname` are removed from stable code. Unsafe string/byte conversion helpers that alias mutable memory are either removed, moved to an explicitly unsafe experimental package, or replaced with copying standard conversions.

Custom spinlocks, pools, queues, maps, sketches, and probabilistic structures are not promoted because they appear fast. Stable classification requires:

- a real consumer and documented correctness contract;
- race and fuzz tests;
- benchmarks against the standard library or established dependency;
- memory-retention measurements;
- clear ownership and reset semantics;
- no private runtime dependency.

Standard-library synchronization is preferred unless a benchmark demonstrates a material production benefit under representative contention.

### 17.1 Algorithms, timers, and identifiers

`pkg/algorithm` is the canonical home for backoff, limiter, window, cache-selection, and circuit-breaker algorithms. Duplicate backoff helpers under `pkg/utils` become deprecated wrappers. Every constructor validates zero/negative/overflow-prone options; mutable algorithms are race-tested; clock and entropy injection is explicit in tests; callbacks execute outside internal locks and panic containment is documented. A circuit breaker is scoped to one named outbound dependency, never installed as a whole-API inbound breaker.

`pkg/timer.SystemTimer` is the stable correctness clock. `CachedTimer` is permitted only for approximate metrics/admission paths after `NewCachedTimer` validates a positive step, construction returns an error, `Stop` is idempotent/concurrent-safe, the updater has an owned lifecycle, and maximum staleness is documented. It is forbidden for token expiry, lease validity, audit timestamps, contest time, database ordering, or another security/durability decision.

`pkg/unique` distinguishes random public display IDs from security tokens and durable primary keys. Random-length/prefix inputs are bounded and invalid input returns an error instead of allocating/panicking; randomness is unbiased `crypto/rand`; public IDs document collision probability and still require a database unique constraint plus bounded retry. UUIDv7 is preferred for new durable service identities. Snowflake remains compatibility/experimental: a future stable generator must validate bit shifts/epoch horizon, reject clock rollback/ID overflow, provide a context-aware bounded wait on sequence exhaustion, prove worker-ID coordination, and never spin forever while holding its mutex. Judgify does not use it as a production primary key.

### 17.2 Encoding, DTO, constraints, and utility disposition

The byte-oriented `pkg/encoding/base62` implementation is canonical. The legacy signed-integer `pkg/encoding.Base62Encode/Base62Decode` API delegates through a documented compatibility codec and is deprecated; the two alphabets/order/overflow semantics cannot remain accidentally interchangeable. Decoders cap input, reject non-canonical/invalid encodings, and expose allocation limits. `pkg/encoding/json` may hide Sonic internally but supplies an explicit strict, size-bounded decoder for untrusted contracts; callers do not assume ordinary `Unmarshal` rejects unknown or duplicate fields.

`pkg/dto` remains only for genuinely transport-neutral value objects such as validated pagination metadata. Database-specific geo/sorted-set DTOs move behind their database adapter or remain deprecated compatibility aliases. Pagination constructors validate positive bounded page size/current page and checked arithmetic; they never divide by zero or return a successful-looking page for invalid input.

`pkg/constraints` keeps only compile-time type sets. Context-key constants move to the package that owns their value so generic constraints cannot become a hidden cross-package context registry. Broad `pkg/utils` gains no new business/security APIs: copying string/byte conversion is the stable default; aliasing unsafe converters move behind an explicitly unsafe experimental package with ownership contracts or are removed; unchecked byte-to-integer/slice conversions validate length/alignment or return errors; overflow-prone math returns checked results instead of panicking. `utils/token` is deprecated in favor of `security/authentication/jwt`, and duplicate time/backoff helpers delegate to their focused packages during the compatibility window.

### 17.3 CDC parsers

`pkg/cdc` remains experimental until each supported source format has a versioned contract suite. Debezium parsing caps total input, decoded stringified-JSON size, nesting, and source metadata; accepts only the closed operation set; validates operation-specific before/after/tombstone invariants; handles deletes and snapshots deliberately; and returns bounded typed parse errors. It does not accept arbitrary recursive `Source any` on stable paths or silently reinterpret malformed envelope/flat formats. Mongo key/date helpers reject malformed, ambiguous, or out-of-range values. Fuzzing covers envelopes, stringified payloads, tombstones, duplicate fields, oversized values, and parser allocation budgets.

### 17.4 Complete package catalog gate

The release-owned machine-readable catalog enumerates every public package currently under `pkg/`, including algorithms, CDC, constraints, data structures, DTOs, encoding, OAuth, pools, timers, identifiers, and utilities. Each entry names its owner, maturity, public contract/test command, vendor exposure, goroutine/unsafe use, known consumers, and replacement when deprecated. An uncatalogued package or exported symbol cannot be imported by a stable critical path. This is how the whole module is governed without forcing a destructive package-tree rewrite.

## 18. Error model and failure behavior

`pkg/common/apperr` remains the stable application error concept. It is extended without binding core services to HTTP:

- stable machine code;
- bounded typed parameters for caller-side message mapping;
- an optional compatibility diagnostic message that is never the canonical localized UI copy;
- wrapped internal cause;
- retryable classification where meaningful;
- optional field validation codes and typed parameters;
- `errors.Is`/`errors.As` support;
- transport-specific mapping in HTTP/gRPC/WebSocket adapters.

Libraries return errors; they do not log and return the same failure at every layer. The layer with operational context logs once. Cancellation and deadline errors remain recognizable. Constructors return validation errors rather than panic. Panics are confined to programmer invariants and recovered only at well-defined process/request/worker boundaries.

## 19. Compatibility and migration

Migration is incremental and release-safe:

1. Add the new package/API with complete tests and documentation.
2. Make the old API delegate to the new implementation when semantics can be preserved.
3. Mark the old symbol with Go's `Deprecated:` convention and link the replacement.
4. Migrate Judgify as the first consumer and run contract/integration tests.
5. Search all known consumers and publish a migration guide.
6. Keep the wrapper through at least one compatibility release.
7. Remove it only in a documented major-version boundary or before the first stable v1 when no external consumer depends on it.

Specific compatibility paths:

- `pkg/cid` delegates to `pkg/correlation` and transport-specific propagation.
- root `pkg/security` password functions delegate to `pkg/security/password`.
- key-based JWT middleware delegates to `security/authentication/jwt`.
- the current `AuthorizationEnforcer` shape adapts to `security/authorization.Authorizer`.
- unsafe `Compose`/`Parallel`, inbound circuit breaker, and ambiguous retry APIs receive safe replacements; wrappers cannot retain behavior that races or returns false success.

No source-wide import rename is performed in one commit. Each migration is a focused change with tests and a reversible consumer update.

## 20. Testing strategy

### 20.1 Test layers

- **Characterization tests:** lock the intended current behavior before internal fixes.
- **Unit tests:** deterministic and free of Docker/network dependencies.
- **Contract tests:** run the same semantic suite against each implementation of an interface.
- **Integration tests:** isolated with build tags or explicit commands and disposable dependencies.
- **Race tests:** mandatory for lifecycle, cache, logger level control, WebSocket, worker pools, MQ, Forge, and shared registries.
- **Fuzz tests:** JWT parsing, correlation validation, HTTP parsing, WebSocket envelopes/frames, pagination/cursors, MQ records, and Forge recovery.
- **Fault-injection tests:** disk/write failures, partial batches, connection resets, context cancellation, shutdown races, corrupt segments, and stale leases where applicable.
- **Benchmarks:** HTTP middleware/client, logger enrichment, correlation propagation, cache, WebSocket fan-out/backpressure, batchers, hashing, and Forge append/read/recovery.

Ordinary `go test ./...` must not boot Docker or hang waiting for Kafka, Redis, Elasticsearch, MongoDB, or a wide-column database.

### 20.2 Required behavioral suites

Authentication:

- access/refresh purpose separation;
- issuer/audience/key rotation/clock skew;
- malformed and adversarial token corpus;
- optional-auth invalid-credential rejection;
- authenticated/anonymous principal invariants and proof that only absent credentials may deliberately resolve anonymous.

Authorization:

- concurrent evaluation during atomic snapshot swap;
- old-or-new snapshot visibility only;
- fail-closed errors;
- desired/applied revision mismatch health.

HTTP:

- middleware order and single execution;
- exact CORS origin behavior;
- body/header limits;
- retry idempotency and replayable bodies;
- cancellation and final response/error behavior;
- bounded fan-out.

WebSocket:

- Autobahn compatibility for the private protocol engine integration;
- origin/authentication/limit checks, including explicit anonymous resolution versus present-invalid credential rejection;
- trusted-proxy remote-IP derivation, anonymous/default-deny topic authorization, and global/principal/remote-IP connection plus upgrade-rate quotas;
- principal-invariant validation before allocation and proof that anonymous clients are never grouped under the empty subject for principal quotas;
- every join path invokes the captured topic authorizer, handlers cannot call a bypassing `Conn.Subscribe`, and unsubscribe removes only the caller connection's membership;
- declarative close-selector validation/match algebra, atomic closing-before-unlock, enqueue/join rejection, queued-byte release exactly once, idempotent repeats, and no callback under Hub locks;
- one-writer guarantee;
- exact per-connection and Hub-global logical-byte reserve/release accounting under fan-out, coalescing, drop, dequeue, cancellation, and close;
- outbound oversize rejection before queue mutation and slow-client backpressure for every delivery class;
- zero-subscriber broadcast success, typed missing-target behavior, and no global partial-failure error when one destination is dropped or closed;
- empty-subject principal send rejection and proof that principal-targeted sends never group anonymous connections;
- inbound token-bucket flood control, serial default handler ordering, bounded keyed parallelism, global/per-connection in-flight caps, timeout, panic recovery, and slot release;
- connection-limit and reconnect storm tests;
- graceful drain and goroutine/file-descriptor leak tests.

Password:

- checked-in Argon2id policy benchmark and deterministic PHC parse/round-trip vectors;
- CSPRNG failure, maximum input, malformed/oversized PHC, integer overflow, and attacker-declared resource-limit rejection before Argon allocation;
- constant-time derived-key comparison, bounded hashing concurrency, cancellation, saturation, and RSS tests;
- preflight-observed configured legacy bcrypt costs (including expected cost 12), unsupported/unsafe cost rejection, successful-login Argon2id upgrade, compare-and-swap race behavior, and no upgrade on mismatch;
- exact byte semantics for whitespace and distinct Unicode-normalization forms.

OAuth:

- constructor/redirect/scope validation, PKCE S256 and exact verifier pairing;
- context cancellation, total timeout, status/content-type/body-size limits, and response-body closure;
- immutable provider-subject semantics, verified/unverified/missing email handling, typed normalized bounds, provider error classification, and secret/code/token/body redaction;
- deprecated state-only URL/metadata-map wrapper compatibility without use on stable Judgify paths.

Algorithms, time, identifiers, encoding, and utilities:

- invalid/overflow constructor inputs, injected clock/entropy vectors, race/callback reentrancy, and no global-random dependence;
- idempotent concurrent cached-timer stop, bounded staleness, and architecture checks forbidding it on security/durability clocks;
- Snowflake rollback/exhaustion/overflow tests while it remains noncritical; UUIDv7/public-ID collision-retry contracts;
- canonical Base62 round trips/noncanonical rejection/input caps and compatibility-codec separation;
- strict bounded JSON, checked pagination/math/byte conversion, no stable unsafe aliasing, and deprecated focused-package delegation.

CDC:

- Debezium create/update/delete/read/tombstone invariants, flat/envelope/stringified formats, duplicate/unknown fields, closed operations, malformed Mongo keys/dates, input/nesting/allocation limits, and fuzz corpus;
- no stable critical-path import while the package remains experimental.

Forge:

- append/commit/restart/retention/compaction model tests;
- crash injection at every persistence boundary;
- corruption and maximum-batch recovery;
- ordering under concurrent producers;
- long soak with bounded RSS and file descriptors.

## 21. CI and release gates

Every release candidate must pass:

```text
gofmt check
go vet ./...
golangci-lint with zero new issues and zero issues in stable packages
go test ./...
go test -race for concurrency-sensitive packages
go test integration suites through explicit jobs
go test fuzz smoke corpus
govulncheck ./...
generated/docs/example drift checks
license and dependency review
```

Coverage is ratcheted upward and cannot regress. Security, authentication, authorization, correlation, lifecycle, health, HTTP retry, and WebSocket core target at least 90% statement/branch-relevant coverage. Other stable packages target at least 80%, with risk-based exceptions documented in the release note rather than hidden by a repository-wide average.

Performance-sensitive packages have stored benchmark baselines. A material regression blocks release unless the change is explicitly accepted with measured rationale. Concurrency packages must pass repeated race runs and leak checks.

A stable tag is not created while:

- a critical package imports an experimental package;
- release builds require a local `replace` directive;
- examples or package documentation describe APIs that do not exist;
- integration tests start implicitly during unit tests;
- a public API has unspecified acknowledgment, ownership, or shutdown semantics;
- known P0/P1 security or data-loss defects remain.

## 22. Versioning and documentation

`go-common` is released with semantic tags. Judgify may use a local `go.work` during development, but release builds consume a tagged module version and contain no filesystem `replace`.

Each stable package provides:

- package overview and guarantees;
- configuration/default table;
- minimal executable example;
- error and cancellation behavior;
- concurrency safety and ownership rules;
- lifecycle/shutdown instructions;
- security considerations;
- migration note for replaced APIs.

The root README describes the actual package tree rather than an aspirational architecture. A maturity matrix lists package status, owner, production consumers, test categories, and latest compatibility version.

## 23. Judgify integration boundary

Judgify proves the generic APIs but retains its domain:

| Common library owns | Judgify owns |
|---|---|
| Correlation generation and propagation | Submission, contest, problem, and user identifiers |
| Context logger and generic fields | Domain field enrichment and domain log messages |
| JWT verification/issuance primitives | Session tables, refresh rotation, revocation, user status |
| OAuth provider exchange/profile primitives | State/PKCE storage, completion tickets, linking/onboarding, sessions and audit |
| Authorization interface and Casbin runtime snapshot | Ent role/policy catalog, admin mutations, invariants, audit |
| HTTP/gRPC/WebSocket transport mechanics | Endpoint/method/topic permissions and object ownership |
| WebSocket envelope, hub, private local fan-out, backpressure | Durable outbox, replay retention, event catalog, audience rules |
| Cache interfaces/adapters | What may be cached and invalidation triggers |
| MQ adapters and Forge experimental primitive | PostgreSQL judge jobs/outbox as durable truth |
| Lifecycle and health registry | Which Judgify components are critical and their readiness rules |

An abstraction moves from Judgify into `go-common` only when it is independent of Judgify entities, has more than one real use case, has a stable contract, and can be replaced without changing application behavior.

## 24. Acceptance criteria

The `go-common` production-library upgrade is complete when:

1. The single-module, current-structure constraint is preserved.
2. Approved taxonomy packages exist with documented, tested contracts.
3. Access and refresh tokens cannot cross purpose boundaries.
4. HTTP CORS, composition, retry, pagination, fan-out, rate limit, and circuit-breaker semantics are corrected.
5. Correlation propagates through HTTP, gRPC, WebSocket, Kafka, and owned async handlers without domain hardcoding.
6. Production logs default to info, development to debug, and all emitted levels honor secret/cardinality rules.
7. Every background component is bounded, context-owned, and drainable.
8. WebSocket uses the hidden `coder/websocket` core with per-connection/global item-and-byte bounds, inbound rate and handler-concurrency bounds, bounded outbound messages, public Hub broadcast/targeted-send/declarative-close methods, and no vendor or broker leakage.
9. Database and cache adapters honor caller context, secure configuration, and explicit failure behavior.
10. Forge is either proven against its full durability suite or remains clearly experimental and outside critical paths.
11. Unit tests do not start external infrastructure, while explicit integration jobs cover supported adapters.
12. Stable packages pass lint, vet, race, vulnerability, coverage, documentation, and benchmark gates.
13. Judgify consumes tagged `go-common` releases without local `replace` in release builds.
14. Deprecated compatibility paths are documented, tested, and do not preserve races, data loss, or false-success behavior.
15. Every current public package is present in the machine-readable maturity catalog; OAuth, timer/identifier, encoding/utility, and CDC dispositions satisfy their explicit gates, and no uncatalogued/experimental symbol enters a stable critical path.

This design produces a reusable production library without turning `go-common` into a service-specific framework or a root god package.
