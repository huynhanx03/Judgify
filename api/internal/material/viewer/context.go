// Package viewer carries a privacy-preserving, transport-derived anonymous
// viewer fingerprint to material application services. It is not an identity
// or authentication authority.
package viewer

import "context"

type anonymousFingerprintKey struct{}

func WithAnonymousFingerprint(ctx context.Context, fingerprint [32]byte) context.Context {
	if ctx == nil || fingerprint == ([32]byte{}) {
		return ctx
	}
	return context.WithValue(ctx, anonymousFingerprintKey{}, fingerprint)
}

func AnonymousFingerprint(ctx context.Context) ([32]byte, bool) {
	if ctx == nil {
		return [32]byte{}, false
	}
	fingerprint, ok := ctx.Value(anonymousFingerprintKey{}).([32]byte)
	return fingerprint, ok && fingerprint != ([32]byte{})
}
