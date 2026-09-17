package contentrender

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func TestMathDialectRendersInlineAndStandaloneDisplayExpressions(t *testing.T) {
	t.Parallel()

	artifact := renderMathMarkdown(t, []byte(
		"Inline $x < y$ remains in its paragraph.\n\n"+
			"$$ E = mc^2 $$\n\n"+
			"$$\n"+
			"\\sum_{i=1}^{n} i\n"+
			"= \\frac{n(n+1)}{2}\n"+
			"$$\n",
	))

	require.Equal(t, StatusReady, artifact.Status)
	require.Contains(t, artifact.SanitizedHTML, `<span class="math-inline"><span class="katex"><math`)
	require.Contains(t, artifact.SanitizedHTML, `<mi>x</mi><mo>&lt;</mo><mi>y</mi>`)
	require.Contains(t, artifact.SanitizedHTML, `<span class="math-display"><span class="katex"><math`)
	require.Contains(t, artifact.SanitizedHTML, `<msup><mi>c</mi><mn>2</mn></msup>`)
	require.Contains(t, artifact.SanitizedHTML, `<munderover><mo>∑</mo>`)
	require.Contains(t, artifact.SanitizedHTML, `<mfrac>`)
}

func TestMathDialectLeavesEscapedDelimitersAndCodeLiteral(t *testing.T) {
	t.Parallel()

	artifact := renderMathMarkdown(t, []byte(
		"Escaped \\$not math\\$ and \\$\\$not display\\$\\$.\n\n"+
			"`$unclosed inline and $$misplaced display`\n\n"+
			"```tex\n"+
			"$unclosed_inline\n"+
			"$$ unclosed_display\n"+
			"```\n",
	))

	require.Equal(t, StatusReady, artifact.Status)
	require.NotContains(t, artifact.SanitizedHTML, "math-inline")
	require.NotContains(t, artifact.SanitizedHTML, "math-display")
	require.Contains(t, artifact.SanitizedHTML, "Escaped $not math$ and $$not display$$.")
	require.Contains(t, artifact.SanitizedHTML, "<code>$unclosed inline and $$misplaced display</code>")
	require.Contains(t, artifact.SanitizedHTML, "$unclosed_inline")
	require.Contains(t, artifact.SanitizedHTML, "$$ unclosed_display")
}

func TestMathDialectSupportsStandaloneDisplayInsideMarkdownContainers(t *testing.T) {
	t.Parallel()

	artifact := renderMathMarkdown(t, []byte(
		"> $$\n"+
			"> x + y\n"+
			"> $$\n\n"+
			"- $$\n"+
			"  a + b\n"+
			"  $$\n",
	))

	require.Equal(t, StatusReady, artifact.Status)
	require.Equal(t, 2, strings.Count(artifact.SanitizedHTML, `class="math-display"`))
	require.Contains(t, artifact.SanitizedHTML, `<annotation encoding="application/x-tex">x + y</annotation>`)
	require.Contains(t, artifact.SanitizedHTML, `<annotation encoding="application/x-tex">a + b</annotation>`)
}

func TestMathDialectRejectsMalformedDelimitersFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
	}{
		{name: "unclosed inline", markdown: "prefix $x + y\n"},
		{name: "empty inline", markdown: "prefix $   $ suffix\n"},
		{name: "double delimiter inside prose", markdown: "prefix $$x$$ suffix\n"},
		{name: "mixed inline closer", markdown: "prefix $x$$ suffix\n"},
		{name: "unclosed display", markdown: "$$\nx + y\n"},
		{name: "empty display", markdown: "$$\n  \n$$\n"},
		{name: "display trailing prose", markdown: "$$x$$ trailing\n"},
		{name: "nested delimiter in display", markdown: "$$\nx + $y$\n$$\n"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			artifact := renderMathMarkdown(t, []byte(test.markdown))

			require.Equal(t, StatusRejected, artifact.Status)
			require.Equal(t, []Diagnostic{{Code: DiagnosticRenderRejected}}, artifact.Diagnostics)
			require.Empty(t, artifact.SanitizedHTML)
		})
	}
}

func TestMathDialectRejectsLayoutAmplificationFailClosed(t *testing.T) {
	t.Parallel()

	for _, expression := range []string{
		`\raisebox{999999em}{x}`,
		`\def\arraystretch{999999}\begin{matrix}a\\b\end{matrix}`,
		`\kern-999999em x`,
		`\mkern-999999mu x`,
	} {
		expression := expression
		t.Run(expression, func(t *testing.T) {
			t.Parallel()

			artifact := renderMathMarkdown(t, []byte("$"+expression+"$\n"))

			require.Equal(t, StatusRejected, artifact.Status)
			require.Equal(t, []Diagnostic{{Code: DiagnosticRenderRejected}}, artifact.Diagnostics)
			require.Empty(t, artifact.SanitizedHTML)
		})
	}
}

