package contentrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
	"github.com/huynhanx03/judgify/internal/platform/contentrender/katex"
)

const (
	CurrentRendererVersion        = "goldmark-gfm-katex0.16.44-v5"
	CurrentSanitizerPolicyVersion = "authored-content-mathml-v4"

	DefaultMaxRenderedHTMLBytes = rendercontract.MaximumRenderedHTMLBytes
	DefaultMaxNestingDepth      = 64
	DefaultMaxDiagnostics       = rendercontract.MaximumDiagnostics

	astNodeBudgetMultiplier = 2
	astNodeBudgetSlack      = 64
	maximumASTNodeBudget    = DefaultMaxSourceBytes*astNodeBudgetMultiplier + astNodeBudgetSlack
)

var errASTWorkBudgetExceeded = errors.New("content render: AST work budget exceeded")

// Limits bound every attacker-controlled rendering dimension.
type Limits struct {
	MaxSourceBytes       int
	MaxRenderedHTMLBytes int
	MaxNestingDepth      int
	MaxDiagnostics       int
}

// Option configures trusted process-owned renderer limits.
type Option func(*deterministicRenderer) error

// WithLimits overrides all renderer limits. Partial/zero configurations are
// rejected so deployments cannot accidentally remove a bound.
func WithLimits(limits Limits) Option {
	return func(renderer *deterministicRenderer) error {
		if limits.MaxSourceBytes < 1 || limits.MaxSourceBytes > DefaultMaxSourceBytes ||
			limits.MaxRenderedHTMLBytes < 1 || limits.MaxRenderedHTMLBytes > DefaultMaxRenderedHTMLBytes ||
			limits.MaxNestingDepth < 1 || limits.MaxNestingDepth > DefaultMaxNestingDepth ||
			limits.MaxDiagnostics < 1 || limits.MaxDiagnostics > DefaultMaxDiagnostics {
			return errors.New("content render: invalid renderer limits")
		}
		renderer.limits = limits
		return nil
	}
}

// Renderer is a deterministic, concurrency-safe Markdown renderer. The
// sanitizer is mandatory even though raw HTML is also disabled in Goldmark.
type Renderer struct {
	implementation *deterministicRenderer
}

type deterministicRenderer struct {
	markdown        goldmark.Markdown
	sanitizer       *sanitizer
	mathRenderer    mathRenderer
	limits          Limits
	rendererVersion string
}

type mathRenderer interface {
	Render(context.Context, []byte, bool) ([]byte, error)
}

// NewRenderer constructs the pinned GFM/math parser and reviewed sanitizer.
func NewRenderer(options ...Option) (*Renderer, error) {
	implementation := &deterministicRenderer{
		markdown: goldmark.New(goldmark.WithExtensions(
			extension.GFM,
			mathExtension{},
		)),
		sanitizer: newSanitizer(),
		limits:    canonicalLimits(),
	}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("content render: nil renderer option")
		}
		if err := option(implementation); err != nil {
			return nil, err
		}
	}
	mathEngine, err := katex.Default()
	if err != nil {
		return nil, fmt.Errorf("content render: initialize math engine: %w", err)
	}
	implementation.mathRenderer = mathEngine
	implementation.rendererVersion = rendererVersionForLimits(implementation.limits)
	return &Renderer{implementation: implementation}, nil
}

func canonicalLimits() Limits {
	return Limits{
		MaxSourceBytes:       DefaultMaxSourceBytes,
		MaxRenderedHTMLBytes: DefaultMaxRenderedHTMLBytes,
		MaxNestingDepth:      DefaultMaxNestingDepth,
		MaxDiagnostics:       DefaultMaxDiagnostics,
	}
}

