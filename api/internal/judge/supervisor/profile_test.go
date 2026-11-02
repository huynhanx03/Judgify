package supervisor

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeExecuteRequestBuildsClosedDeterministicProfile(t *testing.T) {
	t.Parallel()

	request := validExecuteRequest()
	request.Env = []string{"GOPROXY=off", "GOFLAGS=-mod=vendor"}
	normalized, err := normalizeExecuteRequest(request)
	if err != nil {
		t.Fatalf("normalizeExecuteRequest() error = %v", err)
	}
	wantPrefix := []string{
		"HOME=/tmp",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TZ=UTC",
	}
	for index, value := range wantPrefix {
		if normalized.Env[index] != value {
			t.Fatalf("Env[%d] = %q, want %q; env=%v", index, normalized.Env[index], value, normalized.Env)
		}
	}
	if normalized.Command[0] != sandboxInitPath ||
		normalized.Command[1] != "--" ||
		normalized.Command[2] != request.Argv[0] {
		t.Fatalf("Command = %v, missing trusted bootstrap/direct argv", normalized.Command)
	}
	if normalized.User != sandboxUser ||
		normalized.NetworkMode != "none" ||
		!normalized.ReadOnlyRoot ||
		normalized.Privileged ||
		len(normalized.Capabilities) != 1 ||
		normalized.Capabilities[0] != "ALL" {
		t.Fatalf("normalized sandbox policy is not closed: %+v", normalized)
	}
}

func TestNormalizeExecuteRequestRejectsRawShellAndUntrustedShape(t *testing.T) {
	t.Parallel()

	for _, mutate := range []func(*ExecuteRequest){
		func(request *ExecuteRequest) { request.Identity.JobID = uuid.Nil },
		func(request *ExecuteRequest) { request.Boundary = "unknown" },
		func(request *ExecuteRequest) { request.ImageDigest = "runtime:latest" },
		func(request *ExecuteRequest) { request.Argv = []string{"/bin/sh", "-c", "id"} },
		func(request *ExecuteRequest) { request.Argv = []string{"python3", "main.py"} },
		func(request *ExecuteRequest) { request.Argv = []string{"/usr/bin/python3", "bad\nargument"} },
		func(request *ExecuteRequest) { request.WorkingDir = "/workspace/../etc" },
		func(request *ExecuteRequest) { request.Env = []string{"LD_PRELOAD=/tmp/hook.so"} },
		func(request *ExecuteRequest) {
			request.Env = []string{"GOPROXY=off", "GOPROXY=https://example.invalid"}
		},
		func(request *ExecuteRequest) { request.StdinBytes = request.Limits.FileBytes + 1 },
		func(request *ExecuteRequest) { request.InputFiles[0].Path = "../escape" },
		func(request *ExecuteRequest) { request.InputFiles[0].Path = "/workspace/source/main.go" },
		func(request *ExecuteRequest) { request.InputFiles[0].Mode = 0o777 },
		func(request *ExecuteRequest) { request.InputFiles[0].Size = request.Limits.FileBytes + 1 },
		func(request *ExecuteRequest) { request.Limits.MemoryBytes = 0 },
		func(request *ExecuteRequest) { request.Limits.TmpfsBytes = request.Limits.FileBytes - 1 },
		func(request *ExecuteRequest) { request.Limits.WallTime = 0 },
	} {
		request := validExecuteRequest()
		mutate(&request)
		if _, err := normalizeExecuteRequest(request); err == nil {
			t.Fatalf("normalizeExecuteRequest(%+v) error = nil", request)
		}
	}
}

func TestNormalizeExecuteRequestRejectsDuplicateInputPathsAndBudgetOverflow(t *testing.T) {
	t.Parallel()

	request := validExecuteRequest()
	request.InputFiles = append(request.InputFiles, InputFile{
		Path: "artifact/main", Mode: 0o400, Size: 1, Reader: bytes.NewReader([]byte("x")),
	})
	if _, err := normalizeExecuteRequest(request); err == nil {
		t.Fatal("duplicate input path accepted")
	}

	request = validExecuteRequest()
	request.InputFiles = []InputFile{
		{Path: "source/a", Mode: 0o400, Size: 40 << 20, Reader: bytes.NewReader(nil)},
		{Path: "source/b", Mode: 0o400, Size: 40 << 20, Reader: bytes.NewReader(nil)},
	}
	request.Limits.FileBytes = 64 << 20
	if _, err := normalizeExecuteRequest(request); err == nil {
		t.Fatal("aggregate input-file budget overflow accepted")
	}

	for _, paths := range [][]string{
		{"artifact", "artifact/main"},
		{"artifact/main", "artifact"},
		{"artifact/.wh.main"},
		{".wh..wh..opq"},
	} {
		request = validExecuteRequest()
		request.InputFiles = make([]InputFile, 0, len(paths))
		for _, inputPath := range paths {
			request.InputFiles = append(request.InputFiles, InputFile{
				Path: inputPath, Mode: 0o400, Size: 1, Reader: bytes.NewReader([]byte("x")),
			})
		}
		if _, err := normalizeExecuteRequest(request); err == nil {
			t.Fatalf("input path set %v accepted", paths)
		}
	}
}

func TestNormalizeExecuteRequestSortsInputFilesForCanonicalArchive(t *testing.T) {
	t.Parallel()

	request := validExecuteRequest()
	request.InputFiles = []InputFile{
		{Path: "source/z.go", Mode: 0o400, Size: 1, Reader: bytes.NewReader([]byte("z"))},
		{Path: "artifact/a", Mode: 0o500, Size: 1, Reader: bytes.NewReader([]byte("a"))},
	}
	normalized, err := normalizeExecuteRequest(request)
	if err != nil {
		t.Fatalf("normalizeExecuteRequest() error = %v", err)
	}
	if normalized.InputFiles[0].Path != "artifact/a" ||
		normalized.InputFiles[1].Path != "source/z.go" {
		t.Fatalf("InputFiles = %v, want path-sorted canonical order", normalized.InputFiles)
	}
}

func TestNormalizeExecuteRequestAcceptsPublishedMinimumCPUContract(t *testing.T) {
	t.Parallel()

	request := validExecuteRequest()
	request.Limits.CPUTime = 10 * time.Millisecond
	request.Limits.WallTime = 10 * time.Millisecond
	if _, err := normalizeExecuteRequest(request); err != nil {
		t.Fatalf(
			"normalizeExecuteRequest() error = %v, published 10ms CPU limit must remain executable",
			err,
		)
	}
}

func validExecuteRequest() ExecuteRequest {
	return ExecuteRequest{
		Identity: ExecutionIdentity{
			JobID:        uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			AttemptID:    uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			WorkerBootID: uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		},
		Boundary:    BoundaryTestcase,
		ImageDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Argv:        []string{"/workspace/artifact/main"},
		WorkingDir:  "/workspace",
		Stdin:       bytes.NewReader([]byte("1 2\n")),
		StdinBytes:  4,
		InputFiles: []InputFile{{
			Path: "artifact/main", Mode: 0o500, Size: 4, Reader: bytes.NewReader([]byte("ELF!")),
		}},
		Limits: ResourceLimits{
			WallTime: 2 * time.Second, CPUTime: time.Second,
			MemoryBytes: 128 << 20, Pids: 64,
			OutputBytes: 8 << 20, DiagnosticBytes: 64 << 10,
			FileBytes: 64 << 20, TmpfsBytes: 128 << 20, OpenFiles: 64,
		},
	}
}
