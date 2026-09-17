// Package assets verifies and decodes immutable testcase assets inside the
// trusted worker process. Decoded expected output never enters a contestant
// namespace and neither encoded nor decoded content is logged.
package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var (
	ErrInvalidAsset   = errors.New("judge assets: invalid snapshot")
	ErrAssetIntegrity = errors.New("judge assets: integrity mismatch")
	ErrAssetDecode    = errors.New("judge assets: decode failed")
)

type Decoder struct{}

func (Decoder) Decode(
	ctx context.Context,
	asset submissionports.JudgeAssetSnapshot,
) ([]byte, error) {
	if ctx == nil || asset.ID == uuid.Nil || asset.SHA256 == ([32]byte{}) ||
		!problemcontract.IsAssetEncoding(asset.Encoding) ||
		asset.UncompressedSize < 0 ||
		asset.UncompressedSize > problemcontract.MaximumTestAssetBytes ||
		len(asset.Content) > problemcontract.MaximumTestAssetBytes {
		return nil, ErrInvalidAsset
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var decoded []byte
	switch asset.Encoding {
	case problemcontract.AssetEncodingIdentity:
		if int64(len(asset.Content)) != asset.UncompressedSize {
			return nil, ErrAssetIntegrity
		}
		decoded = append([]byte(nil), asset.Content...)
	case problemcontract.AssetEncodingZstd:
		if len(asset.Content) == 0 {
			return nil, ErrInvalidAsset
		}
		value, err := decodeZstd(ctx, asset.Content, asset.UncompressedSize)
		if err != nil {
			return nil, err
		}
		decoded = value
	default:
		return nil, ErrInvalidAsset
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(decoded)) != asset.UncompressedSize ||
		sha256.Sum256(decoded) != asset.SHA256 {
		clear(decoded)
		return nil, ErrAssetIntegrity
	}
	return decoded, nil
}

func decodeZstd(
	ctx context.Context,
	encoded []byte,
	uncompressedSize int64,
) ([]byte, error) {
	const decoderOverhead = uint64(256 << 10)
	decoder, err := zstd.NewReader(
		&contextReader{ctx: ctx, reader: bytes.NewReader(encoded)},
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxWindow(uint64(problemcontract.MaximumTestAssetBytes)),
		zstd.WithDecoderMaxMemory(
			uint64(problemcontract.MaximumTestAssetBytes)+decoderOverhead,
		),
	)
	if err != nil {
		return nil, errors.Join(ErrAssetDecode, err)
	}
	defer decoder.Close()

	value, err := io.ReadAll(io.LimitReader(decoder, uncompressedSize+1))
	if err != nil {
		return nil, errors.Join(ErrAssetDecode, err)
	}
	if int64(len(value)) != uncompressedSize {
		clear(value)
		return nil, fmt.Errorf("%w: decoded size mismatch", ErrAssetIntegrity)
	}
	return value, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(destination []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(destination)
}
