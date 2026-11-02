package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var (
	ErrInvalidRuntimeProfile     = errors.New("runtime registry: invalid runtime profile")
	ErrRuntimeProfileUnavailable = ports.ErrRuntimeProfileUnavailable
	ErrInvalidCheckerProfile     = errors.New("runtime registry: invalid checker profile")
	ErrCheckerProfileUnavailable = ports.ErrCheckerProfileUnavailable
	ErrInvalidCheckerConfig      = ports.ErrInvalidCheckerConfiguration
)

var (
	profileIdentifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	workerValuePattern       = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	environmentNamePattern   = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
)

// RuntimeRegistry exposes the closed runtime/checker catalog to problem
// publication without giving the API process access to a sandbox runtime.
type RuntimeRegistry struct {
	runtimes        map[string]entity.RuntimeProfile
	runtimeVersions map[string]entity.RuntimeProfile
	checkers        map[string]entity.CheckerProfile
}

func NewRuntimeRegistry(
	runtimes []entity.RuntimeProfile,
	checkers []entity.CheckerProfile,
) (*RuntimeRegistry, error) {
	registry := &RuntimeRegistry{
		runtimes:        make(map[string]entity.RuntimeProfile, len(runtimes)),
		runtimeVersions: make(map[string]entity.RuntimeProfile, len(runtimes)),
		checkers:        make(map[string]entity.CheckerProfile, len(checkers)),
	}
	for _, profile := range runtimes {
		if err := validateRuntimeProfile(profile); err != nil {
			return nil, err
		}
		identity := profileIdentity(profile.Key, profile.ProfileVersion)
		if _, exists := registry.runtimeVersions[identity]; exists {
			return nil, fmt.Errorf("%w: duplicate identity %q", ErrInvalidRuntimeProfile, identity)
		}
		cloned := cloneRuntimeProfile(profile)
		registry.runtimeVersions[identity] = cloned
		if current, exists := registry.runtimes[profile.Key]; !exists || (!current.Active && profile.Active) {
			registry.runtimes[profile.Key] = cloned
		}
	}
	for _, profile := range checkers {
		if err := validateCheckerProfile(profile); err != nil {
			return nil, err
		}
		identity := profileIdentity(profile.Key, profile.Version)
		if _, exists := registry.checkers[identity]; exists {
			return nil, fmt.Errorf("%w: duplicate identity %q", ErrInvalidCheckerProfile, identity)
		}
		registry.checkers[identity] = profile
	}
	return registry, nil
}

func (registry *RuntimeRegistry) ValidateRuntimeKeys(ctx context.Context, keys []string) error {
	if registry == nil || ctx == nil || len(keys) == 0 {
		return ErrRuntimeProfileUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, duplicate := seen[key]; duplicate {
			return ErrRuntimeProfileUnavailable
		}
		seen[key] = struct{}{}
		profile, exists := registry.runtimes[key]
		if !exists || !profile.Active {
			return ErrRuntimeProfileUnavailable
		}
	}
	return nil
}

