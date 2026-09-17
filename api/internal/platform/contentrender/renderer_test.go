package contentrender

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

func TestNormalizeMarkdownIsVersionedCanonicalAndDoesNotAliasInput(t *testing.T) {
	t.Parallel()

	input := []byte("\xef\xbb\xbfCafe\u0301\r\nline two\rline three\n")
	normalized, err := NormalizeMarkdown(input)
	require.NoError(t, err)
	require.Equal(t, NormalizationPolicyVersion, normalized.PolicyVersion)
	require.Equal(t, []byte("Caf\u00e9\nline two\nline three\n"), normalized.Bytes)
	require.Equal(t, sha256.Sum256(normalized.Bytes), normalized.Checksum)

	input[3] = 'X'
	require.Equal(t, []byte("Caf\u00e9\nline two\nline three\n"), normalized.Bytes)
}

func TestEquivalentMarkdownProducesIdenticalRequestAndArtifact(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	sourceID := uuid.MustParse("01987654-3210-7abc-8def-0123456789ab")
	decomposed := []byte("# Cafe\u0301\r\n\r\n- **one**\r\n- `two`\r\n")
	composed := []byte("# Caf\u00e9\n\n- **one**\n- `two`\n")

	firstRequest, err := NewRenderRequest(sourceID, ProblemStatement, decomposed)
	require.NoError(t, err)
	secondRequest, err := NewRenderRequest(sourceID, ProblemStatement, composed)
	require.NoError(t, err)
	require.Equal(t, firstRequest, secondRequest)

	first, err := renderer.Render(context.Background(), firstRequest)
	require.NoError(t, err)
	second, err := renderer.Render(context.Background(), secondRequest)
	require.NoError(t, err)

	require.Equal(t, StatusReady, first.Status)
	require.Equal(t, first, second)
	require.NotEmpty(t, first.SanitizedHTML)
	require.NotEqual(t, [32]byte{}, first.ArtifactChecksum)
	require.Equal(t, CurrentRendererVersion, first.RendererVersion)
	require.Equal(t, CurrentSanitizerPolicyVersion, first.SanitizerPolicyVersion)
}

func TestRendererUsesPinnedServerSideKaTeXMathML(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		ProblemStatement,
		[]byte("Inline $x^2$.\n\n$$\n\\frac{a}{b}\n$$\n"),
	)
	require.NoError(t, err)

	artifact, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, StatusReady, artifact.Status)
	require.Equal(t, "goldmark-gfm-katex0.16.44-v5", artifact.RendererVersion)
	require.Equal(t, "authored-content-mathml-v4", artifact.SanitizerPolicyVersion)
	require.Contains(t, artifact.SanitizedHTML, `<span class="math-inline"><span class="katex"><math`)
	require.Contains(t, artifact.SanitizedHTML, `<msup><mi>x</mi><mn>2</mn></msup>`)
	require.Contains(t, artifact.SanitizedHTML, `<span class="math-display"><span class="katex"><math`)
	require.Contains(t, artifact.SanitizedHTML, `<mfrac><mi>a</mi><mi>b</mi></mfrac>`)
	for _, forbidden := range []string{"style=", "href=", "src=", "<svg", "<mglyph", "onload="} {
		require.NotContains(t, strings.ToLower(artifact.SanitizedHTML), forbidden)
	}
}

func TestRendererRejectsInvalidOrForbiddenKaTeXWithoutLeakingSource(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	for _, markdown := range []string{
		`$private-token-\frac{$`,
		`$\href{https://evil.example}{owned}$`,
		"$\\gdef\\privateMacro{x}$ and $\\privateMacro$",
	} {
		request, requestErr := NewRenderRequest(
			uuid.New(),
			ProblemStatement,
			[]byte(markdown),
		)
		require.NoError(t, requestErr)
		artifact, renderErr := renderer.Render(context.Background(), request)
		require.NoError(t, renderErr)
		require.Equal(t, StatusRejected, artifact.Status)
		require.Equal(t, []Diagnostic{{Code: DiagnosticRenderRejected}}, artifact.Diagnostics)
		require.Empty(t, artifact.SanitizedHTML)
		require.NotContains(t, string(artifact.Diagnostics[0].Code), "private-token")
	}
}