func rendererVersionForLimits(limits Limits) string {
	if limits == canonicalLimits() {
		return CurrentRendererVersion
	}
	return fmt.Sprintf(
		"%s-l%s.%s.%s.%s",
		CurrentRendererVersion,
		strconv.FormatInt(int64(limits.MaxSourceBytes), 36),
		strconv.FormatInt(int64(limits.MaxRenderedHTMLBytes), 36),
		strconv.FormatInt(int64(limits.MaxNestingDepth), 36),
		strconv.FormatInt(int64(limits.MaxDiagnostics), 36),
	)
}

// Render verifies the registered source identity before parsing. Content
// validation failures become deterministic rejected artifacts; caller/program
// contract failures and cancellation remain errors.
func (value *Renderer) Render(ctx context.Context, request RenderRequest) (Artifact, error) {
	if value == nil || value.implementation == nil {
		return Artifact{}, errors.New("content render: nil renderer")
	}
	if ctx == nil {
		return Artifact{}, errors.New("content render: nil context")
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}
	if request.SourceID == uuid.Nil {
		return Artifact{}, ErrInvalidSourceID
	}
	if !request.SourceKind.Valid() {
		return Artifact{}, ErrInvalidSourceKind
	}

	normalized, err := normalizeMarkdown(request.NormalizedMarkdown, value.implementation.limits.MaxSourceBytes)
	if err != nil {
		if code := diagnosticForNormalizationError(err); code != "" {
			return rejectedArtifact(
				value.implementation.rendererVersion,
				CurrentSanitizerPolicyVersion,
				value.implementation.limits.MaxDiagnostics,
				code,
			), nil
		}
		return Artifact{}, err
	}
	if !bytes.Equal(normalized.Bytes, request.NormalizedMarkdown) {
		return Artifact{}, ErrSourceNotNormalized
	}
	if normalized.Checksum != request.SourceChecksum {
		return Artifact{}, ErrSourceChecksumMismatch
	}

	if err := preflightMarkdown(ctx, request.NormalizedMarkdown, value.implementation.limits.MaxNestingDepth); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Artifact{}, ctxErr
		}
		if errors.Is(err, errNestingTooDeep) {
			return value.rejected(DiagnosticNestingTooDeep), nil
		}
		return Artifact{}, fmt.Errorf("content render: preflight Markdown: %w", err)
	}

	parseContext := parser.NewContext()
	mathState := newMathParseState(ctx)
	parseContext.Set(mathParseStateKey, mathState)
	document := value.implementation.markdown.Parser().Parse(
		text.NewReader(request.NormalizedMarkdown),
		parser.WithContext(parseContext),
	)
	if err := mathState.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Artifact{}, ctxErr
		}
		return value.rejected(DiagnosticRenderRejected), nil
	}
	if err := validateRenderAST(
		ctx,
		document,
		value.implementation.limits.MaxNestingDepth,
		astNodeBudgetForSource(len(request.NormalizedMarkdown)),
	); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Artifact{}, ctxErr
		}
		switch {
		case errors.Is(err, errNestingTooDeep):
			return value.rejected(DiagnosticNestingTooDeep), nil
		case errors.Is(err, errASTWorkBudgetExceeded):
			return value.rejected(DiagnosticRenderRejected), nil
		default:
			return Artifact{}, fmt.Errorf("content render: validate AST: %w", err)
		}
	}
	if err := prepareMathNodes(
		ctx,
		document,
		func(renderContext context.Context, input mathRenderInput) ([]byte, error) {
			return value.implementation.mathRenderer.Render(
				renderContext,
				input.Expression,
				input.Display,
			)
		},
	); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Artifact{}, ctxErr
		}
		switch {
		case errors.Is(err, katex.ErrOutputTooLarge),
			errors.Is(err, errMathLimitExceeded):
			return value.rejected(DiagnosticOutputTooLarge), nil
		case errors.Is(err, katex.ErrInvalidExpression),
			errors.Is(err, katex.ErrForbiddenCommand),
			errors.Is(err, katex.ErrUnsafeOutput),
			errors.Is(err, katex.ErrInputTooLarge):
			return value.rejected(DiagnosticRenderRejected), nil
		default:
			return Artifact{}, fmt.Errorf("content render: prepare math: %w", err)
		}
	}

	raw := &boundedBuffer{ctx: ctx, maximumBytes: value.implementation.limits.MaxRenderedHTMLBytes}
	if err := value.implementation.markdown.Renderer().Render(raw, request.NormalizedMarkdown, document); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Artifact{}, ctxErr
		}
		if errors.Is(err, errOutputTooLarge) {
			return value.rejected(DiagnosticOutputTooLarge), nil
		}
		return Artifact{}, fmt.Errorf("content render: markdown conversion: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}

	sanitized, err := value.implementation.sanitizer.sanitize(
		ctx,
		raw.Bytes(),
		value.implementation.limits.MaxRenderedHTMLBytes,
		value.implementation.limits.MaxNestingDepth,
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Artifact{}, ctxErr
		}
		switch {
		case errors.Is(err, errOutputTooLarge):
			return value.rejected(DiagnosticOutputTooLarge), nil
		case errors.Is(err, errNestingTooDeep):
			return value.rejected(DiagnosticNestingTooDeep), nil
		default:
			return Artifact{}, fmt.Errorf("content render: sanitize: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}

	artifact := Artifact{
		RendererVersion:        value.implementation.rendererVersion,
		SanitizerPolicyVersion: CurrentSanitizerPolicyVersion,
		SanitizedHTML:          string(sanitized),
		ArtifactChecksum:       checksumArtifact(request, value.implementation.rendererVersion, CurrentSanitizerPolicyVersion, sanitized),
		Status:                 StatusReady,
		Diagnostics:            []Diagnostic{},
	}
	if err := artifact.Validate(); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}

func astNodeBudgetForSource(sourceBytes int) int {
	if sourceBytes <= 0 {
		return astNodeBudgetSlack
	}
	if sourceBytes >= DefaultMaxSourceBytes {
		return maximumASTNodeBudget
	}
	return sourceBytes*astNodeBudgetMultiplier + astNodeBudgetSlack
}

type astTraversalFrame struct {
	node  ast.Node
	depth int
}

func validateRenderAST(ctx context.Context, root ast.Node, maximumDepth, maximumNodes int) error {
	if ctx == nil || root == nil || maximumDepth < 1 || maximumNodes < 1 {
		return errors.New("content render: invalid AST validation")
	}
	stack := []astTraversalFrame{{node: root}}
	visited := 0
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		last := len(stack) - 1
		frame := stack[last]
		stack = stack[:last]
		visited++
		if visited > maximumNodes {
			return errASTWorkBudgetExceeded
		}
		// The document is depth zero. One terminal leaf beyond the configured
		// element depth is safe because it adds no recursive child traversal.
		if frame.depth > maximumDepth+1 {
			return errNestingTooDeep
		}
		for child := frame.node.LastChild(); child != nil; child = child.PreviousSibling() {
			if err := ctx.Err(); err != nil {
				return err
			}
			if visited+len(stack)+1 > maximumNodes {
				return errASTWorkBudgetExceeded
			}
			stack = append(stack, astTraversalFrame{node: child, depth: frame.depth + 1})
		}
	}
	return nil
}

func (value *Renderer) rejected(code string) Artifact {
	return rejectedArtifact(
		value.implementation.rendererVersion,
		CurrentSanitizerPolicyVersion,
		value.implementation.limits.MaxDiagnostics,
		code,
	)
}

func diagnosticForNormalizationError(err error) string {
	switch {
	case errors.Is(err, ErrInvalidUTF8):
		return DiagnosticInvalidUTF8
	case errors.Is(err, ErrAmbiguousEncoding):
		return DiagnosticAmbiguousEncoding
	case errors.Is(err, ErrSourceTooLarge):
		return DiagnosticSourceTooLarge
	default:
		return ""
	}
}
