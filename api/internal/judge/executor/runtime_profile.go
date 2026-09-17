package executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"

	"github.com/huynhanx03/judgify/internal/judge/capability"
	submissionports "github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var ErrInvalidRuntimeExecution = errors.New("judge executor: invalid runtime execution contract")

type RuntimeProfileCatalog interface {
	RuntimeProfile(key string, version string) (problementity.RuntimeProfile, bool)
}

// ResolveRuntimeProfile reconstructs executable commands from the reviewed
// code-owned catalog and proves that every durable snapshot field names that
// exact contract. No command text is accepted from PostgreSQL.
func ResolveRuntimeProfile(
	catalog RuntimeProfileCatalog,
	plan submissionports.JudgePlan,
) (problementity.RuntimeProfile, error) {
	if catalog == nil {
		return problementity.RuntimeProfile{}, ErrInvalidRuntimeExecution
	}
	profile, exists := catalog.RuntimeProfile(
		plan.Runtime.Key,
		plan.Runtime.ProfileVersion,
	)
	if !exists || !profile.Active ||
		profile.Language != plan.Language ||
		profile.CompilerVersion != plan.Runtime.CompilerVersion ||
		profile.RuntimeBootstrapVersion != plan.Runtime.BootstrapVersion ||
		profile.ImageDigest != plan.Runtime.SandboxImageDigest ||
		profile.CompileCommandID != plan.Runtime.CompileCommandID ||
		profile.RunCommandID != plan.Runtime.RunCommandID ||
		profile.ManifestVersion != plan.Runtime.ArtifactManifestVersion ||
		profile.ManifestChecksum != plan.Runtime.ArtifactManifestChecksum ||
		!artifactManifestMatchesProfile(
			plan.Runtime.ArtifactManifest,
			profile.Artifacts,
		) {
		return problementity.RuntimeProfile{}, ErrInvalidRuntimeExecution
	}
	tuple := capability.CapabilityTuple{
		JudgeSpecProtocol:             plan.JudgeSpecProtocol,
		RuntimeKey:                    profile.Key,
		RuntimeProfileVersion:         profile.ProfileVersion,
		CompilerVersion:               profile.CompilerVersion,
		RuntimeBootstrapVersion:       profile.RuntimeBootstrapVersion,
		CompileCommandID:              profile.CompileCommandID,
		RunCommandID:                  profile.RunCommandID,
		ImageDigest:                   profile.ImageDigest,
		ManifestVersion:               profile.ManifestVersion,
		ManifestChecksum:              profile.ManifestChecksum,
		SupervisorProtocol:            plan.Runtime.SupervisorProtocol,
		CheckerKey:                    plan.Checker.Key,
		CheckerVersion:                plan.Checker.Version,
		CheckerImplementationChecksum: plan.Release.CheckerImplementationChecksum,
		CapabilityProfileHash:         plan.Runtime.CapabilityProfileHash,
	}
	computed, err := capability.ComputeCapabilityProfileHash(tuple)
	if err != nil || computed != tuple.CapabilityProfileHash {
		return problementity.RuntimeProfile{}, ErrInvalidRuntimeExecution
	}
	return cloneArtifactRuntimeProfile(profile), nil
}

func artifactManifestMatchesProfile(
	raw json.RawMessage,
	expected []problementity.RuntimeArtifact,
) bool {
	if len(raw) == 0 || len(raw) > 32<<10 || len(expected) == 0 {
		return false
	}
	var decoded []problementity.RuntimeArtifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return false
	}
	return slices.Equal(decoded, expected)
}

type renderedCommand struct {
	argv        []string
	environment []string
	workingDir  string
}

