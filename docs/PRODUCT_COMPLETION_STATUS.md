# Judgify — trạng thái hoàn thiện sản phẩm

> **Cập nhật 2026-09-13:** Envoy gateway (TLS, request/correlation identity, OTel tracing, ext_authz mTLS), Loki/Tempo/Alloy stack, private API/worker Prometheus probes và contract/E2E checks đã được triển khai. Các lệnh `topology_test.sh`, `observability_stack_test.sh`, `envoy_renderer_test.sh`, `envoy_e2e.sh`, cùng focused Go tracing/authz/infrastructure tests đang xanh. Full Go suite hiện còn lỗi architecture metadata ở `material/contract` và một schema integration timeout khi thiếu database ổn định; chưa đánh dấu release hoàn tất.

> Cập nhật: 2026-08-22 (Asia/Ho_Chi_Minh) — đợt kiểm chứng thứ ba  
> Phạm vi: Judgify + `../go-common`, single-tenant, chạy production trên một máy trước.  
> Trạng thái tổng quát: **backend suite xanh; migration/roles trên PostgreSQL thật đã chứng minh; backup/restore đã chứng minh end-to-end lần đầu; judge pipeline đi qua 10/13 bước. Chưa gọi là production hoàn tất — ba bước cuối của judge pipeline và toàn bộ load/chaos cần host Linux rootless dockerd; Playwright release E2E và một phần supply-chain còn mở.**
>
> **Lưu ý handoff (2026-08-22):** phần lịch sử bên dưới còn một số snapshot cũ. Danh sách release blocker, thứ tự xử lý và cách phân biệt evidence đã chạy với claim cần chạy lại nằm tại [`docs/superpowers/plans/2026-08-22-release-completion-and-proof-plan.md`](superpowers/plans/2026-08-22-release-completion-and-proof-plan.md). Không dùng một đoạn lịch sử đơn lẻ làm nguồn trạng thái release.

## 0A. Đợt thứ ba — backup/restore và judge pipeline

Lần đầu chạy thật `scripts/backup/postgres.sh`, `scripts/restore/postgres.sh` và `test/e2e/stack.sh judge`. Bốn defect nữa, tất cả nằm trong code **chưa từng chạy lần nào**:

| # | Triệu chứng | Root cause | Fix |
|---|---|---|---|
| 10 | Backup báo `backup published` nhưng restore chết: `unrecognized configuration parameter "transaction_timeout"` | Host có libpq 18, server PostgreSQL 16. `pg_dump` 18 phát ra `SET transaction_timeout = 0;` — tham số chỉ có từ PostgreSQL 17. **Backup sinh ra file không restore được mà vẫn báo thành công.** Đây là kiểu hỏng tệ nhất của backup: chỉ lộ khi cần dùng | `ops_require_matching_postgres_client` trong `scripts/lib/operations.sh`; backup **từ chối chạy** khi major của `pg_dump` khác server |
| 11 | Restore có thể phá database đích rồi mới phát hiện không tương thích | `restore/postgres.sh` fence + kiểm blank + terminate session **trước khi** biết dump có replay được không | Kiểm `pg_restore` vs target và major của backup vs major của target **trước** mọi thao tác phá huỷ |
| 12 | `release-manifest sandbox-env` chưa từng ghi được một byte | `WriteSandboxMakefile` cấm `:` trong giá trị field, nhưng mọi `ManifestChecksum` đều là `sha256:<hex>` → `ErrInvalidDraft` cho **mọi** draft hợp lệ. Cả sandbox build chain đứng từ đầu | Bỏ `:` khỏi tập cấm; `=` vẫn cấm nên `:=` không lọt vào được. Kèm test 6 dạng injection |
| 13 | `release-manifest approve` chưa từng thành công | `writeExclusive` từ chối mọi bit world (`mode & 0o007`), call site lại yêu cầu `0o444` → `0o444 & 0o007 = 0o004`. Guard làm chính caller của nó bất khả thi | Rút thành hằng `AuthoredFileMode = 0o440` dùng chung hai call site. Đây là **siết**, không nới: bỏ quyền đọc world khỏi release manifest |

Hai defect 12/13 nằm trong package **không có một test nào**. Nay `internal/operator/releasemanifest/files_test.go` phủ: draft thật round-trip được, mode dùng chung thoả đồng thời `writeExclusive` và `releasecatalog.ValidateFilePath`, và 6 dạng injection vào giá trị Make vẫn bị chặn.

**Bài học chung của đợt này:** guard chặt quá tay có thể làm chính caller của nó bất khả thi, và nó **im lặng** — không có test nào để phát hiện. Cả bốn defect đều nằm ở đường mà unit test không đi qua: dump bằng client thật, ghi file với mode thật, boot worker thật. Code chưa từng chạy thì coi như chưa tồn tại.

Một defect thứ năm ở harness: `find_free_port` được định nghĩa **lồng bên trong** `cmd_up` nên `cmd_judge` không thấy — hàm lồng chỉ tồn tại sau khi hàm bao đã chạy. Đã nâng ra file scope.

### Judge pipeline: 10/13 bước

`bash test/e2e/stack.sh up && bash test/e2e/stack.sh judge` đi qua: build CLI → template → 12 cặp capability → sandbox-env → build sandbox image theo `sandbox.mk` → resolve digest → approve → verify → build và boot judge-worker (DB connect, outbox relay chạy) → **capability probe từ chối host**.

`ProbeDockerSecurity` đòi `name=rootless` trong `docker info SecurityOptions`. Máy này trả `["name=seccomp,profile=builtin","name=cgroupns"]`. OrbStack/Docker Desktop trên macOS cô lập bằng Linux VM chứ không bằng rootless user namespace, nên mục đó vắng **theo thiết kế**. Probe 2–4 còn đọc `/proc/self/cgroup` và `/sys/fs/cgroup` — macOS không có.

**Probe từ chối là đúng. Không nới nó để có một lần chạy xanh trên máy không đủ điều kiện.** Bước 11–13 cần host Linux rootless `dockerd` + cgroup v2 delegation; trên host đó `stack.sh judge` chạy tiếp không cần sửa gì.

Load/chaos chung đúng một blocker này: 14/23 kịch bản trong `test/chaos/scenarios.tsv` nhắm vào `judge-worker`.

## 0. Đợt thứ hai đã đổi những gì

Chín defect production thật đã được tìm ra bằng cách chạy hệ thống thật, không phải bằng đọc code. Không cái nào bị unit test bắt được:

| # | Triệu chứng | Root cause | Fix |
|---|---|---|---|
| 1 | `cmd/migrate` không mở được database: `sql: unknown driver "postgres"` | Khi tách migrate runtime khỏi Ent graph, `lib/pq` (đăng ký ở `go-common/pkg/database/ent/client.go`) rớt theo. Migrate container fail mọi lần deploy. | Blank import trong `internal/ent/migrate/runner.go` + test `TestPostgresDatabaseDriverIsRegisteredForTheOperatorBinary` |
| 2 | API chết lúc khởi động: `Tx.ExecContext is not supported` | `logDriver`/`logTx` trong go-common embed *interface* `dialect.Driver`/`dialect.Tx`; `ExecContext`/`QueryContext` không thuộc hai interface đó nên không được promote. Ent generated client type-assert đúng hai method này cho mọi raw SQL → **mọi advisory lock fail**. | Forward 4 method, giữ logging/observer parity + test `TestWrapLoggingForwardsContextStatementsOnDriverAndTransaction` |
| 3 | Database mới bootstrap xong API không boot: `cultivation: invalid trait offer` | `RefreshCatalog` lúc startup đòi catalog hợp lệ (`RootPool >= 1`, `TalentPool >= 6`). DB vừa migrate + bootstrap-admin có 0 trait. Cách duy nhất tạo trait là admin API — vốn cần API đang chạy. **Cài mới không bao giờ boot được.** | Gộp `RefreshCatalog`/`InvalidatePool` về `resyncCatalog` dùng chung tolerance đã có sẵn cho catalog đầu tiên + 3 test |

| 4 | Publish/start/end/cancel cuộc thi đều fail trên PostgreSQL thật | `UPDATE contests SET status = $3 ... published_at = CASE WHEN $3 = 'upcoming' ...` — PostgreSQL suy ra `character varying` cho `$3` từ mệnh đề SET và `text` từ các so sánh trong CASE, rồi từ chối ngay ở bước prepare: `42P08 inconsistent types deduced for parameter $3, DETAIL: text versus character varying`. **Toàn bộ contest lifecycle transition hỏng.** sqlmock không chạy planner nên không unit test nào thấy được. | Ép kiểu `$3::text` ở cả 5 chỗ dùng, cộng guard prepare mô tả bên dưới |

