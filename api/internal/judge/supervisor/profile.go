package supervisor

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	sandboxInitPath = "/usr/local/bin/judgify-sandbox-init"
	sandboxUser     = "65532:65532"

	maximumArguments     = 128
	maximumArgumentBytes = 4096
	maximumCommandBytes  = 32 << 10
	maximumEnvironment   = 32
	maximumInputFiles    = 128
)

var (
	ErrInvalidExecuteRequest = errors.New("sandbox supervisor: invalid execute request")

	digestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	environmentPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
)

var allowedEnvironment = map[string]struct{}{
	"GOCACHE":           {},
	"GOFLAGS":           {},
	"GOMODCACHE":        {},
	"GONOSUMDB":         {},
	"GOPATH":            {},
	"GOPROXY":           {},
	"GOSUMDB":           {},
	"JAVA_TOOL_OPTIONS": {},
	"PYTHONHASHSEED":    {},
}

var forbiddenExecutables = map[string]struct{}{
	"ash": {}, "bash": {}, "busybox": {}, "dash": {}, "env": {},
	"sh": {}, "zsh": {},
}

type normalizedRequest struct {
	ExecuteRequest
	Command      []string
	User         string
	NetworkMode  string
	ReadOnlyRoot bool
	Privileged   bool
	Capabilities []string
	SecurityOpts []string
}

func normalizeExecuteRequest(request ExecuteRequest) (normalizedRequest, error) {
	if request.Identity.JobID == [16]byte{} ||
		request.Identity.AttemptID == [16]byte{} ||
		request.Identity.WorkerBootID == [16]byte{} ||
		!validBoundary(request.Boundary) ||
		!digestPattern.MatchString(request.ImageDigest) ||
		!validWorkingDirectory(request.WorkingDir) ||
		request.Stdin == nil ||
		request.StdinBytes < 0 {
		return normalizedRequest{}, ErrInvalidExecuteRequest
	}
	if err := validateLimits(request.Limits); err != nil ||
		request.StdinBytes > request.Limits.FileBytes {
		return normalizedRequest{}, ErrInvalidExecuteRequest
	}
	if err := validateArgv(request.Argv); err != nil {
		return normalizedRequest{}, err
	}
	environment, err := normalizeEnvironment(request.Env)
	if err != nil {
		return normalizedRequest{}, err
	}
	if err := validateInputFiles(request.InputFiles, request.Limits.FileBytes); err != nil {
		return normalizedRequest{}, err
	}
	if request.Capture != nil {
		if request.Boundary != BoundaryCompile ||
			request.Capture.Path != "/workspace/artifact" ||
			request.Capture.MaximumBytes < 1024 ||
			request.Capture.MaximumBytes > request.Limits.FileBytes+(1<<20) {
			return normalizedRequest{}, ErrInvalidExecuteRequest
		}
		capture := *request.Capture
		request.Capture = &capture
	}

	command := make([]string, 0, len(request.Argv)+2)
	command = append(command, sandboxInitPath, "--")
	command = append(command, request.Argv...)
	request.Argv = slices.Clone(request.Argv)
	request.Env = environment
	request.InputFiles = slices.Clone(request.InputFiles)
	slices.SortFunc(request.InputFiles, func(left InputFile, right InputFile) int {
		return strings.Compare(left.Path, right.Path)
	})
	return normalizedRequest{
		ExecuteRequest: request,
		Command:        command,
		User:           sandboxUser,
		NetworkMode:    "none",
		ReadOnlyRoot:   true,
		Privileged:     false,
		Capabilities:   []string{"ALL"},
		SecurityOpts:   []string{"no-new-privileges=true", "seccomp=builtin"},
	}, nil
}

func validBoundary(boundary BoundaryKind) bool {
	switch boundary {
	case BoundaryCompile, BoundaryTestcase, BoundaryChecker:
		return true
	default:
		return false
	}
}

func validateArgv(argv []string) error {
	if len(argv) == 0 || len(argv) > maximumArguments {
		return ErrInvalidExecuteRequest
	}
	total := 0
	for index, argument := range argv {
		if argument == "" || len(argument) > maximumArgumentBytes ||
			!utf8.ValidString(argument) || containsControl(argument) {
			return ErrInvalidExecuteRequest
		}
		total += len(argument)
		if total > maximumCommandBytes {
			return ErrInvalidExecuteRequest
		}
		if index == 0 {
			if !path.IsAbs(argument) || path.Clean(argument) != argument ||
				strings.Contains(argument, "\\") {
				return ErrInvalidExecuteRequest
			}
			base := path.Base(argument)
			if _, forbidden := forbiddenExecutables[base]; forbidden {
				return ErrInvalidExecuteRequest
			}
		}
	}
	return nil
}

