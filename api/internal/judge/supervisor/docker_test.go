package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDockerEngineNegotiatesVersionAndSendsClosedCreatePayload(t *testing.T) {
	t.Parallel()

	socket := testDockerSocketPath(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen Unix socket: %v", err)
	}
	var mu sync.Mutex
	var create dockerCreateRequest
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/version":
			_ = json.NewEncoder(writer).Encode(map[string]string{
				"ApiVersion": "1.54", "MinAPIVersion": "1.40",
			})
		case "/v1.46/containers/create":
			mu.Lock()
			defer mu.Unlock()
			if err := json.NewDecoder(request.Body).Decode(&create); err != nil {
				t.Errorf("decode create request: %v", err)
			}
			writer.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"Id":       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"Warnings": []string{},
			})
		default:
			http.NotFound(writer, request)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	engine, err := NewDockerEngine(DockerEngineConfig{
		SocketPath:  socket,
		DialTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewDockerEngine() error = %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	request := validExecuteRequest()
	normalized, err := normalizeExecuteRequest(request)
	if err != nil {
		t.Fatalf("normalizeExecuteRequest() error = %v", err)
	}
	spec := buildContainerSpec(
		normalized,
		request.Identity.AttemptID,
	)
	id, err := engine.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id == "" {
		t.Fatal("Create() returned empty ID")
	}
	mu.Lock()
	defer mu.Unlock()
	if create.Image != request.ImageDigest ||
		create.Entrypoint[0] != sandboxInitPath ||
		create.HostConfig.NetworkMode != "none" ||
		!create.HostConfig.ReadonlyRootfs ||
		create.HostConfig.Privileged ||
		create.HostConfig.MemorySwap != create.HostConfig.Memory ||
		len(create.HostConfig.CapDrop) != 1 ||
		create.HostConfig.CapDrop[0] != "ALL" ||
		create.HostConfig.LogConfig.Type != "none" ||
		len(create.HostConfig.LogConfig.Config) != 0 ||
		create.HostConfig.Init == nil ||
		!*create.HostConfig.Init {
		t.Fatalf("Docker create payload weakened policy: %+v", create)
	}
}

func TestDockerEngineBoundsAndSanitizesDaemonErrors(t *testing.T) {
	t.Parallel()

	socket := testDockerSocketPath(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen Unix socket: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/version" {
			_ = json.NewEncoder(writer).Encode(map[string]string{
				"ApiVersion": "1.46", "MinAPIVersion": "1.42",
			})
			return
		}
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(writer, `{"message":"daemon failed`+"\n"+`with control text"}`)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})
	engine, err := NewDockerEngine(DockerEngineConfig{SocketPath: socket})
	if err != nil {
		t.Fatalf("NewDockerEngine() error = %v", err)
	}
	_, err = engine.Create(context.Background(), buildContainerSpec(
		mustNormalizeTestRequest(t),
		validExecuteRequest().Identity.AttemptID,
	))
	if err == nil || len(err.Error()) > maximumEngineErrorBytes+128 {
		t.Fatalf("Create() error = %v, want bounded error", err)
	}
}

func TestDockerEngineGetsOnlyTheClosedArtifactArchivePath(t *testing.T) {
	t.Parallel()

	socket := testDockerSocketPath(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen Unix socket: %v", err)
	}
	archiveBody := []byte("bounded-tar-stream")
	server := &http.Server{Handler: http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		switch request.URL.Path {
		case "/version":
			_ = json.NewEncoder(writer).Encode(map[string]string{
				"ApiVersion": "1.46", "MinAPIVersion": "1.42",
			})
		case "/v1.46/containers/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/archive":
			if request.Method != http.MethodGet ||
				request.URL.Query().Get("path") != "/workspace/artifact" {
				http.Error(writer, "invalid archive request", http.StatusBadRequest)
				return
			}
			writer.Header().Set("Content-Type", "application/x-tar")
			_, _ = writer.Write(archiveBody)
		default:
			http.NotFound(writer, request)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	engine, err := NewDockerEngine(DockerEngineConfig{SocketPath: socket})
	if err != nil {
		t.Fatalf("NewDockerEngine() error = %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	archive, err := engine.GetArchive(
		context.Background(),
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"/workspace/artifact",
	)
	if err != nil {
		t.Fatalf("GetArchive() error = %v", err)
	}
	value, readErr := io.ReadAll(archive)
	closeErr := archive.Close()
	if readErr != nil || closeErr != nil || string(value) != string(archiveBody) {
		t.Fatalf(
			"archive = %q, read error = %v, close error = %v",
			value,
			readErr,
			closeErr,
		)
	}
	if _, err := engine.GetArchive(
		context.Background(),
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"/workspace/source",
	); !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("GetArchive(untrusted path) error = %v", err)
	}
}

func mustNormalizeTestRequest(t *testing.T) normalizedRequest {
	t.Helper()
	value, err := normalizeExecuteRequest(validExecuteRequest())
	if err != nil {
		t.Fatalf("normalizeExecuteRequest() error = %v", err)
	}
	return value
}

func testDockerSocketPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("/tmp", "judgify-"+uuid.NewString()[:8]+".sock")
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}