func TestRendererIsDeterministicAcrossRepeatedRuns(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		MaterialBody,
		[]byte("## Table\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n~~done~~\n"),
	)
	require.NoError(t, err)

	want, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	for range 100 {
		got, renderErr := renderer.Render(context.Background(), request)
		require.NoError(t, renderErr)
		require.Equal(t, want, got)
	}
}

func TestRendererV5GoldenArtifact(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		ProblemStatement,
		[]byte("# Golden\n\nSafe **bold**, [link](https://example.com/path), and $x < y$.\n"),
	)
	require.NoError(t, err)
	artifact, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)

	require.Equal(t, "goldmark-gfm-katex0.16.44-v5", artifact.RendererVersion)
	require.Equal(t, "authored-content-mathml-v4", artifact.SanitizerPolicyVersion)
	require.Equal(t,
		"<h1>Golden</h1>\n"+
			"<p>Safe <strong>bold</strong>, <a href=\"https://example.com/path\" rel=\"nofollow noreferrer noopener\" target=\"_blank\">link</a>, and "+
			"<span class=\"math-inline\"><span class=\"katex\"><math xmlns=\"http://www.w3.org/1998/Math/MathML\"><semantics><mrow><mi>x</mi><mo>&lt;</mo><mi>y</mi></mrow>"+
			"<annotation encoding=\"application/x-tex\">x &lt; y</annotation></semantics></math></span></span>.</p>\n",
		artifact.SanitizedHTML,
	)
	require.Equal(t, "da8c947a70407c998348856cb799b62a301d48dfd85f497e0d069f9e1a176cc9", hex.EncodeToString(artifact.ArtifactChecksum[:]))
}

func TestRendererIsDeterministicUnderConcurrentUse(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(WithLimits(Limits{
		MaxSourceBytes:       DefaultMaxSourceBytes,
		MaxRenderedHTMLBytes: DefaultMaxRenderedHTMLBytes,
		MaxNestingDepth:      DefaultMaxNestingDepth,
		MaxDiagnostics:       DefaultMaxDiagnostics - 1,
	}))
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		ContestClarification,
		[]byte("Concurrent **render** with $x^2$ and `escaped` content.\n"),
	)
	require.NoError(t, err)
	want, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	require.NotEqual(t, CurrentRendererVersion, want.RendererVersion)

	const workers = 32
	results := make(chan Artifact, workers)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			artifact, renderErr := renderer.Render(context.Background(), request)
			results <- artifact
			errors <- renderErr
		}()
	}
	wait.Wait()
	close(results)
	close(errors)

	for renderErr := range errors {
		require.NoError(t, renderErr)
	}
	for artifact := range results {
		require.Equal(t, want, artifact)
	}
}

func TestRendererLimitsCanOnlyTightenCanonicalMaximums(t *testing.T) {
	t.Parallel()

	canonical := Limits{
		MaxSourceBytes:       DefaultMaxSourceBytes,
		MaxRenderedHTMLBytes: DefaultMaxRenderedHTMLBytes,
		MaxNestingDepth:      DefaultMaxNestingDepth,
		MaxDiagnostics:       DefaultMaxDiagnostics,
	}
	_, err := NewRenderer(WithLimits(canonical))
	require.NoError(t, err)

	tests := []struct {
		name   string
		loosen func(*Limits)
	}{
		{name: "source bytes", loosen: func(limits *Limits) { limits.MaxSourceBytes++ }},
		{name: "rendered HTML bytes", loosen: func(limits *Limits) { limits.MaxRenderedHTMLBytes++ }},
		{name: "nesting depth", loosen: func(limits *Limits) { limits.MaxNestingDepth++ }},
		{name: "diagnostics", loosen: func(limits *Limits) { limits.MaxDiagnostics++ }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limits := canonical
			test.loosen(&limits)
			_, renderErr := NewRenderer(WithLimits(limits))
			require.Error(t, renderErr)
		})
	}
}

