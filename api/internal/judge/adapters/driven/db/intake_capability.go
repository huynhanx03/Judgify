package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	judgecapability "github.com/huynhanx03/judgify/internal/judge/capability"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	"github.com/huynhanx03/judgify/pkg/judging"
)

var (
	intakeCapabilityIdentifier = regexp.MustCompile(
		`^[a-z0-9][a-z0-9._-]{0,127}$`,
	)
	intakeCapabilityDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type SubmissionCapabilityConfig struct {
	ReleaseID                       string
	ApprovedReleaseManifestChecksum string
	MaximumReadyBacklog             int
	MaximumOldestReadyAge           time.Duration
	MinimumFreeDiskBytes            uint64
	AdmissionDiskPath               string
}

// FreeDiskProbe is deliberately tiny so admission can be tested without
// coupling the application workflow to a filesystem implementation.
type FreeDiskProbe interface {
	FreeBytes(context.Context, string) (uint64, error)
}

type SubmissionCapabilityResolver struct {
	client *dbent.EntClient
	config SubmissionCapabilityConfig
	disk   FreeDiskProbe
}

func NewSubmissionCapabilityResolver(
	client *dbent.EntClient,
	config SubmissionCapabilityConfig,
) (*SubmissionCapabilityResolver, error) {
	if client == nil ||
		!intakeCapabilityIdentifier.MatchString(config.ReleaseID) ||
		!validCapabilityDigest(config.ApprovedReleaseManifestChecksum) ||
		config.MaximumReadyBacklog < 1 ||
		config.MaximumOldestReadyAge <= 0 ||
		config.MinimumFreeDiskBytes < 1 ||
		config.AdmissionDiskPath == "" ||
		config.AdmissionDiskPath != strings.TrimSpace(config.AdmissionDiskPath) {
		return nil, intake.ErrCapabilityUnavailable
	}
	return &SubmissionCapabilityResolver{
		client: client,
		config: config,
		disk:   unixFreeDiskProbe{},
	}, nil
}

func (resolver *SubmissionCapabilityResolver) Resolve(
	ctx context.Context,
	request intake.CapabilityRequest,
) (intake.CapabilitySnapshot, error) {
	if resolver == nil || resolver.client == nil || resolver.disk == nil ||
		ctx == nil || !validCapabilityRequest(request) {
		return intake.CapabilitySnapshot{}, intake.ErrCapabilityUnavailable
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return intake.CapabilitySnapshot{}, err
	}
	client := resolver.client.DB(ctx)
	rows, err := client.ActivatedJudgeCapability.QueryContext(
		ctx,
		exactActivatedCapabilitySQL,
		resolver.config.ReleaseID,
		resolver.config.ApprovedReleaseManifestChecksum,
		request.RuntimeKey,
		request.CheckerKey,
		request.CheckerVersion,
	)
	if err != nil {
		return intake.CapabilitySnapshot{}, fmt.Errorf(
			"lock activated submission capability: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return intake.CapabilitySnapshot{}, fmt.Errorf(
				"lock activated submission capability: %w",
				err,
			)
		}
		return intake.CapabilitySnapshot{}, intake.ErrCapabilityUnavailable
	}
	var result intake.CapabilitySnapshot
	if err := rows.Scan(
		&result.ReleaseID,
		&result.ApprovedReleaseManifestChecksum,
		&result.Language,
		&result.JudgeSpecProtocol,
		&result.RuntimeProfileVersion,
		&result.CompilerVersion,
		&result.RuntimeBootstrapVersion,
		&result.SandboxImageDigest,
		&result.CompileCommandID,
		&result.RunCommandID,
		&result.ArtifactManifestVersion,
		&result.ArtifactManifestChecksum,
		&result.ArtifactManifest,
		&result.SupervisorProtocolVersion,
		&result.CapabilityProfileHash,
		&result.CheckerImplementationChecksum,
		&result.CompileCPUTimeMilliseconds,
		&result.CompileWallTimeMilliseconds,
	); err != nil {
		return intake.CapabilitySnapshot{}, fmt.Errorf(
			"scan activated submission capability: %w",
			err,
		)
	}
	if rows.Next() {
		return intake.CapabilitySnapshot{}, intake.ErrCapabilityUnavailable
	}
	if err := rows.Err(); err != nil {
		return intake.CapabilitySnapshot{}, fmt.Errorf(
			"read activated submission capability: %w",
			err,
		)
	}
	if err := rows.Close(); err != nil {
		return intake.CapabilitySnapshot{}, fmt.Errorf(
			"close activated submission capability: %w",
			err,
		)
	}
	if !validCapabilitySnapshot(result) ||
		!capabilitySnapshotMatchesRequest(resolver.config, request, result) {
		return intake.CapabilitySnapshot{}, intake.ErrCapabilityUnavailable
	}
	if err := lockBacklogAdmission(ctx, client); err != nil {
		return intake.CapabilitySnapshot{}, err
	}
	if err := resolver.requireBacklogCapacity(ctx, client); err != nil {
		return intake.CapabilitySnapshot{}, err
	}
	freeBytes, err := resolver.disk.FreeBytes(
		ctx,
		resolver.config.AdmissionDiskPath,
	)
	if err != nil {
		return intake.CapabilitySnapshot{}, errors.Join(
			intake.ErrCapabilityUnavailable,
			fmt.Errorf("measure judge admission disk: %w", err),
		)
	}
	if freeBytes < resolver.config.MinimumFreeDiskBytes {
		return intake.CapabilitySnapshot{}, intake.ErrCapabilityUnavailable
	}
	return result, nil
}

