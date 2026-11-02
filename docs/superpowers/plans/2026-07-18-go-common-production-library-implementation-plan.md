# Go-common Production Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn go-common into a versioned, bounded, context-aware production library that Judgify and future Go services can reuse without leaking Judgify concepts or third-party transport types.

**Architecture:** Stable packages expose vendor-neutral security, correlation, lifecycle, health, HTTP, gRPC, and WebSocket contracts. Existing APIs move through tested compatibility facades; unsafe or ambiguous packages are catalogued and prevented from entering stable critical paths. `coder/websocket`, Casbin, Zap, Gin, and other vendors remain private implementation details except inside explicitly named adapters.

**Tech Stack:** Go 1.26.5, standard library, Zap, Gin, gRPC, Casbin v3, `coder/websocket`, Sonic behind package-owned JSON contracts, Argon2id, bcrypt compatibility, and existing database/MQ adapters.

## Global Constraints

- Module remains `github.com/huynhanx03/go-common` and one Go module.
- Do not create a root package that imports Gin, Ent, Casbin, Kafka, Redis, and WebSocket together.
- Never import Judgify or hardcode its topics, roles, `submission_id`, or business rules.
- Every async API documents ownership, boundedness, cancellation, acknowledgment, and shutdown.
- `pkg/correlation` is canonical; `pkg/cid` is only a compatibility facade using the same context key.
- `pkg/logger` remains the logging home; do not create an umbrella observability folder.
- `pkg/common/websocket` is parallel to HTTP and gRPC; do not place it under `realtime` or `http`.
- `coder/websocket` must not appear in any stable public signature.
- Constructors validate configuration and return errors; library packages do not call `Fatal`.
- Production defaults to `INFO`, development to `DEBUG`, tests to `WARN`.
- No log level may expose credentials, tokens, cookies, source code, hidden tests, or lease secrets.
- Stable security/correlation/lifecycle/health/HTTP retry/WebSocket packages target at least 90% meaningful coverage.
- Ordinary `go test ./...` must not require Docker, Kafka, Redis, MongoDB, Elasticsearch, or a wide-column database.
- No commits; use path-scoped diff and test checkpoints.

---

## File Structure

```text
pkg/
├── correlation/
├── security/
│   ├── password/
│   ├── authentication/
│   │   └── jwt/
│   └── authorization/
│       └── casbin/
├── common/
│   ├── lifecycle/
│   ├── health/
│   ├── http/
│   ├── grpc/
│   │   └── interceptors/
│   └── websocket/
│       └── internal/
├── logger/
├── oauth/
└── maturity/
```

Compatibility packages remain at `pkg/cid`, root `pkg/security`, and existing middleware paths until Judgify and known consumers migrate.

### Task 1: Establish the Package Maturity and Stable-Import Gate

**Files:**
- Create: `pkg/maturity/catalog.json`
- Create: `pkg/maturity/catalog.schema.json`
- Create: `pkg/maturity/catalog_test.go`
- Create: `internal/stableimports/analyzer.go`
- Create: `internal/stableimports/analyzer_test.go`
- Create: `cmd/stableimports/main.go`
- Modify: `README.md`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Produces:

```go
type Status string

const (
    Stable       Status = "stable"
    Experimental Status = "experimental"
    Deprecated   Status = "deprecated"
    Internal     Status = "internal"
)

type Entry struct {
    Package       string   `json:"package"`
    Owner         string   `json:"owner"`
    Status        Status   `json:"status"`
    TestCommand   string   `json:"test_command"`
    VendorTypes   []string `json:"vendor_types"`
    Goroutines    bool     `json:"goroutines"`
    Unsafe        bool     `json:"unsafe"`
    Consumers     []string `json:"consumers"`
    Replacement   string   `json:"replacement,omitempty"`
}
```

- [x] **Step 1: Write failing catalog tests**

Tests walk `./pkg/...` with `go list -json`, load the catalog, and assert:

```go
require.ElementsMatch(t, discoveredPublicPackages, catalogPackages)
require.NotEmpty(t, entry.Owner)
require.NotEmpty(t, entry.TestCommand)
if entry.Status == Deprecated {
    require.NotEmpty(t, entry.Replacement)
}
```

Analyzer fixtures include a stable package importing `pkg/runtime` and assert the diagnostic `stable package imports experimental package`.

- [x] **Step 2: Run tests and verify the catalog is absent**

Run:

```bash
go test ./pkg/maturity ./internal/stableimports -count=1
```

Expected: FAIL because the packages/catalog do not exist.

- [x] **Step 3: Implement schema, catalog, and analyzer**

Catalog every existing public package returned by:

```bash
go list -f '{{if not .Internal}}{{.ImportPath}}{{end}}' ./pkg/...
```

Initial required classifications:

- stable: `pkg/common/apperr`, focused cache interfaces, HTTP response/validation, database/Ent helpers that pass contracts, `pkg/encoding/base62`, `pkg/encoding/json`, `pkg/logger`, `pkg/unique` public IDs;
- experimental: CDC, Forge, custom runtime/locks/pools/data structures lacking full proof, Snowflake, MongoDB/wide-column/Elasticsearch repository adapters;
- deprecated: `pkg/cid`, ambiguous root Base62, `pkg/utils/token`, duplicate backoff/time helpers;
- internal: private implementation paths.

The analyzer reads the catalog once, resolves imports through `go/packages`, and emits deterministic diagnostics for stable-to-experimental imports and uncatalogued public imports.

- [x] **Step 4: Wire CI and documentation**

Add:

```yaml
- name: Verify package maturity
  working-directory: .
  run: |
    go test ./pkg/maturity ./internal/stableimports
    go run ./cmd/stableimports ./...
```

Document the four statuses and compatibility policy in `README.md`.

- [x] **Step 5: Verify the checkpoint**

Run:

```bash
go test ./pkg/maturity ./internal/stableimports -count=1
go run ./cmd/stableimports ./...
go test ./...
git diff --check
git diff -- pkg/maturity internal/stableimports cmd/stableimports README.md .github/workflows/ci.yml
```

Expected: all commands exit 0 and every current package is catalogued. Do not commit.

### Task 2: Add Bounded Argon2id Password Hashing and Bcrypt Upgrade

**Files:**
- Create: `pkg/security/password/password.go`
- Create: `pkg/security/password/policy.go`
- Create: `pkg/security/password/phc.go`
- Create: `pkg/security/password/password_test.go`
- Create: `pkg/security/password/phc_test.go`
- Create: `pkg/security/password/password_benchmark_test.go`
- Modify: `pkg/security/password.go`

**Interfaces:**
- Produces:

```go
type Policy struct {
    MemoryKiB       uint32
    Iterations      uint32
    Parallelism     uint8
    SaltBytes       uint32
    KeyBytes        uint32
    MaxPasswordBytes int
    MaxPHCBytes      int
    MaxConcurrent    int
}

type Verification struct {
    Valid       bool
    NeedsRehash bool
    Algorithm   string
}

func New(policy Policy, options ...Option) (*Hasher, error)
func DefaultPolicy() Policy
func (h *Hasher) Hash(ctx context.Context, password []byte) (string, error)
func (h *Hasher) Verify(ctx context.Context, encoded string, password []byte) (Verification, error)
```

