package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	judgedb "github.com/huynhanx03/judgify/internal/judge/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/judge/capability"
	"github.com/huynhanx03/judgify/internal/judge/runtimecatalog"
	"github.com/huynhanx03/judgify/internal/operator/releasemanifest"
)

const judgeCapabilityReceiptSchema = "judgify.judge-capability-receipt.v1"

var errJudgeCapabilityCommandDenied = errors.New("judge capability command denied")

type operatorIdentity struct {
	actorID uuid.UUID
	auditID uuid.UUID
	reason  string
}

type activationReceipt struct {
	SchemaVersion                   string                     `json:"schema_version"`
	Action                          string                     `json:"action"`
	CID                             string                     `json:"cid"`
	ActivationID                    uuid.UUID                  `json:"activation_id"`
	ReleaseID                       string                     `json:"release_id"`
	ApprovedReleaseManifestChecksum string                     `json:"approved_release_manifest_checksum"`
	CapabilityProfileHash           string                     `json:"capability_profile_hash"`
	RuntimeKey                      string                     `json:"runtime_key"`
	RuntimeProfileVersion           string                     `json:"runtime_profile_version"`
	CheckerKey                      string                     `json:"checker_key"`
	CheckerVersion                  string                     `json:"checker_version"`
	CheckerImplementationChecksum   string                     `json:"checker_implementation_checksum"`
	SmokeChecksum                   string                     `json:"smoke_checksum"`
	GoldenChecksum                  string                     `json:"golden_checksum"`
	ProofWorkerInstanceID           uuid.UUID                  `json:"proof_worker_instance_id"`
	ProofBootID                     uuid.UUID                  `json:"proof_boot_id"`
	ProofEvidenceID                 uuid.UUID                  `json:"proof_evidence_id"`
	State                           capability.ActivationState `json:"state"`
	ActivatedAt                     time.Time                  `json:"activated_at"`
	ActorID                         uuid.UUID                  `json:"actor_id"`
	AuditID                         uuid.UUID                  `json:"audit_id"`
	Reason                          string                     `json:"reason"`
}

type retirementReceipt struct {
	SchemaVersion string                     `json:"schema_version"`
	Action        string                     `json:"action"`
	CID           string                     `json:"cid"`
	ActivationID  uuid.UUID                  `json:"activation_id"`
	State         capability.ActivationState `json:"state"`
	RetiredAt     time.Time                  `json:"retired_at"`
	ActorID       uuid.UUID                  `json:"actor_id"`
	AuditID       uuid.UUID                  `json:"audit_id"`
	Reason        string                     `json:"reason"`
}

type judgeCapabilityOperator struct {
	ctx     context.Context
	process *bootstrap.Process
	service *capability.ActivationService
}

func runJudgeCapability(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) == 0 {
		writeJudgeCapabilityUsage(stderr)
		return 2
	}
	switch args[0] {
	case "activate":
		return runJudgeCapabilityActivate(ctx, args[1:], stdout, stderr)
	case "retire":
		return runJudgeCapabilityRetire(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		writeJudgeCapabilityUsage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "judgify judge-capability: unknown command %q\n", args[0])
		writeJudgeCapabilityUsage(stderr)
		return 2
	}
}