| 5 | `POST /traits` luôn 500 | `traitDocument()` nhét con trỏ struct `*TraitEffectRevision` vào audit document, nhưng `boundedjson` cố tình chỉ nhận map/scalar để hàm redact chạy được theo từng key. Nó trả `ErrInvalid` → rollback → 500. Test không thấy vì `NewTraitService` nhận journal qua variadic; test gọi không truyền journal nên nhánh audit bị bỏ qua hoàn toàn. | Thêm `traitEffectRevisionDocument()` dựng map tường minh, giống 4 hàm `*Document` khác. **Không** sửa `boundedjson` cho nhận struct — làm vậy là phá lớp redaction |
| 6 | Ghi trait effect fail khi không có element | `resolveTraitEffectElements` trả `nil`; `json.Marshal(nil)` ra `null`, mà cột `target_element_ids` là `NOT NULL` kèm CHECK `jsonb_typeof(...) = 'array'` | Chuẩn hoá thành slice rỗng ở constructor và ở tầng ghi |
| 7 | Catalog cultivation lưu xuống không đọc lại được → không lấy được trait offer | `CatalogTrait.EffectRevision.Checksum` khai báo `json:"-"`, nên khi lưu cả pool dưới dạng JSON thì checksum rụng; đọc lên checksum = 0 → `Valid()` false. Kiểm tra `CatalogChecksum` trước khi sửa: nó marshal đúng shape đó nên **cũng** không chứa checksum từng trait, tức checksum cấp catalog phủ đúng bằng khối JSON được lưu → dựng lại checksum lúc đọc là an toàn | `rehydrateEffectChecksums()` ở `scanCatalog`, và **cùng lỗi ở `loadOffer`** — chặn đăng ký member |
| 8 | 17 câu SQL khoá dòng trên bảng append-only → đăng ký member, chấm bài, nộp bài contest đều hỏng | 37 bảng append-only bị REVOKE `UPDATE` với cả `judgify_api` lẫn `judgify_worker`. PostgreSQL đòi quyền `UPDATE` cho **mọi** kiểu khoá dòng — `FOR UPDATE`, `FOR SHARE`, `FOR KEY SHARE` như nhau. Kiểm chứng thật: cả ba đều `permission denied`, SELECT thường thì chạy | Bỏ khoá ở chỗ nào append-only; chỗ nào khoá **cả** bảng mutable thì chỉ bỏ alias append-only, giữ khoá mutable. Kèm guard tĩnh |

| 9 | Trình duyệt không gọi được API ở **mọi** môi trường, kể cả image production | `constants/api/base.ts` đặt `BASE_API_URL = configuredApiUrl \|\| "/api"`, còn `lib/api/client.ts` gọi `new URL(normalized)` — mà `new URL("/api")` **ném `TypeError: Invalid URL`** vì URL tương đối cần base. Lỗi bị nuốt bởi `.catch(() => undefined)` trong session context nên chỉ thấy màn hình "Kết nối phân quyền bị gián đoạn". `docker/website/Dockerfile` cũng build với đúng `/api`. | `normalizeBaseUrl` nhận path prefix tương đối, `resolveRequestUrl` ghép với browser origin; giữ nguyên mọi kiểm tra same-origin và vẫn chặn `//evil.com` |

**Bài học từ defect 8 — vì sao nó lọt tới tận E2E:** `PREPARE` **không** kiểm tra quyền (đã test: câu hỏng vẫn PREPARE thành công bằng `judgify_api`), nên guard prepare 349 câu ở defect 4 mù hoàn toàn với lớp này. Tệ hơn: mọi gate database đều nối bằng `judgify_migrator`, mà migrator **có** quyền UPDATE — tức là toàn bộ hệ thống kiểm chứng đang chạy bằng đúng cái role duy nhất mà 17 câu đó không lỗi.

Guard mới: `internal/architecture/append_only_row_lock_test.go`. Đọc catalog từ `schemaextras.AppendOnlyTables()`, quét SQL thô, phân giải alias `FROM`/`JOIN`, phân biệt `FOR ... OF a, b` (chỉ khoá a, b) với `FOR ...` trần (khoá **mọi** bảng trong câu), và bắt được cả SQL nối chuỗi động `statement += " FOR SHARE OF t"`. Đã fault-inject chứng minh nó cắn. Chính guard này tìm ra chỗ thứ 17 mà ba agent sửa bỏ sót.

**Bài học từ defect 4 — quan trọng hơn chính bản vá:** lỗi này chỉ tồn tại ở *prepare time*. Mọi unit test trong repo dùng sqlmock, vốn không gọi PostgreSQL planner, nên cả lớp lỗi này **về mặt cấu trúc là vô hình** với test hiện có. Đã thêm `internal/contest/adapters/driven/db/raw_sql_postgres_test.go`: nó dùng `go/ast` duyệt source non-test của package, lấy mọi string literal truyền vào `QueryContext`/`ExecContext`, rồi PREPARE từng câu trên database thật. Không cần fixture, tự skip khi thiếu `JUDGIFY_TEST_DATABASE_URL`, và tự động phủ luôn câu lệnh thêm sau này. Đã kiểm chứng guard thật sự cắn: bỏ cast đi thì test fail đúng lỗi 42P08.

Guard nay đã phủ **17 package, 349 câu lệnh, tất cả prepare sạch** trên PostgreSQL đã migrate. Collector nằm ở `internal/testutil/rawsql`; mỗi package chỉ cần một file test 14 dòng gọi `rawsql.PrepareAll`.

**Bốn dạng call site mà một collector ngây thơ sẽ bỏ sót** — mỗi dạng đều từng làm con số đẹp hơn thực tế:

1. literal ngay tại call site — dạng duy nhất bản đầu tiên nhìn thấy;
2. const/var ở package level rồi truyền theo tên (`workerLeaseReadinessSQL`, `createRevisionTableSQL`, ...);
3. const nối với phần đuôi literal;
4. helper bọc `QueryContext` và forward tham số `string` — literal khi đó nằm ở *caller của helper*, tên hàm không phải `QueryContext` nên không bao giờ bị bắt.

Và một lỗ thứ năm chỉ lộ ra khi tự inject lỗi để thử: collector ban đầu chỉ tìm `QueryContext`/`ExecContext`, **bỏ sót toàn bộ `QueryRowContext`** — 57 call site. Thêm method đó vào đưa số câu từ 292 lên 349. Bài học: khi một guard báo "0 defect", hãy tự tay tạo lỗi ở đúng dạng call site mình nghi ngờ nhất, đừng tin con số.

Đã fault-inject bốn lần, mỗi lần một đường resolve khác nhau, tất cả đều bị bắt đúng: literal (42P08), forwarder (42703), const (42703), `QueryRowContext` (42703).

**Giới hạn đã biết, đừng nhầm là đã phủ:**

- Khoảng 33 call site dựng SQL lúc chạy bằng `strings.Builder`/`strings.Join`/số placeholder động (`announcement_content.go`, `durable.go`, `finalization.go`, `outbox.go`). Không phân tích tĩnh nào prepare được; muốn phủ phải bắt ở runtime. Guard in ra danh sách này mỗi lần chạy thay vì bỏ qua im lặng.
- `internal/platform/eventing` chỉ có một query duy nhất và nó dựng bằng `strings.Builder`, nên không thể host guard.
- Guard prepare bằng role `judgify_migrator`. Prepare xác minh object tồn tại và suy luận kiểu, **không** xác minh quyền row-level của role `judgify_api`/`judgify_worker` lúc chạy thật.

Ngoài ra một data race thật trong `internal/judge`: `TestDurableWorkerWipesLeaseCapability...` đọc `lease.LeaseToken` trong spin loop trong khi scheduler goroutine `wipeRawLeaseToken` ghi zero lên cùng backing array. Đã đồng bộ qua `FreeExecutionSlots()`.

Và năm defect fail-open/portability trong chính verification script — nghĩa là một số gate trước đây báo xanh mà không thật sự kiểm tra gì:

- `topology.sh`: `seconds()` không parse Go duration ghép (`1m30s`) nên worker `stop_grace_period: 90s` bị đọc thành 0 → gate báo sai. Parser mới fail-closed.
- `migrations.sh`: gọi `rg` trong script; `rg` không tồn tại → `if` trả false → check “cấm runtime auto-migration” **im lặng pass**. Đổi sang `grep -REn`.
- `database_baseline.sh`: ba danh sách append-only hard-code (14/13/13 mục) đã stale so với catalog thật 37 bảng; `activated_judge_capabilities` bị exclude nhầm khỏi check DML (fail-open) và một check không bao giờ chạy tới. Nay derive từ baseline SQL + drift guard đối chiếu `pg_trigger`.
- `resilience.sh`: awk `!(` xuống dòng — GNU-awk only, không chạy trên macOS awk.
- 10 script dùng `${TMPDIR}` có trailing slash → path non-canonical → `ops_require_secure_directory` fail. Thêm `ops_temp_root()`.