- [x] **Step 1: Write failing policy, PHC, and compatibility tests**

Cover exact byte semantics, empty/oversized password, CSPRNG failure, malformed PHC, duplicate fields, excessive memory/iterations/parallelism/key length before allocation, constant-time match, cancellation before semaphore admission, saturation, and bcrypt costs.

Use deterministic vectors:

```go
policy := Policy{
    MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2,
    SaltBytes: 16, KeyBytes: 32, MaxPasswordBytes: 1024,
    MaxPHCBytes: 512, MaxConcurrent: 2,
}
```

The bcrypt suite accepts observed safe costs including the current cost 12, returns `NeedsRehash=true` after a valid legacy verification, and rejects unsupported/unsafe cost before an expensive comparison.

- [x] **Step 2: Run focused tests and verify failure**

Run:

```bash
go test ./pkg/security/password -count=1
```

Expected: FAIL because the focused package does not exist.

- [x] **Step 3: Implement bounded hashing**

Use `argon2.IDKey`, `crypto/rand.Reader`, `subtle.ConstantTimeCompare`, and a buffered semaphore sized by `MaxConcurrent`. Validate all integer multiplications and policy limits before decoding base64 or allocating Argon2 memory. The PHC format is:

```text
$argon2id$v=19$m=65536,t=3,p=2$<raw-base64-salt>$<raw-base64-key>
```

`Hash` and `Verify` acquire the semaphore with:

```go
select {
case h.slots <- struct{}{}:
    defer func() { <-h.slots }()
case <-ctx.Done():
    return result, ctx.Err()
}
```

Verification dispatches only to closed algorithms `argon2id` and `bcrypt`; unknown prefixes return a typed unsupported-hash error.

- [x] **Step 4: Preserve the root compatibility API**

Make `pkg/security/password.go` delegate to a package-level default hasher:

```go
// Deprecated: use password.New and (*password.Hasher).Hash.
func HashPassword(raw string) (string, error)

// Deprecated: use (*password.Hasher).Verify.
func ComparePassword(encoded, raw string) error
```

The wrapper converts an invalid password to `bcrypt.ErrMismatchedHashAndPassword` only where legacy callers require that sentinel; resource/policy errors remain distinguishable.

- [x] **Step 5: Benchmark and verify**

Run:

```bash
go test ./pkg/security/password ./pkg/security -count=1
go test -race ./pkg/security/password -count=1
go test ./pkg/security/password -run '^$' -bench 'BenchmarkArgon2id' -benchmem
go test ./...
```

Record median hash time and peak RSS for the production host in `pkg/security/password/README.md`; tune only within the approved policy and retain the benchmark evidence. Do not commit.

### Task 3: Introduce Typed Authentication and JWT Purpose Enforcement

**Files:**
- Create: `pkg/security/authentication/principal.go`
- Create: `pkg/security/authentication/token.go`
- Create: `pkg/security/authentication/errors.go`
- Create: `pkg/security/authentication/principal_test.go`
- Create: `pkg/security/authentication/jwt/claims.go`
- Create: `pkg/security/authentication/jwt/verifier.go`
- Create: `pkg/security/authentication/jwt/issuer.go`
- Create: `pkg/security/authentication/jwt/keyset.go`
- Create: `pkg/security/authentication/jwt/verifier_test.go`
- Create: `pkg/security/authentication/jwt/fuzz_test.go`
- Modify: `pkg/utils/token.go`

**Interfaces:**

```go
type TokenPurpose string

const (
    PurposeAccess  TokenPurpose = "access"
    PurposeRefresh TokenPurpose = "refresh"
)

type Principal struct {
    Authenticated      bool
    Subject            string
    UserID             string
    SessionID          string
    Username           string
    TokenID            string
    AuthenticatedUntil time.Time
}

func Anonymous() Principal
func (p Principal) Validate(now time.Time) error

type Verifier interface {
    Verify(ctx context.Context, raw string, expected TokenPurpose) (Principal, error)
}

type IssueRequest struct {
    Purpose   TokenPurpose
    Subject   string
    UserID    string
    SessionID string
    Username  string
    TokenID   string
    IssuedAt  time.Time
    ExpiresAt time.Time
}

type IssuedToken struct {
    Raw       string
    TokenID   string
    ExpiresAt time.Time
    KeyID     string
}
```

- [x] **Step 1: Write invariant and adversarial-token tests**

Test that anonymous contains no identity fields, authenticated requires non-empty subject and future expiry, issuer/audience/purpose are mandatory, access and refresh never cross boundaries, `kid` is required, only the configured RSA algorithm is accepted, clock skew is bounded, and unknown/retired keys return typed errors.

Fuzz `Verifier.Verify` with arbitrary bytes and assert no panic, no raw token in an error string, and no authenticated principal on any error.

- [x] **Step 2: Run focused tests**

Run:

```bash
go test ./pkg/security/authentication/... -count=1
```

Expected: FAIL because the package does not exist.

- [x] **Step 3: Implement principal and key-set validation**

Use immutable key snapshots:

```go
type KeySet struct {
    Active  map[string]*rsa.PublicKey
    Retired map[string]*rsa.PublicKey
}
```

Copy maps during construction, reject duplicate/empty key IDs, and return no writable map. Parse JWT with explicit valid methods and `jwt.WithIssuer`, `jwt.WithAudience`, `jwt.WithExpirationRequired`, `jwt.WithIssuedAt`, and bounded leeway. Claims include a closed `purpose` value and generic subject/session fields.

- [x] **Step 4: Implement issuer and deprecated token facade**

Issuer signs only with an active private key whose `kid` is included in the protected header. It validates temporal ordering and maximum TTL supplied by configuration. `pkg/utils/token.go` delegates to the focused issuer/verifier where semantics match and carries Go `Deprecated:` comments; stable paths are blocked from importing it by the maturity analyzer.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/security/authentication/... -count=1
go test -race ./pkg/security/authentication/... -count=1
go test ./pkg/security/authentication/jwt -fuzz=FuzzVerify -fuzztime=10s
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 4: Harden OAuth Provider Adapters with PKCE and Typed Profiles

**Files:**
- Modify: `pkg/oauth/provider.go`
- Modify: `pkg/oauth/google.go`
- Modify: `pkg/oauth/github.go`
- Create: `pkg/oauth/errors.go`
- Create: `pkg/oauth/options.go`
- Create: `pkg/oauth/provider_contract_test.go`
- Create: `pkg/oauth/google_test.go`
- Create: `pkg/oauth/github_test.go`

**Interfaces:**

```go
type AuthorizationRequest struct {
    State         string
    PKCEChallenge string
    RedirectURI   string
}

type ExchangeRequest struct {
    Code         string
    PKCEVerifier string
    RedirectURI  string
}

type Profile struct {
    Subject       string
    Email         string
    EmailVerified bool
    DisplayName   string
    Login         string
    AvatarURL     string
}

type Provider interface {
    Name() string
    AuthorizationURL(ctx context.Context, request AuthorizationRequest) (string, error)
    Exchange(ctx context.Context, request ExchangeRequest) (Profile, error)
}
```

- [x] **Step 1: Write a reusable TLS contract suite**

