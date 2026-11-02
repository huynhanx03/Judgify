package katex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestNewEngineVerifiesVendoredKaTeX(t *testing.T) {
	t.Parallel()

	require.Equal(t, "0.16.44", Version)
	require.Equal(t, "781acf302f284be795aea416fe3566a98a8db8023fa6a61cacea5cd4715bef51", AssetSHA256)
	digest := sha256.Sum256([]byte(katexSource))
	require.Equal(t, AssetSHA256, hex.EncodeToString(digest[:]))

	engine, err := New(WithPoolSize(1))
	require.NoError(t, err)
	require.Equal(t, 1, cap(engine.pool))

	slot := <-engine.pool
	require.NotNil(t, slot)
	katexObject := slot.runtime.Get("katex").ToObject(slot.runtime)
	require.Equal(t, Version, katexObject.Get("version").String())
	engine.pool <- slot
}

func TestNewEngineUsesBoundedDefaultPool(t *testing.T) {
	t.Parallel()

	engine, err := New()
	require.NoError(t, err)

	expected := min(runtime.GOMAXPROCS(0), maximumPoolSize)
	require.Equal(t, expected, cap(engine.pool))
	require.Equal(t, expected, len(engine.pool))
}

func TestDefaultEngineIsProcessShared(t *testing.T) {
	first, err := Default()
	require.NoError(t, err)
	second, err := Default()
	require.NoError(t, err)
	require.Same(t, first, second)
}

func TestNewEngineRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	for _, size := range []int{-1, 0, maximumPoolSize + 1} {
		_, err := New(WithPoolSize(size))
		require.ErrorIs(t, err, ErrInvalidConfiguration)
		require.Equal(t, ErrInvalidConfiguration.Error(), err.Error())
	}
}

func TestRenderProducesExactKaTeXMathML(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)

	inline, err := engine.Render(context.Background(), []byte("x^2"), false)
	require.NoError(t, err)
	require.Equal(t,
		`<span class="katex"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><msup><mi>x</mi><mn>2</mn></msup></mrow><annotation encoding="application/x-tex">x^2</annotation></semantics></math></span>`,
		string(inline),
	)

	display, err := engine.Render(context.Background(), []byte("x^2"), true)
	require.NoError(t, err)
	require.Equal(t,
		`<span class="katex"><math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><semantics><mrow><msup><mi>x</mi><mn>2</mn></msup></mrow><annotation encoding="application/x-tex">x^2</annotation></semantics></math></span>`,
		string(display),
	)
}

func TestRenderRejectsHostileTrustCommandsBeforeJavaScript(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	for _, command := range []string{
		`\href{https://example.test}{x}`,
		`\url{https://example.test}`,
		`\includegraphics{x}`,
		`\htmlClass{x}{y}`,
		`\htmlId{x}{y}`,
		`\htmlStyle{color:red}{x}`,
		`\htmlData{x=y}{z}`,
		`\def\a{x}\a`,
		`\gdef\a{x}\a`,
		`\edef\a{x}\a`,
		`\xdef\a{x}\a`,
		`\newcommand{\a}{x}\a`,
		`\renewcommand{\a}{x}\a`,
		`\providecommand{\a}{x}\a`,
		`\let\a=x\a`,
		`\futurelet\a\x\a`,
		`\global\def\a{x}`,
		`\long\def\a{x}`,
		`\tag@literal{x}`,
		`\raisebox{1mm}{x}`,
		`\Huge x`,
		`\pmb{x}`,
		`\fcolorbox{red}{blue}{x}`,
	} {
		command := command
		t.Run(commandName(command), func(t *testing.T) {
			result, err := engine.Render(context.Background(), []byte(command), false)
			require.ErrorIs(t, err, ErrForbiddenCommand)
			require.Equal(t, ErrForbiddenCommand.Error(), err.Error())
			require.Empty(t, result)
			require.NotContains(t, err.Error(), command)
		})
	}
}