## 1. Đang làm gì ở thời điểm này?

Pha hiện tại không còn là dựng skeleton hay nối mock. Công việc đang tập trung vào năm gate cuối:

1. ~~Chạy full backend suite và sửa test/fixture đã lệch sau đợt refactor lớn~~ — **xong**, xem mục 6.
2. Chạy đầy đủ frontend verification, production build, bundle budget và Playwright E2E cho public/member/admin. Build và bundle đã xanh; **Playwright release E2E còn mở**.
3. ~~Chứng minh schema/migration trên PostgreSQL thật~~ — **xong** cho baseline/roles/append-only; backup/restore đang làm.
4. Chứng minh topology một máy: API không chạm Docker socket, judge-worker độc quyền sandbox, local Forge queue/outbox phục hồi đúng sau crash. **Topology check và cả 5 image đã xanh; rootless/cgroup probe và crash recovery còn mở.**
5. Dọn source cuối kỳ, loại dead code/test cũ, chạy race/fuzz/security/supply-chain/load/chaos, rồi mới đưa ra kết luận release. **`-race` đã xanh cả hai module; load/chaos còn mở.**

Không có commit hoặc stage nào được tạo theo yêu cầu. Worktree hiện rất lớn và đang dirty có chủ đích:

- Judgify: khoảng 2.350 path thay đổi/được tạo/xóa; tracked diff hiện khoảng 1.091 file, 283.833 dòng thêm và 92.393 dòng xóa; phần lớn file mới/generate chưa tracked.
- go-common: 239 path thay đổi; tracked diff hiện khoảng 122 file, 11.774 dòng thêm và 3.757 dòng xóa.

Con số trên giải thích vì sao pha kiểm chứng kéo dài: đây là một lần cutover gần như toàn bộ product surface, không phải một feature đơn lẻ.

## 1A. Handoff nhanh cho AI nhận việc

Đọc mục này trước; sau đó dùng các mục 3–7 làm catalog chi tiết. Không cần đọc toàn bộ lịch sử chat để tiếp tục đúng hướng.

### Repository và điểm vào

| Thành phần | Đường dẫn | Điểm vào/chú ý |
|---|---|---|
| Product root | `/Users/jerry/nhanhmc/coding/Judgify` | Worktree dirty có chủ đích; không reset/checkout/clean. |
| API | `/Users/jerry/nhanhmc/coding/Judgify/api` | Go modular monolith; runtime `cmd/server`, `cmd/judge-worker`, `cmd/migrate`. |
| Frontend | `/Users/jerry/nhanhmc/coding/Judgify/website` | Next.js; dùng service/runtime schema/copy catalog, không direct fetch. |
| Shared library | `/Users/jerry/nhanhmc/coding/go-common` | Generic library; tuyệt đối không đưa vocabulary/domain Judgify vào đây. |
| Main status | `docs/PRODUCT_COMPLETION_STATUS.md` | File này là current handoff. |
| Approved design | `docs/superpowers/specs/` | Quyết định sản phẩm/architecture đã chốt. |
| Implementation plans | `docs/superpowers/plans/` | Chỉ để tra rationale và coverage; status ở file hiện tại mới hơn. |

### Quyết định đã chốt — không tự đổi

- Single-tenant, một host trước; không tự thêm multi-tenancy, microservices, Redis, Kafka, SSE, hoặc framework frontend khác.
- Giữ Go + Gin + Ent + PostgreSQL + Casbin + ent-adapter + Docker + Next.js + React + Tailwind + shadcn/Base UI + Framer Motion.
- Forge là local durable queue thay Kafka cho Judgify; PostgreSQL/outbox là authoritative state. CDC trong go-common phải generic.
- Realtime browser là native WebSocket qua `coder/websocket` được che trong `go-common/pkg/common/websocket`; không quay lại SSE/EventSource/polling.
- API không có Docker socket. Chỉ `judge-worker` thực thi code và Docker sandbox.
- Browser auth dùng secure same-origin cookies + CSRF; không `localStorage` bearer token/profile.
- Phân quyền là Ent + Casbin + ent-adapter, với revisioned snapshot. Admin cập nhật quyền qua capability/role workflow, không sửa raw Casbin tuple.
- Các mutation nhạy cảm dùng exact capability, optimistic version, idempotency, preview/review HMAC, recent-auth, audit và outbox receipt khi contract yêu cầu.
- UI chỉ một ngôn ngữ Vietnamese hiện tại, nhưng text phải đi qua typed catalog để sẵn sàng i18n. Mọi route có light/dark/system.
- Logs dùng CID từ go-common; domain fields như `submission_id`, `job_id`, `event_id` chỉ thêm ở Judgify. Không log token, password, code, hidden testcase, policy secret hay lease token.
- Không commit/stage khi chưa được user yêu cầu. Không dùng `git reset --hard`, `git checkout --` hoặc `git clean`.

### Cách làm an toàn cho AI tiếp theo

1. Backend suite đã xanh. Việc còn lại là integration/E2E/ops evidence, không phải sửa unit test.
2. Với mỗi lỗi: tái hiện bằng test hẹp, tìm root cause, sửa source hoặc fixture theo contract mới, rồi chạy test hẹp lại. Không nới security/limits/architecture gate để làm xanh.
3. Khi đổi Ent schema/port: generate lại Ent/mocks theo Makefile, cập nhật baseline/Atlas bằng workflow đã có; không sửa tay `api/internal/ent/generate`.
4. Không chạy destructive migration/restore/Compose reset trên dữ liệu không rõ scope. Các integration test cần database disposable rõ ràng.
5. Sau một nhóm sửa: chạy `gofmt`, `git diff --check`, focused test; sau đó mới full suite.
6. Cập nhật chính file này khi một gate đổi trạng thái, bao gồm command và kết quả thực tế.

### Full API failure snapshot — đã đóng

```text
cd /Users/jerry/nhanhmc/coding/Judgify/api
go test ./... -count=1
```

Kết quả: **PASS**, exit 0, 85 package. Bảng lỗi cũ đã được gỡ khỏi tài liệu này vì mọi mục trong đó đã đóng; chi tiết root cause và cách sửa nằm ở mục 0 và mục 5.

Điều còn đáng nhớ: lần chạy trước, panic ở `TestSessionManagementLogoutAllLocksSessionsBeforeRefreshTokens` làm test binary của package đó dừng giữa chừng, nên các test phía sau **chưa từng chạy**. Sau khi sửa panic mới lộ ra hai enum drift. Khi một package panic, đừng tin danh sách lỗi của nó là đầy đủ.

### Lệnh kiểm chứng theo thứ tự

```text
# 1. Sau mỗi fix backend
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test <package> -count=1
cd /Users/jerry/nhanhmc/coding/Judgify/api && go test ./... -count=1

# 2. Shared library — hiện đã pass, chạy lại khi có thay đổi ở go-common
cd /Users/jerry/nhanhmc/coding/go-common && go test ./... -count=1

# 3. Frontend source gate — hiện đã pass
cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run verify

# 4. Frontend release gate — build và bundle đã pass; E2E cần stack thật ở bước 6
cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run build
cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run check:bundle
cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run test:e2e:release

# 5. Product gates (chạy sau source freeze) — tất cả đã pass
cd /Users/jerry/nhanhmc/coding/Judgify && make verify-generated
cd /Users/jerry/nhanhmc/coding/Judgify && make verify-architecture-strict
cd /Users/jerry/nhanhmc/coding/Judgify && make verify-migrations
cd /Users/jerry/nhanhmc/coding/Judgify && make verify-packaging

# 6. Gate nặng, chạy trực tiếp bằng script
cd /Users/jerry/nhanhmc/coding/Judgify && bash scripts/verify/go.sh          # gồm -race cả 2 module
cd /Users/jerry/nhanhmc/coding/Judgify && bash scripts/verify/contracts.sh
cd /Users/jerry/nhanhmc/coding/Judgify && bash scripts/verify/database_baseline.sh   # dựng PG16 disposable

# 7. Stack đầy đủ cho E2E (tự dựng PostgreSQL riêng, tự dọn)
cd /Users/jerry/nhanhmc/coding/Judgify && bash test/e2e/stack.sh up
cd /Users/jerry/nhanhmc/coding/Judgify && bash test/e2e/stack.sh env    # in đường dẫn file env
cd /Users/jerry/nhanhmc/coding/Judgify && bash test/e2e/stack.sh down
```

