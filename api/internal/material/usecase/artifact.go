package usecase

import (
	"bytes"
	"context"
	"errors"

	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
)

// loadValidatedRevisionArtifact fails closed unless the raw revision, source
// registry row, and sanitized artifact form one exact immutable identity.
func loadValidatedRevisionArtifact(
	ctx context.Context,
	repository contentrender.Repository,
	revision entity.MaterialRevision,
	requireCurrentPipeline bool,
) (contentrender.Artifact, error) {
	if ctx == nil || repository == nil {
		return contentrender.Artifact{}, entity.ErrMaterialInvalidCommand
	}
	if err := revision.Validate(); err != nil {
		return contentrender.Artifact{}, rejectedRevisionArtifact(err)
	}
	normalized, err := contentrender.NormalizeMarkdown(revision.ContentMarkdown)
	if err != nil || normalized.Checksum != revision.NormalizedSourceChecksum ||
		!bytes.Equal(normalized.Bytes, revision.ContentMarkdown) {
		return contentrender.Artifact{}, rejectedRevisionArtifact(
			contentrender.ErrSourceChecksumMismatch,
		)
	}

	source, err := repository.GetSource(ctx, revision.RenderSourceID)
	if err != nil {
		if ctx.Err() != nil {
			return contentrender.Artifact{}, ctx.Err()
		}
		return contentrender.Artifact{}, rejectedRevisionArtifact(err)
	}
	if source.ID != revision.RenderSourceID ||
		source.OwnerRevisionID != revision.ID ||
		source.Kind != revision.RenderSourceKind ||
		source.Checksum != revision.NormalizedSourceChecksum {
		return contentrender.Artifact{}, rejectedRevisionArtifact(
			contentrender.ErrArtifactSourceMismatch,
		)
	}

	artifact, err := repository.GetArtifact(ctx, revision.RenderArtifactID)
	if err != nil {
		if ctx.Err() != nil {
			return contentrender.Artifact{}, ctx.Err()
		}
		return contentrender.Artifact{}, rejectedRevisionArtifact(err)
	}
	if artifact.ID != revision.RenderArtifactID || artifact.Status != contentrender.StatusReady {
		return contentrender.Artifact{}, rejectedRevisionArtifact(
			contentrender.ErrArtifactSourceMismatch,
		)
	}
	if requireCurrentPipeline &&
		(artifact.RendererVersion != contentrender.CurrentRendererVersion ||
			artifact.SanitizerPolicyVersion != contentrender.CurrentSanitizerPolicyVersion) {
		return contentrender.Artifact{}, entity.ErrMaterialArtifactRejected
	}
	if err := artifact.ValidateFor(source); err != nil {
		return contentrender.Artifact{}, rejectedRevisionArtifact(err)
	}
	return artifact, nil
}

func rejectedRevisionArtifact(cause error) error {
	if cause == nil {
		return entity.ErrMaterialArtifactRejected
	}
	return errors.Join(entity.ErrMaterialArtifactRejected, cause)
}
