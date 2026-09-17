package contentrender

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizerPreservesTheReviewedKaTeXMathMLSubset(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<span class="math-display"><span class="katex">` +
			`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block">` +
			`<semantics><mrow><mtable rowspacing="0.16em" columnalign="center center" columnspacing="1em">` +
			`<mtr><mtd class="mtr-glue"></mtd><mtd><mstyle scriptlevel="0" displaystyle="false" mathcolor="#c00">` +
			`<mfrac linethickness="0px"><mi mathvariant="bold">a</mi><msqrt><mn>2</mn></msqrt></mfrac>` +
			`<mover accent="true"><mo stretchy="true">‾</mo><mtext>x</mtext></mover>` +
			`</mstyle></mtd><mtd class="mml-eqn-num"></mtd></mtr></mtable></mrow>` +
			`<annotation encoding="application/x-tex">\frac{a}{\sqrt{2}}</annotation>` +
			`</semantics></math></span></span>`,
	)

	sanitized, err := newSanitizer().sanitize(
		context.Background(),
		input,
		DefaultMaxRenderedHTMLBytes,
		DefaultMaxNestingDepth,
	)
	require.NoError(t, err)
	output := string(sanitized)

	for _, preserved := range []string{
		`class="math-display"`,
		`class="katex"`,
		`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block">`,
		`<semantics>`,
		`<mtable rowspacing="0.16em" columnalign="center center" columnspacing="1em">`,
		`<mtd class="mtr-glue"></mtd>`,
		`<mtd class="mml-eqn-num"></mtd>`,
		`<mstyle scriptlevel="0" displaystyle="false" mathcolor="#c00">`,
		`<mfrac linethickness="0px">`,
		`<mi mathvariant="bold">a</mi>`,
		`<msqrt>`,
		`<mover accent="true">`,
		`<mo stretchy="true">`,
		`<annotation encoding="application/x-tex">`,
	} {
		require.Contains(t, output, preserved)
	}
}

func TestSanitizerRemovesEveryScriptableOrUnreviewedMathMLSurface(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<span class="katex evil" style="display:block" onclick="alert(1)">` +
			`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block" ` +
			`style="background:url(javascript:alert(1))" href="javascript:alert(1)">` +
			`<semantics><mrow>` +
			`<mglyph src="https://evil.example/pixel.svg" alt="x"></mglyph>` +
			`<mi id="owned" class="owned" data-owned="yes" onmouseover="alert(1)" ` +
			`href="javascript:alert(1)" style="color:red">x</mi>` +
			`<svg><foreignObject><script>alert(1)</script></foreignObject></svg>` +
			`</mrow><annotation encoding="text/html"><img src=x onerror=alert(1)></annotation>` +
			`</semantics></math></span>`,
	)

	sanitized, err := newSanitizer().sanitize(
		context.Background(),
		input,
		DefaultMaxRenderedHTMLBytes,
		DefaultMaxNestingDepth,
	)
	require.NoError(t, err)
	output := strings.ToLower(string(sanitized))

	for _, forbidden := range []string{
		"style=", "href=", "src=", "onclick", "onmouseover", "onerror",
		"id=", "class=", "data-", "<mglyph", "<svg", "foreignobject",
		"<script", "<img", `encoding="text/html"`,
	} {
		require.NotContains(t, output, forbidden)
	}
	require.Contains(t, output, `<math xmlns="http://www.w3.org/1998/math/mathml" display="block">`)
	require.Contains(t, output, "<mi>x</mi>")
}

func TestSanitizerRejectsUnsafeMathMLAttributeValuesByOmission(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<math xmlns="https://evil.example/math" display="fullscreen">` +
			`<mstyle mathcolor="url(javascript:alert(1))" scriptlevel="-1">` +
			`<mtable columnalign="center javascript" columnspacing="calc(1px + 1em)">` +
			`<menclose notation="circle onclick"><mi mathvariant="owned">x</mi></menclose>` +
			`</mtable></mstyle></math>`,
	)

	sanitized, err := newSanitizer().sanitize(
		context.Background(),
		input,
		DefaultMaxRenderedHTMLBytes,
		DefaultMaxNestingDepth,
	)
	require.NoError(t, err)
	output := string(sanitized)

	for _, forbidden := range []string{
		"evil.example", "fullscreen", "javascript", `scriptlevel="-1"`, "calc(",
		`notation=`, `mathvariant=`, `mathcolor=`, `columnalign=`, `columnspacing=`,
	} {
		require.NotContains(t, output, forbidden)
	}
	require.Contains(t, output, "<math>")
	require.Contains(t, output, "<mi>x</mi>")
}

func TestSanitizerStripsUnboundedMathMLDimensions(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block">` +
			`<mpadded voffset="999999em" width="-999999em" height="20.01em">` +
			`<mtable rowspacing="999998.16em" columnspacing="0em 1em">` +
			`<mtd width="50%"><mspace width="-20em"></mspace></mtd>` +
			`</mtable></mpadded></math>`,
	)

	sanitized, err := newSanitizer().sanitize(
		context.Background(),
		input,
		DefaultMaxRenderedHTMLBytes,
		DefaultMaxNestingDepth,
	)
	require.NoError(t, err)
	output := string(sanitized)

	for _, forbidden := range []string{
		`voffset="999999em"`,
		`width="-999999em"`,
		`height="20.01em"`,
		`rowspacing="999998.16em"`,
	} {
		require.NotContains(t, output, forbidden)
	}
	for _, preserved := range []string{
		`columnspacing="0em 1em"`,
		`width="50%"`,
		`width="-20em"`,
	} {
		require.Contains(t, output, preserved)
	}
}