func validWorkingDirectory(value string) bool {
	return path.IsAbs(value) &&
		path.Clean(value) == value &&
		(value == "/workspace" || strings.HasPrefix(value, "/workspace/")) &&
		!strings.Contains(value, "\\")
}

func normalizeEnvironment(values []string) ([]string, error) {
	if len(values) > maximumEnvironment {
		return nil, ErrInvalidExecuteRequest
	}
	environment := []string{
		"HOME=/tmp",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TZ=UTC",
	}
	seen := map[string]struct{}{
		"HOME": {}, "LANG": {}, "LC_ALL": {}, "PATH": {}, "TZ": {},
	}
	custom := make([]string, 0, len(values))
	for _, value := range values {
		name, content, found := strings.Cut(value, "=")
		if !found || !environmentPattern.MatchString(name) ||
			len(content) > maximumArgumentBytes || !utf8.ValidString(content) ||
			containsControl(content) {
			return nil, ErrInvalidExecuteRequest
		}
		if _, allowed := allowedEnvironment[name]; !allowed {
			return nil, ErrInvalidExecuteRequest
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, ErrInvalidExecuteRequest
		}
		seen[name] = struct{}{}
		custom = append(custom, name+"="+content)
	}
	slices.Sort(custom)
	return append(environment, custom...), nil
}

func validateInputFiles(files []InputFile, maximumBytes int64) error {
	if len(files) > maximumInputFiles {
		return ErrInvalidExecuteRequest
	}
	seen := make(map[string]struct{}, len(files))
	var total int64
	for _, file := range files {
		if file.Reader == nil || file.Size < 0 ||
			(file.Mode != 0o400 && file.Mode != 0o500) ||
			file.Path == "" || path.IsAbs(file.Path) ||
			path.Clean(file.Path) != file.Path ||
			file.Path == "." || strings.HasPrefix(file.Path, "../") ||
			strings.Contains(file.Path, "\\") ||
			len(file.Path) > 512 || !utf8.ValidString(file.Path) ||
			containsControl(file.Path) {
			return ErrInvalidExecuteRequest
		}
		for _, component := range strings.Split(file.Path, "/") {
			if component == "" || strings.HasPrefix(component, ".wh.") {
				return ErrInvalidExecuteRequest
			}
		}
		if _, duplicate := seen[file.Path]; duplicate {
			return ErrInvalidExecuteRequest
		}
		for parent := path.Dir(file.Path); parent != "."; parent = path.Dir(parent) {
			if _, conflictsWithFile := seen[parent]; conflictsWithFile {
				return ErrInvalidExecuteRequest
			}
		}
		for existing := range seen {
			if strings.HasPrefix(existing, file.Path+"/") {
				return ErrInvalidExecuteRequest
			}
		}
		seen[file.Path] = struct{}{}
		if file.Size > maximumBytes-total {
			return ErrInvalidExecuteRequest
		}
		total += file.Size
	}
	return nil
}

func validateLimits(limits ResourceLimits) error {
	if limits.WallTime < 10*time.Millisecond || limits.WallTime > 60*time.Second ||
		limits.CPUTime < 10*time.Millisecond || limits.CPUTime > 60*time.Second ||
		limits.MemoryBytes < 8<<20 || limits.MemoryBytes > 2<<30 ||
		limits.Pids < 1 || limits.Pids > 512 ||
		limits.OutputBytes < 1 || limits.OutputBytes > 64<<20 ||
		limits.DiagnosticBytes < 1 || limits.DiagnosticBytes > 1<<20 ||
		limits.FileBytes < 1 || limits.FileBytes > 256<<20 ||
		limits.TmpfsBytes < limits.FileBytes || limits.TmpfsBytes > 512<<20 ||
		limits.OpenFiles < 16 || limits.OpenFiles > 1024 {
		return fmt.Errorf("%w: resource limits", ErrInvalidExecuteRequest)
	}
	return nil
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return true
		}
	}
	return false
}