The suite injects an `httptest.NewTLSServer` client and verifies constructor validation, exact redirect URI, PKCE S256, cancellation, total timeout, connection limit, status/content-type/body cap, response closure, immutable subject, verified-email semantics, duplicate/unknown field handling, and error redaction.

- [x] **Step 2: Run provider tests**

Run: `go test ./pkg/oauth -count=1`

Expected: FAIL because current constructors and provider methods do not satisfy the typed contract.

- [x] **Step 3: Implement bounded constructors and exchanges**

Constructors return `(*GoogleProvider, error)` / `(*GitHubProvider, error)`, validate non-empty client ID, HTTPS production endpoints, exact callback, bounded scopes, and injected `*http.Client`. Use `http.NewRequestWithContext`, `io.LimitReader(maxBodyBytes+1)`, strict JSON decoding, status/content-type checks, and body drain/close. Never return provider tokens or raw response maps.

- [x] **Step 4: Add compatibility methods**

Retain old `AuthCodeURL(state)` and `ExchangeCode(ctx, code)` only as deprecated wrappers on an explicitly configured compatibility provider. They are not used by Judgify stable paths and cannot bypass constructor validation.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/oauth -count=1
go test -race ./pkg/oauth -count=1
go test ./...
```

Expected: all exit 0 with no external network request. Do not commit.

### Task 5: Add Generic Authorization and Atomic Casbin Snapshots

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `pkg/security/authorization/authorization.go`
- Create: `pkg/security/authorization/errors.go`
- Create: `pkg/security/authorization/authorization_test.go`
- Create: `pkg/security/authorization/casbin/model.go`
- Create: `pkg/security/authorization/casbin/runtime.go`
- Create: `pkg/security/authorization/casbin/snapshot.go`
- Create: `pkg/security/authorization/casbin/runtime_test.go`

**Interfaces:**

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

type PolicyRule struct {
    Type string
    V0   string
    V1   string
    V2   string
    V3   string
    V4   string
    V5   string
}

type Snapshot struct {
    Revision int64
    Model    string
    Rules    []PolicyRule
}

func NewRuntime(initial Snapshot, options ...Option) (*Runtime, error)
func (r *Runtime) Prepare(ctx context.Context, candidate Snapshot) (*PreparedSnapshot, error)
func (r *Runtime) Publish(prepared *PreparedSnapshot) error
func (r *Runtime) Authorize(ctx context.Context, request authorization.Request) (authorization.Decision, error)
func (r *Runtime) AppliedRevision() int64
func (r *Runtime) SetDesiredRevision(revision int64)
func (r *Runtime) Healthy() error
```

- [x] **Step 1: Write fail-closed and concurrency tests**

Tests assert malformed/anonymous requests fail closed, reasons are closed machine codes, candidate validation performs no live mutation, concurrent readers see only complete old revision N or complete new revision N+1, stale desired/applied state is unhealthy, double publish is rejected, and evaluator errors never allow.

- [x] **Step 2: Run focused tests**

Run:

```bash
go test ./pkg/security/authorization/... -count=1
```

Expected: FAIL because the packages do not exist.

- [x] **Step 3: Implement immutable preparation and atomic publication**

Add Casbin v3 as a direct dependency. Build each candidate in isolation using `model.NewModelFromString`, load copied rules with AutoSave disabled, validate every policy line, and store:

```go
type liveSnapshot struct {
    revision int64
    enforcer *casbin.Enforcer
}
```

Publish with `atomic.Pointer[liveSnapshot]`. `PreparedSnapshot` contains an unexported runtime nonce and single-use atomic flag so another runtime or a second publication is rejected. `Authorize` validates principal/resource/action before loading one pointer and calling the enforcer.

- [x] **Step 4: Document persistence boundary**

Package docs state that persistence, role catalogs, self-lockout rules, audit, and transactions belong to the consuming service. The runtime consumes complete snapshots and never calls an Ent adapter or writes rows.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/security/authorization/... -count=100
go test -race ./pkg/security/authorization/... -count=10
go test ./...
```

Expected: no mixed-revision observation and all commands exit 0. Do not commit.

### Task 6: Replace CID with Validated Correlation and Enrich Logging Safely

**Files:**
- Create: `pkg/correlation/correlation.go`
- Create: `pkg/correlation/validation.go`
- Create: `pkg/correlation/correlation_test.go`
- Modify: `pkg/cid/cid.go`
- Modify: `pkg/cid/cid_test.go`
- Modify: `pkg/logger/context.go`
- Modify: `pkg/logger/zap.go`
- Modify: `pkg/logger/context_test.go`
- Modify: `pkg/logger/zap_test.go`
- Create: `pkg/common/http/middlewares/correlation.go`
- Create: `pkg/common/http/request/correlation.go`
- Modify: `pkg/common/http/middlewares/logger.go`

**Interfaces:**

```go
const Header = "X-Correlation-ID"

func New() string
func WithContext(ctx context.Context, id string) context.Context
func FromContext(ctx context.Context) string
func EnsureContext(ctx context.Context) context.Context
func Validate(id string) error

type Field struct {
    Key   string
    Value string
}

func String(key, value string) Field
func WithFields(ctx context.Context, fields ...Field) context.Context
```

- [x] **Step 1: Write validation and shared-context-key tests**

Accept canonical UUIDv7 plus explicitly allowed bounded upstream identifiers of 1–128 visible ASCII characters from `[A-Za-z0-9._:-]`. Reject whitespace, control characters, non-ASCII, oversize input, and empty inbound values. Prove a value set through `cid.WithContext` is returned by `correlation.FromContext` and vice versa.

Logger tests prove fields are copied, duplicate reserved keys are rejected, cardinality/value sizes are bounded, production defaults to info, logger construction returns an error on an invalid file path instead of panicking, and secrets are redacted by HTTP logging.

- [x] **Step 2: Run focused tests**

Run:

```bash
go test ./pkg/correlation ./pkg/cid ./pkg/logger ./pkg/common/http/middlewares -count=1
```

Expected: FAIL because correlation and vendor-neutral fields do not exist.

- [x] **Step 3: Implement one canonical context key**

Keep the unexported context key in `pkg/correlation`. `pkg/cid` delegates:

```go
const Header = correlation.Header

// Deprecated: use correlation.New.
func New() string { return correlation.New() }

// Deprecated: use correlation.WithContext.
func WithContext(ctx context.Context, id string) context.Context {
    return correlation.WithContext(ctx, id)
}
```

Inbound middleware validates the supplied header and mints a new ID on invalid input; outbound transport preserves a valid caller-set value and otherwise copies the context ID without mutating the original request.

- [x] **Step 4: Implement context-first logger fields**

Store immutable package fields in context and translate them to Zap only inside `FromContext`. Reserve `timestamp`, `level`, `service`, `version`, `environment`, `component`, `message`, and `cid`. Cap key length, value length, and fields per context. Change `NewLogger` to return `(*LoggerZap, error)`; keep a deprecated `MustNewLogger` for executable bootstrap only.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/correlation ./pkg/cid ./pkg/logger ./pkg/common/http/... -count=1
go test -race ./pkg/logger ./pkg/correlation -count=10
go test ./...
```

