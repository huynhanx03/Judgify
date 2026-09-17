package katex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputResourcePolicyAllowsOnlyReviewedElementAttributePairs(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOutputResourceBounds([]byte(
		`<span class="katex"><math><semantics><mrow>`+
			`<mo lspace="0.05em" rspace="0.17em" minsize="3.0em" maxsize="3em">x</mo>`+
			`<mpadded width="+0.6em" height="+6pt" lspace="-0.5width" voffset="-0.1ex">`+
			`<mstyle mathsize="1.2em"><mspace width="-16em"/></mstyle></mpadded>`+
			`<mfrac linethickness="0px"><mi>a</mi><mi>b</mi></mfrac>`+
			`<mtable width="100%" rowspacing="0.16em" columnspacing="0em 1em 0em">`+
			`<mtr><mtd width="50%"></mtd></mtr></mtable>`+
			`</mrow></semantics></math></span>`,
	)))

	for _, fragment := range []string{
		`<math><mi width="1em">x</mi></math>`,
		`<math><mtd width="100%"></mtd></math>`,
		`<math><mtable width="50%"></mtable></math>`,
		`<math><mpadded lspace="-20width">x</mpadded></math>`,
		`<math><mpadded voffset="1mm">x</mpadded></math>`,
		`<math><mspace width="1e2em"/></math>`,
		`<math><mspace width="NaNem"/></math>`,
		`<math><mtable columnspacing="1em 0em"></mtable></math>`,
		`<math><mtable rowspacing="0.17em"></mtable></math>`,
	} {
		require.ErrorIs(t, validateOutputResourceBounds([]byte(fragment)), ErrUnsafeOutput, fragment)
	}
}

func TestOutputResourcePolicyBoundsNestedScaleAndAggregateDimensions(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOutputResourceBounds([]byte(
		`<math><mstyle mathsize="2.488em"><mspace width="8em"/></mstyle></math>`,
	)))
	require.ErrorIs(t, validateOutputResourceBounds([]byte(
		`<math><mstyle mathsize="2.488em"><mstyle mathsize="2.488em">`+
			`<mi>x</mi></mstyle></mstyle></math>`,
	)), ErrUnsafeOutput)

	require.NoError(t, validateOutputResourceBounds([]byte(
		`<math><mrow>`+strings.Repeat(`<mspace width="20em"/>`, 12)+`</mrow></math>`,
	)))
	require.ErrorIs(t, validateOutputResourceBounds([]byte(
		`<math><mrow>`+strings.Repeat(`<mspace width="20em"/>`, 13)+`</mrow></math>`,
	)), ErrUnsafeOutput)

	spacing := make([]string, maximumMathMLDimensionTokens+1)
	for index := range spacing {
		if index%2 == 0 {
			spacing[index] = "0em"
		} else {
			spacing[index] = "1em"
		}
	}
	require.ErrorIs(t, validateOutputResourceBounds([]byte(
		`<math><mtable columnspacing="`+strings.Join(spacing, " ")+`"></mtable></math>`,
	)), ErrUnsafeOutput)
}

func TestOutputResourcePolicyAcceptsEmptyOneColumnSpacingAndRejectsMalformedMarkup(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOutputResourceBounds([]byte(
		`<span class="katex"><math><mtable columnspacing=""><mtr><mtd>x</mtd>`+
			`</mtr></mtable></math></span>`,
	)))
	require.ErrorIs(t, validateOutputResourceBounds([]byte(
		`<span><math></span></math>`,
	)), ErrUnsafeOutput)
	require.ErrorIs(t, validateOutputResourceBounds([]byte(
		`<span><math></math>`,
	)), ErrUnsafeOutput)
}