Yêu cầu công cụ trên máy dev: `docker`, `atlas`, `node`, `npm`, `golangci-lint`, `govulncheck`, `syft`, `grype`,
`gitleaks`, `cosign`, `k6`, `age`, và PostgreSQL client (`psql`/`pg_dump`/`pg_restore`, Homebrew libpq không nằm
sẵn trên PATH — thêm `/opt/homebrew/opt/libpq/bin`). Script dùng bash 3.2 của macOS nên không được dùng
`mapfile`/`readarray`/`declare -A`, và không được gọi `rg` bên trong script.

## 2. Quy ước trạng thái trong tài liệu

| Trạng thái | Ý nghĩa |
|---|---|
| **Implemented** | Source production và wiring đã có, nhưng chưa đủ bằng chứng release cuối. |
| **Verified** | Có lệnh kiểm chứng gần nhất chạy xanh trên snapshot hiện tại. |
| **Partially verified** | Một hoặc nhiều suite scoped xanh nhưng full gate vẫn còn đỏ/chưa chạy. |
| **Open** | Chưa làm xong, chưa nối đủ, hoặc chưa có bằng chứng bắt buộc. |

## 3. Những phần đã làm

### 3.1. Kiến trúc production kernel

Trạng thái: **Implemented, phần architecture chính đã Verified**.

- Giữ modular monolith Go, nhưng tách ba runtime role rõ ràng: API, `judge-worker`, và `migrate`.
- PostgreSQL là nguồn dữ liệu bền vững; cache, WebSocket hub, wakeup và local worker state chỉ là accelerator có thể tái tạo.
- Bỏ global mutable dependencies; config/logger/Ent client/cache đi qua explicit dependency graph và transaction-scoped client.
- Tách HTTP/Gin ra khỏi platform core. `judge-worker` không còn kéo Gin hay HTTP adapter.
- Tách migration command khỏi bootstrap, Ent-generated graph và product domains; `cmd/migrate` chỉ kéo configuration, logging và migration runtime cần thiết.
- API role không sở hữu Docker socket; chỉ worker được quyền thực thi contestant code.
- Architecture tests đã chứng minh worker graph không kéo Gin/HTTP, API graph không kéo Docker executor và module dependency graph không có violation mới.
- Audit, operation, observability, transaction, content-render và event/outbox trở thành platform primitives dùng chung giữa các domain.

### 3.2. `go-common` — thư viện dùng chung

Trạng thái: **Verified ở full unit suite; release hardening sâu hơn còn Open**.

Đã bổ sung/harden:

- Correlation/CID xuyên HTTP, gRPC, lifecycle, outbox và WebSocket; logging theo level, cấu hình production `INFO`, development `DEBUG`.
- Logger có bounds, rotation/permission an toàn, redaction contract, debug-window và context-aware structured fields.
- Authentication primitives: password Argon2id, JWT, principal/session contracts; nil/config/entropy/secret-buffer handling được siết lại.
- Authorization primitives và Casbin adapter boundary có API generic, không hard-code Judgify.
- Lifecycle, health, observability, worker pool, locks, transaction abstractions và bounded resilience helpers.
- HTTP middleware/handler/request/response/validation, security boundary và gRPC interceptors.
- `pkg/common/websocket` che hoàn toàn `coder/websocket`:
  - strict versioned envelope;
  - Hub/connection/topic/subscription;
  - authenticated principal routing;
  - bounded mailbox và ba lớp backpressure;
  - latest-state coalescing;
  - origin normalization;
  - ping/pong, auth-expiry, graceful drain;
  - connection revocation và CID logging;
  - strict nested JSON/duplicate-key rejection.
- Forge local durable queue:
  - append-only commit log/segments/index recovery;
  - Ack/Fsync semantics;
  - retention/storage ceilings;
  - deterministic topics;
  - bounded producer/consumer behavior;
  - outbox bridge cho worker-owned projections.
- Generic outbox package có concurrency/message/lease/retry ceilings và panic/backoff containment.
- CDC package và generic Kafka Connect provisioning helper vẫn trung lập domain; không nhúng judge worker hay tên product.
- Stable/import maturity catalog, public API snapshot, domain-neutral checker và docs package được cập nhật.
- Database helpers cho Ent, Redis, MongoDB, Elasticsearch, wide-column và cache vẫn giữ compatibility surface hiện tại.

Bằng chứng mới nhất:

```text
cd ../go-common && go test ./... -count=1
PASS — toàn bộ packages, gồm websocket, Forge, outbox, security, maturity và stable-import checks.
```

### 3.3. Identity, authentication, profile và authorization

Trạng thái: **Implemented; Partially verified**.

Đã làm:

- Cookie-first browser session; không lưu auth token/profile production trong `localStorage`.
- CSRF/Origin boundary, refresh coalescing, session listing/revoke/logout-all và unknown-outcome convergence.
- Password login/register/change/forgot/reset; one-time token dùng URL fragment và scrub khỏi browser-visible URL.
- Recovery contact có verification, resend, revoke, masked projection và security email durable delivery.
- OAuth completion ticket và onboarding tách inspection/finalization; không đưa access token về browser.
- Administrative invitation:
  - token namespace/crypto riêng;
  - create/resend/revoke/activate;
  - user tự đặt mật khẩu;
  - activation page dùng fragment token;
  - invite lifecycle và delivery không dùng lẫn recovery token.
- Dynamic onboarding/profile schema:
  - version/checksum evidence;
  - typed attribute renderer dùng chung;
  - revisioned values;
  - exact active-revision CAS;
  - required/user-editable/admin visibility rules;
  - display order;
  - number domain an toàn với Go/JavaScript;
  - two-phase bounded attribute migration và provenance.
- Self profile typed mutation, public profile tối thiểu fail-closed, privacy private-by-default và privacy CAS/audit/outbox.
- Ent + Casbin + ent-adapter authorization snapshot:
  - canonical resource/action catalog;
  - atomic old-or-new policy publication;
  - revision reconciliation;
  - exact effective capabilities;
  - role metadata/status/version;
  - role policy và user-role assignment;
  - admin preview → review → recent re-auth → apply;
  - HMAC confirmation bind actor/session/idempotency/action/resource/version/request hash;
  - audit + outbox + durable replay receipt;
  - archive role xóa live Casbin grants;
  - chặn vô hiệu hóa final active superadmin.
- Admin có UI cập nhật role/permission và user-role bằng capability keys, không chỉnh raw Casbin tuple.

Phần chưa đóng:

- Full identity suite còn các fixture/contract test lệch ở security-email ordering, Resend config, CSRF/cookie expectations, attribute-migration batches và token lifecycle.
- Cần chạy lại toàn bộ invitation/recovery concurrency/replay/expiry/final-superadmin tests sau khi các fixture trên được sửa.
- Bootstrap-admin test còn lệch SQL sequence sau khi thêm system profile definitions/revisions; source bootstrap đã có behavior mới nhưng test chưa theo kịp.

### 3.4. Problem authoring, submission và judge

Trạng thái: **Implemented; submission/problem Verified, judge Partially verified**.

Đã làm:

- Versioned problem draft/publication model; authored content rendering, catalog-version fencing và optimistic concurrency.
- Immutable problem revision, testset revision, testcase revision, content-addressed test assets, taxonomy/reward snapshots.
- Public samples tách khỏi hidden/admin test data; anonymous API không đọc draft/hidden content.
- Runtime/checker capability catalog do server sở hữu; FE không có language allowlist hard-coded.
- Submission intake dùng idempotent command, bounded source, exact runtime key và contest scoping.
- Durable job/attempt/judgement state, leasing/reaping/finalization và immutable judge spec.
- Worker supervisor/capability probe, hostile archive staging, TAR truncation/alignment rejection và per-container state.
- Docker execution boundary chỉ thuộc worker; API không có Docker dependency.
- Finalization/outbox/realtime invalidation dùng committed event; FE luôn REST-refresh canonical submission thay vì tin partial WS row.
- Rejudge/retirement/query/admin submission inspection paths theo safe summary/detail separation.

Bằng chứng:

```text
go test ./internal/judge/... ./internal/submission/...
PASS ở lượt scoped trước.

go test ./internal/problem/adapters/driven/db ./internal/profile -count=1
PASS sau khi đồng bộ revision repository và generated Ent fixtures.
```

Lượt integrated `go test ./... -count=1` ngày 2026-08-19 lại làm lộ một regression test ở `internal/judge`: durable worker không ghi nhận ba heartbeat calls trước khi hủy local execution. Vì vậy chưa được coi judge gate là xanh ổn định cho tới khi test này pass cả scoped lẫn full-suite scheduling.

### 3.5. Contest, standings và rating

Trạng thái: **Implemented; scoped full suite Verified**.

Đã làm:

- Contest draft/content revisions, lifecycle và publish/freeze invariants.
- Registration có source/registered timestamp/version/count và membership projections.
- Contest-scoped submissions, scoring attempts, standings snapshots và safe public/admin/self projections.
- Announcements/clarifications và admin contest control surfaces.
- Durable rating ledger, rerating chain/checksum, reconciliation và projection generation.
- Realtime standings dùng WebSocket làm invalidation, sau đó REST-refresh canonical state.