func renderRuntimeCommand(
	command problementity.CommandTemplate,
	workspace problementity.RuntimeWorkspace,
	numericLimits map[string]int64,
) (renderedCommand, error) {
	if !path.IsAbs(command.Executable) || path.Clean(command.Executable) != command.Executable {
		return renderedCommand{}, ErrInvalidRuntimeExecution
	}
	values := make(map[string]string, len(workspace.Values))
	for _, value := range workspace.Values {
		if value.Name == "" || !path.IsAbs(value.Path) ||
			path.Clean(value.Path) != value.Path ||
			(value.Path != "/workspace" && !hasWorkspacePrefix(value.Path)) {
			return renderedCommand{}, ErrInvalidRuntimeExecution
		}
		if _, duplicate := values[value.Name]; duplicate {
			return renderedCommand{}, ErrInvalidRuntimeExecution
		}
		values[value.Name] = value.Path
	}
	workingDirectory := "/workspace"
	if command.WorkingDirectory != (problementity.CommandArgument{}) {
		if command.WorkingDirectory.Kind != problementity.ArgumentWorkerPath {
			return renderedCommand{}, ErrInvalidRuntimeExecution
		}
		resolved, exists := values[command.WorkingDirectory.Value]
		if !exists {
			return renderedCommand{}, ErrInvalidRuntimeExecution
		}
		workingDirectory = resolved
	}
	arguments := make([]string, 1, len(command.Arguments)+1)
	arguments[0] = command.Executable
	for _, argument := range command.Arguments {
		switch argument.Kind {
		case problementity.ArgumentLiteral:
			if argument.Value == "" || argument.Prefix != "" || argument.Suffix != "" {
				return renderedCommand{}, ErrInvalidRuntimeExecution
			}
			arguments = append(arguments, argument.Value)
		case problementity.ArgumentWorkerPath:
			resolved, exists := values[argument.Value]
			if !exists || argument.Prefix != "" || argument.Suffix != "" {
				return renderedCommand{}, ErrInvalidRuntimeExecution
			}
			arguments = append(arguments, resolved)
		case problementity.ArgumentNumericLimit:
			value, exists := numericLimits[argument.Value]
			if !exists || value < 0 {
				return renderedCommand{}, ErrInvalidRuntimeExecution
			}
			arguments = append(
				arguments,
				argument.Prefix+strconv.FormatInt(value, 10)+argument.Suffix,
			)
		default:
			return renderedCommand{}, ErrInvalidRuntimeExecution
		}
	}
	environment := make([]string, 0, len(command.Environment))
	for _, variable := range command.Environment {
		if variable.Name == "" {
			return renderedCommand{}, ErrInvalidRuntimeExecution
		}
		environment = append(environment, variable.Name+"="+variable.Value)
	}
	return renderedCommand{
		argv:        arguments,
		environment: environment,
		workingDir:  workingDirectory,
	}, nil
}

func runtimeNumericLimits(memoryBytes int64) map[string]int64 {
	return map[string]int64{
		"memory_bytes": memoryBytes,
		"memory_mb":    memoryBytes / (1 << 20),
	}
}

func hasWorkspacePrefix(value string) bool {
	return len(value) > len("/workspace/") && value[:len("/workspace/")] == "/workspace/"
}

func artifactArchiveMaximum(
	profile problementity.RuntimeProfile,
	fileLimit int64,
) (int64, error) {
	const metadataAllowance = int64(1 << 20)
	var total int64
	for _, rule := range profile.Artifacts {
		count := rule.MaximumFiles
		if count < 1 || rule.MaximumBytes < 0 ||
			rule.MaximumBytes > (1<<63-1)/int64(count) {
			return 0, ErrInvalidRuntimeExecution
		}
		value := rule.MaximumBytes * int64(count)
		if value > (1<<63-1)-total {
			return 0, ErrInvalidRuntimeExecution
		}
		total += value
	}
	if total < 1 || fileLimit < 1 {
		return 0, ErrInvalidRuntimeExecution
	}
	if total > (1<<63-1)-metadataAllowance ||
		fileLimit > (1<<63-1)-metadataAllowance {
		return 0, ErrInvalidRuntimeExecution
	}
	maximum := min(total+metadataAllowance, fileLimit+metadataAllowance)
	if maximum < 1024 {
		return 0, fmt.Errorf("%w: archive bound", ErrInvalidRuntimeExecution)
	}
	return maximum, nil
}
