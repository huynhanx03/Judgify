package cache

import "github.com/pkg/errors"

var (
	ErrNotSupported = errors.New("operation not supported by this cache engine")
	ErrWrongType    = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	ErrKeyNotFound  = errors.New("key not found")
)
