package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedchecker "github.com/huynhanx03/judgify/internal/ent/generate/checkerprofile"
	generatedruntime "github.com/huynhanx03/judgify/internal/ent/generate/runtimeprofile"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var ErrReleaseProfileInstall = errors.New("release profile install: rejected")

// ReleaseProfileInstaller projects reviewed release identities into the
// database for API/admin selection. Records are append-or-verify only: an
// already-existing identity must compare exactly and is never updated.
type ReleaseProfileInstaller struct {
	client *internalent.EntClient
}

func NewReleaseProfileInstaller(client *internalent.EntClient) (*ReleaseProfileInstaller, error) {
	if client == nil || client.Client() == nil {
		return nil, ErrReleaseProfileInstall
	}
	return &ReleaseProfileInstaller{client: client}, nil
}

func (installer *ReleaseProfileInstaller) InstallAndVerify(
	ctx context.Context,
	runtimes []problementity.RuntimeProfile,
	checkers []problementity.CheckerProfile,
) error {
	if installer == nil || installer.client == nil || ctx == nil || ctx.Err() != nil ||
		len(runtimes) == 0 || len(checkers) == 0 {
		return ErrReleaseProfileInstall
	}
	for _, runtime := range runtimes {
		if err := installer.installRuntime(ctx, runtime); err != nil {
			return err
		}
	}
	for _, checker := range checkers {
		if err := installer.installChecker(ctx, checker); err != nil {
			return err
		}
	}
	return nil
}

func (installer *ReleaseProfileInstaller) installRuntime(
	ctx context.Context,
	profile problementity.RuntimeProfile,
) error {
	artifactManifest, err := json.Marshal(profile.Artifacts)
	if err != nil {
		return errors.Join(ErrReleaseProfileInstall, err)
	}
	client := installer.client.Client()
	record, err := client.RuntimeProfile.Query().Where(
		generatedruntime.RuntimeKeyEQ(profile.Key),
		generatedruntime.ProfileVersionEQ(profile.ProfileVersion),
	).Only(ctx)
	if generate.IsNotFound(err) {
		record, err = client.RuntimeProfile.Create().
			SetID(profile.ID).SetRuntimeKey(profile.Key).SetLanguage(profile.Language).
			SetDisplayName(profile.DisplayName).SetProfileVersion(profile.ProfileVersion).
			SetCompilerVersion(profile.CompilerVersion).
			SetRuntimeBootstrapVersion(profile.RuntimeBootstrapVersion).
			SetSandboxImageDigest(profile.ImageDigest).
			SetCompileCommandID(profile.CompileCommandID).SetRunCommandID(profile.RunCommandID).
			SetDefaultCompileTimeoutMs(int(profile.DefaultCompileTimeout.Milliseconds())).
			SetManifestVersion(profile.ManifestVersion).SetManifestChecksum(profile.ManifestChecksum).
			SetArtifactManifest(artifactManifest).SetActive(profile.Active).Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("%w: install runtime %s/%s: %v", ErrReleaseProfileInstall, profile.Key, profile.ProfileVersion, err)
	}
	if !sameRuntimeProjection(record, profile, artifactManifest) {
		return fmt.Errorf("%w: runtime identity conflicts %s/%s", ErrReleaseProfileInstall, profile.Key, profile.ProfileVersion)
	}
	return nil
}

func (installer *ReleaseProfileInstaller) installChecker(
	ctx context.Context,
	profile problementity.CheckerProfile,
) error {
	client := installer.client.Client()
	record, err := client.CheckerProfile.Query().Where(
		generatedchecker.CheckerKeyEQ(profile.Key),
		generatedchecker.CheckerVersionEQ(profile.Version),
	).Only(ctx)
	if generate.IsNotFound(err) {
		record, err = client.CheckerProfile.Create().
			SetID(profile.ID).SetCheckerKey(profile.Key).SetCheckerVersion(profile.Version).
			SetImplementationChecksum(profile.ImplementationChecksum).
			SetConfigKind(generatedchecker.ConfigKind(profile.ConfigKind)).
			SetActive(profile.Active).Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("%w: install checker %s/%s: %v", ErrReleaseProfileInstall, profile.Key, profile.Version, err)
	}
	if record.ID != profile.ID || record.CheckerKey != profile.Key ||
		record.CheckerVersion != profile.Version ||
		record.ImplementationChecksum != profile.ImplementationChecksum ||
		record.ConfigKind != generatedchecker.ConfigKind(profile.ConfigKind) ||
		record.Active != profile.Active {
		return fmt.Errorf("%w: checker identity conflicts %s/%s", ErrReleaseProfileInstall, profile.Key, profile.Version)
	}
	return nil
}

func sameRuntimeProjection(
	record *generate.RuntimeProfile,
	profile problementity.RuntimeProfile,
	wantArtifacts []byte,
) bool {
	if record == nil || record.ID != profile.ID || record.RuntimeKey != profile.Key ||
		record.Language != profile.Language || record.DisplayName != profile.DisplayName ||
		record.ProfileVersion != profile.ProfileVersion ||
		record.CompilerVersion != profile.CompilerVersion ||
		record.RuntimeBootstrapVersion != profile.RuntimeBootstrapVersion ||
		record.SandboxImageDigest != profile.ImageDigest ||
		record.CompileCommandID != profile.CompileCommandID || record.RunCommandID != profile.RunCommandID ||
		record.DefaultCompileTimeoutMs != int(profile.DefaultCompileTimeout.Milliseconds()) ||
		record.ManifestVersion != profile.ManifestVersion || record.ManifestChecksum != profile.ManifestChecksum ||
		record.Active != profile.Active {
		return false
	}
	var got, want []problementity.RuntimeArtifact
	return json.Unmarshal(record.ArtifactManifest, &got) == nil &&
		json.Unmarshal(wantArtifacts, &want) == nil && slices.Equal(got, want) &&
		bytes.Equal(mustCanonicalJSON(got), mustCanonicalJSON(want))
}

func mustCanonicalJSON(value []problementity.RuntimeArtifact) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}
