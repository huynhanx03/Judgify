package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/operator/releasemanifest"
)

func runReleaseManifest(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) == 0 {
		writeReleaseManifestUsage(stderr)
		return 2
	}
	switch args[0] {
	case "template":
		return runReleaseManifestTemplate(ctx, args[1:], stdout, stderr)
	case "sandbox-env":
		return runReleaseManifestSandboxEnv(ctx, args[1:], stdout, stderr)
	case "approve":
		return runReleaseManifestApprove(ctx, args[1:], stdout, stderr)
	case "verify":
		return runReleaseManifestVerify(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		writeReleaseManifestUsage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "judgify release-manifest: unknown command %q\n", args[0])
		writeReleaseManifestUsage(stderr)
		return 2
	}
}

func runReleaseManifestTemplate(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("judgify release-manifest template", flag.ContinueOnError)
	flags.SetOutput(stderr)
	releaseID := flags.String("release-id", "", "immutable release identifier")
	output := flags.String("output", "", "new absolute draft path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || ctx == nil || ctx.Err() != nil ||
		*releaseID == "" || *output == "" {
		_, _ = fmt.Fprintln(stderr, "release-manifest template: --release-id and --output are required")
		return 2
	}
	draft, err := releasemanifest.NewDraft(*releaseID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest template: %v\n", err)
		return 1
	}
	if err := releasemanifest.WriteJSONExclusive(*output, draft, 0o600); err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest template: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "release manifest draft created: %s\n", *output)
	return 0
}

func runReleaseManifestSandboxEnv(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("judgify release-manifest sandbox-env", flag.ContinueOnError)
	flags.SetOutput(stderr)
	draftPath := flags.String("draft", "", "absolute release draft path")
	output := flags.String("output", "", "new absolute Make fragment path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *draftPath == "" || *output == "" {
		_, _ = fmt.Fprintln(stderr, "release-manifest sandbox-env: --draft and --output are required")
		return 2
	}
	draft, err := releasemanifest.ReadDraft(ctx, *draftPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest sandbox-env: %v\n", err)
		return 1
	}
	if err := releasemanifest.WriteSandboxMakefile(*output, draft); err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest sandbox-env: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "sandbox build identities created: %s\n", *output)
	return 0
}

func runReleaseManifestApprove(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("judgify release-manifest approve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	draftPath := flags.String("draft", "", "absolute reviewed draft path")
	output := flags.String("output", "", "new absolute approved manifest path")
	imageDigest := flags.String("sandbox-image-digest", "", "resolved sha256 sandbox image digest")
	approvedByText := flags.String("approved-by", "", "approver UUID")
	auditIDText := flags.String("audit-id", "", "external approval/audit UUID")
	approvedAtText := flags.String("approved-at", "", "approval timestamp in RFC3339")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *draftPath == "" || *output == "" ||
		*imageDigest == "" || *approvedByText == "" ||
		*auditIDText == "" || *approvedAtText == "" {
		_, _ = fmt.Fprintln(
			stderr,
			"release-manifest approve: --draft, --output, --sandbox-image-digest, --approved-by, --audit-id, and --approved-at are required",
		)
		return 2
	}
	approvedBy, approvedByErr := uuid.Parse(*approvedByText)
	auditID, auditIDErr := uuid.Parse(*auditIDText)
	approvedAt, approvedAtErr := time.Parse(time.RFC3339Nano, *approvedAtText)
	if approvedByErr != nil || auditIDErr != nil || approvedAtErr != nil {
		_, _ = fmt.Fprintln(stderr, "release-manifest approve: invalid approval metadata")
		return 2
	}
	draft, err := releasemanifest.ReadDraft(ctx, *draftPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest approve: %v\n", err)
		return 1
	}
	manifest, summary, err := releasemanifest.BuildApproved(ctx, draft, releasemanifest.Approval{
		SandboxImageDigest: *imageDigest,
		ApprovedBy:         approvedBy,
		AuditID:            auditID,
		ApprovedAt:         approvedAt,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest approve: %v\n", err)
		return 1
	}
	if err := releasemanifest.WriteJSONExclusive(*output, manifest, releasemanifest.AuthoredFileMode); err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest approve: %v\n", err)
		return 1
	}
	return writeReleaseManifestSummary(stdout, stderr, "approve", summary)
}

func runReleaseManifestVerify(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("judgify release-manifest verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "absolute approved manifest path")
	releaseID := flags.String("release-id", "", "exact expected release identifier")
	imageDigest := flags.String("sandbox-image-digest", "", "exact expected sha256 sandbox image digest")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *manifestPath == "" || *releaseID == "" || *imageDigest == "" {
		_, _ = fmt.Fprintln(
			stderr,
			"release-manifest verify: --manifest, --release-id, and --sandbox-image-digest are required",
		)
		return 2
	}
	manifest, err := releasemanifest.ReadApproved(ctx, *manifestPath, *releaseID)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "release-manifest verify: manifest denied")
		return 1
	}
	summary, err := releasemanifest.Verify(ctx, manifest, *imageDigest)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "release-manifest verify: manifest denied")
		return 1
	}
	return writeReleaseManifestSummary(stdout, stderr, "verify", summary)
}

func writeReleaseManifestSummary(
	stdout io.Writer,
	stderr io.Writer,
	action string,
	summary releasemanifest.Summary,
) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(true)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		_, _ = fmt.Fprintf(stderr, "release-manifest %s: write summary: %v\n", action, err)
		return 1
	}
	return 0
}

func writeReleaseManifestUsage(writer io.Writer) {
	lines := []string{
		"Usage: judgify release-manifest template --release-id ID --output /absolute/draft.json",
		"       judgify release-manifest sandbox-env --draft /absolute/draft.json --output /absolute/sandbox.mk",
		"       judgify release-manifest approve --draft /absolute/draft.json --output /absolute/release.json \\",
		"         --sandbox-image-digest sha256:... --approved-by UUID --audit-id UUID --approved-at RFC3339",
		"       judgify release-manifest verify --manifest /absolute/release.json --release-id ID \\",
		"         --sandbox-image-digest sha256:...",
	}
	_, _ = fmt.Fprintln(writer, strings.Join(lines, "\n"))
}