func TestRendererLimitsMayTightenCanonicalMaximums(t *testing.T) {
	t.Parallel()

	_, err := NewRenderer(WithLimits(Limits{
		MaxSourceBytes:       DefaultMaxSourceBytes / 2,
		MaxRenderedHTMLBytes: DefaultMaxRenderedHTMLBytes / 2,
		MaxNestingDepth:      DefaultMaxNestingDepth / 2,
		MaxDiagnostics:       DefaultMaxDiagnostics / 2,
	}))
	require.NoError(t, err)
}

func TestCanonicalLimitsUseTheExportedPipelineVersions(t *testing.T) {
	t.Parallel()

	defaultRenderer, err := NewRenderer()
	require.NoError(t, err)
	explicitRenderer, err := NewRenderer(WithLimits(Limits{
		MaxSourceBytes:       DefaultMaxSourceBytes,
		MaxRenderedHTMLBytes: DefaultMaxRenderedHTMLBytes,
		MaxNestingDepth:      DefaultMaxNestingDepth,
		MaxDiagnostics:       DefaultMaxDiagnostics,
	}))
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		ProblemStatement,
		[]byte("canonical limits\n"),
	)
	require.NoError(t, err)

	for _, renderer := range []*Renderer{defaultRenderer, explicitRenderer} {
		artifact, renderErr := renderer.Render(context.Background(), request)
		require.NoError(t, renderErr)
		require.Equal(t, CurrentRendererVersion, artifact.RendererVersion)
		require.Equal(t, CurrentSanitizerPolicyVersion, artifact.SanitizerPolicyVersion)
	}
}

func TestEveryEffectiveLimitChangesTheRendererVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		limits Limits
	}{
		{name: "source", limits: Limits{DefaultMaxSourceBytes - 1, DefaultMaxRenderedHTMLBytes, DefaultMaxNestingDepth, DefaultMaxDiagnostics}},
		{name: "output", limits: Limits{DefaultMaxSourceBytes, DefaultMaxRenderedHTMLBytes - 1, DefaultMaxNestingDepth, DefaultMaxDiagnostics}},
		{name: "depth", limits: Limits{DefaultMaxSourceBytes, DefaultMaxRenderedHTMLBytes, DefaultMaxNestingDepth - 1, DefaultMaxDiagnostics}},
		{name: "diagnostics", limits: Limits{DefaultMaxSourceBytes, DefaultMaxRenderedHTMLBytes, DefaultMaxNestingDepth, DefaultMaxDiagnostics - 1}},
	}
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		MaterialBody,
		[]byte("limit identity\n"),
	)
	require.NoError(t, err)
	source := Source{
		ID:              request.SourceID,
		OwnerRevisionID: uuid.MustParse("01987654-3210-7abc-8def-2123456789ab"),
		Kind:            request.SourceKind,
		Checksum:        request.SourceChecksum,
	}
	versions := make(map[string]string, len(tests))
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			renderer, rendererErr := NewRenderer(WithLimits(test.limits))
			require.NoError(t, rendererErr)
			artifact, renderErr := renderer.Render(context.Background(), request)
			require.NoError(t, renderErr)
			require.Equal(t, StatusReady, artifact.Status)
			require.NotEqual(t, CurrentRendererVersion, artifact.RendererVersion)
			require.LessOrEqual(t, len(artifact.RendererVersion), 64)
			require.True(t, validPipelineVersion(artifact.RendererVersion))
			require.NoError(t, artifact.ValidateFor(source))
			if previous, exists := versions[artifact.RendererVersion]; exists {
				t.Fatalf("limits %s and %s share renderer version %q", previous, test.name, artifact.RendererVersion)
			}
			versions[artifact.RendererVersion] = test.name
		})
	}
}

