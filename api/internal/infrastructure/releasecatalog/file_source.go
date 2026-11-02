// Package releasecatalog owns the bounded deployment-file boundary shared by
// API and worker composition. It never supplies defaults and never reads
// runtime/checker commands from mutable storage.
package releasecatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/huynhanx03/judgify/internal/judge/runtimecatalog"
	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

const MaximumManifestBytes = 256 << 10

var ErrInvalidManifest = errors.New("release catalog: invalid deployment manifest")

type FileSource struct {
	path string
}

func NewFileSource(path string) (*FileSource, error) {
	if ValidateFilePath(path) != nil {
		return nil, ErrInvalidManifest
	}
	return &FileSource{path: path}, nil
}

func (source *FileSource) LoadApprovedReleaseManifest(
	ctx context.Context,
	wantReleaseID string,
) (problemservice.ApprovedReleaseManifest, error) {
	if source == nil {
		return problemservice.ApprovedReleaseManifest{}, ErrInvalidManifest
	}
	return LoadManifest(ctx, source.path, wantReleaseID)
}

func Load(
	ctx context.Context,
	releaseID string,
	manifestPath string,
) (*runtimecatalog.Catalog, error) {
	source, err := NewFileSource(manifestPath)
	if err != nil {
		return nil, err
	}
	provider, err := runtimecatalog.NewProvider(releaseID, source)
	if err != nil {
		return nil, errors.Join(ErrInvalidManifest, err)
	}
	catalog, err := provider.Load(ctx)
	if err != nil {
		return nil, errors.Join(ErrInvalidManifest, err)
	}
	return catalog, nil
}

func LoadManifest(
	ctx context.Context,
	path string,
	wantReleaseID string,
) (problemservice.ApprovedReleaseManifest, error) {
	if wantReleaseID == "" {
		return problemservice.ApprovedReleaseManifest{}, ErrInvalidManifest
	}
	manifest, err := ReadManifest(ctx, path)
	if err != nil || manifest.ReleaseID != wantReleaseID {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	return manifest, nil
}

// ReadManifest reads one bounded strict JSON document without granting it an
// approved release identity. Operator tooling uses this boundary to load a
// not-yet-approved draft; runtime callers must continue through LoadManifest
// with an exact expected release ID.
func ReadManifest(
	ctx context.Context,
	path string,
) (problemservice.ApprovedReleaseManifest, error) {
	if ctx == nil || ctx.Err() != nil || ValidateFilePath(path) != nil {
		return problemservice.ApprovedReleaseManifest{}, ErrInvalidManifest
	}
	file, err := os.Open(path)
	if err != nil {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 ||
		info.Size() > MaximumManifestBytes {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	contents, err := io.ReadAll(
		io.LimitReader(file, MaximumManifestBytes+1),
	)
	if err != nil || len(contents) == 0 || len(contents) > MaximumManifestBytes {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	manifest, err := Decode(contents)
	if err != nil {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	return manifest, nil
}

func Decode(
	contents []byte,
) (problemservice.ApprovedReleaseManifest, error) {
	if len(contents) == 0 || len(contents) > MaximumManifestBytes {
		return problemservice.ApprovedReleaseManifest{}, ErrInvalidManifest
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest problemservice.ApprovedReleaseManifest
	if err := decoder.Decode(&manifest); err != nil {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return problemservice.ApprovedReleaseManifest{}, errors.Join(
			ErrInvalidManifest,
			err,
		)
	}
	return manifest, nil
}

func ValidateFilePath(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path ||
		strings.ContainsAny(path, "\x00\r\n") {
		return ErrInvalidManifest
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 {
		return errors.Join(ErrInvalidManifest, err)
	}
	return nil
}