Bằng chứng:

```text
go test ./internal/contest/... -count=1
PASS
```

### 3.6. Cultivation, onboarding reward và ranking

Trạng thái: **Implemented; scoped full suite Verified**.

Đã làm:

- Versioned elements, levels, ranks, rarities, traits và immutable trait-effect revisions.
- Server-owned onboarding trait catalog/offer, exact selection và durable reward profile.
- Effective-dated reward profile/effect snapshot và provenance.
- Solve-reward processed transition, progression ledger, projection/reconciliation và idempotent apply.
- Ranking read model, privacy changes, preview/apply fencing và bounded reconciliation operations.
- Dead legacy CRUD/gacha/user-stats surfaces không còn production caller đã được loại; chỉ giữ repository capability thật sự cần cho provisioning/projection.

Bằng chứng:

```text
go test ./internal/cultivation/... -count=1
PASS
```

### 3.7. Materials

Trạng thái: **Implemented; service suite Verified, release E2E còn Open**.

Đã làm:

- Material identity/canonical slug, immutable revisions/tags/categories và public projection.
- Lifecycle đầy đủ: draft → in-review → publish, reviewer return-to-draft, archive và rollback.
- Administrative lifecycle dùng preview → exact reviewed hash → recent-auth → HMAC confirmation → optimistic apply → audit/outbox receipt/replay.
- Public list/detail/category counts chỉ nhìn published/public content; không rò draft/private visibility.
- Bounded view increments, canonical routing, related content và admin draft editing bằng API thật.

Bằng chứng:

```text
go test ./internal/material/core/service -count=1
PASS
```

### 3.8. Realtime và notifications

Trạng thái: **Implemented; core suite Verified, system E2E còn Open**.

Đã làm:

- Không còn SSE/EventSource hay timer polling trong production FE.
- Một shared native WebSocket connection cho mỗi tab; anonymous mode và single-use ticket mode.
- Topic ACK/readiness, replay cursor, protocol errors, reconnect và REST reconciliation.
- Durable outbox là producer duy nhất cho notification/browser invalidation; bỏ direct post-commit WS publish.
- Submission tailer polling cũ đã rời production composition; `submission.judged.v1` map sang user/contest invalidation.
- Authorization outbox xử lý revoke/fence trước event; polling tailer chỉ fail-safe đóng stale sockets, không phát browser event trùng.
- Persisted notification inbox và notification campaigns/revisions/explicit users.
- Bounded delivery/replay state và realtime health/readiness.

Bằng chứng:

```text
go test ./internal/realtime -count=1
PASS sau khi test được cập nhật theo semantics “poller chỉ fence, outbox mới publish”.
```

### 3.9. Operations, observability và release assets

Trạng thái: **Implemented; Partially verified**.

Đã làm:

- Audit explorer, operation plane, batches/control receipts, readiness/health và observability endpoints.
- CID-based logs; domain IDs thêm dưới dạng bounded structured fields, không hard-code vào go-common.
- Admin observability/debug-window có capability gate và audited activation.
- API/worker/migrate Docker targets, one-host production Compose, network/volume/resource boundaries.
- Rootless Docker/cgroup capability contracts và worker readiness probes.
- Local Forge queue topology; PostgreSQL/outbox là durable source of truth.
- Migration tooling, generated Ent schema, one-file baseline, Atlas checksum/inventory và schema extras append-only triggers.
- Runbooks cho deploy/rollback, API availability, auth/authz, judge queue, outbox/realtime lag, capacity/disk, sandbox incident, contest rating, security email và database restore.
- Backup policy, backup/restore scripts, topology checks, supply-chain scripts và resilience/chaos/load harness đã có source.
- Resilience catalog có các boundary cases và WebSocket slow-client path dùng socket thật không đọc TCP/TLS.

Đã verify:

- `go build ./...` từng pass sau khi tách runtime graph.
- Full architecture suite đã pass sau migration/worker refactor.
- `go test ./internal/ent/migrate/...` và scoped race test migration đã pass sau khi cập nhật inventory 122 declarations / 119 Ent tables.

Chưa verify ở mức release:

- PostgreSQL migration integration trên database sạch và database restore thật.
- Docker image build cho API/worker/migrate/website/sandbox trên snapshot cuối.
- Rootless/cgroup/sandbox probe trên host đích.
- Crash/load/slow-client/queue-pressure/restore/rollback evidence thật.
- Supply-chain evidence, vulnerability scan và release manifest cuối.

## 4. Frontend đã làm gì?

Trạng thái: **Implemented; `npm run verify` Verified; production build/E2E Open**.

### 4.1. Foundation

- Next.js/React/Tailwind/shadcn/Base UI/Framer Motion giữ nguyên tech stack.
- Light/dark/system theme; không dùng effect hack gây hydration mismatch.
- Toàn bộ product copy mới đi qua typed Vietnamese catalog; không dựng song ngữ runtime nhưng cấu trúc sẵn sàng thêm locale sau.
- Central API client, endpoints, runtime schemas, UUID/number/date formatting, safe presentation errors và AbortSignal/stale-response fencing.
- Session context là authority duy nhất cho browser auth/capabilities.
- Realtime context sở hữu một WebSocket/tab; không SSE, không polling thay thế, không duplicate socket theo component.
- Không còn production mock/fake/fixture/direct `fetch`/`any` bypass theo source architecture scan gần nhất.
- Responsive states phân biệt initial loading, refreshing, empty, stale, error và retry; giữ dữ liệu cũ khi refresh fail ở các màn hình quan trọng.
- Accessibility: labels, keyboard interaction, focus lỗi, dialog primitives, 44px controls, reduced motion và semantic status/live regions.

### 4.2. Public/member routes

- Landing/about.
- Login/register/forgot/reset/verify recovery contact.
- OAuth completion/onboarding.
- Invitation activation.
- Problem arena/list/detail/submission workbench.
- Contest discovery/detail/arena/registration/standings.
- Materials list/detail.
- Ranking.
- Notifications.
- Self profile, typed profile editor, recovery/session security/privacy.
- Public profile `/u/[username]` fail-closed theo visibility.

### 4.3. Admin routes

- Dashboard capability-aware.
- Users và invitations.
- Roles, policies và user-role assignments với review/re-auth/apply flow.
- Attribute definitions/revisions/lifecycle/migration.
- Problems/create/edit/publish/testset/runtime.
- Submissions safe monitor/detail.
- Contests/detail/clarifications/lifecycle.
- Materials/categories/lifecycle.
- Difficulties, tags, elements, levels, ranks, rarities, traits, user traits và progression.
- Judge operations.
- Notification campaigns.
- Audit explorer.
- Operations và observability/debug window.

### 4.4. Bằng chứng frontend mới nhất

```text
cd website && npm run verify

PASS Frontend architecture: 387 files, 0 baselined violations
PASS Route/acceptance contract: 47 routes, 1 shared WebSocket
PASS Contract tests: 158/158
PASS Vitest: 16 files, 59/59 tests
PASS TypeScript: tsc --noEmit
PASS ESLint (nằm trong npm run lint)
```

Chưa được tính là release pass vì vẫn cần:

```text
npm run build
npm run check:bundle
npm run test:e2e:release
```

E2E cần backend/database/worker/email delivery và browser fixtures thật; không được thay bằng mock chỉ để xanh.

## 5. Những gì còn thiếu — danh sách có thứ tự

### P0 — đóng full backend test suite

Trạng thái: **Closed** — `go test ./... -count=1` exit 0, 85 package.

Toàn bộ tám nhóm đỏ trước đây đã đóng. Cách sửa và lý do ghi ở mục 0. Vài điểm cần nhớ cho người sau:

- Fix nào cũng chỉ được đụng vào fixture khi contract production đúng. Các contract *mới* đã giữ nguyên: security-email khóa `security_email_deliveries` trước để chặn TOCTOU trên lease; Resend fail-closed ngay lúc construct; CSRF/Origin không có đường vòng cho test; bounds của attribute-migration batch giữ nguyên; token lifecycle composite giữ nguyên.
- Một vòng review đối kháng đã bắt được ba test "xanh nhưng sai lý do". Đây là dạng lỗi nguy hiểm nhất trong đợt này, đáng nhớ:
  - `TestResendPasswordResetDeliveryFailsClosedForInsecureOrPlaceholderConfig`: fixture nền thiếu `InvitationURL`, nên cả năm sub-case dừng ở gate rỗng-field chứ không phải ở lỗi mà tên test nói. `require.Error` vẫn xanh mà không kiểm tra gì. Nay assert bằng `require.EqualError` gắn đúng thông điệp từng defect, cộng một `require.NoError` chứng minh fixture nền hợp lệ.
  - `TestSecurityEmailPrepareCancelsVerificationWhenExactContactIsNotPending`: mock thiếu cột `purpose` nên scan ra chuỗi rỗng, cancel đi qua purpose-guard thay vì status-guard. Đã bổ sung cột và thêm test riêng cho contact purpose `invitation`.
  - `TestLogoutClearsAllBrowserAuthorityEvenWhenRevocationFails`: gọi thẳng handler nên middleware authentication không chạy, và chỉ assert 4 trong 6 cookie. Nay drive qua router thật với principal + CSRF hợp lệ, và lấy danh sách cookie từ `BrowserCookiePolicy` chứ không chép tay.
