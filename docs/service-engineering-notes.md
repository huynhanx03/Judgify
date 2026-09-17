# Ghi chú: tư duy xây một service tốt cho Judgify

Ghi chú này tổng hợp các pattern đáng học từ `XProject/myid-main`, một identity service Go, và cách áp dụng có chọn lọc vào Judgify. Mục tiêu không phải làm kiến trúc phức tạp hơn; mục tiêu là giữ code dễ thay đổi, an toàn khi chấm bài và dễ vận hành.

## Nguyên tắc cốt lõi

> Một application function nhận input rõ ràng, gọi hạ tầng qua interface, kiểm tra domain rule, thay đổi state atomically, rồi trả result hoặc lỗi có ý nghĩa.

Một flow nên có dạng:

```text
HTTP/gRPC handler
  -> application service / use case
    -> domain validation and state transition
      -> repository, queue, sandbox, realtime adapters
```

Handler không nên chứa rule chấm bài; domain/application service không nên gọi Docker, Redis hoặc SQL trực tiếp nếu có thể đặt chúng sau interface.

## 1. Lập trình theo interface và dependency injection

XProject khai báo các dependency dưới dạng interface như `Token`, `Tenant`, `Forwarder`, `Streamer`, sau đó khởi tạo implementation bằng constructor `New(...)`.

Điều này làm dependency của một service hiện ngay trong constructor; test có thể thay implementation thật bằng fake/mock; và sau này đổi Redis, queue hoặc sandbox mà không lan thay đổi vào business logic.

Áp dụng cho Judgify:

```go
type SandboxRunner interface {
    Run(ctx context.Context, plan JudgePlan) (JudgeResult, error)
}

type SubmissionRepository interface {
    CreateAndQueue(ctx context.Context, command CreateSubmissionCommand) (Submission, error)
    ClaimNext(ctx context.Context, workerID string) (Job, error)
    Finalize(ctx context.Context, result JudgeResult) error
}

type ResultPublisher interface {
    PublishJudged(ctx context.Context, event SubmissionJudged) error
}
```

`SubmissionService` và `JudgingService` chỉ phụ thuộc các interface này. Docker sandbox, Ent/Postgres, Redis và WebSocket là adapter ở rìa hệ thống.

Không cần tạo interface cho mọi struct. Chỉ tạo interface ở ranh giới có khả năng thay thế, cần test độc lập, hoặc là I/O: database, queue, file/object storage, HTTP/gRPC client, sandbox và clock/ID generator khi cần test deterministic.

## 2. Tách transport khỏi application logic

Ở XProject, lớp gRPC mỏng: nhận request, gọi manager/service, map lỗi sang response. Business rule nằm trong `Manager`.

Trong Judgify, nên duy trì ranh giới sau:

```text
POST /submissions
  -> parse + auth + request validation
  -> SubmissionService.Submit(...)
  -> DB transaction: submission + job + outbox event
  -> HTTP response

Judge worker
  -> claim job
  -> JudgingService.Execute(...)
  -> SandboxRunner.Run(...)
  -> SubmissionFinalizer.Finalize(...)
```

Không để HTTP handler tự tạo bản ghi job, gọi Redis, chạy sandbox hoặc tự sửa `status`. Worker cũng không được tự cập nhật submission bằng SQL/ORM bỏ qua domain transition.

## 3. Transaction bao quanh một business action

XProject bao các thay đổi liên quan vào `WithTransaction`: đọc record với lock, kiểm tra rule, ghi thay đổi và audit log, sau đó commit hoặc rollback cùng nhau.

Đối với Judgify, mỗi transaction nên tương ứng một hành động nghiệp vụ hoàn chỉnh, ví dụ:

- Submit: tạo submission, reservation contest (nếu có), judge job và outbox event.
- Claim: claim đúng một job, gắn worker/lease và tăng attempt.
- Finalize: lưu attempt result, immutable judgement, submission state, solve projection và outbox event.
- Rejudge: tạo generation mới và queue job mới, không sửa đè kết quả immutable cũ.