// ValidateExecutionPairings is the code-owned registry behavior used by
// focused domain tests and non-release callers: every registered runtime may
// use every registered checker. Release catalogs override this boundary with
// their explicit capability tuple allow-list.
func (registry *RuntimeRegistry) ValidateExecutionPairings(
	ctx context.Context,
	runtimeKeys []string,
	checkerKey string,
	checkerVersion string,
) error {
	if registry == nil || ctx == nil {
		return ports.ErrExecutionCapabilityUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := registry.ValidateRuntimeKeys(ctx, runtimeKeys); err != nil {
		return errors.Join(ports.ErrExecutionCapabilityUnavailable, err)
	}
	checker, exists := registry.checkers[profileIdentity(checkerKey, checkerVersion)]
	if !exists || !checker.Active {
		return ports.ErrExecutionCapabilityUnavailable
	}
	return nil
}

func (registry *RuntimeRegistry) ValidateChecker(
	ctx context.Context,
	key string,
	version string,
	config json.RawMessage,
) error {
	if registry == nil || ctx == nil {
		return ErrCheckerProfileUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	profile, exists := registry.checkers[profileIdentity(key, version)]
	if !exists || !profile.Active {
		return ErrCheckerProfileUnavailable
	}
	return validateCheckerConfiguration(profile, config)
}

func validateCheckerConfiguration(
	profile entity.CheckerProfile,
	config json.RawMessage,
) error {
	switch profile.ConfigKind {
	case entity.CheckerConfigNone:
		var object map[string]json.RawMessage
		if err := decodeOneJSON(config, &object); err != nil || object == nil || len(object) != 0 {
			return ErrInvalidCheckerConfig
		}
	case entity.CheckerConfigFloatTolerance:
		var value struct {
			Absolute *float64 `json:"abs_tol"`
			Relative *float64 `json:"rel_tol"`
		}
		decoder := json.NewDecoder(bytes.NewReader(config))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil || ensureJSONEOF(decoder) != nil ||
			value.Absolute == nil || value.Relative == nil ||
			!validTolerance(*value.Absolute) || !validTolerance(*value.Relative) {
			return ErrInvalidCheckerConfig
		}
	default:
		return ErrInvalidCheckerConfig
	}
	return nil
}

// RuntimeProfile resolves one whole versioned row for exact worker matching.
func (registry *RuntimeRegistry) RuntimeProfile(key string, version string) (entity.RuntimeProfile, bool) {
	if registry == nil {
		return entity.RuntimeProfile{}, false
	}
	profile, exists := registry.runtimeVersions[profileIdentity(key, version)]
	if !exists {
		return entity.RuntimeProfile{}, false
	}
	return cloneRuntimeProfile(profile), true
}

// CheckerProfile resolves one whole versioned row for exact worker matching.
func (registry *RuntimeRegistry) CheckerProfile(key string, version string) (entity.CheckerProfile, bool) {
	if registry == nil {
		return entity.CheckerProfile{}, false
	}
	profile, exists := registry.checkers[profileIdentity(key, version)]
	return profile, exists
}

func validateCheckerProfile(profile entity.CheckerProfile) error {
	if profile.ID == uuid.Nil || !profileIdentifierPattern.MatchString(profile.Key) ||
		!profileIdentifierPattern.MatchString(profile.Version) ||
		!validReleaseDigest(profile.ImplementationChecksum) {
		return ErrInvalidCheckerProfile
	}
	switch profile.ConfigKind {
	case entity.CheckerConfigNone, entity.CheckerConfigFloatTolerance:
		return nil
	default:
		return ErrInvalidCheckerProfile
	}
}

func profileIdentity(key string, version string) string {
	return key + "\x00" + version
}

func decodeOneJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return ErrInvalidCheckerConfig
	}
	return err
}