func runJudgeCapabilityActivate(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("judgify judge-capability activate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "absolute approved release manifest path")
	releaseID := flags.String("release-id", "", "exact approved release identifier")
	profileHash := flags.String("profile-hash", "", "exact approved capability profile sha256")
	actorIDText := flags.String("actor", "", "operator UUID")
	auditIDText := flags.String("audit-id", "", "external audit UUID")
	reason := flags.String("reason", "", "reviewed activation reason")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	identity, err := parseOperatorIdentity(*actorIDText, *auditIDText, *reason)
	if flags.NArg() != 0 || *manifestPath == "" || *releaseID == "" ||
		*profileHash == "" || err != nil || ctx == nil || ctx.Err() != nil {
		_, _ = fmt.Fprintln(
			stderr,
			"judge-capability activate: --manifest, --release-id, --profile-hash, --actor, --audit-id, and --reason are required",
		)
		return 2
	}

	manifest, err := releasemanifest.ReadApproved(ctx, *manifestPath, *releaseID)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "judge-capability activate: approved manifest denied")
		return 1
	}
	catalog, err := runtimecatalog.LoadApprovedRelease(manifest)
	if err != nil || catalog.ReleaseID() != *releaseID {
		_, _ = fmt.Fprintln(stderr, "judge-capability activate: approved release catalog denied")
		return 1
	}
	tuple, approved := catalog.ApprovedTuple(*profileHash)
	if !approved {
		_, _ = fmt.Fprintln(stderr, "judge-capability activate: profile is not approved by the release")
		return 1
	}

	operator, err := newJudgeCapabilityOperator(ctx, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "judge-capability activate: operator bootstrap denied")
		return 1
	}
	activated, mutationErr := operator.service.Activate(operator.ctx, capability.ActivationRequest{
		ReleaseID:                       catalog.ReleaseID(),
		ApprovedReleaseManifestChecksum: catalog.ApprovalChecksum(),
		ActorID:                         identity.actorID, AuditID: identity.auditID,
		Reason: identity.reason, Tuple: tuple,
	}, time.Now().UTC())
	if mutationErr != nil {
		operator.logFailure("activate", mutationErr, identity, uuid.Nil, *releaseID, *profileHash)
	} else {
		operator.logSuccess("activate", activated, identity)
	}
	closeErr := operator.close()
	if mutationErr != nil || closeErr != nil {
		_, _ = fmt.Fprintf(
			stderr,
			"judge-capability activate: denied; cid=%s\n",
			correlation.FromContext(operator.ctx),
		)
		return 1
	}

	receipt := activationReceipt{
		SchemaVersion: judgeCapabilityReceiptSchema, Action: "activate",
		CID:          correlation.FromContext(operator.ctx),
		ActivationID: activated.ID, ReleaseID: activated.ReleaseID,
		ApprovedReleaseManifestChecksum: activated.ApprovedReleaseManifestChecksum,
		CapabilityProfileHash:           activated.Tuple.CapabilityProfileHash,
		RuntimeKey:                      activated.Tuple.RuntimeKey,
		RuntimeProfileVersion:           activated.Tuple.RuntimeProfileVersion,
		CheckerKey:                      activated.Tuple.CheckerKey, CheckerVersion: activated.Tuple.CheckerVersion,
		CheckerImplementationChecksum: activated.CheckerImplementationChecksum,
		SmokeChecksum:                 activated.SmokeChecksum, GoldenChecksum: activated.GoldenChecksum,
		ProofWorkerInstanceID: activated.ProofWorkerInstanceID,
		ProofBootID:           activated.ProofBootID, ProofEvidenceID: activated.ProofEvidenceID,
		State: activated.State, ActivatedAt: activated.ActivatedAt,
		ActorID: activated.ActorID, AuditID: activated.AuditID, Reason: activated.Reason,
	}
	if err := writeJudgeCapabilityReceipt(stdout, receipt); err != nil {
		_, _ = fmt.Fprintf(
			stderr,
			"judge-capability activate: receipt write failed; activation_id=%s cid=%s\n",
			activated.ID,
			correlation.FromContext(operator.ctx),
		)
		return 1
	}
	return 0
}

func runJudgeCapabilityRetire(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("judgify judge-capability retire", flag.ContinueOnError)
	flags.SetOutput(stderr)
	activationIDText := flags.String("activation-id", "", "durable activation UUID")
	actorIDText := flags.String("actor", "", "operator UUID")
	auditIDText := flags.String("audit-id", "", "external audit UUID")
	reason := flags.String("reason", "", "reviewed retirement reason")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	activationID, activationIDErr := parseCanonicalUUID(*activationIDText)
	identity, identityErr := parseOperatorIdentity(*actorIDText, *auditIDText, *reason)
	if flags.NArg() != 0 || activationIDErr != nil || identityErr != nil ||
		ctx == nil || ctx.Err() != nil {
		_, _ = fmt.Fprintln(
			stderr,
			"judge-capability retire: --activation-id, --actor, --audit-id, and --reason are required",
		)
		return 2
	}

	operator, err := newJudgeCapabilityOperator(ctx, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "judge-capability retire: operator bootstrap denied")
		return 1
	}
	retiredAt := time.Now().UTC()
	mutationErr := operator.service.Retire(operator.ctx, capability.RetirementRequest{
		ActivationID: activationID, ActorID: identity.actorID,
		AuditID: identity.auditID, Reason: identity.reason,
	}, retiredAt)
	if mutationErr != nil {
		operator.logFailure("retire", mutationErr, identity, activationID, "", "")
	} else {
		operator.logSuccess("retire", capability.ActivatedCapability{
			ID: activationID, State: capability.ActivationStateRetired, RetiredAt: &retiredAt,
		}, identity)
	}
	closeErr := operator.close()
	if mutationErr != nil || closeErr != nil {
		_, _ = fmt.Fprintf(
			stderr,
			"judge-capability retire: denied; cid=%s\n",
			correlation.FromContext(operator.ctx),
		)
		return 1
	}

	receipt := retirementReceipt{
		SchemaVersion: judgeCapabilityReceiptSchema, Action: "retire",
		CID: correlation.FromContext(operator.ctx), ActivationID: activationID,
		State: capability.ActivationStateRetired, RetiredAt: retiredAt,
		ActorID: identity.actorID, AuditID: identity.auditID, Reason: identity.reason,
	}
	if err := writeJudgeCapabilityReceipt(stdout, receipt); err != nil {
		_, _ = fmt.Fprintf(
			stderr,
			"judge-capability retire: receipt write failed; activation_id=%s cid=%s\n",
			activationID,
			correlation.FromContext(operator.ctx),
		)
		return 1
	}
	return 0
}