const exactActivatedCapabilitySQL = `
SELECT activation.release_id,
       activation.approved_release_manifest_checksum,
       profile.language,
       activation.judge_spec_protocol,
       profile.profile_version,
       profile.compiler_version,
       profile.runtime_bootstrap_version,
       profile.sandbox_image_digest,
       profile.compile_command_id,
       profile.run_command_id,
       profile.manifest_version,
       profile.manifest_checksum,
       profile.artifact_manifest,
       activation.supervisor_protocol,
       activation.capability_profile_hash,
		activation.checker_implementation_checksum,
       profile.default_compile_timeout_ms,
       profile.default_compile_timeout_ms
FROM activated_judge_capabilities AS activation
JOIN runtime_profiles AS profile
  ON profile.runtime_key = activation.runtime_key
 AND profile.profile_version = activation.runtime_profile_version
 AND profile.compiler_version = activation.compiler_version
 AND profile.runtime_bootstrap_version = activation.runtime_bootstrap_version
 AND profile.compile_command_id = activation.compile_command_id
 AND profile.run_command_id = activation.run_command_id
 AND profile.sandbox_image_digest = activation.image_digest
 AND profile.manifest_version = activation.manifest_version
 AND profile.manifest_checksum = activation.manifest_checksum
WHERE activation.release_id = $1
  AND activation.approved_release_manifest_checksum = $2
  AND activation.state = 'active'
  AND activation.retired_at IS NULL
  AND profile.active
  AND activation.runtime_key = $3
  AND activation.checker_key = $4
  AND activation.checker_version = $5
FOR SHARE OF activation, profile`

func lockBacklogAdmission(
	ctx context.Context,
	client *generate.Client,
) error {
	rows, err := client.ActivatedJudgeCapability.QueryContext(
		ctx,
		`SELECT pg_advisory_xact_lock($1)`,
		submissioncontract.JudgeBacklogAdmissionAdvisoryLock,
	)
	if err != nil {
		return fmt.Errorf("lock judge backlog admission: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("lock judge backlog admission: %w", err)
		}
		return intake.ErrPersistence
	}
	return rows.Err()
}

func (resolver *SubmissionCapabilityResolver) requireBacklogCapacity(
	ctx context.Context,
	client *generate.Client,
) error {
	rows, err := client.ActivatedJudgeCapability.QueryContext(ctx, `
SELECT COUNT(*), MIN(created_at), clock_timestamp()
FROM judge_jobs
WHERE state IN ('ready', 'retry_wait')`)
	if err != nil {
		return fmt.Errorf("read judge backlog admission: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read judge backlog admission: %w", err)
		}
		return intake.ErrPersistence
	}
	var count int
	var oldest *time.Time
	var now time.Time
	if err := rows.Scan(&count, &oldest, &now); err != nil {
		return fmt.Errorf("scan judge backlog admission: %w", err)
	}
	if rows.Next() {
		return intake.ErrPersistence
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read judge backlog admission: %w", err)
	}
	if count < 0 || now.IsZero() ||
		(count == 0 && oldest != nil) ||
		(count > 0 && oldest == nil) {
		return intake.ErrPersistence
	}
	if count >= resolver.config.MaximumReadyBacklog {
		return intake.ErrCapabilityUnavailable
	}
	if oldest != nil {
		oldestUTC := oldest.UTC()
		nowUTC := now.UTC()
		if oldestUTC.After(nowUTC) {
			return intake.ErrPersistence
		}
		if nowUTC.Sub(oldestUTC) >= resolver.config.MaximumOldestReadyAge {
			return intake.ErrCapabilityUnavailable
		}
	}
	return nil
}

func validCapabilityRequest(request intake.CapabilityRequest) bool {
	return intakeCapabilityIdentifier.MatchString(request.RuntimeKey) &&
		intakeCapabilityIdentifier.MatchString(request.CheckerKey) &&
		intakeCapabilityIdentifier.MatchString(request.CheckerVersion)
}