func TestForbiddenCommandScannerMatchesWholeTeXControlWords(t *testing.T) {
	t.Parallel()

	require.True(t, containsForbiddenCommand([]byte(`\href{x}{y}`)))
	require.True(t, containsForbiddenCommand([]byte(`prefix \htmlData{x=y}{z}`)))
	require.True(t, containsForbiddenCommand([]byte(`\def\a{x}\a`)))
	require.True(t, containsForbiddenCommand([]byte(`\newcommand{\a}{x}`)))
	require.True(t, containsForbiddenCommand([]byte(`\tag@literal{x}`)))
	require.True(t, containsForbiddenCommand([]byte(`\Huge\Huge x`)))
	require.True(t, containsForbiddenCommand([]byte(`\pmb{x}`)))
	require.True(t, containsForbiddenCommand([]byte(`\fcolorbox{red}{blue}{x}`)))
	require.False(t, containsForbiddenCommand([]byte(`\\href`)))
	require.False(t, containsForbiddenCommand([]byte(`\hrefExtra{x}`)))
	require.False(t, containsForbiddenCommand([]byte(`\definecolor{x}{RGB}{1,2,3}`)))
	require.False(t, containsForbiddenCommand([]byte(`\text{@example}`)))
	require.False(t, containsForbiddenCommand([]byte(`href`)))
}

func TestRenderRejectsMacroAmplificationBeforeJavaScript(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	slot := <-engine.pool
	called := false
	slot.render = func(goja.Value, ...goja.Value) (goja.Value, error) {
		called = true
		return goja.Null(), nil
	}
	engine.pool <- slot

	// This remains below MaximumInputBytes and maxExpand, but would otherwise
	// materialize several megabytes inside JavaScript before the Go-side output
	// length check could run.
	expression := `\def\a{` + strings.Repeat("x+", 1700) + `}` + strings.Repeat(`\a`, 200)
	require.LessOrEqual(t, len(expression), MaximumInputBytes)

	output, err := engine.Render(context.Background(), []byte(expression), false)
	require.ErrorIs(t, err, ErrForbiddenCommand)
	require.Empty(t, output)
	require.False(t, called)
}

func TestRenderRejectsInvalidAndOversizedInputWithoutLeakingDetails(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	tests := []struct {
		name       string
		expression []byte
		want       error
	}{
		{name: "empty", expression: nil, want: ErrInvalidExpression},
		{name: "invalid utf8", expression: []byte{0xff}, want: ErrInvalidExpression},
		{name: "malformed", expression: []byte(`private-token-\frac{`), want: ErrInvalidExpression},
		{name: "oversized", expression: bytes.Repeat([]byte("x"), MaximumInputBytes+1), want: ErrInputTooLarge},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := engine.Render(context.Background(), test.expression, false)
			require.ErrorIs(t, err, test.want)
			require.Equal(t, test.want.Error(), err.Error())
			require.Empty(t, result)
			require.NotContains(t, err.Error(), "private-token")
		})
	}

	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	result, err := engine.Render(missingContext, []byte("x^2"), false)
	require.ErrorIs(t, err, ErrInvalidContext)
	require.Empty(t, result)
}

func TestMalformedExpressionDoesNotDiscardHealthyRuntime(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	original := <-engine.pool
	engine.pool <- original

	output, err := engine.Render(context.Background(), []byte(`\frac{`), false)
	require.ErrorIs(t, err, ErrInvalidExpression)
	require.Empty(t, output)

	reused := <-engine.pool
	require.Same(t, original, reused)
	engine.pool <- reused
}

func TestRenderEnforcesStrictParsingExpansionAndSizePolicies(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)

	output, err := engine.Render(context.Background(), []byte("é"), false)
	require.ErrorIs(t, err, ErrInvalidExpression)
	require.Equal(t, ErrInvalidExpression.Error(), err.Error())
	require.Empty(t, output)

	output, err = engine.Render(context.Background(), []byte(`\def\a{\a}\a`), false)
	require.ErrorIs(t, err, ErrForbiddenCommand)
	require.Equal(t, ErrForbiddenCommand.Error(), err.Error())
	require.Empty(t, output)

	output, err = engine.Render(
		context.Background(),
		[]byte(`\rule{100em}{100em}`),
		false,
	)
	require.NoError(t, err)
	require.Contains(t, string(output), `width="20em" height="20em"`)
	require.NotContains(t, string(output), `width="100em"`)
}

