package schema

import (
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// TestAsset stores content-addressed test bytes. The checksum always covers
// the uncompressed body; encoding is a storage concern and cannot alter judge
// semantics.
type TestAsset struct{ ent.Schema }

func (TestAsset) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TestAsset) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.Bytes("sha256").
			NotEmpty().
			Immutable(),
		field.Enum("encoding").
			Values(problemcontract.AssetEncodings()...).
			Immutable(),
		field.Bytes("content").
			MaxLen(problemcontract.MaximumTestAssetBytes).
			Immutable().
			Sensitive(),
		field.Int64("uncompressed_size").
			Min(0).
			Max(problemcontract.MaximumTestAssetBytes).
			Immutable(),
	)
}

func (TestAsset) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("input_testcase_revisions", TestcaseRevision.Type).Immutable(),
		edge.To("expected_testcase_revisions", TestcaseRevision.Type).Immutable(),
	}
}

func (TestAsset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sha256").Unique(),
		index.Fields("created_at", "id"),
	}
}

func (TestAsset) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"test_asset_nonzero_identity": "id <> " + problemNilUUIDSQL,
			"test_asset_checksum_shape":   "octet_length(sha256) = 32 AND sha256 <> " + problemZeroDigestSQL,
			"test_asset_encoding_closed":  contentRenderClosedDomainCheck("encoding", problemcontract.AssetEncodings()),
			"test_asset_content_bound": fmt.Sprintf(
				"octet_length(content) BETWEEN 0 AND %d",
				problemcontract.MaximumTestAssetBytes,
			),
			"test_asset_encoding_size_consistent": fmt.Sprintf(`(
				encoding = '%s' AND uncompressed_size = octet_length(content)
			) OR (
				encoding = '%s' AND octet_length(content) BETWEEN 1 AND %d
				AND uncompressed_size BETWEEN 0 AND %d
			)`,
				problemcontract.AssetEncodingIdentity,
				problemcontract.AssetEncodingZstd,
				problemcontract.MaximumTestAssetBytes,
				problemcontract.MaximumTestAssetBytes,
			),
		}),
	}
}
