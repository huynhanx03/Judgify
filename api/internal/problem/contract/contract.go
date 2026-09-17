// Package contract owns the closed, dependency-free storage and validation
// vocabulary for immutable problem authoring. Ent schemas and the Problem
// domain both import this package so bounds cannot drift between layers.
package contract

const (
	LifecycleDraft     = "draft"
	LifecyclePublished = "published"
	LifecycleArchived  = "archived"

	RenderSourceKindProblemStatement = "problem_statement"

	AssetEncodingIdentity = "identity"
	AssetEncodingZstd     = "zstd"

	TestKindSample = "sample"
	TestKindHidden = "hidden"

	ProblemSlugUniqueConstraintName = "problems_slug_key"

	MaximumSlugBytes                 = 128
	MaximumTitleBytes                = 300
	MaximumStatementMarkdownBytes    = 1 << 20
	MaximumCheckerKeyBytes           = 64
	MaximumCheckerVersionBytes       = 64
	MaximumCheckerConfigBytes        = 4 << 10
	MaximumRuntimeKeyBytes           = 64
	MaximumAllowedRuntimes           = 16
	MaximumTags                      = 64
	MaximumElements                  = 32
	MaximumRewardSnapshotJSONBytes   = 64 << 10
	MaximumTestCases                 = 256
	MaximumTestcaseGroups            = 64
	MaximumTestAssetBytes            = 1 << 20
	MaximumTestsetUncompressedBytes  = 512 << 20
	DefaultCPUTimeLimitMilliseconds  = 1000
	DefaultWallTimeLimitMilliseconds = 2000
	DefaultMemoryLimitKilobytes      = 256 << 10
	DefaultOutputLimitBytes          = 1 << 20
	DefaultProcessLimit              = 32
	MinimumCPUTimeLimitMilliseconds  = 10
	MaximumCPUTimeLimitMilliseconds  = 30_000
	MaximumWallTimeLimitMilliseconds = 60_000
	MinimumMemoryLimitKilobytes      = 16 << 10
	MaximumMemoryLimitKilobytes      = 2 << 20
	MinimumOutputLimitBytes          = 1
	MaximumOutputLimitBytes          = 1 << 20
	MinimumProcessLimit              = 1
	MaximumProcessLimit              = 64
	MaximumBaseExperience            = int64(1_000_000_000)
	MaximumRewardWeightBasisPoints   = 10_000
	MaximumPublicationReasonBytes    = 1024
	MaximumProblemRevisionNumber     = int64(1_000_000_000)
	MaximumTestsetRevisionNumber     = int64(1_000_000_000)
	MaximumDifficultyRevision        = int64(1_000_000_000)
	MaximumRewardMappingRevision     = int64(1_000_000_000)
)

var (
	lifecycles     = [...]string{LifecycleDraft, LifecyclePublished, LifecycleArchived}
	assetEncodings = [...]string{AssetEncodingIdentity, AssetEncodingZstd}
	testKinds      = [...]string{TestKindSample, TestKindHidden}
)

func Lifecycles() []string {
	return append([]string(nil), lifecycles[:]...)
}

func AssetEncodings() []string {
	return append([]string(nil), assetEncodings[:]...)
}

func TestKinds() []string {
	return append([]string(nil), testKinds[:]...)
}

func IsLifecycle(value string) bool {
	return contains(lifecycles[:], value)
}

func IsAssetEncoding(value string) bool {
	return contains(assetEncodings[:], value)
}

func IsTestKind(value string) bool {
	return contains(testKinds[:], value)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