Không mở transaction xuyên qua thời gian chạy sandbox. Claim/lease trước, chạy code bên ngoài transaction, rồi finalize trong transaction ngắn. Điều đó giữ lock ngắn và tránh treo database khi code người dùng chạy lâu.

## 4. State machine là chủ sở hữu của rule chuyển trạng thái

XProject chặn các thao tác vô lý, như lock một user đã lock hoặc đã banned. Judgify cần làm tương tự nhưng chặt hơn cho submission/job:

```text
submission: queued -> judging -> finalizing -> finished
                         |             |
                         v             v
                      retryable       system_error

job: queued -> claimed -> running -> completed
                   |          |
                   v          v
                expired    retryable/failed
```

Rule transition phải ở domain/application layer và được kiểm tra khi persist. Không để endpoint, cron job hay worker tùy ý `UPDATE status = ...`.

Mỗi job/result cần liên kết với `submission_id`, `generation`, `attempt_id` và worker lease. Khi worker cũ trả kết quả muộn, finalizer phải từ chối nếu generation hoặc lease không còn hợp lệ.

## 5. Validation theo tầng

XProject validate config khi boot, validate request contract, rồi validate rule nghiệp vụ trong manager.

Judgify nên giữ bốn lớp:

1. **Startup/config**: không boot nếu thiếu secret, runtime profile sai, queue URL sai hoặc limit không hợp lệ.
2. **Request shape**: source code rỗng/quá lớn, language không hỗ trợ, UUID/header sai.
3. **Domain rule**: contest đóng, user vượt quota, problem chưa publish, rejudge không được phép, state transition không hợp lệ.
4. **Infrastructure**: lỗi database/Redis/sandbox/object storage; phân loại retryable hay terminal.

Đừng dùng lỗi database raw như lỗi API. Map chúng thành lỗi application rõ nghĩa, còn log giữ nguyên root cause.

## 6. Cache và queue không phải source of truth

XProject dùng local LRU -> Redis -> service nguồn. Tư duy đúng là cache được phép miss, hết hạn hoặc mất hoàn toàn.

Trong Judgify:

- Postgres là nguồn dữ liệu chuẩn cho submission, job, judgement, problem revision và contest state.
- Redis/queue dùng để đánh thức worker, rate limit, lease hoặc cache; không là nơi duy nhất chứa kết quả chấm.
- Queue delivery phải chấp nhận **at-least-once**: job có thể đến hai lần. Claim/finalize phải idempotent.
- Khi queue publish lỗi sau DB commit, transactional outbox phải phát lại event.

## 7. Observability phải đi xuyên suốt

XProject có logger, metrics, tracing và gRPC health check. Với hệ thống chấm bài, log text đơn thuần thường không đủ.

Mọi log/metric/trace trong flow chấm nên có các field:

```text
request_id, user_id, contest_id, problem_id,
submission_id, generation, job_id, attempt_id,
worker_id, language, verdict, duration_ms
```

Metrics tối thiểu:

- queue depth và tuổi job cũ nhất;
- thời gian queue, compile, run và finalize;
- số job claim/finalize/retry/lease-expired;
- verdict theo language/problem;
- sandbox failure theo image/runtime;
- số submission active theo user.

Expose `healthz` (process còn sống) và `readyz` (DB/queue dependency sẵn sàng) riêng. Worker cần graceful shutdown: dừng claim job mới, hoàn thành hoặc nhả lease job đang chạy.

## 8. Context, identity và correlation ID

XProject chuẩn hóa các header như user ID, tenant ID, role ID và forward chúng xuống service khác.

Với Judgify, không được tin những header nội bộ do public client gửi lên. API gateway/auth middleware phải xác thực token và tự gắn identity vào `context.Context`. Khi gọi internal service hoặc publish event, chỉ forward allowlist metadata.

Nên luôn tạo/nhận `request_id` ở edge, ghi nó vào submission/job/outbox payload, rồi tạo trace span cho worker. Khi có report “bài em pending”, chỉ cần một ID là lần được từ API sang queue, worker, sandbox và finalize.

## 9. Audit log cho hành động quản trị

