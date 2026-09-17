package usecase

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"strconv"

	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// ReleaseManifestTemplate returns the executable identities owned by this
// source revision. Artifact and image identities are deliberately supplied by
// the release pipeline; an operator cannot use this function to activate a
// runtime by configuration alone.
func ReleaseManifestTemplate(releaseID string) (ApprovedReleaseManifest, error) {
	if !releaseIdentifierPattern.MatchString(releaseID) {
		return ApprovedReleaseManifest{}, ErrInvalidReleaseManifest
	}
	runtimeProfiles := initialRuntimeTemplates()
	checkerProfiles := initialCheckerTemplates()
	runtimes := make([]RuntimeRelease, 0, len(runtimeProfiles))
	for _, profile := range runtimeProfiles {
		checksum, err := ComputeRuntimeManifestChecksum(profile)
		if err != nil {
			return ApprovedReleaseManifest{}, err
		}
		runtimes = append(runtimes, RuntimeRelease{
			RuntimeKey: profile.Key, ProfileVersion: profile.ProfileVersion,
			CompilerVersion:         profile.CompilerVersion,
			RuntimeBootstrapVersion: profile.RuntimeBootstrapVersion,
			CompileCommandID:        profile.CompileCommandID,
			RunCommandID:            profile.RunCommandID,
			ManifestVersion:         profile.ManifestVersion,
			ManifestChecksum:        checksum,
		})
	}
	checkers := make([]CheckerRelease, 0, len(checkerProfiles))
	for _, profile := range checkerProfiles {
		checkers = append(checkers, CheckerRelease{
			CheckerKey: profile.Key, Version: profile.Version,
		})
	}
	return ApprovedReleaseManifest{
		ReleaseID: releaseID,
		Runtimes:  runtimes,
		Checkers:  checkers,
	}, nil
}

// ComputeRuntimeManifestChecksum content-addresses the complete code-owned
// compile, run, workspace, and artifact contract. It intentionally excludes
// the image digest and its own checksum so the value can be embedded as an OCI
// label before the image is built and later verified against the resolved
// immutable image digest.
func ComputeRuntimeManifestChecksum(profile entity.RuntimeProfile) (string, error) {
	if profile.Key == "" || profile.ProfileVersion == "" ||
		profile.CompilerVersion == "" || profile.RuntimeBootstrapVersion == "" ||
		profile.CompileCommandID == "" || profile.RunCommandID == "" ||
		profile.ManifestVersion == "" || profile.DefaultCompileTimeout <= 0 ||
		len(profile.Artifacts) == 0 {
		return "", ErrInvalidReleaseManifest
	}
	digest := sha256.New()
	writeRuntimeManifestPart(digest, "judgify.runtime-artifact-manifest.v1")
	for _, value := range []string{
		profile.Key,
		profile.ProfileVersion,
		profile.CompilerVersion,
		profile.RuntimeBootstrapVersion,
		profile.CompileCommandID,
		profile.RunCommandID,
		profile.ManifestVersion,
		strconv.FormatInt(profile.DefaultCompileTimeout.Nanoseconds(), 10),
	} {
		writeRuntimeManifestPart(digest, value)
	}
	writeRuntimeManifestPart(digest, "compile")
	writeRuntimeCommand(digest, profile.Compile)
	writeRuntimeManifestPart(digest, "run")
	writeRuntimeCommand(digest, profile.Run)
	writeRuntimeManifestPart(digest, "workspace")
	writeRuntimeManifestPart(digest, profile.Workspace.SourcePath)
	writeRuntimeManifestPart(digest, strconv.Itoa(len(profile.Workspace.Values)))
	for _, value := range profile.Workspace.Values {
		writeRuntimeManifestPart(digest, value.Name)
		writeRuntimeManifestPart(digest, value.Path)
	}
	writeRuntimeManifestPart(digest, strconv.Itoa(len(profile.Workspace.TrustedFiles)))
	for _, file := range profile.Workspace.TrustedFiles {
		writeRuntimeManifestPart(digest, file.Path)
		writeRuntimeManifestPart(digest, file.Content)
	}
	writeRuntimeManifestPart(digest, "artifacts")
	writeRuntimeManifestPart(digest, strconv.Itoa(len(profile.Artifacts)))
	for _, artifact := range profile.Artifacts {
		writeRuntimeManifestPart(digest, artifact.Path)
		writeRuntimeManifestPart(digest, strconv.FormatBool(artifact.Executable))
		writeRuntimeManifestPart(digest, strconv.FormatInt(artifact.MaximumBytes, 10))
		writeRuntimeManifestPart(digest, strconv.Itoa(artifact.MaximumFiles))
		writeRuntimeManifestPart(digest, strconv.FormatBool(artifact.Required))
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), nil
}

func writeRuntimeCommand(digest hash.Hash, command entity.CommandTemplate) {
	writeRuntimeManifestPart(digest, command.Executable)
	writeRuntimeArgument(digest, command.WorkingDirectory)
	writeRuntimeManifestPart(digest, strconv.Itoa(len(command.Arguments)))
	for _, argument := range command.Arguments {
		writeRuntimeArgument(digest, argument)
	}
	writeRuntimeManifestPart(digest, strconv.Itoa(len(command.Environment)))
	for _, variable := range command.Environment {
		writeRuntimeManifestPart(digest, variable.Name)
		writeRuntimeManifestPart(digest, variable.Value)
	}
}

func writeRuntimeArgument(digest hash.Hash, argument entity.CommandArgument) {
	writeRuntimeManifestPart(digest, string(argument.Kind))
	writeRuntimeManifestPart(digest, argument.Value)
	writeRuntimeManifestPart(digest, argument.Prefix)
	writeRuntimeManifestPart(digest, argument.Suffix)
}

func writeRuntimeManifestPart(digest hash.Hash, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}
