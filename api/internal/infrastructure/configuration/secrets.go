package configuration

import (
	"context"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// SecretSource keeps the loader independent from Docker secrets, systemd
// credentials, Kubernetes volumes, or a future external secret manager.
type SecretSource interface {
	Read(context.Context, string) ([]byte, error)
}

// OSFileSecretSource reads one bounded, regular, non-writable-by-group secret
// file. The returned bytes are detached from the read buffer and never logged.
type OSFileSecretSource struct{}

func (OSFileSecretSource) Read(
	ctx context.Context,
	name string,
) ([]byte, error) {
	if ctx == nil || strings.TrimSpace(name) == "" {
		return nil, ErrInvalidSecretSource
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	linkInfo, err := os.Lstat(name)
	if err != nil || linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidSecretSource
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, ErrInvalidSecretSource
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() ||
		info.Mode().Perm()&0o022 != 0 ||
		info.Size() < 1 || info.Size() > maximumSecretBytes {
		return nil, ErrInvalidSecretSource
	}
	value, err := io.ReadAll(
		io.LimitReader(file, maximumSecretBytes+1),
	)
	if err != nil || len(value) > maximumSecretBytes {
		return nil, ErrInvalidSecretSource
	}
	value = []byte(strings.TrimRight(string(value), "\r\n"))
	if len(value) == 0 || !utf8.Valid(value) ||
		strings.ContainsRune(string(value), '\x00') {
		return nil, ErrInvalidSecretSource
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return value, nil
}

var _ SecretSource = OSFileSecretSource{}