func TestReadyAndRejectedArtifactsFromDifferentLimitsCannotSharePipelineIdentity(t *testing.T) {
	t.Parallel()

	defaultRenderer, err := NewRenderer()
	require.NoError(t, err)
	tightRenderer, err := NewRenderer(WithLimits(Limits{
		MaxSourceBytes:       16 * 1024,
		MaxRenderedHTMLBytes: 64,
		MaxNestingDepth:      16,
		MaxDiagnostics:       4,
	}))
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		NotificationCampaignBody,
		bytes.Repeat([]byte("**x** "), 32),
	)
	require.NoError(t, err)

	ready, err := defaultRenderer.Render(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, StatusReady, ready.Status)
	rejected, err := tightRenderer.Render(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, StatusRejected, rejected.Status)
	require.NotEqual(t, ready.RendererVersion, rejected.RendererVersion)
	require.Equal(t, ready.SanitizerPolicyVersion, rejected.SanitizerPolicyVersion)
}

func TestNormalizationRejectionUsesLimitFingerprintedRendererVersion(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(WithLimits(Limits{
		MaxSourceBytes:       8,
		MaxRenderedHTMLBytes: DefaultMaxRenderedHTMLBytes,
		MaxNestingDepth:      DefaultMaxNestingDepth,
		MaxDiagnostics:       DefaultMaxDiagnostics,
	}))
	require.NoError(t, err)
	markdown := []byte("nine-byte")
	artifact, err := renderer.Render(context.Background(), RenderRequest{
		SourceID:           uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		SourceKind:         ProblemStatement,
		SourceChecksum:     sha256.Sum256(markdown),
		NormalizedMarkdown: markdown,
	})
	require.NoError(t, err)
	require.Equal(t, StatusRejected, artifact.Status)
	require.Equal(t, []Diagnostic{{Code: DiagnosticSourceTooLarge}}, artifact.Diagnostics)
	require.NotEqual(t, CurrentRendererVersion, artifact.RendererVersion)
}

func TestValidateRenderASTRejectsAnIterativelyBuiltDeepTree(t *testing.T) {
	t.Parallel()

	root := ast.NewDocument()
	parent := ast.Node(root)
	for range DefaultMaxNestingDepth + 1 {
		child := ast.NewBlockquote()
		parent.AppendChild(parent, child)
		parent = child
	}
	parent.AppendChild(parent, ast.NewParagraph())

	err := validateRenderAST(
		context.Background(),
		root,
		DefaultMaxNestingDepth,
		astNodeBudgetForSource(DefaultMaxSourceBytes),
	)
	require.ErrorIs(t, err, errNestingTooDeep)
}

func TestValidateRenderASTEnforcesItsNodeWorkBudget(t *testing.T) {
	t.Parallel()

	root := ast.NewDocument()
	for range 16 {
		root.AppendChild(root, ast.NewParagraph())
	}

	err := validateRenderAST(context.Background(), root, DefaultMaxNestingDepth, 8)
	require.ErrorIs(t, err, errASTWorkBudgetExceeded)
}

func TestASTNodeBudgetIsSafelyDerivedAndCappedBySourceBytes(t *testing.T) {
	t.Parallel()

	require.Equal(t, astNodeBudgetSlack, astNodeBudgetForSource(0))
	require.Equal(t, maximumASTNodeBudget, astNodeBudgetForSource(DefaultMaxSourceBytes))
	require.Equal(t, maximumASTNodeBudget, astNodeBudgetForSource(int(^uint(0)>>1)))
}

func TestValidateRenderASTSamplesCancellationDuringTraversal(t *testing.T) {
	t.Parallel()

	root := ast.NewDocument()
	for range 16 {
		root.AppendChild(root, ast.NewParagraph())
	}
	ctx := newCancelAfterChecksContext(4)

	err := validateRenderAST(ctx, root, DefaultMaxNestingDepth, 64)
	require.ErrorIs(t, err, context.Canceled)
}

func TestMarkdownPreflightRejectsPathologicalContainerPrefixesBeforeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
	}{
		{name: "block quotes", prefix: "> "},
		{name: "bullet lists", prefix: "- "},
		{name: "ordered lists", prefix: "1. "},
		{name: "mixed containers", prefix: "> - "},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(strings.Repeat(test.prefix, DefaultMaxNestingDepth+1) + "payload\n")

			err := preflightMarkdown(context.Background(), source, DefaultMaxNestingDepth)

			require.ErrorIs(t, err, errNestingTooDeep)
		})
	}
}

