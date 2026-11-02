package worker

import (
	"context"
	"errors"

	"github.com/huynhanx03/judgify/internal/infrastructure/releasecatalog"
	"github.com/huynhanx03/judgify/internal/judge/runtimecatalog"
	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

func loadReleaseCatalog(ctx context.Context, releaseID string, manifestPath string) (*runtimecatalog.Catalog, error) {
	catalog, err := releasecatalog.Load(ctx, releaseID, manifestPath)
	if err != nil {
		return nil, errors.Join(ErrInvalidWorker, err)
	}
	return catalog, nil
}

func decodeApprovedReleaseManifest(
	contents []byte,
) (problemservice.ApprovedReleaseManifest, error) {
	manifest, err := releasecatalog.Decode(contents)
	if err != nil {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(ErrInvalidWorker, err)
	}
	return manifest, nil
}

func validateReleaseManifestPath(path string) error {
	if err := releasecatalog.ValidateFilePath(path); err != nil {
		return errors.Join(ErrInvalidWorker, err)
	}
	return nil
}
