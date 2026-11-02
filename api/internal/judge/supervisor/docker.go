package supervisor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	minimumDockerAPIVersion = "1.42"
	maximumDockerAPIVersion = "1.46"
	maximumEngineBodyBytes  = 64 << 10
	maximumEngineErrorBytes = 512
)

var (
	containerIDPattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
	containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	apiVersionPattern    = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}$`)
)

type DockerEngineConfig struct {
	SocketPath  string
	DialTimeout time.Duration
}

type DockerEngine struct {
	socketPath string
	client     *http.Client
	transport  *http.Transport
	dialer     net.Dialer

	versionMu  sync.Mutex
	apiVersion string
	closed     bool
}

func NewDockerEngine(config DockerEngineConfig) (*DockerEngine, error) {
	if !filepath.IsAbs(config.SocketPath) ||
		filepath.Clean(config.SocketPath) != config.SocketPath ||
		strings.ContainsAny(config.SocketPath, "\x00\r\n") {
		return nil, ErrEngineUnavailable
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.DialTimeout < 100*time.Millisecond || config.DialTimeout > 30*time.Second {
		return nil, ErrEngineUnavailable
	}
	engine := &DockerEngine{
		socketPath: config.SocketPath,
		dialer:     net.Dialer{Timeout: config.DialTimeout, KeepAlive: 30 * time.Second},
	}
	engine.transport = &http.Transport{
		DisableCompression:  true,
		ForceAttemptHTTP2:   false,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     30 * time.Second,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return engine.dialer.DialContext(ctx, "unix", engine.socketPath)
		},
	}
	engine.client = &http.Client{
		Transport: engine.transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("sandbox engine: redirect rejected")
		},
	}
	return engine, nil
}

func (engine *DockerEngine) Create(
	ctx context.Context,
	spec ContainerSpec,
) (string, error) {
	if engine == nil || ctx == nil || !validContainerSpec(spec) {
		return "", ErrEngineUnavailable
	}
	initProcess := true
	payload := dockerCreateRequest{
		Hostname:     spec.Hostname,
		User:         spec.User,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		OpenStdin:    true,
		StdinOnce:    true,
		Env:          append([]string(nil), spec.Environment...),
		Image:        spec.ImageDigest,
		WorkingDir:   spec.WorkingDir,
		Entrypoint:   []string{spec.Command[0]},
		Cmd:          append([]string(nil), spec.Command[1:]...),
		Labels:       cloneStringMap(spec.Labels),
		StopSignal:   "SIGKILL",
		StopTimeout:  1,
		HostConfig: dockerHostConfig{
			NetworkMode:    spec.NetworkMode,
			ReadonlyRootfs: spec.ReadOnlyRoot,
			Privileged:     spec.Privileged,
			CapDrop:        append([]string(nil), spec.DropCapabilities...),
			SecurityOpt:    append([]string(nil), spec.SecurityOptions...),
			IpcMode:        spec.IPCMode,
			PidMode:        spec.PIDMode,
			UTSMode:        spec.UTSMode,
			Memory:         spec.MemoryBytes,
			MemorySwap:     spec.MemorySwapBytes,
			NanoCPUs:       spec.NanoCPUs,
			PidsLimit:      spec.PidsLimit,
			Tmpfs:          cloneStringMap(spec.Tmpfs),
			Ulimits:        append([]Ulimit(nil), spec.Ulimits...),
			LogConfig: dockerLogConfig{
				Type: "none",
			},
			AutoRemove: false,
			Init:       &initProcess,
		},
	}
	var response dockerCreateResponse
	err := engine.doJSON(
		ctx,
		http.MethodPost,
		"/containers/create?name="+url.QueryEscape(spec.Name),
		payload,
		http.StatusCreated,
		&response,
	)
	if err != nil {
		return "", err
	}
	if !containerIDPattern.MatchString(response.ID) {
		return "", ErrEngineUnavailable
	}
	if len(response.Warnings) != 0 {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = engine.Remove(cleanupCtx, response.ID, true)
		return response.ID, ErrEngineUnavailable
	}
	return response.ID, nil
}

func (engine *DockerEngine) PutArchive(
	ctx context.Context,
	containerID string,
	destination string,
	archive io.Reader,
) error {
	if engine == nil || ctx == nil || !containerIDPattern.MatchString(containerID) ||
		destination != "/workspace" || archive == nil {
		return ErrEngineUnavailable
	}
	endpoint := "/containers/" + containerID + "/archive?path=" + url.QueryEscape(destination)
	return engine.doStream(
		ctx,
		http.MethodPut,
		endpoint,
		"application/x-tar",
		archive,
		http.StatusOK,
	)
}

func (engine *DockerEngine) GetArchive(
	ctx context.Context,
	containerID string,
	source string,
) (io.ReadCloser, error) {
	if engine == nil || ctx == nil || !containerIDPattern.MatchString(containerID) ||
		source != "/workspace/artifact" {
		return nil, ErrEngineUnavailable
	}
	version, err := engine.negotiatedVersion(ctx)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://docker/v"+version+"/containers/"+containerID+
			"/archive?path="+url.QueryEscape(source),
		nil,
	)
	if err != nil {
		return nil, ErrEngineUnavailable
	}
	request.Header.Set("User-Agent", "judgify-sandbox-supervisor/1")
	response, err := engine.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: get archive", ErrEngineUnavailable)
	}
	if response.StatusCode != http.StatusOK {
		message := readEngineError(response.Body)
		_ = response.Body.Close()
		return nil, engineStatusError(response.StatusCode, message)
	}
	if !strings.HasPrefix(
		strings.ToLower(response.Header.Get("Content-Type")),
		"application/x-tar",
	) {
		_ = response.Body.Close()
		return nil, fmt.Errorf("%w: archive stream has invalid content type", ErrEngineUnavailable)
	}
	return response.Body, nil
}

func (engine *DockerEngine) Attach(
	ctx context.Context,
	containerID string,
) (AttachConnection, error) {
	if engine == nil || ctx == nil || !containerIDPattern.MatchString(containerID) {
		return nil, ErrEngineUnavailable
	}
	version, err := engine.negotiatedVersion(ctx)
	if err != nil {
		return nil, err
	}
	connection, err := engine.dialer.DialContext(ctx, "unix", engine.socketPath)
	if err != nil {
		return nil, fmt.Errorf("%w: attach dial", ErrEngineUnavailable)
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = connection.Close()
		}
	}()
	endpoint := fmt.Sprintf(
		"http://docker/v%s/containers/%s/attach?logs=0&stream=1&stdin=1&stdout=1&stderr=1",
		version,
		containerID,
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, ErrEngineUnavailable
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "tcp")
	request.Header.Set("User-Agent", "judgify-sandbox-supervisor/1")
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	if err := request.Write(connection); err != nil {
		return nil, fmt.Errorf("%w: attach request", ErrEngineUnavailable)
	}
	reader := bufio.NewReaderSize(connection, 32<<10)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		return nil, fmt.Errorf("%w: attach response", ErrEngineUnavailable)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		message := readEngineError(response.Body)
		_ = response.Body.Close()
		return nil, engineStatusError(response.StatusCode, message)
	}
	if !strings.HasPrefix(
		strings.ToLower(response.Header.Get("Content-Type")),
		"application/vnd.docker.multiplexed-stream",
	) {
		return nil, fmt.Errorf("%w: attach stream is not multiplexed", ErrEngineUnavailable)
	}
	_ = connection.SetDeadline(time.Time{})
	closeOnError = false
	return &dockerAttachConnection{connection: connection, reader: reader}, nil
}

func (engine *DockerEngine) Start(ctx context.Context, containerID string) error {
	if !validEngineContainerRequest(engine, ctx, containerID) {
		return ErrEngineUnavailable
	}
	return engine.doJSON(
		ctx,
		http.MethodPost,
		"/containers/"+containerID+"/start",
		nil,
		http.StatusNoContent,
		nil,
	)
}

func (engine *DockerEngine) Inspect(
	ctx context.Context,
	containerID string,
) (ContainerState, error) {
	if !validEngineContainerRequest(engine, ctx, containerID) {
		return ContainerState{}, ErrEngineUnavailable
	}
	var response dockerInspectResponse
	if err := engine.doJSON(
		ctx,
		http.MethodGet,
		"/containers/"+containerID+"/json",
		nil,
		http.StatusOK,
		&response,
	); err != nil {
		return ContainerState{}, err
	}
	if response.State.PID < 0 || response.State.ExitCode < 0 ||
		!boundedEngineText(response.State.Error) {
		return ContainerState{}, ErrEngineUnavailable
	}
	return ContainerState{
		Running:   response.State.Running,
		PID:       response.State.PID,
		ExitCode:  response.State.ExitCode,
		OOMKilled: response.State.OOMKilled,
		Error:     response.State.Error,
	}, nil
}

func (engine *DockerEngine) Wait(
	ctx context.Context,
	containerID string,
) (WaitResult, error) {
	if !validEngineContainerRequest(engine, ctx, containerID) {
		return WaitResult{}, ErrEngineUnavailable
	}
	var response dockerWaitResponse
	if err := engine.doJSON(
		ctx,
		http.MethodPost,
		"/containers/"+containerID+"/wait?condition=not-running",
		nil,
		http.StatusOK,
		&response,
	); err != nil {
		return WaitResult{}, err
	}
	if response.StatusCode < 0 || !boundedEngineText(response.Error.Message) {
		return WaitResult{}, ErrEngineUnavailable
	}
	return WaitResult{StatusCode: response.StatusCode, Error: response.Error.Message}, nil
}

func (engine *DockerEngine) Kill(ctx context.Context, containerID string) error {
	if !validEngineContainerRequest(engine, ctx, containerID) {
		return ErrEngineUnavailable
	}
	err := engine.doJSON(
		ctx,
		http.MethodPost,
		"/containers/"+containerID+"/kill?signal=KILL",
		nil,
		http.StatusNoContent,
		nil,
	)
	return err
}

func (engine *DockerEngine) Remove(
	ctx context.Context,
	containerID string,
	force bool,
) error {
	if !validEngineContainerRequest(engine, ctx, containerID) {
		return ErrEngineUnavailable
	}
	endpoint := "/containers/" + containerID + "?v=1"
	if force {
		endpoint += "&force=1"
	}
	return engine.doJSON(
		ctx,
		http.MethodDelete,
		endpoint,
		nil,
		http.StatusNoContent,
		nil,
	)
}

func (engine *DockerEngine) Close() error {
	if engine == nil {
		return nil
	}
	engine.versionMu.Lock()
	if engine.closed {
		engine.versionMu.Unlock()
		return nil
	}
	engine.closed = true
	engine.versionMu.Unlock()
	engine.transport.CloseIdleConnections()
	return nil
}

type dockerAttachConnection struct {
	connection net.Conn
	reader     *bufio.Reader
}

func (connection *dockerAttachConnection) Read(value []byte) (int, error) {
	return connection.reader.Read(value)
}

func (connection *dockerAttachConnection) Write(value []byte) (int, error) {
	return connection.connection.Write(value)
}

func (connection *dockerAttachConnection) CloseWrite() error {
	writer, ok := connection.connection.(interface{ CloseWrite() error })
	if !ok {
		return ErrEngineUnavailable
	}
	return writer.CloseWrite()
}

func (connection *dockerAttachConnection) Close() error {
	return connection.connection.Close()
}

func (engine *DockerEngine) doJSON(
	ctx context.Context,
	method string,
	endpoint string,
	input any,
	expectedStatus int,
	output any,
) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil || len(encoded) > maximumEngineBodyBytes {
			return ErrEngineUnavailable
		}
		body = bytes.NewReader(encoded)
	}
	version, err := engine.negotiatedVersion(ctx)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		"http://docker/v"+version+endpoint,
		body,
	)
	if err != nil {
		return ErrEngineUnavailable
	}
	request.Header.Set("User-Agent", "judgify-sandbox-supervisor/1")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := engine.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: engine request", ErrEngineUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != expectedStatus {
		return engineStatusError(response.StatusCode, readEngineError(response.Body))
	}
	if output == nil {
		count, err := io.Copy(io.Discard, io.LimitReader(response.Body, maximumEngineBodyBytes+1))
		if err != nil || count > maximumEngineBodyBytes {
			return ErrEngineUnavailable
		}
		return nil
	}
	return decodeEngineJSON(response.Body, output)
}

func (engine *DockerEngine) doStream(
	ctx context.Context,
	method string,
	endpoint string,
	contentType string,
	body io.Reader,
	expectedStatus int,
) error {
	version, err := engine.negotiatedVersion(ctx)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		"http://docker/v"+version+endpoint,
		body,
	)
	if err != nil {
		return ErrEngineUnavailable
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("User-Agent", "judgify-sandbox-supervisor/1")
	response, err := engine.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: engine stream request", ErrEngineUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != expectedStatus {
		return engineStatusError(response.StatusCode, readEngineError(response.Body))
	}
	count, err := io.Copy(io.Discard, io.LimitReader(response.Body, maximumEngineBodyBytes+1))
	if err != nil || count > maximumEngineBodyBytes {
		return ErrEngineUnavailable
	}
	return nil
}

func (engine *DockerEngine) negotiatedVersion(ctx context.Context) (string, error) {
	engine.versionMu.Lock()
	defer engine.versionMu.Unlock()
	if engine.closed {
		return "", ErrEngineUnavailable
	}
	if engine.apiVersion != "" {
		return engine.apiVersion, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/version", nil)
	if err != nil {
		return "", ErrEngineUnavailable
	}
	request.Header.Set("User-Agent", "judgify-sandbox-supervisor/1")
	response, err := engine.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: negotiate API", ErrEngineUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", engineStatusError(response.StatusCode, readEngineError(response.Body))
	}
	var version dockerVersionResponse
	if err := decodeEngineJSON(response.Body, &version); err != nil {
		return "", err
	}
	serverMaximum, okMaximum := parseAPIVersion(version.APIVersion)
	serverMinimum, okMinimum := parseAPIVersion(version.MinAPIVersion)
	clientMinimum, _ := parseAPIVersion(minimumDockerAPIVersion)
	clientMaximum, _ := parseAPIVersion(maximumDockerAPIVersion)
	if !okMaximum || !okMinimum || serverMinimum > serverMaximum ||
		serverMaximum < clientMinimum || serverMinimum > clientMaximum {
		return "", ErrEngineUnavailable
	}
	selected := min(serverMaximum, clientMaximum)
	if selected < max(serverMinimum, clientMinimum) {
		return "", ErrEngineUnavailable
	}
	engine.apiVersion = formatAPIVersion(selected)
	return engine.apiVersion, nil
}

func decodeEngineJSON(reader io.Reader, output any) error {
	limited := io.LimitReader(reader, maximumEngineBodyBytes+1)
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(output); err != nil {
		return ErrEngineUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrEngineUnavailable
	}
	return nil
}

func readEngineError(reader io.Reader) string {
	var payload struct {
		Message string `json:"message"`
	}
	value, err := io.ReadAll(io.LimitReader(reader, maximumEngineBodyBytes+1))
	if err != nil || len(value) > maximumEngineBodyBytes ||
		json.Unmarshal(value, &payload) != nil {
		return "bounded engine error"
	}
	return sanitizeEngineText(payload.Message)
}

func engineStatusError(status int, message string) error {
	switch status {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrEngineContainerNotFound, message)
	case http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrEngineContainerNotRunning, message)
	default:
		return fmt.Errorf("%w: status=%d: %s", ErrEngineUnavailable, status, message)
	}
}

func sanitizeEngineText(value string) string {
	if !utf8.ValidString(value) {
		return "invalid engine error"
	}
	var output strings.Builder
	output.Grow(min(len(value), maximumEngineErrorBytes))
	for _, character := range value {
		if output.Len() >= maximumEngineErrorBytes {
			break
		}
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			output.WriteByte(' ')
			continue
		}
		output.WriteRune(character)
	}
	return strings.TrimSpace(output.String())
}

func boundedEngineText(value string) bool {
	return len(value) <= maximumEngineErrorBytes &&
		utf8.ValidString(value) &&
		sanitizeEngineText(value) == value
}

func parseAPIVersion(value string) (int, bool) {
	if !apiVersionPattern.MatchString(value) {
		return 0, false
	}
	majorText, minorText, _ := strings.Cut(value, ".")
	major, majorErr := strconv.Atoi(majorText)
	minor, minorErr := strconv.Atoi(minorText)
	if majorErr != nil || minorErr != nil || major < 0 || minor < 0 || minor > 999 {
		return 0, false
	}
	return major*1000 + minor, true
}

func formatAPIVersion(value int) string {
	return strconv.Itoa(value/1000) + "." + strconv.Itoa(value%1000)
}

func validEngineContainerRequest(
	engine *DockerEngine,
	ctx context.Context,
	containerID string,
) bool {
	return engine != nil && ctx != nil && containerIDPattern.MatchString(containerID)
}

func validContainerSpec(spec ContainerSpec) bool {
	if !containerNamePattern.MatchString(spec.Name) ||
		!digestPattern.MatchString(spec.ImageDigest) ||
		spec.Hostname != "sandbox" ||
		spec.User != sandboxUser ||
		!validWorkingDirectory(spec.WorkingDir) ||
		len(spec.Command) < 3 ||
		spec.Command[0] != sandboxInitPath ||
		spec.Command[1] != "--" ||
		validateArgv(spec.Command[2:]) != nil ||
		!validNormalizedEnvironment(spec.Environment) ||
		!validContainerLabels(spec.Labels) ||
		spec.NetworkMode != "none" ||
		!spec.ReadOnlyRoot ||
		spec.Privileged ||
		len(spec.DropCapabilities) != 1 ||
		spec.DropCapabilities[0] != "ALL" ||
		!slices.Equal(
			spec.SecurityOptions,
			[]string{"no-new-privileges=true", "seccomp=builtin"},
		) ||
		spec.IPCMode != "none" ||
		spec.PIDMode != "" ||
		spec.UTSMode != "" ||
		spec.MemoryBytes < 8<<20 ||
		spec.MemorySwapBytes != spec.MemoryBytes ||
		spec.NanoCPUs != sandboxCPUNanoQuota ||
		spec.PidsLimit < 1 ||
		len(spec.Tmpfs) != 2 ||
		!validTmpfsPolicy(spec.Tmpfs["/workspace"], true) ||
		!validTmpfsPolicy(spec.Tmpfs["/tmp"], false) ||
		!validUlimits(spec.Ulimits) {
		return false
	}
	return true
}

func validNormalizedEnvironment(environment []string) bool {
	if len(environment) < 5 {
		return false
	}
	fixed := []string{
		"HOME=/tmp",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TZ=UTC",
	}
	if !slices.Equal(environment[:5], fixed) {
		return false
	}
	normalized, err := normalizeEnvironment(environment[5:])
	return err == nil && slices.Equal(normalized, environment)
}

func validContainerLabels(labels map[string]string) bool {
	if len(labels) != 6 ||
		labels["io.judgify.managed"] != "true" ||
		!validBoundary(BoundaryKind(labels["io.judgify.boundary_kind"])) {
		return false
	}
	for _, key := range []string{
		"io.judgify.job_id",
		"io.judgify.attempt_id",
		"io.judgify.worker_boot_id",
		"io.judgify.boundary_id",
	} {
		if _, err := uuid.Parse(labels[key]); err != nil {
			return false
		}
	}
	return true
}

func validTmpfsPolicy(value string, executable bool) bool {
	if value == "" ||
		!strings.Contains(value, "rw,") ||
		!strings.Contains(value, "nosuid") ||
		!strings.Contains(value, "nodev") ||
		!strings.Contains(value, "size=") ||
		!strings.Contains(value, "nr_inodes=") ||
		!strings.Contains(value, "uid=65532") ||
		!strings.Contains(value, "gid=65532") ||
		!strings.Contains(value, "mode=0700") {
		return false
	}
	if executable {
		return strings.Contains(value, "exec") && !strings.Contains(value, "noexec")
	}
	return strings.Contains(value, "noexec")
}

func validUlimits(values []Ulimit) bool {
	if len(values) != 3 {
		return false
	}
	expected := map[string]struct{}{
		"core": {}, "fsize": {}, "nofile": {},
	}
	for _, value := range values {
		if _, ok := expected[value.Name]; !ok ||
			value.Soft < 0 || value.Hard < value.Soft {
			return false
		}
		switch value.Name {
		case "core":
			if value.Soft != 0 || value.Hard != 0 {
				return false
			}
		case "fsize":
			if value.Soft < 1 || value.Soft != value.Hard {
				return false
			}
		case "nofile":
			if value.Soft < 16 || value.Soft > 1024 || value.Soft != value.Hard {
				return false
			}
		}
		delete(expected, value.Name)
	}
	return len(expected) == 0
}

func cloneStringMap(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, content := range value {
		result[key] = content
	}
	return result
}

// The production payload intentionally has no fields for binds, mounts,
// devices, exposed ports, privileged namespaces, or Docker socket forwarding.
// Adding one requires a new reviewed supervisor protocol.
type dockerCreateRequest struct {
	Hostname     string            `json:"Hostname"`
	User         string            `json:"User"`
	AttachStdin  bool              `json:"AttachStdin"`
	AttachStdout bool              `json:"AttachStdout"`
	AttachStderr bool              `json:"AttachStderr"`
	Tty          bool              `json:"Tty"`
	OpenStdin    bool              `json:"OpenStdin"`
	StdinOnce    bool              `json:"StdinOnce"`
	Env          []string          `json:"Env"`
	Image        string            `json:"Image"`
	WorkingDir   string            `json:"WorkingDir"`
	Entrypoint   []string          `json:"Entrypoint"`
	Cmd          []string          `json:"Cmd"`
	Labels       map[string]string `json:"Labels"`
	StopSignal   string            `json:"StopSignal"`
	StopTimeout  int               `json:"StopTimeout"`
	HostConfig   dockerHostConfig  `json:"HostConfig"`
}

type dockerHostConfig struct {
	NetworkMode    string            `json:"NetworkMode"`
	ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
	Privileged     bool              `json:"Privileged"`
	CapDrop        []string          `json:"CapDrop"`
	SecurityOpt    []string          `json:"SecurityOpt"`
	IpcMode        string            `json:"IpcMode"`
	PidMode        string            `json:"PidMode"`
	UTSMode        string            `json:"UTSMode"`
	Memory         int64             `json:"Memory"`
	MemorySwap     int64             `json:"MemorySwap"`
	NanoCPUs       int64             `json:"NanoCpus"`
	PidsLimit      int64             `json:"PidsLimit"`
	Tmpfs          map[string]string `json:"Tmpfs"`
	Ulimits        []Ulimit          `json:"Ulimits"`
	LogConfig      dockerLogConfig   `json:"LogConfig"`
	AutoRemove     bool              `json:"AutoRemove"`
	Init           *bool             `json:"Init"`
}

type dockerLogConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config,omitempty"`
}

type dockerCreateResponse struct {
	ID       string   `json:"Id"`
	Warnings []string `json:"Warnings"`
}

type dockerInspectResponse struct {
	State struct {
		Running   bool   `json:"Running"`
		PID       int    `json:"Pid"`
		ExitCode  int    `json:"ExitCode"`
		OOMKilled bool   `json:"OOMKilled"`
		Error     string `json:"Error"`
	} `json:"State"`
}

type dockerWaitResponse struct {
	StatusCode int `json:"StatusCode"`
	Error      struct {
		Message string `json:"Message"`
	} `json:"Error"`
}

type dockerVersionResponse struct {
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion"`
}

var _ Engine = (*DockerEngine)(nil)