XProject có status log cho các hành động lock/ban. Judgify nên audit append-only cho:

- tạo/sửa/xóa/publish problem và testset revision;
- thay đổi runtime/image/checker;
- rejudge, cancel, retry hoặc override submission;
- mở/đóng contest và đổi scoreboard setting;
- đổi role/quyền admin;
- thay đổi config giới hạn chấm bài.

Mỗi audit record cần actor, action, target, timestamp, request ID và JSON snapshot tối thiểu trước/sau. Audit log không nên ghi source code, token, password hoặc test case bí mật.

## 10. Error taxonomy và retry

Phân biệt lỗi giúp worker không retry vô ích:

| Nhóm | Ví dụ | Xử lý |
| --- | --- | --- |
| User/domain | source quá lớn, contest đóng | trả lỗi ngay, không retry |
| Judge verdict | WA, TLE, CE, RE | kết quả hợp lệ, finalize |
| Transient infrastructure | Redis timeout, network, DB unavailable | retry có backoff |
| Sandbox/platform | image không tải được, cgroup lỗi | retry giới hạn, cảnh báo |
| Integrity/security | checksum sai, job không hợp lệ, lease mất | không tin kết quả; fail/triage |

Retry phải có maximum attempts, exponential backoff, dead-letter/triage state và alert. Không viết vòng `for { retry }`.

## 11. Security boundary của judge worker

Đây là phần Judgify phải nghiêm hơn identity service:

- API không nhận Docker command từ client; chỉ gửi judge plan đã validate.
- Sandbox không có Docker socket, không network mặc định, không privileged mode.
- Bắt buộc giới hạn CPU, memory, process count, output size, file size, thời gian và disk/tmpfs.
- Image/runtime/checker được allowlist, pin digest và có provenance/checksum.
- Source, output và log được giới hạn kích thước trước khi persist/publish.
- Worker identity dùng token/capability ngắn hạn và chỉ có quyền claim/finalize phạm vi cần thiết.

## Những phần không nên copy nguyên xi từ XProject

- **Kafka cho mọi event**: quá nặng cho một Oracle Always Free VM; Redis queue hoặc database-backed outbox/job của Judgify phù hợp hơn.
- **Safe ID tự sinh**: chỉ hữu ích cho mã người dùng dễ đọc; UUID nội bộ của Judgify phù hợp hơn.
- **Quá nhiều shared/internal package**: làm project khó tách, khó chạy local và dependency nặng.
- **Global mutable state**: constructor injection và explicit dependency tốt hơn.
- **Log token hoặc secret**: tuyệt đối tránh. Các log auth của hệ thống mới cần redact credential.

## Checklist khi viết một use case mới

- [ ] Input có DTO/command rõ và validate được shape.
- [ ] Handler/consumer chỉ làm transport; không chứa business rule.
- [ ] Dependency I/O được inject qua interface phù hợp.
- [ ] Domain kiểm soát trạng thái và invariant.
- [ ] Một transaction bao trùm tất cả thay đổi durable của action.
- [ ] Không chạy sandbox hoặc network call lâu trong DB transaction.
- [ ] Event external được ghi outbox cùng transaction.
- [ ] Consumer/worker idempotent và chịu được delivery lặp.
- [ ] Lỗi được phân loại retryable/terminal; retry có giới hạn.
- [ ] Log có correlation ID và entity ID liên quan.
- [ ] Test domain transition, conflict/lease expired, idempotency và rollback.
- [ ] Không đưa secret/source/testcase private vào log, event hoặc audit.

## Ưu tiên áp dụng cho Judgify

1. Chuẩn hóa correlation fields từ API đến worker/sandbox/finalizer.
2. Bổ sung audit log append-only cho các thao tác admin và rejudge.
3. Duy trì configuration cho quota/time/memory/concurrency/runtime; validate trước boot.
4. Giữ submit/finalize transactional + outbox; kiểm thử delivery lặp và worker chết giữa chừng.
5. Hoàn thiện health/readiness và graceful shutdown cho API/worker để deploy Oracle ổn định.
