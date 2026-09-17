# Judgify BE — audit và thiết kế nâng cấp đề xuất

Ngày kiểm tra: 2026-09-09. Đối tượng: working tree hiện tại tại `/Users/jerry/nhanhmc/coding/Judgify`, đặc biệt `api/`, cùng dependency sibling `/Users/jerry/nhanhmc/coding/go-common`.

**Trạng thái:** đã khảo sát, chạy kiểm tra và nâng Go theo yêu cầu riêng. Các thay đổi logic nghiệp vụ trong tài liệu này **chưa triển khai**. Người dùng yêu cầu báo trước để đổi model implementation.

## 1. Nhận xét tổng thể

BE có nền tảng đủ tốt để tiếp tục hoàn thiện trên kiến trúc hiện có: modular monolith, domain/contract/usecase/adapters, root ports, PostgreSQL làm nguồn dữ liệu chính, transactional outbox, API/worker/migrator tách vai trò, durable session và capability-bound sandbox. Kiểm tra kiến trúc nghiêm ngặt đã qua trong lần audit này.

Điểm yếu tập trung ở các đường nối giữa thành phần: cấu hình từ file sang runtime; session refresh với background touch; ledger mới với read model cũ; mutation tức thời với cron reconciliation; outbox replay với rating run đã thay đổi; worker restart với tài nguyên sandbox còn sống. Đây là các lỗi mà nhiều unit test riêng lẻ vẫn có thể cho kết quả xanh.

Không có bằng chứng từ audit này để biện minh cho một đợt viết lại kiến trúc. Cần ưu tiên đóng các lỗi tính đúng đắn và khả năng vận hành trước khi tối ưu tải hoặc thêm tính năng.

Phạm vi khảo sát gồm cấu trúc, dependency, bootstrap/configuration, transport, identity, problem/submission/judge, contest/rating, cultivation/ranking, realtime, migrations và verification tooling. Đã kiểm kê 1.167 file Go do dự án viết, trong đó 304 file test; con số này không có nghĩa mọi dòng đã được review hoặc mọi nhánh đã được chạy. Một số kết luận dưới đây dựa trên trace source, chưa có regression test tái hiện riêng.

## 2. Thay đổi đã thực hiện theo yêu cầu “lên 1.27.1”

- `api/go.mod`: Go `1.27.1`; chạy `go mod tidy` để chuẩn hóa nhóm require.
- `docker/sandbox/inspector-go.mod`: Go `1.27.1`.
- Hai Docker builder: `golang:1.27.1-alpine3.23@sha256:d9e2f2f07b10cc922da3e80e035c3058810b328d5aef82d2c63680967c5e2ec9`.
- CI, integration workflow, release-evidence workflow: Go `1.27.1`.
- Công cụ CI: golangci-lint `v2.13.2`, govulncheck `v1.7.0`.
- README và CONTRIBUTING: prerequisite Go `1.27.1`.

Tổng cộng 9 file có sẵn được chỉnh. Các cặp tên module/version trong `api/go.mod` không đổi; `api/go.sum` và hai manifest của `go-common` không đổi. Không stage/commit.