func newJudgeCapabilityOperator(
	ctx context.Context,
	logWriter io.Writer,
) (*judgeCapabilityOperator, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errJudgeCapabilityCommandDenied
	}
	ctx = correlation.EnsureContext(ctx)
	process, err := bootstrap.PrepareForRole(bootstrap.ProfileOperator)
	if err != nil {
		return nil, errors.Join(errJudgeCapabilityCommandDenied, err)
	}
	ctx = commonlogger.WithContext(ctx, newJudgeCapabilityLogger(process, logWriter))
	database, err := process.OpenDatabase()
	if err != nil {
		return nil, errors.Join(
			errJudgeCapabilityCommandDenied,
			err,
			process.Close(),
			process.Sync(),
		)
	}
	store, err := judgedb.NewPlatformActivationStore(database)
	if err != nil {
		return nil, errors.Join(
			errJudgeCapabilityCommandDenied,
			err,
			process.Close(),
			process.Sync(),
		)
	}
	service, err := capability.NewActivationService(
		store,
		process.Config.Judge.WorkerHeartbeatFreshness,
	)
	if err != nil {
		return nil, errors.Join(
			errJudgeCapabilityCommandDenied,
			err,
			process.Close(),
			process.Sync(),
		)
	}
	return &judgeCapabilityOperator{ctx: ctx, process: process, service: service}, nil
}

func (operator *judgeCapabilityOperator) close() error {
	if operator == nil || operator.process == nil {
		return errJudgeCapabilityCommandDenied
	}
	return errors.Join(operator.process.Close(), operator.process.Sync())
}

func (operator *judgeCapabilityOperator) logFailure(
	action string,
	cause error,
	identity operatorIdentity,
	activationID uuid.UUID,
	releaseID string,
	profileHash string,
) {
	if operator == nil {
		return
	}
	commonlogger.FromContext(operator.ctx).Error(
		"judge capability operator command denied",
		zap.String("action", action),
		zap.String("activation_id", activationID.String()),
		zap.String("release_id", releaseID),
		zap.String("capability_profile_hash", profileHash),
		zap.String("actor_id", identity.actorID.String()),
		zap.String("audit_id", identity.auditID.String()),
		zap.String("cause_type", judgeCapabilityFailureType(cause)),
	)
}

func judgeCapabilityFailureType(cause error) string {
	switch {
	case errors.Is(cause, capability.ErrActivationHasNonTerminalJobs):
		return "non_terminal_jobs"
	case errors.Is(cause, judgedb.ErrInvalidPlatformActivationStore):
		return "persistence_denied"
	case errors.Is(cause, capability.ErrPlatformActivationDenied):
		return "activation_denied"
	default:
		return "unknown_denied"
	}
}

func (operator *judgeCapabilityOperator) logSuccess(
	action string,
	activation capability.ActivatedCapability,
	identity operatorIdentity,
) {
	if operator == nil {
		return
	}
	commonlogger.FromContext(operator.ctx).Info(
		"judge capability operator command committed",
		zap.String("action", action),
		zap.String("activation_id", activation.ID.String()),
		zap.String("release_id", activation.ReleaseID),
		zap.String("capability_profile_hash", activation.Tuple.CapabilityProfileHash),
		zap.String("actor_id", identity.actorID.String()),
		zap.String("audit_id", identity.auditID.String()),
		zap.String("state", string(activation.State)),
	)
}

func newJudgeCapabilityLogger(
	process *bootstrap.Process,
	logWriter io.Writer,
) *zap.Logger {
	if process == nil || process.Logger == nil || process.Logger.Logger == nil ||
		logWriter == nil {
		if process == nil || process.Logger == nil || process.Logger.Logger == nil {
			return zap.NewNop()
		}
		return process.Logger.Named("judge-capability")
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.MessageKey = "message"
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(logWriter),
		process.Logger.Level(),
	)
	return zap.New(core).Named("judge-capability")
}

func parseOperatorIdentity(
	actorIDText string,
	auditIDText string,
	reason string,
) (operatorIdentity, error) {
	actorID, actorErr := parseCanonicalUUID(actorIDText)
	auditID, auditErr := parseCanonicalUUID(auditIDText)
	if actorErr != nil || auditErr != nil || reason == "" ||
		reason != strings.TrimSpace(reason) {
		return operatorIdentity{}, errJudgeCapabilityCommandDenied
	}
	return operatorIdentity{actorID: actorID, auditID: auditID, reason: reason}, nil
}

func parseCanonicalUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, errJudgeCapabilityCommandDenied
	}
	return parsed, nil
}

func writeJudgeCapabilityReceipt(writer io.Writer, receipt any) error {
	if writer == nil {
		return errJudgeCapabilityCommandDenied
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(true)
	encoder.SetIndent("", "  ")
	return encoder.Encode(receipt)
}

func writeJudgeCapabilityUsage(writer io.Writer) {
	lines := []string{
		"Usage: judgify judge-capability activate --manifest /absolute/release.json --release-id ID \\",
		"         --profile-hash sha256:... --actor UUID --audit-id UUID --reason TEXT",
		"       judgify judge-capability retire --activation-id UUID --actor UUID \\",
		"         --audit-id UUID --reason TEXT",
	}
	_, _ = fmt.Fprintln(writer, strings.Join(lines, "\n"))
}
