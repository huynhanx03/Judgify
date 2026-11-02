# Backend Local Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking. Chỉ bắt đầu sau khi người dùng đổi model implementation và yêu cầu tiếp tục triển khai.

**Goal:** API và migrator đọc đúng các biến cấu hình local đã được bootstrap vào `.env`.

**Architecture:** Giữ loader Viper và helper `bindEnvironmentValue` hiện có. Khai báo tường minh các alias không phải secret còn thiếu; giữ process env > dotenv > YAML và giữ riêng cơ chế secret production. Đây là một thay đổi bounded ở configuration, không đổi API hay database schema.

**Tech Stack:** Go 1.27.1, Viper 1.21.0, PostgreSQL 16.

**Spec:** [Audit/design, F01](/Users/jerry/nhanhmc/coding/Judgify/docs/superpowers/specs/2026-09-09-backend-audit-and-upgrade-design.md).

**Plan validation:** Test code bên dưới đã compile và fail đúng ở dotenv DB/OAuth trên bản sao source trong module tạm ngày 2026-09-09. Chưa thêm test vào production repository và chưa áp dụng fix.

## Global Constraints

- Giữ Go 1.27.1.
- Không stage/commit/reset/clean working tree.
- Không in password/token/client secret ra log.
- Không nới kiểm tra secret production hoặc tự chuyển production sang dotenv.
- Không chạy smoke trên database đang có ở cổng 5432.
- Không gộp sửa session, rating, counters hoặc sandbox vào task này.

## Task 1: Chứng minh và sửa dotenv binding qua public Load

**Files:**

- Modify: `api/internal/infrastructure/configuration/config.go` — thêm binding trước Unmarshal.
- Create: `api/internal/infrastructure/configuration/config_load_test.go` — kiểm tra Load với YAML và dotenv thật trong thư mục tạm.

**Interfaces:**

- Consumes: `Load(context.Context, Profile, SecretSource) (settings.Config, error)`.
- Reuses: `bindEnvironmentValue(*viper.Viper, string, string) error`.
- Produces: cùng public interface, đúng precedence cho `database.host/port/username/database`, `google.client_id`, `github.client_id`.

- [ ] **Step 1: Thêm regression test vào file mới.**

Test phải gọi Load thật, không chỉ test helper. Dùng nội dung dưới đây làm bộ test ban đầu:

```go
package configuration

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

const localEnvironmentFixture = `DATABASE_HOST=127.0.0.1
DATABASE_PORT=55199
DATABASE_USERNAME=audit_user
DATABASE_DATABASE=audit_database
DATABASE_PASSWORD=local-diagnostic-password-123456789
JWT_SECRET=local-diagnostic-jwt-secret-123456789
GOOGLE_CLIENT_ID=audit-google-client-id
GOOGLE_CLIENT_SECRET=audit-google-client-secret
GITHUB_CLIENT_ID=audit-github-client-id
GITHUB_CLIENT_SECRET=audit-github-client-secret
RESEND_API_KEY=local-diagnostic-resend-key
RESEND_FROM_EMAIL=security@localhost.invalid
RESEND_FROM_NAME=Judgify
RESEND_RESET_URL=http://localhost:3000/reset-password
RESEND_VERIFICATION_URL=http://localhost:3000/verify-recovery-contact
RESEND_INVITATION_URL=http://localhost:3000/accept-invitation
`

func prepareLocalConfigFiles(t *testing.T) {
    t.Helper()
    raw, err := os.ReadFile("../../../config/local.yaml")
    if err != nil { t.Fatal(err) }
    root := t.TempDir()
    if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(root, "config/local.yaml"), raw, 0600); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(root, ".env"), []byte(localEnvironmentFixture), 0600); err != nil { t.Fatal(err) }
    for _, key := range []string{
        "DATABASE_HOST", "DATABASE_PORT", "DATABASE_USERNAME", "DATABASE_DATABASE",
        "DATABASE_PASSWORD", "DATABASE_PASSWORD_FILE", "JWT_SECRET", "JWT_SECRET_FILE",
        "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_CLIENT_SECRET_FILE",
        "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_CLIENT_SECRET_FILE",
        "RESEND_API_KEY", "RESEND_API_KEY_FILE", "RESEND_FROM_EMAIL", "RESEND_FROM_NAME",
        "RESEND_RESET_URL", "RESEND_VERIFICATION_URL", "RESEND_INVITATION_URL",
        "JUDGIFY_VERSION",
    } { t.Setenv(key, "") }
    t.Setenv("GO_ENV", "local")
    t.Chdir(root)
}

func TestLoadLocalDatabaseOverridesFromDotEnv(t *testing.T) {
    prepareLocalConfigFiles(t)
    got, err := Load(context.Background(), ProfileMigrate, OSFileSecretSource{})
    if err != nil { t.Fatal(err) }
    if got.Database.Host != "127.0.0.1" || got.Database.Port != 55199 ||
        got.Database.Username != "audit_user" || got.Database.Database != "audit_database" {
        t.Fatal("database aliases from dotenv were not applied")
    }
}

func TestLoadLocalAPIWithDotEnvOAuthIDs(t *testing.T) {
    prepareLocalConfigFiles(t)
    got, err := Load(context.Background(), ProfileAPI, OSFileSecretSource{})
    if err != nil { t.Fatal(err) }
    if got.Google.ClientID != "audit-google-client-id" || got.GitHub.ClientID != "audit-github-client-id" {
        t.Fatal("OAuth client IDs from dotenv were not applied")
    }
}

func TestLoadProcessEnvironmentOverridesDotEnv(t *testing.T) {
    prepareLocalConfigFiles(t)
    t.Setenv("DATABASE_PORT", "55200")
    t.Setenv("GOOGLE_CLIENT_ID", "process-google-client-id")
    got, err := Load(context.Background(), ProfileAPI, OSFileSecretSource{})
    if err != nil { t.Fatal(err) }
    if got.Database.Port != 55200 || got.Google.ClientID != "process-google-client-id" {
        t.Fatal("process environment must take precedence over dotenv")
    }
}
```