func TestMarkdownPreflightDoesNotInterpretFencedCodeAsContainers(t *testing.T) {
	t.Parallel()

	pathologicalLookingCode := strings.Repeat("> - ", DefaultMaxNestingDepth+1) + "literal code"
	for _, fence := range []string{"```", "~~~~"} {
		fence := fence
		t.Run(fence, func(t *testing.T) {
			t.Parallel()
			source := []byte(fence + "\n" + pathologicalLookingCode + "\n" + fence + "\n")

			require.NoError(t, preflightMarkdown(context.Background(), source, DefaultMaxNestingDepth))
		})
	}
}

func TestMarkdownPreflightDoesNotTreatLiteralContainerFenceAsTopLevelCloser(t *testing.T) {
	t.Parallel()

	source := []byte(
		"```\n" +
			"> ```\n" +
			strings.Repeat("> ", DefaultMaxNestingDepth+1) + "literal code\n" +
			"```\n",
	)

	require.NoError(t, preflightMarkdown(context.Background(), source, DefaultMaxNestingDepth))
}

func TestMarkdownPreflightEndsFenceWhenItsBlockquoteContainerEnds(t *testing.T) {
	t.Parallel()

	source := []byte(
		"> ```\n" +
			"ordinary content outside the blockquote\n" +
			strings.Repeat("> ", DefaultMaxNestingDepth+1) + "deep\n",
	)

	require.ErrorIs(
		t,
		preflightMarkdown(context.Background(), source, DefaultMaxNestingDepth),
		errNestingTooDeep,
	)
}

func TestMarkdownPreflightTracksListOwnedFenceIndentation(t *testing.T) {
	t.Parallel()

	literal := strings.Repeat("> ", DefaultMaxNestingDepth+1) + "literal code"
	source := []byte(
		"- ```\n" +
			"  " + literal + "\n" +
			"  ```\n" +
			"- ordinary\n",
	)

	require.NoError(t, preflightMarkdown(context.Background(), source, DefaultMaxNestingDepth))
}

func TestMarkdownPreflightAllowsOrdinaryLongContent(t *testing.T) {
	t.Parallel()

	source := bytes.Repeat([]byte("ordinary text without block containers "), 4096)
	require.NoError(t, preflightMarkdown(context.Background(), source, DefaultMaxNestingDepth))
}

func TestMarkdownPreflightSamplesCancellationDuringLongLines(t *testing.T) {
	t.Parallel()

	ctx := newCancelAfterChecksContext(3)
	source := bytes.Repeat([]byte("x"), preflightContextCheckInterval*4)

	require.ErrorIs(t, preflightMarkdown(ctx, source, DefaultMaxNestingDepth), context.Canceled)
}

func TestBoundedBufferKeepsCancellationAsAStickyRenderError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	buffer := &boundedBuffer{ctx: ctx, maximumBytes: 64}
	_, err := buffer.WriteString("before cancellation")
	require.NoError(t, err)
	cancel()

	_, err = buffer.WriteString("after cancellation")
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, buffer.Flush(), context.Canceled)
}

func TestSanitizerPropagatesCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newSanitizer().sanitize(ctx, []byte("<p>content</p>"), 1024, 8)
	require.ErrorIs(t, err, context.Canceled)
}

type cancelAfterChecksContext struct {
	context.Context
	remaining int
	done      chan struct{}
	err       error
}

func newCancelAfterChecksContext(checks int) *cancelAfterChecksContext {
	return &cancelAfterChecksContext{
		Context:   context.Background(),
		remaining: checks,
		done:      make(chan struct{}),
	}
}

func (value *cancelAfterChecksContext) Done() <-chan struct{} { return value.done }

func (value *cancelAfterChecksContext) Err() error {
	if value.err != nil {
		return value.err
	}
	value.remaining--
	if value.remaining <= 0 {
		value.err = context.Canceled
		close(value.done)
	}
	return value.err
}