Expected: all exit 0 and no package maintains a second CID context key. Do not commit.

### Task 7: Add Lifecycle and Health Primitives

**Files:**
- Create: `pkg/common/lifecycle/component.go`
- Create: `pkg/common/lifecycle/runner.go`
- Create: `pkg/common/lifecycle/errors.go`
- Create: `pkg/common/lifecycle/runner_test.go`
- Create: `pkg/common/health/checker.go`
- Create: `pkg/common/health/registry.go`
- Create: `pkg/common/health/http.go`
- Create: `pkg/common/health/registry_test.go`
- Create: `pkg/common/health/http_test.go`

**Interfaces:**

```go
type Component interface {
    Name() string
    Run(ctx context.Context) error
    Shutdown(ctx context.Context) error
}

type ComponentOptions struct {
    Required        bool
    ShutdownTimeout time.Duration
}

type Runner struct {
    // private registration and state
}

func New(options Options) (*Runner, error)
func (r *Runner) Add(component Component, options ComponentOptions) error
func (r *Runner) Run(ctx context.Context) error
func (r *Runner) Shutdown(ctx context.Context) error

type Checker interface {
    Check(ctx context.Context) error
}

type Status struct {
    Name      string        `json:"name"`
    Healthy   bool          `json:"healthy"`
    Critical  bool          `json:"critical"`
    Duration  time.Duration `json:"duration"`
    ErrorCode string        `json:"error_code,omitempty"`
}
```

- [x] **Step 1: Write deterministic lifecycle tests**

Use channels and a fake clock to prove concurrent start, unique names, required early-exit cancellation, optional component degradation, reverse shutdown order, per-component and total deadlines, primary error preservation, aggregated shutdown errors, panic containment, and repeated shutdown safety.

Health tests prove bounded concurrency, individual deadline, stable error code, readiness false on critical failure, optional degradation remains ready, and responses never expose raw error text.

- [x] **Step 2: Run focused tests**

Run:

```bash
go test ./pkg/common/lifecycle ./pkg/common/health -count=1
```

Expected: FAIL because the packages do not exist.

- [x] **Step 3: Implement lifecycle state transitions**

The runner has explicit `new`, `running`, `stopping`, and `stopped` states guarded by a mutex; component runs are tracked by `errgroup.WithContext`. The first required unexpected exit becomes the primary error. Shutdown copies the registration slice, walks it in reverse, applies `min(component timeout, remaining total timeout)`, and joins secondary errors with `errors.Join`.

- [x] **Step 4: Implement the bounded health registry**

Registration validates unique bounded names and positive timeouts. `CheckReadiness` copies checker entries under read lock, releases the lock, executes through a semaphore, and returns stable sorted statuses. `HTTPHandler` exposes `/live` and `/ready` adapters with 200 or 503 based on registry state.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/common/lifecycle ./pkg/common/health -count=100
go test -race ./pkg/common/lifecycle ./pkg/common/health -count=10
go test ./...
```

Expected: all exit 0 with no goroutine leak in repeated runner tests. Do not commit.

### Task 8: Harden HTTP Authentication, Authorization, CORS, CSRF, and Server Lifecycle

**Files:**
- Modify: `pkg/common/http/middlewares/authentication.go`
- Modify: `pkg/common/http/middlewares/authorization.go`
- Modify: `pkg/common/http/middlewares/cors.go`
- Modify: `pkg/common/http/middlewares/compose.go`
- Modify: `pkg/common/http/middlewares/recovery.go`
- Modify: `pkg/common/http/middlewares/rate_limit.go`
- Modify: `pkg/common/http/middlewares/body_limit.go`
- Create: `pkg/common/http/middlewares/csrf.go`
- Create: `pkg/common/http/middlewares/security_headers.go`
- Create: `pkg/common/http/middlewares/request_deadline.go`
- Create: `pkg/common/http/middlewares/authentication_test.go`
- Create: `pkg/common/http/middlewares/authorization_test.go`
- Create: `pkg/common/http/middlewares/cors_test.go`
- Create: `pkg/common/http/middlewares/csrf_test.go`
- Create: `pkg/common/http/server.go`
- Create: `pkg/common/http/server_test.go`

**Interfaces:**

```go
func AuthenticationWithVerifier(
    verifier authentication.Verifier,
    options ...AuthenticationOption,
) gin.HandlerFunc

func OptionalAuthenticationWithVerifier(
    verifier authentication.Verifier,
    options ...AuthenticationOption,
) gin.HandlerFunc

type CredentialSource uint8

const (
    CredentialBearer CredentialSource = iota + 1
    CredentialCookie
)

type CORSOptions struct {
    AllowedOrigins   []string
    AllowedMethods   []string
    AllowedHeaders   []string
    ExposedHeaders   []string
    AllowCredentials bool
    MaxAge           time.Duration
}

func CORS(options CORSOptions) (gin.HandlerFunc, error)

type CSRFVerifier interface {
    Verify(ctx context.Context, request *http.Request) error
}

func CSRF(verifier CSRFVerifier, safeMethods ...string) (gin.HandlerFunc, error)

type ServerOptions struct {
    Address         string
    ReadHeaderTimeout time.Duration
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    IdleTimeout     time.Duration
    ShutdownTimeout time.Duration
    MaxHeaderBytes  int
}
```

- [x] **Step 1: Write middleware characterization and security tests**

Prove:

- required missing/invalid credentials return 401;
- optional absent credentials set `authentication.Anonymous()` and continue;
- optional present-invalid/wrong-purpose credentials return 401;
- conflicting cookie/bearer credentials are rejected;
- authorization maps no principal to 401, deny to 403, evaluator failure to 500;
- exact CORS origins only, wildcard plus credentials rejected, correct `Vary: Origin`;
- CSRF executes only for configured unsafe methods and never logs the token;
- composed middleware runs once in declared order;
- panic recovery writes one response and preserves cancellation;
- rate-limit identity ignores forwarding headers unless the peer is a configured trusted proxy.

- [x] **Step 2: Run the focused tests**

Run:

```bash
go test ./pkg/common/http/middlewares ./pkg/common/http -count=1
```

Expected: FAIL on current optional-auth downgrade and reflective CORS behavior.

- [x] **Step 3: Implement thin transport adapters**

Credential extraction returns one of:

```go
type credentialResult struct {
    raw     string
    present bool
    source  CredentialSource
}
```

It rejects malformed schemes, empty cookie values, conflicting sources, and oversize credentials before calling the verifier. Store only the typed principal in authentication context; legacy context values are populated by an explicit compatibility option.

Authorization depends on `authorization.Authorizer` and maps only package-owned typed errors. CORS normalizes each configured origin once at construction and compares scheme/host/port exactly.

- [x] **Step 4: Implement lifecycle-compatible HTTP server**

`Server.Run(ctx)` calls `ListenAndServe`, treats `http.ErrServerClosed` as normal only after cancellation, and never installs a signal handler. `Shutdown` is idempotent and uses the smaller of the caller deadline and configured drain timeout. Reject zero/negative timeout and header-limit options.

- [x] **Step 5: Deprecate unsafe compatibility paths**

Keep `Authentication(publicKey)`, `OptionalAuthentication(publicKey)`, global `CORSMiddleware`, inbound global circuit breaker, and unsafe composition only as documented deprecated facades where safe semantics can be preserved. Delete `Parallel` from stable exports; if an external compatibility symbol must remain, return a constructor error explaining that concurrent Gin context execution is unsupported.

- [x] **Step 6: Verify**

Run:

```bash
go test ./pkg/common/http/... -count=1
go test -race ./pkg/common/http/... -count=10
go test ./...
```

Expected: all exit 0; invalid optional credentials never resolve anonymous. Do not commit.

### Task 9: Make Outbound HTTP Retry and Fan-out Explicitly Bounded

**Files:**
- Modify: `pkg/common/http/http_pool.go`
- Modify: `pkg/common/http/request/retry.go`
- Modify: `pkg/common/http/request/fanout.go`
- Create: `pkg/common/http/request/policy.go`
- Create: `pkg/common/http/request/errors.go`
- Create: `pkg/common/http/request/retry_test.go`
- Create: `pkg/common/http/request/fanout_test.go`
- Create: `pkg/common/http/request/fuzz_test.go`

**Interfaces:**

```go
type RetryPolicy struct {
    MaxRetries       int
    InitialBackoff   time.Duration
    MaxBackoff       time.Duration
    JitterFraction   float64
    RetryStatusCodes []int
    RetryMethods     []string
    RequireIdempotencyKeyForUnsafe bool
}