func validTolerance(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validateRuntimeProfile(profile entity.RuntimeProfile) error {
	if profile.ID == uuid.Nil || !profileIdentifierPattern.MatchString(profile.Key) ||
		!profileIdentifierPattern.MatchString(profile.ProfileVersion) ||
		strings.TrimSpace(profile.Language) == "" || strings.TrimSpace(profile.DisplayName) == "" ||
		strings.TrimSpace(profile.CompilerVersion) == "" || strings.TrimSpace(profile.RuntimeBootstrapVersion) == "" ||
		!validReleaseDigest(profile.ImageDigest) ||
		!profileIdentifierPattern.MatchString(profile.CompileCommandID) ||
		!profileIdentifierPattern.MatchString(profile.RunCommandID) ||
		profile.DefaultCompileTimeout <= 0 || profile.DefaultCompileTimeout > time.Minute ||
		!profileIdentifierPattern.MatchString(profile.ManifestVersion) ||
		!validReleaseDigest(profile.ManifestChecksum) || len(profile.Artifacts) == 0 {
		return ErrInvalidRuntimeProfile
	}
	if err := validateCommandTemplate(profile.Compile); err != nil {
		return errors.Join(ErrInvalidRuntimeProfile, err)
	}
	if err := validateCommandTemplate(profile.Run); err != nil {
		return errors.Join(ErrInvalidRuntimeProfile, err)
	}
	seenArtifacts := make(map[string]struct{}, len(profile.Artifacts))
	requiredArtifacts := 0
	for _, artifact := range profile.Artifacts {
		if artifact.Path == "" || path.IsAbs(artifact.Path) || path.Clean(artifact.Path) != artifact.Path ||
			strings.HasPrefix(artifact.Path, "../") || artifact.MaximumBytes <= 0 ||
			artifact.MaximumFiles < 1 || artifact.MaximumFiles > 4096 {
			return ErrInvalidRuntimeProfile
		}
		if artifact.Required {
			requiredArtifacts++
		}
		if _, err := path.Match(artifact.Path, artifact.Path); err != nil {
			return ErrInvalidRuntimeProfile
		}
		if _, exists := seenArtifacts[artifact.Path]; exists {
			return ErrInvalidRuntimeProfile
		}
		seenArtifacts[artifact.Path] = struct{}{}
	}
	if requiredArtifacts == 0 {
		return ErrInvalidRuntimeProfile
	}
	if profile.Key == entity.RuntimeKeyGo && !isPreparedOfflineGoCommand(profile.Compile) {
		return ErrInvalidRuntimeProfile
	}
	if err := validateRuntimeWorkspace(profile); err != nil {
		return errors.Join(ErrInvalidRuntimeProfile, err)
	}
	return nil
}

func validateRuntimeWorkspace(profile entity.RuntimeProfile) error {
	workspace := profile.Workspace
	if !validWorkspaceRelativePath(workspace.SourcePath) || len(workspace.Values) == 0 {
		return errors.New("invalid runtime workspace")
	}
	values := make(map[string]string, len(workspace.Values))
	for _, value := range workspace.Values {
		if !workerValuePattern.MatchString(value.Name) ||
			!strings.HasPrefix(value.Path, "/workspace/") ||
			path.Clean(value.Path) != value.Path {
			return errors.New("invalid runtime workspace value")
		}
		if _, duplicate := values[value.Name]; duplicate {
			return errors.New("duplicate runtime workspace value")
		}
		values[value.Name] = value.Path
	}
	for _, command := range []entity.CommandTemplate{profile.Compile, profile.Run} {
		if command.WorkingDirectory.Kind == entity.ArgumentWorkerPath {
			if _, exists := values[command.WorkingDirectory.Value]; !exists {
				return errors.New("missing runtime working-directory value")
			}
		}
		for _, argument := range command.Arguments {
			if argument.Kind == entity.ArgumentWorkerPath {
				if _, exists := values[argument.Value]; !exists {
					return errors.New("missing runtime command value")
				}
			}
		}
	}
	seenFiles := map[string]struct{}{workspace.SourcePath: {}}
	for _, file := range workspace.TrustedFiles {
		if !validWorkspaceRelativePath(file.Path) || file.Content == "" ||
			len(file.Content) > 64<<10 || !validTrustedWorkspaceContent(file.Content) {
			return errors.New("invalid trusted runtime workspace file")
		}
		if _, duplicate := seenFiles[file.Path]; duplicate {
			return errors.New("duplicate runtime workspace file")
		}
		seenFiles[file.Path] = struct{}{}
	}
	return nil
}

func validWorkspaceRelativePath(value string) bool {
	return value != "" && !path.IsAbs(value) && path.Clean(value) == value &&
		value != "." && !strings.HasPrefix(value, "../") &&
		!strings.Contains(value, "\\") && utf8.ValidString(value) &&
		!strings.ContainsAny(value, "\r\n\x00")
}

func validTrustedWorkspaceContent(value string) bool {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return false
	}
	for _, character := range value {
		if character == '\n' || character == '\t' {
			continue
		}
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

func validateCommandTemplate(command entity.CommandTemplate) error {
	if !path.IsAbs(command.Executable) || path.Clean(command.Executable) != command.Executable {
		return errors.New("command executable must be an absolute canonical path")
	}
	switch path.Base(command.Executable) {
	case "sh", "bash", "dash", "zsh", "fish":
		return errors.New("shell executable is forbidden")
	}
	if command.WorkingDirectory != (entity.CommandArgument{}) {
		if command.WorkingDirectory.Kind != entity.ArgumentWorkerPath ||
			!workerValuePattern.MatchString(command.WorkingDirectory.Value) ||
			command.WorkingDirectory.Prefix != "" || command.WorkingDirectory.Suffix != "" {
			return errors.New("invalid working directory")
		}
	}
	for _, argument := range command.Arguments {
		if !validCommandArgument(argument) {
			return errors.New("invalid command argument")
		}
	}
	seenEnvironment := make(map[string]struct{}, len(command.Environment))
	for _, variable := range command.Environment {
		if !environmentNamePattern.MatchString(variable.Name) || unsafeShellText(variable.Value) {
			return errors.New("invalid command environment")
		}
		if _, exists := seenEnvironment[variable.Name]; exists {
			return errors.New("duplicate command environment")
		}
		seenEnvironment[variable.Name] = struct{}{}
	}
	return nil
}

func validCommandArgument(argument entity.CommandArgument) bool {
	switch argument.Kind {
	case entity.ArgumentLiteral:
		return argument.Value != "" && argument.Prefix == "" && argument.Suffix == "" && !unsafeShellText(argument.Value)
	case entity.ArgumentWorkerPath:
		return workerValuePattern.MatchString(argument.Value) && argument.Prefix == "" && argument.Suffix == ""
	case entity.ArgumentNumericLimit:
		return workerValuePattern.MatchString(argument.Value) && !unsafeShellText(argument.Prefix) && !unsafeShellText(argument.Suffix)
	default:
		return false
	}
}

func unsafeShellText(value string) bool {
	return strings.Contains(value, "$(") || strings.Contains(value, "${") ||
		strings.Contains(value, "`") || strings.ContainsAny(value, "\r\n\x00")
}

func isPreparedOfflineGoCommand(command entity.CommandTemplate) bool {
	if command.Executable != "/usr/bin/go" ||
		command.WorkingDirectory.Kind != entity.ArgumentWorkerPath ||
		command.WorkingDirectory.Value != "prepared_go_module_workspace" {
		return false
	}
	literals := make(map[string]bool)
	for _, argument := range command.Arguments {
		if argument.Kind == entity.ArgumentLiteral {
			literals[argument.Value] = true
		}
	}
	environment := make(map[string]string, len(command.Environment))
	for _, variable := range command.Environment {
		environment[variable.Name] = variable.Value
	}
	return literals["build"] && literals["-mod=vendor"] &&
		environment["GOPROXY"] == "off" && environment["GOSUMDB"] == "off"
}

func cloneRuntimeProfile(profile entity.RuntimeProfile) entity.RuntimeProfile {
	profile.Compile.Arguments = append([]entity.CommandArgument(nil), profile.Compile.Arguments...)
	profile.Compile.Environment = append([]entity.EnvironmentVariable(nil), profile.Compile.Environment...)
	profile.Run.Arguments = append([]entity.CommandArgument(nil), profile.Run.Arguments...)
	profile.Run.Environment = append([]entity.EnvironmentVariable(nil), profile.Run.Environment...)
	profile.Artifacts = append([]entity.RuntimeArtifact(nil), profile.Artifacts...)
	profile.Workspace.Values = append([]entity.RuntimeWorkspaceValue(nil), profile.Workspace.Values...)
	profile.Workspace.TrustedFiles = append([]entity.RuntimeWorkspaceFile(nil), profile.Workspace.TrustedFiles...)
	return profile
}