Digest Docker được đọc từ registry bằng `docker buildx imagetools inspect golang:1.27.1-alpine3.23`. Golangci-lint hỗ trợ Go 1.27 từ v2.13.0; bản v2.13.2 được chọn đã được build bằng Go 1.27.1 và chạy kiểm tra thực tế. [Changelog chính thức](https://golangci-lint.run/docs/product/changelog/#v2130).

Các binary linter/scanner cài sẵn trên máy được build bằng Go 1.26 nên bị lỗi khi đọc Go 1.27. Audit build các bản tương thích vào thư mục tạm riêng, không ghi đè công cụ toàn máy. Các gate gọi binary toàn máy vẫn cần dùng bản được build lại hoặc thêm thư mục công cụ tương thích vào PATH.

## 3. Bằng chứng kiểm tra mới

| Kiểm tra | Kết quả | Giới hạn diễn giải |
|---|---|---|
| `go version` trong API | `go1.27.1 darwin/arm64` | Toolchain của module đã hoạt động |
| `go mod tidy -diff` sau tidy | Qua | Manifest không còn drift do tidy |
| `go mod verify` | Qua | Xác minh module cache, không thay thế vulnerability scan |
| `go build ./cmd/...` | Qua | Build các command trên macOS/arm64 |
| `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/...` | Qua | Cross-build command cho Linux/arm64; chưa build toàn bộ Docker images |
| `go test ./... -count=1` | Qua, 87 package có test | Không phải chứng minh toàn bộ E2E hoặc mọi race |
| `JUDGIFY_ARCH_STRICT=1 go test ./internal/architecture -count=1` | Qua | Ranh giới import/runtime theo guard hiện có |
| `go vet ./...` | Không qua, 64 chẩn đoán copy lock | Tập trung ở cache adapter và nơi truyền chúng bằng giá trị |
| golangci-lint v2.13.2, build bằng Go 1.27.1 | Không qua, 81 issues: 50 govet, 31 staticcheck | Phần lớn staticcheck là import trùng sau refactor; không tắt rule để làm xanh |
| govulncheck v1.7.0, build bằng Go 1.27.1 | Qua ở mức symbol: 0 vulnerability được xác định là code gọi tới | Có 1 vulnerability ở imported packages và 6 ở required modules mà scanner không thấy đường gọi; không đồng nghĩa dependency hoàn toàn sạch |
| Integration runner trên PostgreSQL disposable | 86 package qua, 1 package lỗi | 7 subtest append-only đòi SQLSTATE khác trigger hiện tại |
| `scripts/verify/workflows_test.sh` | Qua sau nâng version | Kiểm tra wiring/permissions/pinning theo script |
| topology verifier self-test | Qua | Không phải worker sandbox E2E |
| baseline/generated verifier self-tests | Qua | Không chạy generator trên working tree để sửa generated code |
| Probe configuration bằng `configuration.Load` thật | Tái hiện lỗi `.env` | Chỉ load config; không kết nối database hay gọi OAuth |
| Probe 2 lượt cache miss đồng thời cùng key | Backend loader chạy 2 lần, kỳ vọng 1 | Dùng adapter Tag và Ristretto thật |
| Test mẫu của hai implementation plan | Đã compile và fail đúng lỗi config/singleflight trên bản sao source hiện tại | Chạy trong module chẩn đoán tạm; chưa thêm test hoặc fix nghiệp vụ vào repo |

Integration chạy với project riêng `judgify-be-audit-t95bzfvy`, port riêng 60369; runner đã xóa container/network do nó tạo sau khi hoàn tất. Không sử dụng database đang có trên cổng 5432.

Các log chẩn đoán của phiên nằm tại `/var/folders/9n/38stcwsn7y5c0w4h2yp9kh2w0000gn/T/judgify-be-audit-t95bzfvy`. Đây là thư mục tạm, không phải kho evidence release bền vững. Các kết quả quan trọng đã được ghi trong tài liệu này.

## 4. Các phát hiện cần xử lý

P1: nên xử lý trước khi coi BE ổn định để vận hành. P2: lỗi đúng đắn hoặc kiểm chứng cần xử lý trong cùng lộ trình, có thể xếp sau các luồng bị chặn. “Trace source” xác nhận cấu trúc lỗi trong code, nhưng không được trình bày như một sự cố đã tái hiện end-to-end.

### F01 — P1: `.env` không nạp đúng các trường không phải secret

**Bằng chứng:** [configuration/config.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/infrastructure/configuration/config.go:146), đặc biệt phần merge `.env` và danh sách binding ở dòng 176.

Viper merge `.env` thành key phẳng như `database_port`, trong khi cấu hình decode đọc `database.port`. `AutomaticEnv` hỗ trợ biến môi trường của process, không tự chuyển key phẳng trong file sang cấu trúc lồng. Loader hiện bind riêng secret và một số trường Resend, nhưng bỏ sót thông tin DB và OAuth client ID.

Probe gọi `configuration.Load` thật với dữ liệu giả đã cho kết quả:

```text
dotenv-only migrate: port=5432 username=judgify_dev database=judgify error=<nil>
dotenv-only api: error=bootstrap: invalid configuration: google
exported overrides api: port=55199 error=<nil>
```

File probe đặt `DATABASE_PORT=55199`, username/database khác mặc định và client ID hợp lệ. Chỉ khi export biến process thì config đúng. Vì README dùng `make dev-bootstrap` tạo `api/.env`, đây là lỗi ở đường khởi động được tài liệu hướng dẫn.

**Đề xuất:** bind các alias được hỗ trợ bằng helper hiện có; thứ tự ưu tiên process env > `.env` > YAML. Giữ nguyên kiểm tra secret production và lỗi startup chỉ nêu tên field. Kiểm tra thêm tính nhất quán port/user/database giữa Compose và API; máy audit đang có dịch vụ khác chiếm 5432.

**Nghiệm thu:** Load API chỉ với file local hợp lệ thành công; DB/OAuth đọc đúng; biến process ghi đè được; thiếu secret thật vẫn bị từ chối; smoke API dùng DB disposable đạt readiness.

### F02 — P1: cache adapter copy `singleflight.Group`

**Bằng chứng:** [problem_tag.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/cache/problem_tag.go:17), [cultivation_element.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/cache/cultivation_element.go:16), [cache registry](/Users/jerry/nhanhmc/coding/Judgify/api/internal/cache/cache.go:15).

Mười adapter chứa `loads singleflight.Group`, nhưng constructor và method receiver dùng value. Mỗi lời gọi copy group; các cache miss đồng thời không dùng cùng state để gộp lượt tải. `go vet` xác nhận copy lock. Probe với Tag/Ristretto thật cho `backend_loads=2` khi hai caller cùng key overlap, trong khi hợp đồng Fetch yêu cầu một lượt tải.

**Đề xuất:** constructor trả pointer; method dùng pointer receiver; Registry giữ pointer; assertions chứng minh pointer implement cache port. Services đã phụ thuộc narrow interface nên không cần đổi API hay thêm tầng cache mới.

**Nghiệm thu:** burst cùng key chỉ có một backend load; key khác không chặn nhau; invalidate-after-commit và rollback vẫn đúng; focused race test và `go vet ./...` qua sau khi xử lý toàn bộ adapter.

### F03 — P2: integration test lẫn kiểm tra quyền và trigger append-only

**Bằng chứng:** [revision_integration_test.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/adapters/driven/db/revision_integration_test.go:511) kỳ vọng `42501`; [append_only.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/ent/schemaextras/append_only.go:120) chủ động phát `55000`.

Đã chạy trên PostgreSQL 16 riêng: 7 subtest đều bị chặn mutation bằng `55000`, test lại đòi lỗi thiếu quyền `42501`. Không có bằng chứng mutation lọt qua. Chưa chứng minh riêng quyền production từ lần chạy dùng test role này.

**Đề xuất:** test privileged/owner phải xác nhận trigger trả `55000` và row không đổi. Test dùng role API/worker phải kiểm tra quyền cấm UPDATE/DELETE và kỳ vọng `42501` nếu đó là contract của role. Không sửa trigger hoặc chấp nhận mọi lỗi để che lệch contract.

### F04 — P1: refresh hợp lệ có thể bị biến thành logout

**Bằng chứng:** [refresh_rotation.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driven/db/refresh_rotation.go:316), [session.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driven/db/session.go:165), [token_lifecycle.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/usecase/token_lifecycle.go:847), [session_transport.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driver/http/session_transport.go:223).

Trace: refresh đọc snapshot; background last-seen touch gia hạn `IdleExpiresAt`; refresh lấy lock và đòi expiry bằng đúng snapshot cũ. Khác biệt hợp lệ trả lỗi `live state changed`; service phân loại thành unauthorized; HTTP xóa session cookies. Hai thao tác của cùng người dùng đang hoạt động có thể làm họ bị đăng xuất.

**Đề xuất:** tách security fence bất biến/revocation khỏi sliding idle expiry. Kiểm tra trạng thái live dưới lock; không biến gia hạn idle hợp lệ thành credential invalid. Phân biệt storage conflict/transient error với lỗi xác thực; giữ nguyên chống replay, security version và absolute expiry.

**Nghiệm thu:** test barrier chèn TouchLastSeen giữa ContextByHash và Rotate vẫn refresh thành công; revoke/security-version change vẫn bị từ chối; lỗi DB tạm thời không xóa cookie. Chưa chạy riêng interleaving này trong audit.

### F05 — P1: người đang đăng nhập bị chặn xác minh recovery email

**Bằng chứng:** [recovery_contact.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driver/http/recovery_contact.go:208), [browser_session.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driver/http/browser_session.go:588), [pre-session bootstrap](/Users/jerry/nhanhmc/coding/Judgify/api/internal/identity/adapters/driver/http/browser_session.go:644), [frontend bootstrap](/Users/jerry/nhanhmc/coding/Judgify/website/lib/auth/pre-session.ts:52).

Verify handler chỉ nhận pre-session CSRF. Register/login phát session CSRF và xóa pre-session cookie. Frontend thấy CSRF đã có nên không bootstrap lại; kể cả bootstrap, API từ chối access/refresh cookie vì credential ambiguity. Người vừa đăng ký hoặc đổi email trong phiên đang đăng nhập có thể nhận 403 trước khi token email được kiểm tra.

**Đề xuất:** thiết kế rõ hai ngữ cảnh cho riêng lệnh verify email: anonymous pre-session, hoặc authenticated session với session CSRF hợp lệ. Token email quyết định contact đích; nếu session hiện tại thuộc tài khoản khác thì trả conflict rõ ràng, không tự đổi tài khoản. Không nới chính sách login/register hay bỏ Origin/CSRF.

**Nghiệm thu:** verify từ anonymous, vừa register, đang login và session khác tài khoản; token sai/hết hạn/replay; request khác origin. Kết luận hiện dựa trên trace HTTP/frontend, chưa chạy browser E2E.

### F06 — P2: trạng thái đã giải bài đọc bảng legacy

**Bằng chứng:** [GetSolvedProblemIDs](/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/adapters/driven/db/problem.go:215), [problem enrichment](/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/usecase/problem.go:43), [finalization solve projection](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/finalization_helpers.go:1656).

Read path dùng `user_solved_problems`; finalization mới ghi `user_problem_solves`. Không thấy production caller ghi `InsertUserSolved` hay trigger đồng bộ hai bảng. Vì vậy một AC mới có thể vẫn hiển thị `is_solved=false`; dữ liệu legacy cũng có thể giữ trạng thái cũ sau rejudge.

**Đề xuất:** đọc solve projection hiện hành, lọc đúng trạng thái solved; không thêm dual write để duy trì hai nguồn thật. Chưa xóa bảng cũ trước khi kiểm kê dữ liệu và migration.

**Nghiệm thu:** submit AC → list/detail báo solved; rejudge làm mất active solve → unsolved; có AC khác còn hiệu lực → vẫn solved; legacy row đơn lẻ không thắng canonical state.

### F07 — P1: cron statistics sai semantics và có thể mất cập nhật

**Bằng chứng:** [RecalculateStats](/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/adapters/driven/db/problem.go:232), [aggregate query](/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/adapters/driven/db/problem.go:275), [system-error finalization](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/finalization_helpers.go:1168), [accepted delta](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/finalization_helpers.go:465).

Có hai lỗi độc lập. Thứ nhất, cron đếm `status='accepted'` trong khi system-error của một lần rejudge giữ active verdict cũ nhưng đổi status thành `internal_error`. Cron có thể xóa accepted count đang hợp lệ. Một lần rejudge AC tiếp theo có delta AC→AC bằng 0 nên không tự sửa; AC→WA sau đó có thể chạm guard chống counter âm.

Thứ hai, aggregate SELECT và absolute UPDATE không cùng transaction/row lock. Writer có thể commit increment ở giữa rồi bị cron ghi đè. Incremental scan chỉ dựa `submissions.created_at`, không bảo đảm sửa lại thay đổi verdict của submission cũ.

**Đề xuất:** canonical counter semantics phải theo active verdict; reconciliation có transaction và lock problem trước khi lấy snapshot aggregate mới. Kiểm tra lock order cùng writer hiện có; dùng isolation phù hợp để snapshot sau lock thấy commit trước đó. Chọn dirty marker hoặc watermark phản ánh verdict updates, không chỉ created_at.

**Nghiệm thu:** AC → rejudge system error giữ count; AC→AC không đổi; AC→WA giảm đúng; transaction overlap không mất increment; rejudge submission cũ vẫn được reconciliation.

### F08 — P1: lock order đảo giữa Cancel và Heartbeat

**Bằng chứng:** [Cancel](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/operations.go:183), [submission lock](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/operations.go:445), [job lock](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/operations.go:477), [heartbeat job lock](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/job.go:637), [heartbeat submission lock](/Users/jerry/nhanhmc/coding/Judgify/api/internal/submission/adapters/driven/db/job.go:692).

Cancel giữ submission FOR UPDATE rồi đợi job FOR UPDATE. Heartbeat giữ job FOR UPDATE rồi đợi submission FOR SHARE. Khi hai transaction overlap theo thứ tự này, chúng tạo chu trình chờ. PostgreSQL sẽ hủy một transaction; thao tác hủy hoặc duy trì lease có thể thất bại.

**Đề xuất:** lập một lock order thống nhất cho claim/heartbeat/cancel/rejudge/finalize/reap. Sửa các đường trái thứ tự, giữ revalidation generation/lease dưới lock; không chỉ thêm retry để giấu deadlock.

**Nghiệm thu:** PostgreSQL test với hai connection và barrier ép đúng interleaving; cả hủy và heartbeat kết thúc theo contract, không `40P01`, không verdict stale được áp dụng. Audit xác nhận SQL lock order, chưa chạy test concurrency riêng.

### F09 — P2: contest rating không đồng bộ public ranking projection

**Bằng chứng:** [rating settlement](/Users/jerry/nhanhmc/coding/Judgify/api/internal/contest/adapters/driven/db/rating_ledger.go:849), [rerating activation](/Users/jerry/nhanhmc/coding/Judgify/api/internal/contest/adapters/driven/db/rating_rerating.go:824), [public ranking query](/Users/jerry/nhanhmc/coding/Judgify/api/internal/cultivation/adapters/driven/db/ranking_projection.go:58).

Rating update mirror `user_stats.rating`, trong khi public ranking đọc `cultivation_ranking_projections.rating`. Không thấy đường settlement/rerating cập nhật projection này, rating version/timestamp hoặc ranking revision. Ranking có thể giữ điểm cũ đến khi một mutation khác hoặc operator reconciliation refresh projection.

**Đề xuất:** đồng bộ qua shared application boundary/root port trong transaction đã có; cập nhật rating, thời điểm, version và cursor revision cùng nhau. Giữ riêng canonical contest rating ledger và projection dành cho đọc, nhưng nối đầy đủ đường cập nhật.

**Nghiệm thu:** settle/rerate xong ranking thay đổi ngay theo contract; cursor cũ được invalidated đúng; privacy và tie-break timestamp không bị sai; retry không tăng revision thừa.

### F10 — P2: replay rating event lấy nhầm run mới

**Bằng chứng:** [event_delivery_rating.go](/Users/jerry/nhanhmc/coding/Judgify/api/internal/infrastructure/event_delivery_rating.go:46), [GetContestRatingChanges](/Users/jerry/nhanhmc/coding/Judgify/api/internal/contest/usecase/rating.go:201).

Event mang `RatingRunID` bất biến, nhưng consumer gọi API đọc active run theo ContestID. Nếu rerating đã xảy ra trước khi event cũ được xử lý/retry, thông báo có thể chứa số liệu run mới gắn với event cũ. Nếu số participant khác, consumer phân loại cardinality mismatch thành lỗi vĩnh viễn.

**Đề xuất:** consumer đọc immutable run bằng event.RatingRunID, xác minh ContestID/sequence/version/participant count cùng run đó. HTTP endpoint đọc active run hiện tại có thể giữ contract cũ.

**Nghiệm thu:** trì hoãn event A, tạo rerating B, replay A vẫn ra số liệu A; retry sau partial delivery không trộn dữ liệu hoặc tạo notification trùng.

### F11 — P1 trước production: thiếu bằng chứng và đường phục hồi sandbox sau worker crash

**Bằng chứng source:** [Docker AutoRemove](/Users/jerry/nhanhmc/coding/Judgify/api/internal/judge/supervisor/docker.go:134), [Engine contract](/Users/jerry/nhanhmc/coding/Judgify/api/internal/judge/supervisor/engine.go:70), [worker activation](/Users/jerry/nhanhmc/coding/Judgify/api/internal/infrastructure/worker/production_runtime.go:326), [staging initialization](/Users/jerry/nhanhmc/coding/Judgify/api/internal/judge/executor/staging.go:151).

Cleanup bình thường thuộc worker process. Docker daemon chạy riêng, container không auto-remove; Engine contract không có inventory/list để tìm boundary của boot cũ. Startup tạo boot mới và bật claims; stager khởi tạo reservations rỗng trên thư mục bền vững. Không thấy đường startup reconciliation cho container/artifact của boot cũ trong phạm vi đã quét.

Suy luận cần kiểm chứng: worker bị kill/OOM giữa lúc chạy có thể để lại process/container hoặc staging files ngoài accounting của boot mới. Container resource limits vẫn tồn tại; audit không khẳng định sandbox escape hoặc đã tái hiện tài nguyên chạy vô hạn trên production.

**Đề xuất:** startup giữ claims-off đến khi inventory/reconcile tài nguyên của chính Judgify hoàn tất; dùng ownership label và boot fence; xử lý cả cleanup failure khi worker còn sống; rebuild accounting hoặc xóa orphan theo policy. Có watchdog/deadline độc lập nếu host crash model yêu cầu. Không xóa container không thuộc hệ thống.

**Nghiệm thu:** fault injection kill worker khi compile/run/stage, restart, kiểm tra orphan count/disk/active processes, lease expiry và retry; host Linux rootless Docker + cgroup v2 đúng contract. Chưa thực hiện trong audit trên macOS.

## 5. Những giới hạn cần giữ đúng

- Chưa chạy worker judge E2E, kill/restart sandbox, load/soak/chaos hay browser E2E mới trong audit này.
- Unit và integration xanh không chứng minh các interleaving F04/F07/F08; phải có test điều khiển thứ tự transaction.
- Realtime nhiều API instance có dấu hiệu thiếu durable event tailer cho từng instance; topology hiện tại mặc định một API nên ghi nhận là giới hạn mở rộng/rollout cần thử, chưa coi là outage hiện tại.
- Sonic hiện phát cảnh báo fallback sang `encoding/json` trên Go 1.27 trong một số test/probe. Chưa đo ảnh hưởng throughput; không tự nâng thư viện runtime chỉ để mất cảnh báo.
- BACKEND_GUIDE còn mô tả `core/service`, per-domain ports/DI, global singletons và cache cũ; source/architecture spec mới dùng cấu trúc khác. PRODUCT_COMPLETION_STATUS chứa kết quả lịch sử mâu thuẫn giữa các phần. Cần cập nhật theo evidence mới, không dùng nhãn “đã xanh” cũ thay cho chạy test.
- Working tree trước audit có 2.206 entry thay đổi tại Judgify và 403 tại go-common. Đây là baseline người dùng; chỉ thay đổi Go/tooling nêu ở mục 2 được thực hiện trong phiên.

## 6. Ba hướng thực hiện

| Hướng | Giá trị | Đánh đổi |
|---|---|---|
| **A — sửa theo luồng trên kiến trúc hiện tại, đề xuất chọn** | Xử lý startup, session, data correctness và vận hành theo đợt có test | Cần integration/concurrency evidence, không chỉ sửa unit test |
| B — chỉ đưa local boot và các gate cơ bản về xanh | Đạt môi trường phát triển sử dụng được nhanh hơn | Các lỗi ledger/read model, session race, worker recovery còn nguyên |
| C — refactor/viết lại rộng trước | Có thể làm cấu trúc đồng nhất hơn | Rủi ro cao trên working tree đang refactor lớn; chưa có bằng chứng cần thiết |

Chọn A làm đề xuất để bàn giao. Mỗi đợt phải có kết quả sử dụng được và có thể review riêng; không gom các miền nghiệp vụ thành một patch lớn.

## 7. Thứ tự triển khai và tiêu chí kết thúc

| Đợt | Phạm vi | Bằng chứng bắt buộc trước khi chuyển đợt |
|---|---|---|
| 0 — đã làm | Go 1.27.1, builder, CI/tooling | Module verify/tidy, command build, unit suite; ghi rõ lint/integration còn lỗi |
| 1A | F01 config; khớp bootstrap/Compose/API và tài liệu khởi động | Config precedence tests; migrate và API readiness trên DB mới, port riêng |
| 1B | F02 cache; cleanup import/name mà linter đã chỉ ra | Concurrent cache miss regression; vet; linter tương thích; unit suite |
| 1C | F03 integration role/trigger contract | Owner trigger test + API/worker permission test riêng; integration suite qua |
| 2 | F04/F05 session và email verification | HTTP/DB regression, multi-tab refresh, signed-in/anonymous verify, Origin/CSRF/replay tests |
| 3 | F06/F07/F08 submission và thống kê | AC/rejudge/system-error matrix; canonical solved projection; deterministic PostgreSQL concurrency tests |
| 4 | F09/F10 rating và read model/event replay | Settle/rerate/ranking atomicity; cursor privacy; delayed/partial event replay |
| 5 | F11 và evidence production | Rootless Linux worker E2E; crash cleanup; queue recovery; bounded load/soak; backup/restore/rollout evidence |

Các đợt 2–5 cần thiết kế chi tiết riêng trước khi sửa vì chạm security fence, lock protocol và durable contract. Roadmap ở đây xác định rõ hành vi cần đạt; không thay thế review thiết kế từng subsystem.

Kế hoạch chi tiết để bắt đầu sau khi người dùng đổi model:

- [Config local](/Users/jerry/nhanhmc/coding/Judgify/docs/superpowers/plans/2026-09-09-backend-local-configuration.md).
- [Cache ownership](/Users/jerry/nhanhmc/coding/Judgify/docs/superpowers/plans/2026-09-09-backend-cache-singleflight.md).

## 8. Quy tắc bàn giao

Giữ Go 1.27.1 và kiến trúc single-tenant hiện có. Không thay API JSON/routes để chữa wiring nội bộ. Không nới CSRF, recent-auth, generation/lease fence, append-only hoặc rootless/cgroup gate. Không sửa tay Ent/mocks generated. Database test phải disposable rõ ràng. Không stage/commit/reset/clean working tree nếu chưa có yêu cầu.

Người dùng sẽ đổi model implementation trước khi bắt đầu các đợt sửa logic. Điểm tiếp tục là đợt 1A/1B; model tiếp theo đọc tài liệu này và plan liên quan, xác nhận hiện trạng file, tái hiện failing test rồi mới sửa.