type Attempt struct {
    Number     int
    Duration   time.Duration
    StatusCode int
    ErrorCode  string
}

func Do(ctx context.Context, client *http.Client, request *http.Request, policy RetryPolicy) (*http.Response, []Attempt, error)

type FanoutOptions struct {
    MaxConcurrency int
    Mode           FanoutMode
}
```

- [x] **Step 1: Write retry and ownership tests**

Tests assert zero means no retry, non-replayable bodies are rejected before a second attempt, unsafe methods require caller opt-in plus an idempotency key, `Retry-After` respects the remaining deadline, intermediate bodies are drained/closed, cancellation stops immediately, final result is never `(nil, nil)`, attempt metadata excludes URLs/query secrets, fan-out is bounded and ordered, and child panics become typed errors.

- [x] **Step 2: Run focused tests**

Run: `go test ./pkg/common/http/request -count=1`

Expected: FAIL against the ambiguous current retry/fan-out behavior.

- [x] **Step 3: Implement deterministic budget handling**

Clone requests per attempt, using `GetBody` when a body exists. Compute each delay with checked arithmetic and injected entropy/clock options for tests. Before sleeping:

```go
if deadline, ok := ctx.Deadline(); ok && time.Now().Add(delay).After(deadline) {
    return finalResponse, attempts, context.DeadlineExceeded
}
```

Return a final retryable response to the caller without closing it; close only intermediate responses.

- [x] **Step 4: Implement bounded fan-out**

Allocate the result slice at input length, use an `errgroup` plus semaphore, recover panic at the child boundary, and cancel siblings only for the documented first-success/first-error mode. Never retain caller-owned mutable input without copying.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/common/http/request -count=100
go test -race ./pkg/common/http/request -count=10
go test ./...
```

Expected: all exit 0 and leak tests report no retained goroutines/bodies. Do not commit.

### Task 10: Move gRPC Propagation into Its Transport Package

**Files:**
- Create: `pkg/common/grpc/errors.go`
- Create: `pkg/common/grpc/options.go`
- Create: `pkg/common/grpc/interceptors/correlation.go`
- Create: `pkg/common/grpc/interceptors/authentication.go`
- Create: `pkg/common/grpc/interceptors/authorization.go`
- Create: `pkg/common/grpc/interceptors/logging.go`
- Create: `pkg/common/grpc/interceptors/recovery.go`
- Create: `pkg/common/grpc/interceptors/interceptors_test.go`
- Modify: `pkg/cid/grpc.go`
- Modify: `pkg/cid/grpc_test.go`

**Interfaces:**

```go
func UnaryCorrelationClient() grpc.UnaryClientInterceptor
func StreamCorrelationClient() grpc.StreamClientInterceptor
func UnaryCorrelationServer(options CorrelationOptions) grpc.UnaryServerInterceptor
func StreamCorrelationServer(options CorrelationOptions) grpc.StreamServerInterceptor

func UnaryAuthenticationServer(verifier authentication.Verifier, source MetadataSource) grpc.UnaryServerInterceptor
func StreamAuthenticationServer(verifier authentication.Verifier, source MetadataSource) grpc.StreamServerInterceptor
```

- [x] **Step 1: Write bufconn unary/stream tests**

Tests prove bounded metadata, invalid inbound CID replacement, context propagation through wrapped streams, optional-auth present-invalid rejection, typed status mapping, logger CID enrichment, panic recovery, and no internal error text in the response.

- [x] **Step 2: Run focused tests**

Run: `go test ./pkg/common/grpc/... ./pkg/cid -count=1`

Expected: FAIL because gRPC transport packages do not exist.

- [x] **Step 3: Implement transport-peer interceptors**

Use `metadata.MD.Copy`, validate header count/value length, and wrap `grpc.ServerStream.Context()` with the enriched context. Map `apperr` machine codes to gRPC codes in `pkg/common/grpc/errors.go`; unexpected errors map to `codes.Internal` and are logged once at the boundary.

- [x] **Step 4: Delegate deprecated CID interceptors**

Every function in `pkg/cid/grpc.go` becomes a `Deprecated:` wrapper calling the new interceptor. It must use `pkg/correlation` and cannot introduce a legacy context key.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/common/grpc/... ./pkg/cid -count=1
go test -race ./pkg/common/grpc/... -count=10
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 11: Define and Validate the WebSocket Protocol Surface

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `pkg/common/websocket/options.go`
- Create: `pkg/common/websocket/principal.go`
- Create: `pkg/common/websocket/envelope.go`
- Create: `pkg/common/websocket/message.go`
- Create: `pkg/common/websocket/errors.go`
- Create: `pkg/common/websocket/options_test.go`
- Create: `pkg/common/websocket/envelope_test.go`
- Create: `pkg/common/websocket/envelope_fuzz_test.go`

**Interfaces:**

```go
type PrincipalResolver func(context.Context, *http.Request) (authentication.Principal, error)
type TopicAuthorizer func(context.Context, authentication.Principal, string) error

type DeliveryClass uint8

const (
    DeliveryLatestState DeliveryClass = iota + 1
    DeliveryEphemeral
    DeliveryCritical
)

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

type Message struct {
    Envelope    Envelope
    Class       DeliveryClass
    CoalesceKey string
}
```

- [x] **Step 1: Write strict validation and fuzz tests**

Cover every zero/negative/inconsistent option, safe default 64 KiB limits, protocol version, closed operations, bounded topic/type/ID/cursor/coalesce key, duplicate/unknown JSON fields, invalid UTF-8, oversized data, non-positive aggregate version when present, and latest-state without a key. Fuzz decode and assert bounded allocation/no panic.

- [x] **Step 2: Run focused tests**

Run: `go test ./pkg/common/websocket -count=1`

Expected: FAIL because the package does not exist.

- [x] **Step 3: Add the private protocol dependency and implement types**