- [ ] **Step 2: Chạy test và xác nhận lỗi đúng nguyên nhân.**

Từ `/Users/jerry/nhanhmc/coding/Judgify/api`:

```sh
go test ./internal/infrastructure/configuration -run 'TestLoad(Local|Process)' -count=1
```

Trước fix: test DB không đọc đúng override; API gặp `bootstrap: invalid configuration: google`. Không chấp nhận lỗi compile làm bằng chứng RED.

- [ ] **Step 3: Thêm sáu binding vào danh sách hiện có trong Load.**

```go
{"database.host", "DATABASE_HOST"},
{"database.port", "DATABASE_PORT"},
{"database.username", "DATABASE_USERNAME"},
{"database.database", "DATABASE_DATABASE"},
{"google.client_id", "GOOGLE_CLIENT_ID"},
{"github.client_id", "GITHUB_CLIENT_ID"},
```

Dùng vòng lặp `bindEnvironmentValue` đã có. Không gọi `os.Setenv` từ loader; không mở rộng secretBindings sang các trường không phải secret; không thay `validateAPIConfig` để chấp nhận placeholder.

- [ ] **Step 4: Kiểm tra focused và regression.**

```sh
gofmt -w internal/infrastructure/configuration/config.go internal/infrastructure/configuration/config_load_test.go
go test ./internal/infrastructure/configuration ./internal/infrastructure/bootstrap -count=1
go test ./... -count=1
```

Kỳ vọng mọi test mới và suite hiện có qua. Kiểm tra riêng `git diff --check` cho hai file. Những chẩn đoán copy lock còn có sẵn thuộc plan cache; ghi nhận đúng, không sửa lẫn vào đây.

- [ ] **Step 5: Review và ghi kết quả.**

Xác nhận chỉ six-field binding thay đổi production behavior. Ghi command, exit code và giới hạn smoke vào audit/design. Không commit khi chưa có yêu cầu.

## Checkpoint vận hành sau task

Task này sửa loader, chưa tự chuyển port hoặc bootstrap lại môi trường của người dùng. Bước smoke tiếp theo dùng cấu hình/secret/DB disposable riêng, rồi kiểm tra `cmd/migrate` và API readiness. Nếu cần đổi bootstrap script để đồng bộ Compose/API, tách thành task kế tiếp có contract rõ: giữ file hiện có, kiểm tra mismatch, chọn cùng port/user/database cho hai file, không tự ghi đè credentials.

Hoàn tất task khi regression của Load qua và các security validation cũ còn giữ nguyên. Sau đó chuyển plan cache hoặc checkpoint smoke theo thứ tự người dùng đã thống nhất.