- Enum drift lộ ra sau khi sửa panic ở `logout-all`: panic làm test binary dừng sớm nên các test sau đó chưa từng chạy. `User.status` đã thêm `invited` và `UserRecoveryContact.purpose` đã thêm `invitation` — source đúng, test cũ. Đã đưa literal về constant trong `identity_security_constants.go`.

### P0 — generated/schema/migration consistency

Trạng thái: **Closed**.

- `scripts/verify/go.sh` chạy `scripts/verify/generated.sh` và không phát hiện drift.
- `scripts/verify/database_baseline.sh` PASS trên PostgreSQL 16 sạch: role bootstrap, Atlas relational parity, migrator production apply, chạy lại idempotent, ownership, least-privilege grant, append-only trigger/function và hành vi từ chối UPDATE/DELETE/TRUNCATE.
- Ba danh sách append-only trong script đó nay derive từ chính baseline SQL và được đối chiếu với `pg_trigger` của database sống, nên không thể stale âm thầm nữa. Fault injection đã chứng minh gate vẫn cắn.

### P0 — frontend production build

Trạng thái: **Closed**.

- `npm run verify`, `npm run build`, `npm run check:bundle` đều xanh.
- Bundle initial JS của route public giảm từ **373 KB xuống 217.7 KB** (`/`), route nặng nhất `/register` từ 446 KB xuống 249.2 KB. Cách làm:
  - tách namespace `ADMIN` (129 KB trong 212 KB) khỏi catalog dùng chung, thêm `ADMIN_TEXT`/`adminText` với type machinery generic theo catalog; route public không còn kéo copy admin;
  - defer overlay chỉ mở sau tương tác: mobile nav Sheet, notification menu, theme dropdown, trait card, trait codex, profile attribute Select. Trigger vẫn render ngay nên không có layout shift; chunk được preload ở `pointerenter`/`focus` hoặc lúc mount;
  - `sonner` và `TooltipProvider` rời khỏi initial bundle;
  - endpoint catalog import trực tiếp theo sub-module thay vì qua barrel.
- Framer Motion vẫn nằm trong stack và vẫn được dùng thật ở `modules/cultivation/trait-card.tsx`; nó chỉ rời khỏi initial JS của `/register` vì trait card chỉ render sau khi roll.
- **Metric của gate đã đổi có chủ đích:** `scripts/check-bundle.mjs` không tính bundle polyfill nữa. Next phát polyfill kèm thuộc tính `nomodule`, nên mọi trình duyệt sản phẩm hỗ trợ đều bỏ qua — tính nó vào ngân sách là đo sai cái người dùng thật tải về. Số polyfill vẫn được ghi riêng trong `polyfillGzipBytes` và `legacyGzipBytes` để không giấu đi. Budget đặt ở **280 KB**, đo được từ snapshot thật; sàn framework bất khả giảm (react-dom 62.7 + Next runtime 60.6 + webpack 2.7 + main-app 0.2) là **126.2 KB**, phần còn lại dành cho product code.
- Nợ kỹ thuật còn lại: mỗi component được defer render `null` trong khoảng từ lúc điều kiện hiển thị mở tới lúc chunk về. Preload chạy từ lúc mount hoặc lúc bắt đầu roll nên chunk tĩnh đã cache luôn thắng lời gọi API, nhưng cần kiểm tra lại trên mạng bị throttle trước khi release.

### P0 — system integration và E2E

Trạng thái: **Open**.

Phải chạy ít nhất:

- anonymous → register/OAuth/invitation → onboarding → session;
- admin role/policy/user-role update với recent re-auth và durable replay;
- profile schema/value/privacy/public profile;
- problem draft → publish → submit → Forge queue → worker → judgement → WS invalidation → REST refresh;
- contest register/freeze/submit/standings/rating;
- material draft/review/publish/archive/rollback;
- notification inbox/campaign;
- recovery email/reset/revoke;
- admin audit/operation/observability.

Sau đó chạy Playwright ở light/dark/system và canonical responsive widths cho public/member/admin.

### P1 — worker/sandbox/queue production proof

Trạng thái: **Partially verified**.

Đã xong:

- Cả năm image build được: `make docker-build-{api,worker,migrate,website,sandbox}`.
- API container không có Docker: `judgify/api:dev` không chứa docker CLI và không có socket nào; chỉ `judgify/worker:dev` có CLI; `deploy/compose.production.yml` bind `docker.sock` duy nhất cho service `worker`, qua đường socket rootless riêng do vận hành chỉ định.
- Sandbox runtime khớp đúng catalog code-owned (`initial_profiles.go`), kiểm chứng bằng cách chạy image thật.

Còn lại:

- Rootless Docker + cgroup v2 probe trên host đích.
- CPU/wall/memory/process/output/network/filesystem bounds đo thật trong sandbox.
- Worker crash/restart, lease reap, idempotent finalization, không mất/nhân đôi job.
- Forge segment recovery, AckFsync, storage full, DLQ và outbox replay trong topology Judgify thật.

**Nợ kỹ thuật đã biết:** `docker/sandbox/Dockerfile` pin phiên bản gói apk nhưng không pin snapshot repository Alpine. Alpine không giữ lại bản cũ sau khi ra patch mới, nên build sẽ vỡ lại sau mỗi lần Alpine phát hành security release — đúng như đã xảy ra lần này với CPython và OpenJDK. Fix bền vững cần một repository snapshot cố định thời điểm, hoặc lưu chính các package tarball vào artifact store theo content-address rồi `apk add --no-network`. Chưa làm.

### P1 — operations/recovery evidence

Trạng thái: **Open**.

- Backup thật và restore destructive test theo RPO ≤ 5 phút, RTO ≤ 60 phút. `test/operations/backup_restore_test.sh` đã có sẵn và đúng, nhưng **fixture driver mà nó yêu cầu chưa từng tồn tại trong repo**, nên `make verify-integration` chưa bao giờ chạy được. Driver nay đã có ở `test/operations/backup_restore_driver.sh` (dựng container PostgreSQL 16 riêng, bootstrap role, apply baseline bằng `cmd/migrate` thật, chạy `cmd/bootstrap-admin`, tạo target rỗng, rồi so sánh source/target theo bảng, row count, checksum không phụ thuộc thứ tự, trigger append-only và grant). **Chưa chạy được lần nào** vì lệnh bị permission classifier chặn; cần operator chạy tay:

```text
cd /Users/jerry/nhanhmc/coding/Judgify
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
JUDGIFY_BACKUP_RESTORE_TEST_ACK=DESTROY_ONLY_JUDGIFY_RESTORE_TEST_DATABASES \
JUDGIFY_BACKUP_RESTORE_DRIVER=$PWD/test/operations/backup_restore_driver.sh \
JUDGIFY_TEST_SOURCE_PGSERVICE=judgify_src \
JUDGIFY_TEST_TARGET_PGSERVICE=judgify_tgt \
JUDGIFY_TEST_SOURCE_DATABASE=source_judgify_restore_test \
JUDGIFY_TEST_TARGET_DATABASE=target_judgify_restore_test \
sh test/operations/backup_restore_test.sh
```

Driver ghi hai section vào `~/.pg_service.conf` và gỡ đúng hai section đó lúc cleanup; nếu bạn đã có file này với service riêng thì phần của bạn không bị đụng.
- Database migration + deploy + rollback rehearsal.
- Outbox/realtime lag, disk/capacity pressure và security-email failure recovery.
- Slow WebSocket client/backpressure, reconnect/replay và bounded memory.
- Chaos catalog, load test và soak test; lưu evidence dưới `ops/evidence`. Cùng vấn đề: `scripts/verify/resilience.sh --run` yêu cầu `JUDGIFY_RESILIENCE_ASSERTION_DRIVER` và `JUDGIFY_CHAOS_DRIVER`; **cả hai chưa tồn tại**. Script hiện chỉ là harness không có driver, nên `--check` xanh không nói gì về khả năng chịu tải hay phục hồi.

### P1 — security/supply chain

Trạng thái: **Partially verified**.