Add `github.com/coder/websocket` as a direct dependency. No file outside `pkg/common/websocket` or its internal tree may import it. Use the existing focused JSON package for strict bounded decode, encode outbound envelopes before queue admission, and return stable package error codes.

- [x] **Step 4: Verify**

Run:

```bash
go test ./pkg/common/websocket -count=1
go test ./pkg/common/websocket -fuzz=FuzzDecodeEnvelope -fuzztime=10s
go list -deps ./pkg/common/websocket | rg 'github.com/coder/websocket'
```

Expected: tests pass and dependency appears only behind this package. Do not commit.

### Task 12: Implement WebSocket Hub Upgrade, Identity, and Authorized Topics

**Files:**
- Create: `pkg/common/websocket/hub.go`
- Create: `pkg/common/websocket/connection.go`
- Create: `pkg/common/websocket/upgrade.go`
- Create: `pkg/common/websocket/topics.go`
- Create: `pkg/common/websocket/selector.go`
- Create: `pkg/common/websocket/internal/transport.go`
- Create: `pkg/common/websocket/hub_test.go`
- Create: `pkg/common/websocket/upgrade_test.go`
- Create: `pkg/common/websocket/selector_test.go`

**Interfaces:**

```go
func NewHub(options Options) (*Hub, error)
func (h *Hub) Handler(resolve PrincipalResolver, authorize TopicAuthorizer) http.Handler
func (h *Hub) Run(ctx context.Context) error
func (h *Hub) Shutdown(ctx context.Context) error

func (c *Conn) ID() string
func (c *Conn) Principal() authentication.Principal
func (c *Conn) Send(ctx context.Context, message Message) error
func (c *Conn) Close(ctx context.Context, options CloseOptions) error
```

- [x] **Step 1: Write upgrade and topic-ACL tests**

Prove exact origin matching, present-invalid credentials reject upgrade, explicit anonymous only from resolver, principal invariants before allocation, trusted peer/proxy IP extraction, global/principal/IP limits, anonymous not grouped under empty subject, per-IP upgrade token bucket, all subscribe paths call the captured authorizer, no public `Conn.Subscribe`, unsubscribe only removes the caller membership, and reconnect storms remain bounded.

- [x] **Step 2: Run focused tests**

Run: `go test ./pkg/common/websocket -run 'Test(Upgrade|Subscribe|ConnectionLimit|Selector)' -count=1`

Expected: FAIL because Hub behavior is not implemented.

- [x] **Step 3: Implement Hub-owned state**

Maintain:

```go
type hubState struct {
    connections map[string]*Conn
    subjects    map[string]map[string]*Conn
    topics      map[string]map[string]*Conn
    remoteIPs   map[string]map[string]*Conn
}
```

Only authenticated non-empty subjects enter `subjects`. All membership changes occur under one routing mutex, but no resolver, authorizer, network write, handler, or caller callback executes while it is held. Allocate the connection only after origin, rate, principal, and quota checks succeed.

- [x] **Step 4: Implement declarative close selectors**

Validate bounded selectors before locking. Under lock, compute matches, atomically transition open connections to closing, detach them from all indexes, and release the lock. Owned pumps then best-effort close. Repeated calls report `AlreadyClosing` without double cleanup. `All=false` with no selector is invalid; list values are OR within a list and populated groups are ANDed.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/common/websocket -run 'Test(Upgrade|Subscribe|ConnectionLimit|Selector)' -count=100
go test -race ./pkg/common/websocket -run 'Test(Upgrade|Subscribe|ConnectionLimit|Selector)' -count=10
```

Expected: all exit 0. Do not commit.

### Task 13: Implement WebSocket Routing, Backpressure, Handlers, and Drain

**Files:**
- Create: `pkg/common/websocket/routing.go`
- Create: `pkg/common/websocket/mailbox.go`
- Create: `pkg/common/websocket/handler.go`
- Create: `pkg/common/websocket/pump.go`
- Create: `pkg/common/websocket/metrics.go`
- Create: `pkg/common/websocket/routing_test.go`
- Create: `pkg/common/websocket/mailbox_test.go`
- Create: `pkg/common/websocket/handler_test.go`
- Create: `pkg/common/websocket/shutdown_test.go`
- Create: `pkg/common/websocket/benchmark_test.go`

**Interfaces:**

```go
func (h *Hub) Broadcast(ctx context.Context, topic string, message Message) error
func (h *Hub) SendToConnection(ctx context.Context, connectionID string, message Message) error
func (h *Hub) SendToPrincipal(ctx context.Context, subject string, message Message) error
func (h *Hub) CloseConnections(ctx context.Context, selector ConnectionSelector, options CloseOptions) (CloseResult, error)

type Handler[T any] func(context.Context, *Conn, *T) error

type HandlerOptions[T any] struct {
    Parallelism int
    OrderingKey func(*T) string
    Timeout     time.Duration
}