func TestRendererBindsArtifactChecksumToRegisteredSourceIdentity(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	markdown := []byte("same normalized source\n")
	firstRequest, err := NewRenderRequest(uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"), ProblemStatement, markdown)
	require.NoError(t, err)
	secondRequest, err := NewRenderRequest(uuid.MustParse("01987654-3210-7abc-8def-1123456789ab"), ProblemStatement, markdown)
	require.NoError(t, err)

	first, err := renderer.Render(context.Background(), firstRequest)
	require.NoError(t, err)
	second, err := renderer.Render(context.Background(), secondRequest)
	require.NoError(t, err)
	require.Equal(t, first.SanitizedHTML, second.SanitizedHTML)
	require.NotEqual(t, first.ArtifactChecksum, second.ArtifactChecksum)
}

func TestArtifactValidationBindsReadyChecksumToSourceIdentity(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	ownerRevisionID := uuid.MustParse("01987654-3210-7abc-8def-2123456789ab")
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		ProblemStatement,
		[]byte("bound artifact\n"),
	)
	require.NoError(t, err)
	artifact, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	source := Source{
		ID:              request.SourceID,
		OwnerRevisionID: ownerRevisionID,
		Kind:            request.SourceKind,
		Checksum:        request.SourceChecksum,
	}
	require.NoError(t, artifact.ValidateFor(source))

	wrongKind := source
	wrongKind.Kind = MaterialBody
	require.ErrorIs(t, artifact.ValidateFor(wrongKind), ErrArtifactSourceMismatch)
	wrongChecksum := source
	wrongChecksum.Checksum[0] ^= 0xff
	require.ErrorIs(t, artifact.ValidateFor(wrongChecksum), ErrArtifactSourceMismatch)
	wrongID := source
	wrongID.ID = uuid.MustParse("01987654-3210-7abc-8def-1123456789ab")
	require.ErrorIs(t, artifact.ValidateFor(wrongID), ErrArtifactSourceMismatch)
}

func TestSourceAndArtifactValidationRejectNilCreatorIdentity(t *testing.T) {
	t.Parallel()

	nilID := uuid.Nil
	source := Source{
		ID:              uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		OwnerRevisionID: uuid.MustParse("01987654-3210-7abc-8def-1123456789ab"),
		Kind:            ProblemStatement,
		Checksum:        sha256.Sum256([]byte("source")),
		CreatedBy:       &nilID,
	}
	require.ErrorIs(t, source.Validate(), ErrInvalidCreatedBy)

	artifact := rejectedArtifact(
		CurrentRendererVersion,
		CurrentSanitizerPolicyVersion,
		DefaultMaxDiagnostics,
		DiagnosticRenderRejected,
	)
	artifact.CreatedBy = &nilID
	require.ErrorIs(t, artifact.Validate(), ErrInvalidCreatedBy)
}

func TestRendererRejectsNonCanonicalOrChecksumMismatchedRequests(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	sourceID := uuid.MustParse("01987654-3210-7abc-8def-0123456789ab")

	nonCanonical := []byte("line one\r\nline two\n")
	_, err = renderer.Render(context.Background(), RenderRequest{
		SourceID:           sourceID,
		SourceKind:         ProblemStatement,
		SourceChecksum:     sha256.Sum256(nonCanonical),
		NormalizedMarkdown: nonCanonical,
	})
	require.ErrorIs(t, err, ErrSourceNotNormalized)

	request, err := NewRenderRequest(sourceID, ProblemStatement, []byte("canonical\n"))
	require.NoError(t, err)
	request.SourceChecksum[0] ^= 0xff
	_, err = renderer.Render(context.Background(), request)
	require.ErrorIs(t, err, ErrSourceChecksumMismatch)
}

func TestRenderRequestOwnsItsNormalizedBytes(t *testing.T) {
	t.Parallel()

	markdown := []byte("immutable\n")
	request, err := NewRenderRequest(uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"), ContestDescription, markdown)
	require.NoError(t, err)
	markdown[0] = 'X'
	require.True(t, bytes.Equal([]byte("immutable\n"), request.NormalizedMarkdown))
}