func validCapabilitySnapshot(snapshot intake.CapabilitySnapshot) bool {
	language, supported := judging.ParseLanguage(snapshot.Language)
	return supported &&
		string(language) == snapshot.Language &&
		intakeCapabilityIdentifier.MatchString(snapshot.ReleaseID) &&
		validCapabilityDigest(snapshot.ApprovedReleaseManifestChecksum) &&
		intakeCapabilityIdentifier.MatchString(snapshot.JudgeSpecProtocol) &&
		intakeCapabilityIdentifier.MatchString(snapshot.RuntimeProfileVersion) &&
		intakeCapabilityIdentifier.MatchString(snapshot.CompilerVersion) &&
		intakeCapabilityIdentifier.MatchString(snapshot.RuntimeBootstrapVersion) &&
		validCapabilityDigest(snapshot.SandboxImageDigest) &&
		intakeCapabilityIdentifier.MatchString(snapshot.CompileCommandID) &&
		intakeCapabilityIdentifier.MatchString(snapshot.RunCommandID) &&
		intakeCapabilityIdentifier.MatchString(snapshot.ArtifactManifestVersion) &&
		validCapabilityDigest(snapshot.ArtifactManifestChecksum) &&
		jsonArray(snapshot.ArtifactManifest) &&
		intakeCapabilityIdentifier.MatchString(
			snapshot.SupervisorProtocolVersion,
		) &&
		validCapabilityDigest(snapshot.CapabilityProfileHash) &&
		validCapabilityDigest(snapshot.CheckerImplementationChecksum) &&
		snapshot.CompileCPUTimeMilliseconds > 0 &&
		snapshot.CompileCPUTimeMilliseconds <=
			snapshot.CompileWallTimeMilliseconds
}

// capabilitySnapshotMatchesRequest independently reconstructs the tuple hash
// after the database read. The SQL predicate remains the primary selection
// fence; this second fence prevents query drift or a malformed projection row
// from creating work that no exact worker can ever claim.
func capabilitySnapshotMatchesRequest(
	config SubmissionCapabilityConfig,
	request intake.CapabilityRequest,
	snapshot intake.CapabilitySnapshot,
) bool {
	if snapshot.ReleaseID != config.ReleaseID ||
		snapshot.ApprovedReleaseManifestChecksum !=
			config.ApprovedReleaseManifestChecksum {
		return false
	}
	tuple := judgecapability.CapabilityTuple{
		JudgeSpecProtocol:             snapshot.JudgeSpecProtocol,
		RuntimeKey:                    request.RuntimeKey,
		RuntimeProfileVersion:         snapshot.RuntimeProfileVersion,
		CompilerVersion:               snapshot.CompilerVersion,
		RuntimeBootstrapVersion:       snapshot.RuntimeBootstrapVersion,
		CompileCommandID:              snapshot.CompileCommandID,
		RunCommandID:                  snapshot.RunCommandID,
		ImageDigest:                   snapshot.SandboxImageDigest,
		ManifestVersion:               snapshot.ArtifactManifestVersion,
		ManifestChecksum:              snapshot.ArtifactManifestChecksum,
		SupervisorProtocol:            snapshot.SupervisorProtocolVersion,
		CheckerKey:                    request.CheckerKey,
		CheckerVersion:                request.CheckerVersion,
		CheckerImplementationChecksum: snapshot.CheckerImplementationChecksum,
	}
	want, err := judgecapability.ComputeCapabilityProfileHash(tuple)
	return err == nil && want == snapshot.CapabilityProfileHash
}

func jsonArray(value json.RawMessage) bool {
	var document any
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&document) != nil {
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return false
	}
	values, array := document.([]any)
	return array && len(values) > 0
}

func validCapabilityDigest(value string) bool {
	if !intakeCapabilityDigest.MatchString(value) {
		return false
	}
	body := value[len("sha256:"):]
	return body != strings.Repeat("0", len(body))
}

type unixFreeDiskProbe struct{}

func (unixFreeDiskProbe) FreeBytes(
	ctx context.Context,
	path string,
) (uint64, error) {
	if ctx == nil || path == "" {
		return 0, intake.ErrCapabilityUnavailable
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var filesystem unix.Statfs_t
	if err := unix.Statfs(path, &filesystem); err != nil {
		return 0, err
	}
	if filesystem.Bsize <= 0 {
		return 0, intake.ErrCapabilityUnavailable
	}
	blockSize := uint64(filesystem.Bsize)
	if filesystem.Bavail > math.MaxUint64/blockSize {
		return 0, intake.ErrCapabilityUnavailable
	}
	return filesystem.Bavail * blockSize, nil
}

var _ intake.CapabilityResolver = (*SubmissionCapabilityResolver)(nil)