func On[T any](hub *Hub, messageType string, handler Handler[T]) error
func OnWithOptions[T any](hub *Hub, messageType string, options HandlerOptions[T], handler Handler[T]) error
```

- [x] **Step 1: Write routing and exact-accounting tests**

Cover zero-subscriber success, typed missing target, no anonymous principal send, one-writer order, encoded-frame ownership, logical item/byte reserve and release on enqueue/dequeue/coalesce/drop/close/cancel, hub-global budget, latest replacement, ephemeral drop metric, critical 1013 close, oversize pre-admission rejection, and one destination pressure not failing unaffected recipients.

Handler tests prove serial default order, bounded keyed parallelism, same-key serialization, inbound token bucket, per-connection/global slots, smallest timeout, panic recovery, cancellation, slot release, and undrained callback reporting.

- [x] **Step 2: Run focused tests**

Run:

```bash
go test ./pkg/common/websocket -run 'Test(Routing|Mailbox|Handler|Shutdown)' -count=1
```

Expected: FAIL because the routing/pump/handler paths are missing.

- [x] **Step 3: Implement immutable frames and charged mailboxes**

Encode one immutable frame, but charge its byte length once per destination. Mailbox mutation and Hub byte reservation occur under a fixed lock order. Each queue node owns exactly one charge token whose `sync.Once` release decrements connection and Hub totals. Latest-state replacement releases the prior charge before returning; critical admission failure transitions that destination to closing with code 1013.

- [x] **Step 4: Implement tracked pumps and handler dispatch**

Each connection owns exactly one read pump and one writer pump. Register all pumps and handlers in Hub wait groups before starting goroutines. Read pump serializes control frames and validates rate before dispatch. Handler slots are acquired before goroutine start; handler context is derived from connection lifetime and minimum timeout. Recover panics, emit one stable protocol error, and release slots in a defer.

- [x] **Step 5: Implement graceful shutdown**

Shutdown atomically rejects upgrades/subscriptions, cancels tracked handler contexts, drains bounded mailboxes until deadline, closes remaining peers with 1001, and returns a typed `UndrainedError{Handlers:n}` when callbacks ignore cancellation. It never claims to terminate arbitrary goroutines.

- [x] **Step 6: Verify race, leak, and performance behavior**

Run:

```bash
go test ./pkg/common/websocket -count=100
go test -race ./pkg/common/websocket -count=20
go test ./pkg/common/websocket -run '^$' -bench 'Benchmark(Broadcast|Mailbox|Handler)' -benchmem
go test ./...
```

Expected: all tests exit 0, no race/leak report, and benchmark allocations are retained as the initial baseline. Do not commit.

### Task 14: Harden Settings, Cache, Database, MQ, Time, Encoding, and Unsafe APIs

**Files:**
- Modify: `pkg/settings/config.go`
- Create: `pkg/settings/validate.go`
- Create: `pkg/settings/validate_test.go`
- Modify: `pkg/common/cache/local.go`
- Modify: `pkg/common/cache/remote.go`
- Modify: `pkg/database/ent/tx.go`
- Modify: `pkg/database/redis/client.go`
- Modify: `pkg/mq/batcher/batcher.go`
- Modify: `pkg/common/workerpool/pool.go`
- Modify: `pkg/timer/timer.go`
- Modify: `pkg/unique/snowflake.go`
- Modify: `pkg/encoding/base62.go`
- Modify: `pkg/encoding/base62/base62.go`
- Modify: `pkg/encoding/json/json.go`
- Modify: `pkg/dto/pagination.go`
- Modify: `pkg/utils/converter.go`
- Modify: `pkg/utils/backoff.go`
- Modify: `pkg/hash/hash.go`
- Create: `pkg/common/cache/local_contract_test.go`
- Create: `pkg/common/cache/remote_contract_test.go`
- Create: `pkg/database/ent/tx_contract_test.go`
- Create: `pkg/database/redis/client_contract_test.go`
- Create: `pkg/mq/batcher/ownership_test.go`
- Create: `pkg/common/workerpool/lifecycle_test.go`
- Create: `pkg/timer/timer_test.go`
- Create: `pkg/unique/snowflake_test.go`
- Create: `pkg/encoding/base62/limits_test.go`
- Create: `pkg/encoding/json/strict_test.go`
- Create: `pkg/dto/pagination_test.go`
- Create: `pkg/utils/converter_test.go`
- Create: `pkg/utils/backoff_test.go`
- Create: `pkg/hash/hash_test.go`

**Interfaces:**
- Consumes: maturity classification from Task 1.
- Produces: validated constructors, context-aware bounded operations, and deprecated wrappers for ambiguous APIs.

- [x] **Step 1: Write characterization and invalid-input tests**

For each modified package, lock intended current success behavior, then test zero/negative/overflow configuration, cancellation, concurrent close, caller-buffer mutation after async admission, panic in callback, resource ownership, and typed unsupported behavior. Hash tests assert no `go:linkname`/runtime-private dependency. Timer tests prove positive step, bounded staleness, and idempotent concurrent stop.

- [x] **Step 2: Run the focused package tests**

Run:

```bash
go test ./pkg/settings ./pkg/common/cache ./pkg/database/ent ./pkg/database/redis \
  ./pkg/mq/batcher ./pkg/common/workerpool ./pkg/timer ./pkg/unique \
  ./pkg/encoding/... ./pkg/dto ./pkg/utils ./pkg/hash -count=1
```

Expected: at least one new invalid-input/ownership test fails against each unsafe current contract.

- [x] **Step 3: Implement validated and bounded behavior**

Rules applied exactly:

- settings reject missing/invalid environment, secret, port, timeout, pool, and limit values instead of synthesizing unsafe values;
- cache callbacks run outside locks; `GetOrLoad` has cancellation/singleflight semantics and close is consistent;
- Ent transactions preserve panic/commit/rollback error context and never leak a transaction-scoped client;
- Redis removes production prefix `KEYS`, uses bounded scan/versioning, and surfaces partial pipeline failures;
- batcher/worker pool copy retained slices, bound input, accept context, drain deterministically, and recover callback panics;
- cached timer constructor returns error, requires positive step, and stop is concurrent-idempotent;
- Snowflake remains experimental and rejects rollback/overflow/exhaustion without spinning forever;
- byte-oriented Base62 is canonical; root integer codec is a deprecated compatibility codec with separate documented alphabet/overflow behavior;
- strict JSON caps bytes/depth and rejects unknown/duplicate fields for untrusted input;
- pagination validates positive bounded sizes with checked arithmetic;
- stable conversions copy memory and validate lengths; unsafe aliases move behind an experimental package or are removed;
- hash uses a documented stable algorithm and no private runtime symbol.

- [x] **Step 4: Add architecture prohibitions**

Extend the stable-import analyzer to reject:

- `pkg/timer.CachedTimer` in security/durability packages;
- `pkg/unique.Snowflake` in stable primary-key paths;
- unsafe converters and `pkg/runtime` in stable packages;
- `pkg/utils/token` and duplicate backoff/time APIs in new stable consumers.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/settings ./pkg/common/cache ./pkg/database/... ./pkg/mq/... \
  ./pkg/common/workerpool ./pkg/timer ./pkg/unique ./pkg/encoding/... \
  ./pkg/dto ./pkg/utils ./pkg/hash -count=1
go test -race ./pkg/common/cache ./pkg/mq/batcher ./pkg/common/workerpool ./pkg/timer -count=10
go run ./cmd/stableimports ./...
go test ./...
```

Expected: all exit 0; external-service integration suites remain opt-in. Do not commit.

### Task 15: Prove Forge and CDC Experimental Boundaries

**Files:**
- Modify: `pkg/mq/forge/producer.go`
- Modify: `pkg/mq/forge/broker.go`
- Modify: `pkg/mq/forge/commit_log.go`
- Modify: `pkg/mq/forge/consumer.go`
- Modify: `pkg/mq/forge/retention.go`
- Modify: `pkg/mq/forge/compaction.go`
- Modify: `pkg/mq/forge/record_codec.go`
- Create: `pkg/mq/forge/fault_injection_test.go`
- Create: `pkg/mq/forge/recovery_fuzz_test.go`
- Modify: `pkg/cdc/debezium.go`
- Modify: `pkg/cdc/mongo.go`
- Create: `pkg/cdc/debezium_test.go`
- Create: `pkg/cdc/debezium_fuzz_test.go`

**Interfaces:**
- Produces explicit experimental acknowledgment and parse/durability contracts; does not promote either package to stable.

- [x] **Step 1: Write fault and fuzz tests**

Forge tests inject short write, fsync failure, rename failure, corruption, oversized record, append crash, compaction crash, and concurrent producer ordering. CDC tests cover the closed Debezium operation set, before/after/tombstone invariants, flat/envelope/stringified formats, duplicate fields, nesting/input/allocation caps, and malformed Mongo keys/dates.

- [x] **Step 2: Run focused suites**

Run:

```bash
go test ./pkg/mq/forge ./pkg/cdc -count=1
```

Expected: new fault/corruption cases fail against current behavior.

- [x] **Step 3: Implement bounded recovery and parsing**

Forge copies caller bytes before return, distinguishes memory admission/append/fsync acknowledgments, never clears unpersisted batches after append failure, stages compaction with fsync plus atomic rename, and terminates recovery with typed corruption errors. CDC caps raw and nested decoded bytes, accepts only `c/u/d/r`, validates operation-specific fields, and returns bounded typed diagnostics.

