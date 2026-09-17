package runtimecatalog

import (
	"context"
	"errors"

	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

// ManifestSource is a composition-owned deployment boundary. Implementations
// may read a mounted file, a verified release store, or another immutable
// artifact source, but must return one complete approved manifest rather than
// independently queryable runtime/checker lists.
type ManifestSource interface {
	LoadApprovedReleaseManifest(context.Context, string) (problemservice.ApprovedReleaseManifest, error)
}

// Provider gives API and worker composition one strict release catalog source.
// It intentionally has no empty/default catalog path.
type Provider struct {
	releaseID string
	source    ManifestSource
}

func NewProvider(releaseID string, source ManifestSource) (*Provider, error) {
	if !releaseIDPattern.MatchString(releaseID) || source == nil {
		return nil, ErrRuntimeCatalogDenied
	}
	return &Provider{releaseID: releaseID, source: source}, nil
}

func (provider *Provider) Load(ctx context.Context) (*Catalog, error) {
	if provider == nil || provider.source == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrRuntimeCatalogDenied
	}
	manifest, err := provider.source.LoadApprovedReleaseManifest(ctx, provider.releaseID)
	if err != nil || manifest.ReleaseID != provider.releaseID {
		return nil, errors.Join(ErrRuntimeCatalogDenied, err)
	}
	return LoadApprovedRelease(manifest)
}