func TestMathDialectRejectsEveryBoundedWorkDimension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown []byte
	}{
		{
			name:     "single expression",
			markdown: []byte("$" + strings.Repeat("x", maximumMathExpressionBytes+1) + "$\n"),
		},
		{
			name: "display expression",
			markdown: []byte(
				"$$\n" + strings.Repeat("x", maximumMathExpressionBytes+1) + "\n$$\n",
			),
		},
		{
			name: "expression count",
			markdown: []byte(strings.Repeat(
				"$x$ ",
				maximumMathExpressions+1,
			)),
		},
		{
			name: "total expression bytes",
			markdown: []byte(strings.Repeat(
				"$"+strings.Repeat("x", maximumMathExpressionBytes)+"$\n",
				maximumTotalMathBytes/maximumMathExpressionBytes+1,
			)),
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			artifact := renderMathMarkdown(t, test.markdown)

			require.Equal(t, StatusRejected, artifact.Status)
			require.Equal(t, []Diagnostic{{Code: DiagnosticRenderRejected}}, artifact.Diagnostics)
		})
	}
}

func TestMathDialectAcceptsExactSemanticLimits(t *testing.T) {
	t.Parallel()

	markdown := []byte(strings.Repeat(
		"$"+strings.Repeat("x", maximumMathExpressionBytes)+"$\n",
		maximumTotalMathBytes/maximumMathExpressionBytes,
	))
	artifact := renderMathMarkdown(t, markdown)

	require.Equal(t, StatusReady, artifact.Status)
	require.Equal(
		t,
		maximumTotalMathBytes/maximumMathExpressionBytes,
		strings.Count(artifact.SanitizedHTML, `class="math-inline"`),
	)
}

func TestPrepareMathNodesVisitsEachExpressionInDocumentOrder(t *testing.T) {
	t.Parallel()

	markdown := []byte("Inline $x$.\n\n$$\ny + z\n$$\n")
	parseContext := parser.NewContext()
	state := newMathParseState(context.Background())
	parseContext.Set(mathParseStateKey, state)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	document := renderer.implementation.markdown.Parser().Parse(
		text.NewReader(markdown),
		parser.WithContext(parseContext),
	)
	require.NoError(t, state.Err())

	var inputs []mathRenderInput
	err = prepareMathNodes(context.Background(), document, func(_ context.Context, input mathRenderInput) ([]byte, error) {
		inputs = append(inputs, mathRenderInput{
			Expression: append([]byte(nil), input.Expression...),
			Display:    input.Display,
		})
		input.Expression[0] ^= 0xff
		return []byte("<math></math>"), nil
	})
	require.NoError(t, err)
	require.Len(t, inputs, 2)
	require.Equal(t, []byte("x"), inputs[0].Expression)
	require.False(t, inputs[0].Display)
	require.Equal(t, []byte("y + z"), inputs[1].Expression)
	require.True(t, inputs[1].Display)

	renderedNodes := 0
	require.NoError(t, ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if math, ok := node.(mathASTNode); ok {
				require.Equal(t, []byte("<math></math>"), math.renderedMathML())
				renderedNodes++
			}
		}
		return ast.WalkContinue, nil
	}))
	require.Equal(t, 2, renderedNodes)
}

func TestPrepareMathNodesOwnsCallbackInputAndOutput(t *testing.T) {
	t.Parallel()

	payload := newInlineMathNode([]byte("x"))
	document := ast.NewDocument()
	paragraph := ast.NewParagraph()
	document.AppendChild(document, paragraph)
	paragraph.AppendChild(paragraph, payload)
	rendered := []byte("<math>x</math>")

	err := prepareMathNodes(context.Background(), document, func(_ context.Context, input mathRenderInput) ([]byte, error) {
		require.Equal(t, []byte("x"), input.Expression)
		input.Expression[0] = 'y'
		return rendered, nil
	})
	require.NoError(t, err)
	rendered[0] = 'X'

	require.Equal(t, []byte("x"), payload.mathExpression())
	require.True(t, bytes.Equal([]byte("<math>x</math>"), payload.renderedMathML()))
}

func renderMathMarkdown(t *testing.T, markdown []byte) Artifact {
	t.Helper()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	request, err := NewRenderRequest(
		uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		ProblemStatement,
		markdown,
	)
	require.NoError(t, err)
	artifact, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	return artifact
}