Đã xong:

- `go test -race -count=1` xanh trên cả `api` và `go-common` (chạy trong `scripts/verify/go.sh`).
- `go vet`, `go mod verify`, `go mod tidy -diff` xanh cả hai module.
- golangci-lint **0 issue**. Trước đó có 83 finding (29 staticcheck, 24 errcheck, 22 unused, 5 ineffassign, 3 govet) — nợ có sẵn, không phải do đợt nâng cấp. Lần chạy `scripts/verify/go.sh` đầu tiên chết ở bước coverage *trước khi* tới golangci-lint, nên gate này thực tế **chưa từng chạy**; một ví dụ nữa của bài học ở cuối mục 7. Nhóm `unused` là 22 symbol chết thật, đã xóa sau khi chứng minh không còn caller.
- **Cảnh báo về cách sửa lint:** một vòng dọn lint đã im lặng SA1012 bằng cách *xóa* các assertion nil-context ở 7 file test, trong khi production vẫn còn guard `ctx == nil`. Như vậy là mất coverage của hành vi fail-closed. Đã khôi phục toàn bộ bằng cách giữ nil trong một biến `context.Context` thay vì literal — SA1012 chỉ bắt literal. Không dùng `//nolint` ở bất kỳ đâu.
- **Coverage floor phải đo authored code.** Profile thô có 221.499 dòng, trong đó 185.708 dòng (84%) là Ent generated và mock generated, vốn không có test riêng. Total thô vì thế là **6.3%** và đo mức độ sinh code chứ không đo code viết tay; một floor đặt theo số đó sẽ không bao giờ bắt được regression thật. `scripts/verify/go.sh` nay lọc `/internal/ent/generate/` và `/mocks/` trước khi tính, cho **authored coverage 37.8%**. Đặt `JUDGIFY_GO_COVERAGE_FLOOR_PERCENT` theo số này, không theo số cũ.
- `npm audit --package-lock-only --omit=dev --audit-level=high` xanh — **0 vulnerabilities**. Trước đó có một lỗ hổng high: `nanoid < 3.3.18` (GHSA-2v37-7h3g-55p8) đi vào qua `next` → `postcss`. Đã thêm `"nanoid": "3.3.18"` vào khối `overrides` sẵn có trong `website/package.json`, đúng cách repo đang ghim `postcss`/`sharp`. `npm run verify`, `npm run build`, `npm run check:bundle` chạy lại đều xanh sau khi bump.
- Secret scan xanh trên working tree. Có `.gitleaks.toml` với allowlist theo từng path và ghi rõ lý do; thư mục build output bị loại vì không phải source.

Còn lại:

- **golangci-lint đỏ: 83 finding có sẵn** — 29 staticcheck, 24 errcheck, 22 unused, 5 ineffassign, 3 govet. Nhóm `unused` chồng lấn trực tiếp với hạng mục dọn dead code ở P1 cleanup. Nhóm `errcheck` phần lớn là `rows.Close()`/`file.Close()` không kiểm tra lỗi trong tầng DB — cần rà từng cái, không nên bịt hàng loạt bằng `_ =`.
- Fuzz corpora cho parsers/envelopes/archive/batch codecs.
- SBOM/provenance/signature/release manifest verification. Bị chặn cùng lý do với supply-chain evidence bên dưới.

Log redaction — **đã rà, chưa thấy vi phạm**:

- Không có `zap.Any(...)` nào trong production source, nên không có object nào bị log nguyên khối.
- Không có structured field nào tên `code`/`source`/`solution`/`stdin`/`stdout`/`expected`/`password`/`token`/`secret`/`credential`. Bốn hit khi grep chuỗi `lease` đều là `release_id` — định danh công khai, không phải lease token.
- 56 field trong Ent schema được đánh `Sensitive()`.
- Bên go-common, redaction có test riêng: raw JWT (`TestVerifierUsesBoundedClockSkewAndRedactsRawToken`), HTTP request logger (`TestRequestLoggerUsesRouteTemplateAndRedactsSecrets`), CSRF failure, query string, và recovery middleware.
- Migration runner trả `redactedError` nên connection string không bao giờ vào log — đã thấy hoạt động đúng khi debug lỗi driver ở mục 0.

Việc còn lại của mục này là rà thủ công output thật của worker/sandbox khi chạy contestant code, cần bước 6.

**Blocker đã chốt, không phải thiếu sót:** `api/go.mod` giữ `replace github.com/huynhanx03/go-common => ../../go-common` và version `v0.0.0` để phát triển song song hai repo. `scripts/verify/supply_chain.sh` từ chối cả hai điều đó, và chế độ `--evidence` còn đòi working tree sạch. Vì vậy gate supply-chain sẽ đỏ cho tới khi go-common được tag và publish. Đây là quyết định có ý thức, không được coi là đã đóng.

### P1 — cleanup cuối

Trạng thái: **Partially verified**.

Đã kiểm:

- `gofmt -l` sạch trên `api/{cmd,internal,pkg}` và `go-common/pkg`; `git diff --check` sạch ở cả hai repo.
- **Không còn TODO/FIXME/XXX/HACK nào** trong production source của cả api và frontend.
- Không còn SSE/EventSource. Chỉ một `new WebSocket(...)` duy nhất trong `lib/realtime/websocket-client.ts`. Các `setInterval` còn lại là heartbeat ping/pong của chính WebSocket client và một đồng hồ đếm ngược UI ở `modules/profile/recovery-contact-card.tsx` (tự dừng khi hết countdown) — không phải polling mạng.
- Frontend architecture checker xanh: 391 file, 0 violation; không còn production mock/fake/direct `fetch`/`any` bypass.
- Hai plan file bị agent tạo nhầm ở `website/docs/superpowers/plans/` đã chuyển về `docs/superpowers/plans/` để tài liệu chỉ có một nơi chính tắc.

Còn lại:

- Xóa dead legacy tests/files/mocks/exports sau khi chứng minh không còn caller.
- Reconcile README/backend/frontend/deploy/runbooks với source cuối.
- Review toàn bộ untracked/deleted paths trước khi bàn giao; không commit nếu chưa được yêu cầu.

## 6. Bằng chứng đã có và bằng chứng chưa có

Mỗi dòng dưới đây ứng với một lệnh đã chạy thật trên snapshot hiện tại, không phải suy đoán.

| Gate | Lệnh | Kết quả |
|---|---|---|
| API full suite | `cd api && go test ./... -count=1` | **PASS** — 85 package, exit 0 |
| go-common full suite | `cd ../go-common && go test ./... -count=1` | **PASS** — 63 package, exit 0 |
| Go verification tổng | `scripts/verify/go.sh` | **PASS toàn bộ** — gofmt, `go mod verify`, `go mod tidy -diff`, `go vet`, generated-drift, `go test -race` cả hai module, architecture strict, coverage, golangci-lint (0 issue), govulncheck. Authored coverage 37.8% |
| govulncheck | `govulncheck ./...` cả hai module | **PASS** — 0 vulnerabilities sau khi nâng Go 1.26.6 và bump dependency |
| Contract/topology/observability/migration/rollout | `scripts/verify/contracts.sh` | **PASS** |
| PostgreSQL baseline thật | `scripts/verify/database_baseline.sh` | **PASS** — migrator production apply 690 statement trên PG16 sạch, chạy lại = `already_current`, roles/ownership/least-privilege/append-only trigger đều đúng |
| Migration inventory | `scripts/verify/migrations.sh --check` | **PASS** |
| Topology production | `scripts/verify/topology.sh` | **PASS** |
| Docker images | `make docker-build-{api,worker,migrate,website,sandbox}` | **PASS** — cả 5 image build được |
| API không chạm Docker | kiểm chứng trực tiếp trên image | **PASS** — `judgify/api:dev` không có docker CLI, không có socket; chỉ `judgify/worker:dev` có; compose bind `docker.sock` duy nhất cho service `worker` qua socket rootless riêng |
| Sandbox runtime khớp catalog | `docker run judgify/sandbox:dev` | **PASS** — gcc 15.2.0, CPython 3.12.14, OpenJDK 17.0.20, go1.25.10 khớp `initial_profiles.go` |
| Secret scan | `gitleaks detect --no-git` | **PASS** — no leaks (có `.gitleaks.toml`, allowlist từng path kèm lý do) |
| npm audit | `npm audit --package-lock-only --omit=dev --audit-level=high` | **PASS** — 0 vulnerabilities sau khi override `nanoid` 3.3.18 |
| Frontend source gate | `cd website && npm run verify` | **PASS** |
| Frontend production build | `cd website && npm run build` | **PASS** |
| Frontend bundle budget | `cd website && npm run check:bundle` | **PASS** — 9/9 route dưới 280 KB; `/` 217.7 KB, cao nhất `/arena` 265.2 KB |
| API khởi động với DB thật | `test/e2e/stack.sh up` | **PASS đến ready** — migrate, bootstrap-admin, server ready, admin login qua route thật |
| SQL thô prepare được trên PostgreSQL | `JUDGIFY_TEST_DATABASE_URL=... go test ./internal/... -run TestEveryRawStatement -count=1` | **PASS** — 349/349 câu ở 17 package, trên database đã migrate thật |
| E2E stack dựng đủ fixture | `bash test/e2e/stack.sh up` | **PASS** — exit 0; seed đủ 14 biến `JUDGIFY_E2E_*`: admin, member (profile public), problem/contest/material đã publish, catalog cultivation, policy |
| Playwright release E2E | `npm run test:e2e:release` | **7 pass / 567 fail** — mọi fail còn lại cùng một nguyên nhân: `/session/me` trả 401 sau khi `/auth/login` trả 200; xem mục 5 |
| Backup/restore destructive | `test/operations/backup_restore_test.sh` | **CHƯA CHẠY ĐƯỢC TỪ PHIÊN NÀY** — driver đã viết xong (`test/operations/backup_restore_driver.sh`), nhưng lệnh chạy bị permission classifier chặn. Cần operator chạy tay, xem mục 5 |
| Worker/sandbox rootless + cgroup probe | — | **CHƯA CHỨNG MINH** |
| Forge crash recovery / outbox replay trong topology thật | — | **CHƯA CHỨNG MINH** |
| Load/chaos/soak | `scripts/verify/resilience.sh --run` | **CHƯA CHẠY** — cần assertion driver và chaos driver, cả hai chưa tồn tại |
| Supply-chain release evidence | `scripts/verify/supply_chain.sh --evidence` | **BLOCKED** — cần `replace` go-common bị gỡ và worktree sạch; đã chốt giữ `replace` nên gate này đỏ có chủ đích |