- [x] **Step 4: Keep critical-path prohibition during the proof checkpoint**

Catalog status remains `experimental` until the promotion gate below. Analyzer fixtures
prove stable packages cannot accidentally consume Forge or CDC before their queue,
retention, ownership, replay, and lifecycle contracts are complete.

- [x] **Step 5: Verify**

Run:

```bash
go test ./pkg/mq/forge ./pkg/cdc -count=100
go test -race ./pkg/mq/forge -count=10
go test ./pkg/mq/forge -fuzz=FuzzRecovery -fuzztime=10s
go test ./pkg/cdc -fuzz=FuzzDebezium -fuzztime=10s
```

Expected: all exit 0 without unbounded disk/memory growth in fixtures. Do not commit.

### Task 15B: Promote Forge to the Stable Single-Node Queue

**Files:**
- Modify: `pkg/mq/forge/broker.go`
- Modify: `pkg/mq/forge/consumer.go`
- Modify: `pkg/mq/forge/offset_store.go`
- Modify: `pkg/mq/forge/retention.go`
- Modify: `pkg/mq/forge/commit_log.go`
- Modify: `pkg/mq/forge/options.go`
- Create: `pkg/mq/forge/delivery.go`
- Create: `pkg/mq/forge/consumer_lock_unix.go`
- Create: `pkg/mq/forge/consumer_lock_windows.go`
- Create: `pkg/mq/forge/queue_contract_test.go`
- Create: `pkg/mq/forge/process_crash_test.go`
- Modify: `pkg/maturity/catalog.json`

**Interfaces:**
- Produces a stable, one-node, single-owner-process, at-least-once queue transport.
- PostgreSQL transactional outbox remains Judgify’s source of truth; Forge replaces Kafka
  as the local delivery log.

- [x] **Step 1: Write queue contract and process-crash tests**

Cover exclusive Broker data-directory ownership across processes, exclusive
`(consumer_group, topic)` ownership inside that Broker, explicit
delivery offsets, partial-batch commit, redelivery after uncommitted crash, fsynced
producer survival after abrupt process exit, idempotent close, replay, bounded poll,
storage-full backpressure, retention pinned by the slowest committed consumer, and
broker-owned child shutdown.

- [x] **Step 2: Replace unsafe spin locks and implement stable lifecycle**

Forge must use standard blocking mutexes around filesystem I/O, track producers and
consumers owned by a Broker, drain producers before closing topic logs, release
cross-process consumer locks, and reject client creation once shutdown starts.

- [x] **Step 3: Implement safe queue retention and replay**

Default retention never deletes an offset needed by a consumer group. A bounded hard
storage limit returns a typed backpressure error instead of deleting unconsumed data.
Explicit destructive time/size retention remains opt-in for event-log use cases.
Consumers expose delivery offsets, monotonic explicit commits, rollback-to-committed,
and replay without hidden acknowledgments.

- [x] **Step 4: Promote Forge and the bounded CDC parser**

Change `pkg/mq/forge` and `pkg/cdc` to `stable` only after their package contracts,
race tests, fuzz corpus, and API snapshot pass. CDC is a bounded parsing boundary;
Debezium/Kafka Connect is not introduced into the Judge path. External change feeds
may publish normalized CDC envelopes into Forge.

- [x] **Step 5: Verify the stable queue gate**

Run:

```bash
go test ./pkg/mq/forge -count=100
go test -race ./pkg/mq/forge -count=20
go test ./pkg/mq/forge -fuzz=FuzzRecovery -fuzztime=30s
go test ./pkg/cdc -fuzz=FuzzDebezium -fuzztime=30s
go run ./cmd/stableimports ./...
```

Expected: all exit 0, subprocess crash fixtures recover without loss, and Forge has no
dependency on experimental or unsafe packages. Do not commit.

### Task 16: Publish Documentation, Compatibility Tests, and Release Gates

**Files:**
- Create: `docs/migration-v0-to-v1.md`
- Create: `docs/security.md`
- Create: `docs/lifecycle.md`
- Create: `docs/websocket.md`
- Create: `docs/package-maturity.md`
- Modify: `README.md`
- Modify: `.github/workflows/ci.yml`
- Create: `scripts/verify-public-api.sh`
- Create: `scripts/verify-docs.sh`

**Interfaces:**
- Produces a tagged-module-ready library contract. Judgify may develop through `go.work`, but release consumers use a semantic tag and no filesystem `replace`.

- [ ] **Step 1: Write docs/API drift checks**

The script runs examples, validates every stable catalog entry has package docs/config/error/concurrency/lifecycle/security sections, checks deprecated symbols include replacement text, and compares exported API snapshots with the checked-in compatibility file.

- [ ] **Step 2: Run checks and verify documentation gaps**

Run:

```bash
bash scripts/verify-docs.sh
bash scripts/verify-public-api.sh
```

Expected: FAIL until all stable packages are documented and snapshot is created.

- [ ] **Step 3: Write exact migration and operational documentation**

Document:

- root security password to focused Argon2id hasher;
- `pkg/cid` to `pkg/correlation` plus transport propagation;
- key-based JWT middleware to typed verifier;
- old authorization enforcer to `authorization.Authorizer`;
- old OAuth URL/exchange wrappers to PKCE requests;
- unsafe middleware/retry behavior to bounded replacements;
- WebSocket delivery classes, non-durability, authorization boundary, drain semantics, and caller resync responsibility.

Examples must compile and use only public types.

- [ ] **Step 4: Add complete CI gates**

CI runs format, vet, stable-import analyzer, unit tests, race matrix for concurrency packages, fuzz seed corpus, vulnerability scan, docs/examples/API drift, coverage thresholds, and separate opt-in integration jobs. Store benchmark baselines for logger/correlation/HTTP/WebSocket/cache/batcher/hash/Forge.

- [ ] **Step 5: Final library verification**

Run:

```bash
gofmt -w pkg internal cmd
go vet ./...
go test ./...
go test -race ./pkg/common/lifecycle ./pkg/common/health ./pkg/common/websocket \
  ./pkg/common/cache ./pkg/common/workerpool ./pkg/logger ./pkg/mq/batcher
bash scripts/verify-docs.sh
bash scripts/verify-public-api.sh
go run ./cmd/stableimports ./...
git diff --check
```

Expected: every command exits 0. Inspect the complete go-common diff and do not commit or tag.

## Plan Acceptance Checklist

- [ ] Every public package appears exactly once in the maturity catalog.
- [ ] No stable critical path imports an experimental/unsafe/deprecated implementation.
- [ ] Password, JWT, OAuth, authorization, correlation, lifecycle, health, HTTP retry, and WebSocket behavioral suites meet the approved coverage target.
- [ ] Optional authentication distinguishes absent credentials from present-invalid credentials.
- [ ] Casbin evaluation observes only complete old or complete new snapshots.
- [ ] CID and deprecated CID APIs share one context key.
- [ ] WebSocket has bounded item/byte/in-flight/rate/connection state and no public subscription bypass.
- [ ] Every goroutine-owning package documents and tests shutdown.
- [ ] Ordinary unit tests require no external infrastructure.
- [ ] Judgify can migrate package-by-package without a source-wide rename.
- [ ] No commit or semantic tag has been created.