func TestRenderRejectsUnboundedMathMLDimensions(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	for _, expression := range []string{
		`\raisebox{999999em}{x}`,
		`\def\arraystretch{999999}\begin{matrix}a\\b\end{matrix}`,
		`\kern-999999em x`,
		`\mkern-999999mu x`,
	} {
		expression := expression
		t.Run(commandName(expression), func(t *testing.T) {
			output, err := engine.Render(context.Background(), []byte(expression), true)
			if strings.HasPrefix(expression, `\def`) ||
				strings.HasPrefix(expression, `\raisebox`) {
				require.ErrorIs(t, err, ErrForbiddenCommand)
			} else {
				require.ErrorIs(t, err, ErrUnsafeOutput)
			}
			require.Empty(t, output)
		})
	}
}

func TestRenderPreservesBoundedDimensionsAndEquationTagWidths(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	tests := []struct {
		expression string
		contains   []string
	}{
		{
			expression: `\kern-20em x`,
			contains:   []string{`width="-20em"`},
		},
		{
			expression: `\tag{1}x`,
			contains:   []string{`width="100%"`, `width="50%"`},
		},
		{
			expression: `\begin{aligned}x\end{aligned}`,
		},
		{
			expression: `\colorbox{red}{x}`,
			contains:   []string{`mathbackground="red"`, `width="+6pt"`},
		},
		{
			expression: `\dddot{x}`,
			contains:   []string{`voffset="-0.1ex"`, `mathsize="1em"`},
		},
		{
			expression: `\xrightarrow{x}`,
			contains:   []string{`width="+0.6em"`},
		},
		{
			expression: `\begin{matrix}a&b\\c&d\end{matrix}`,
			contains:   []string{`rowspacing="0.16em"`, `columnspacing="1em"`},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(commandName(test.expression), func(t *testing.T) {
			output, err := engine.Render(context.Background(), []byte(test.expression), true)
			require.NoError(t, err)
			for _, expected := range test.contains {
				require.Contains(t, string(output), expected)
			}
		})
	}
}

func TestRenderAcceptsReviewedDSAMathCompatibilityCorpus(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	for _, expression := range []string{
		`\Biggl(\frac{a}{b}\Biggr)`,
		`\left\{x\middle|y\right\}`,
		`{,}\mathbin{+}\mathpunct{;}`,
		`\smash[b]{x}+\llap{x}+\mathclap{x}`,
		`\begin{smallmatrix}a&b\\c&d\end{smallmatrix}`,
		`\begin{cases}x&x>0\\-x&x\leq0\end{cases}`,
		`\xrightarrow[x]{y}`,
		`\genfrac(){0pt}{}{a}{b}`,
		`\color{#c00}{x}`,
		`\cancel{x}+\bcancel{y}+\xcancel{z}`,
	} {
		expression := expression
		t.Run(commandName(expression), func(t *testing.T) {
			output, err := engine.Render(context.Background(), []byte(expression), true)
			require.NoError(t, err)
			require.NotEmpty(t, output)
			require.Contains(t, string(output), `<math `)
		})
	}
}

func TestRenderRejectsAggregateSpacingAmplification(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	output, err := engine.Render(
		context.Background(),
		[]byte(strings.Repeat(`\kern20em `, 13)+`x`),
		false,
	)
	require.ErrorIs(t, err, ErrUnsafeOutput)
	require.Empty(t, output)
}

func TestRenderHonorsCancellationBeforeAndWhileWaitingForRuntime(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := engine.Render(canceled, []byte("x"), false)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, result)

	slot := <-engine.pool
	waiting, stopWaiting := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer stopWaiting()
	result, err = engine.Render(waiting, []byte("x"), false)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, result)
	engine.pool <- slot
}

func TestRenderInterruptsRunningJavaScriptAndDoesNotPoisonNextRuntime(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	slot := <-engine.pool
	started := make(chan struct{})
	require.NoError(t, slot.runtime.Set("__testStarted", func() { close(started) }))
	value, err := slot.runtime.RunString(`(function () { __testStarted(); while (true) {} })`)
	require.NoError(t, err)
	slot.render, _ = goja.AssertFunction(value)
	engine.pool <- slot

	ctx, cancel := context.WithCancel(context.Background())
	resultChannel := make(chan renderResult, 1)
	go func() {
		result, renderErr := engine.Render(ctx, []byte("x"), false)
		resultChannel <- renderResult{output: result, err: renderErr}
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("JavaScript render did not start")
	}
	cancel()

	select {
	case result := <-resultChannel:
		require.ErrorIs(t, result.err, context.Canceled)
		require.Empty(t, result.output)
	case <-time.After(time.Second):
		t.Fatal("canceled JavaScript render did not return")
	}

	output, err := engine.Render(context.Background(), []byte("x"), false)
	require.NoError(t, err)
	require.Contains(t, string(output), "<mi>x</mi>")
}