## 7. Trình tự tiếp tục đề xuất

Không mở thêm feature lớn trước khi đóng các gate này:

1. ~~Sửa toàn bộ identity/bootstrap test contracts và đưa API full suite về xanh.~~ **Xong.**
2. ~~Chạy generated drift, gofmt, Go build/vet/architecture/migration checks.~~ **Xong** (`scripts/verify/go.sh`, `contracts.sh`).
3. ~~Chạy frontend production build + bundle gate.~~ **Xong.**
4. ~~Khởi tạo PostgreSQL sạch, apply baseline, bootstrap admin~~ **Xong**; API chạy thật tới trạng thái ready. Worker và website chạy thật trong cùng topology thì **chưa**.
5. **Còn đúng một lỗi chặn.** `bash test/e2e/stack.sh up` exit 0, seed đủ 14 biến. Playwright chạy được: **7 pass / 567 fail**, và toàn bộ fail còn lại cùng một nguyên nhân.

   **Triệu chứng chính xác:** `POST /auth/login` → **200**, nhưng `GET /session/me` → **401**, lặp 1170 lần trong một lượt chạy. Đăng nhập thành công nhưng cookie phiên không quay lại, nên mọi route render ra trạng thái chưa đăng nhập và `support/quality.ts:141` fail vì có console error 401.

   **Hai giả thuyết, chưa cái nào được đo:**
   - `api/internal/identity/adapters/driver/http/browser_session.go:219` từ chối policy có `Secure=false`, tức cookie luôn mang `Secure`; proxy của harness chạy HTTP thường còn production là HTTPS qua nginx. **Nhưng** Chrome coi `127.0.0.1` là origin tin cậy nên vẫn nhận cookie `Secure` — có thể không phải nguyên nhân.
   - Proxy Node tự viết trong `test/e2e/stack.sh` (tìm `proxy.js`) có thể đang làm rơi hoặc ghi đè header `Set-Cookie`/`Cookie`. Đây là phần thuộc harness nên rẻ nhất để loại trừ trước.

   **Cách đo, đừng đoán:** viết một script Playwright nhỏ đăng nhập qua `/admin/login` rồi in `context.cookies()` — xem cookie có tồn tại không và thuộc tính gì; hoặc bắt raw `Set-Cookie` khi login qua proxy. Trong đợt này đã hai lần đoán sai và mỗi lần mất trọn một vòng lặp.

   **Ranh giới:** không được tắt yêu cầu `Secure` trong `api/`; không sửa `website/e2e/**` hay `website/playwright.config.ts`. Chỉ sửa `test/e2e/stack.sh`.

6. **Chưa** — sandbox/Forge/crash/load/backpressure/backup-restore evidence. Xem cảnh báo bên dưới.
7. **Một phần** — race/vet/secret scan/npm audit/govulncheck đều xanh; golangci-lint còn 83 finding có sẵn; supply-chain evidence bị chặn có chủ đích.
8. **Chưa** — dọn source và chạy lại toàn bộ release command một lần cuối.

Chỉ sau bước 8 mới có thể ghi "Judgify production hoàn tất".

### Cảnh báo quan trọng cho người nhận việc

Ba gate P1 nhìn như "chỉ cần chạy lệnh" nhưng thực chất là **hạng mục cần viết mới**, không phải chạy lại:

| Gate | Cái đã có | Cái chưa có |
|---|---|---|
| Backup/restore | `test/operations/backup_restore_test.sh` đầy đủ và đúng | fixture driver cho `prepare`/`validate`/`cleanup` mà nó gọi |
| Load/chaos/soak | `scripts/verify/resilience.sh`, `test/chaos/scenarios.tsv`, `test/load/*.js` | `JUDGIFY_RESILIENCE_ASSERTION_DRIVER` và `JUDGIFY_CHAOS_DRIVER` |
| Release evidence | `scripts/verify/supply_chain.sh --evidence` | go-common đã tag/publish và working tree sạch |

Đừng để `--check` xanh đánh lừa: `resilience.sh --check` chỉ xác nhận file asset tồn tại và catalog đúng cú pháp, nó **không** chạy load hay chaos nào. Tương tự, `make verify-integration` chưa từng chạy được lần nào vì thiếu driver.

Bài học chung của đợt này: **script verification cũng có bug, và bug của chúng thường là fail-open.** Trong một buổi đã tìm ra sáu cái, trong đó bốn cái khiến gate báo xanh mà không kiểm tra gì:

- `topology.sh` đọc sai duration ghép nên báo lỗi giả;
- `migrations.sh` gọi `rg` không tồn tại nên check auto-migration im lặng pass;
- `database_baseline.sh` exclude nhầm một bảng khỏi check DML và có một check không bao giờ chạy tới;
- `go.sh` chết ở bước coverage trước khi tới golangci-lint, nên linter **chưa từng chạy** — và coverage total tính cả generated code nên floor đặt theo nó không bao giờ bắt được regression.

Khi một gate xanh, hãy thử phá nó một lần để chắc nó còn cắn. Ba guard mới trong đợt này đều đã được fault-inject để chứng minh điều đó.

**Cạm bẫy thứ hai: đừng để harness đi vòng qua lỗi production.** Một vòng làm E2E gặp `POST /contests/{id}/publish` trả 500 và đã tự thêm fallback ghi thẳng SQL qua `docker exec psql` để seeding chạy tiếp. Nếu để nguyên, suite E2E sẽ xanh trong khi endpoint publish vẫn hỏng ngoài production — test lúc đó chứng minh cái fallback chạy được, không chứng minh sản phẩm chạy được. Fallback đã bị gỡ; publish fail giờ là `fail` cứng. Chính cái 500 đó là defect 4 ở mục trên.

## 8. Tài liệu thiết kế và kế hoạch gốc

- `docs/superpowers/specs/2026-07-16-judgify-production-master-design.md`
- `docs/superpowers/plans/2026-07-18-judgify-production-execution-plan.md`
- `docs/superpowers/plans/2026-07-18-go-common-production-library-implementation-plan.md`
- `docs/superpowers/plans/2026-07-18-identity-access-implementation-plan.md`
- `docs/superpowers/plans/2026-07-18-problem-submission-judge-implementation-plan.md`
- `docs/superpowers/plans/2026-07-18-realtime-contest-notifications-implementation-plan.md`
- `docs/superpowers/plans/2026-07-18-cultivation-material-admin-implementation-plan.md`
- `docs/superpowers/plans/2026-07-18-frontend-product-implementation-plan.md`
- `docs/superpowers/plans/2026-07-18-production-operations-release-implementation-plan.md`
- `.superpowers/sdd/2026-07-18-judgify-production-execution-plan/progress.md`

---

Tài liệu này là snapshot trung thực của worktree, không phải release note. Mỗi lần một gate lớn chuyển trạng thái, cần cập nhật cả mục “Bằng chứng” và “Những gì còn thiếu”, không chỉ tick checklist.
