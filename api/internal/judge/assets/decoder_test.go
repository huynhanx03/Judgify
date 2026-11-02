package assets

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func TestDecoderVerifiesIdentityAndZstdAssets(t *testing.T) {
	t.Parallel()

	body := []byte("hidden expected output\n")
	checksum := sha256.Sum256(body)
	for _, test := range []struct {
		name     string
		encoding string
		content  []byte
	}{
		{
			name:     "identity",
			encoding: problemcontract.AssetEncodingIdentity,
			content:  append([]byte(nil), body...),
		},
		{
			name:     "zstd",
			encoding: problemcontract.AssetEncodingZstd,
			content:  encodeZstdForTest(t, body),
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := (Decoder{}).Decode(
				context.Background(),
				submissionports.JudgeAssetSnapshot{
					ID: uuid.New(), SHA256: checksum, Encoding: test.encoding,
					Content: test.content, UncompressedSize: int64(len(body)),
				},
			)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if string(value) != string(body) {
				t.Fatalf("Decode() = %q", value)
			}
		})
	}
}

func TestDecoderRejectsBombSizeAndChecksumMismatch(t *testing.T) {
	t.Parallel()

	body := []byte("small")
	encoded := encodeZstdForTest(t, body)
	_, err := (Decoder{}).Decode(
		context.Background(),
		submissionports.JudgeAssetSnapshot{
			ID: uuid.New(), SHA256: sha256.Sum256(body),
			Encoding: problemcontract.AssetEncodingZstd,
			Content:  encoded, UncompressedSize: int64(len(body) - 1),
		},
	)
	if !errors.Is(err, ErrAssetIntegrity) {
		t.Fatalf("Decode() size error = %v", err)
	}

	badChecksum := sha256.Sum256([]byte("different"))
	_, err = (Decoder{}).Decode(
		context.Background(),
		submissionports.JudgeAssetSnapshot{
			ID: uuid.New(), SHA256: badChecksum,
			Encoding: problemcontract.AssetEncodingIdentity,
			Content:  body, UncompressedSize: int64(len(body)),
		},
	)
	if !errors.Is(err, ErrAssetIntegrity) {
		t.Fatalf("Decode() checksum error = %v", err)
	}
}

func encodeZstdForTest(t *testing.T, value []byte) []byte {
	t.Helper()
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	defer func() { _ = encoder.Close() }()
	return encoder.EncodeAll(value, nil)
}