func TestRenderDiscardsPanickedAndInternallyFailedRuntimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		render     goja.Callable
		javascript string
	}{
		{
			name: "panic",
			render: func(goja.Value, ...goja.Value) (goja.Value, error) {
				panic("private panic")
			},
		},
		{
			name: "internal error",
			render: func(goja.Value, ...goja.Value) (goja.Value, error) {
				return nil, errors.New("private internal error")
			},
		},
		{
			name:       "unexpected JavaScript exception",
			javascript: `(function () { throw new Error("private JavaScript error"); })`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			engine := newTestEngine(t)
			slot := <-engine.pool
			if test.javascript != "" {
				value, err := slot.runtime.RunString(test.javascript)
				require.NoError(t, err)
				slot.render, _ = goja.AssertFunction(value)
			} else {
				slot.render = test.render
			}
			engine.pool <- slot

			output, err := engine.Render(context.Background(), []byte("x"), false)
			require.ErrorIs(t, err, ErrInternal)
			require.Equal(t, ErrInternal.Error(), err.Error())
			require.Empty(t, output)

			output, err = engine.Render(context.Background(), []byte("x"), false)
			require.NoError(t, err)
			require.Contains(t, string(output), "<mi>x</mi>")
		})
	}
}

func TestRenderRejectsOversizedOutput(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t)
	slot := <-engine.pool
	value, err := slot.runtime.RunString(`(function () { return "x".repeat(1048577); })`)
	require.NoError(t, err)
	slot.render, _ = goja.AssertFunction(value)
	engine.pool <- slot

	output, err := engine.Render(context.Background(), []byte("x"), false)
	require.ErrorIs(t, err, ErrOutputTooLarge)
	require.Equal(t, ErrOutputTooLarge.Error(), err.Error())
	require.Empty(t, output)

	// Do not retain a runtime that has just materialized an adversarially large
	// JavaScript string. Rebuilding on the next call bounds retained heap.
	require.Nil(t, <-engine.pool)
	engine.pool <- nil
	output, err = engine.Render(context.Background(), []byte("x"), false)
	require.NoError(t, err)
	require.Contains(t, string(output), "<mi>x</mi>")
}

func TestRenderIsConcurrentDeterministicAndRejectsPersistentMacros(t *testing.T) {
	t.Parallel()

	engine, err := New(WithPoolSize(4))
	require.NoError(t, err)

	const goroutineCount = 64
	outputs := make([][]byte, goroutineCount)
	errs := make([]error, goroutineCount)
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutineCount)
	for index := range goroutineCount {
		index := index
		go func() {
			defer waitGroup.Done()
			outputs[index], errs[index] = engine.Render(
				context.Background(),
				[]byte(`\frac{a}{b}`),
				index%2 == 0,
			)
		}()
	}
	waitGroup.Wait()

	for index := range goroutineCount {
		require.NoError(t, errs[index])
		require.NotEmpty(t, outputs[index])
		require.Equal(t, index%2 == 0, strings.Contains(string(outputs[index]), `display="block"`))
		for comparison := index + 2; comparison < goroutineCount; comparison += 2 {
			require.Equal(t, outputs[index], outputs[comparison])
		}
	}

	_, err = engine.Render(context.Background(), []byte(`\gdef\privateMacro{x}`), false)
	require.ErrorIs(t, err, ErrForbiddenCommand)
	output, err := engine.Render(context.Background(), []byte(`\privateMacro`), false)
	require.ErrorIs(t, err, ErrInvalidExpression)
	require.Empty(t, output)
}

func TestRenderRejectsInvalidReceiver(t *testing.T) {
	t.Parallel()

	var engine *Engine
	output, err := engine.Render(context.Background(), []byte("x"), false)
	require.ErrorIs(t, err, ErrInvalidEngine)
	require.Empty(t, output)
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()

	engine, err := New(WithPoolSize(1))
	require.NoError(t, err)
	return engine
}

func commandName(command string) string {
	command = strings.TrimPrefix(command, `\`)
	if index := strings.IndexByte(command, '{'); index >= 0 {
		command = command[:index]
	}
	return command
}

type renderResult struct {
	output []byte
	err    error
}
