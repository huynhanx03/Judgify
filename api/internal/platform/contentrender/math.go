package contentrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	goldmarkrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const (
	// These fixed semantic ceilings are part of CurrentRendererVersion. They
	// bound both parser work and the future per-node KaTeX execution boundary.
	maximumMathExpressionBytes = 4 << 10
	maximumMathExpressions     = 256
	maximumTotalMathBytes      = 64 << 10
	maximumMathMLFragmentBytes = 1 << 20
)

var (
	errMalformedMath       = errors.New("content render: malformed math delimiter")
	errMathLimitExceeded   = errors.New("content render: math limit exceeded")
	errInvalidMathRenderer = errors.New("content render: invalid math renderer")
)

var mathParseStateKey = parser.NewContextKey()

// mathParseState is request-scoped. A Goldmark parser is shared safely between
// requests, while every parse gets its own fail-closed error and work budget.
type mathParseState struct {
	ctx             context.Context
	firstErr        error
	expressionCount int
	totalBytes      int
}

func newMathParseState(ctx context.Context) *mathParseState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &mathParseState{ctx: ctx}
}

func (state *mathParseState) Err() error {
	if state == nil {
		return errMalformedMath
	}
	if err := state.ctx.Err(); err != nil {
		return err
	}
	return state.firstErr
}

func (state *mathParseState) reject(err error) {
	if state == nil || state.firstErr != nil {
		return
	}
	if err == nil {
		err = errMalformedMath
	}
	state.firstErr = err
}

// accept validates raw delimiter contents, accounts for them exactly once,
// and returns an owned, whitespace-canonical expression for an AST node.
func (state *mathParseState) accept(raw []byte) ([]byte, bool) {
	if state == nil {
		return nil, false
	}
	if err := state.ctx.Err(); err != nil {
		state.reject(err)
		return nil, false
	}
	if state.firstErr != nil {
		return nil, false
	}

	expression := bytes.TrimSpace(raw)
	if len(expression) == 0 {
		state.reject(errMalformedMath)
		return nil, false
	}
	if len(raw) > maximumMathExpressionBytes ||
		state.expressionCount >= maximumMathExpressions ||
		len(raw) > maximumTotalMathBytes-state.totalBytes {
		state.reject(errMathLimitExceeded)
		return nil, false
	}

	state.expressionCount++
	state.totalBytes += len(raw)
	return append([]byte(nil), expression...), true
}

func mathState(parseContext parser.Context) *mathParseState {
	if parseContext == nil {
		return newMathParseState(context.Background())
	}
	value := parseContext.ComputeIfAbsent(mathParseStateKey, func() interface{} {
		return newMathParseState(context.Background())
	})
	state, ok := value.(*mathParseState)
	if !ok || state == nil {
		return newMathParseState(context.Background())
	}
	return state
}

// mathRenderInput is the stable hand-off from the Markdown AST to a future
// server-side math engine. Expression is always an owned copy, so a renderer
// cannot mutate the parsed document.
type mathRenderInput struct {
	Expression []byte
	Display    bool
}

type mathNodePayload struct {
	expression []byte
	display    bool
	mathML     []byte
}

func newMathNodePayload(expression []byte, display bool) mathNodePayload {
	return mathNodePayload{
		expression: append([]byte(nil), expression...),
		display:    display,
	}
}

func (payload *mathNodePayload) mathExpression() []byte {
	return payload.expression
}

func (payload *mathNodePayload) displayMath() bool {
	return payload.display
}

func (payload *mathNodePayload) renderedMathML() []byte {
	return payload.mathML
}

func (payload *mathNodePayload) setRenderedMathML(mathML []byte) {
	payload.mathML = append(payload.mathML[:0], mathML...)
}

// mathASTNode lets the rendering pipeline prepare every math node after the
// full AST has passed structural validation and before Goldmark emits output.
type mathASTNode interface {
	ast.Node
	mathExpression() []byte
	displayMath() bool
	renderedMathML() []byte
	setRenderedMathML([]byte)
}

type inlineMathNode struct {
	ast.BaseInline
	mathNodePayload
}

var inlineMathNodeKind = ast.NewNodeKind("ContentRenderInlineMath")

func newInlineMathNode(expression []byte) *inlineMathNode {
	return &inlineMathNode{mathNodePayload: newMathNodePayload(expression, false)}
}

func (node *inlineMathNode) Kind() ast.NodeKind { return inlineMathNodeKind }

func (node *inlineMathNode) Dump(source []byte, level int) {
	ast.DumpHelper(node, source, level, map[string]string{"display": "false"}, nil)
}

type displayMathNode struct {
	ast.BaseBlock
	mathNodePayload
	body      []byte
	closed    bool
	oversized bool
}

var displayMathNodeKind = ast.NewNodeKind("ContentRenderDisplayMath")

func newDisplayMathNode() *displayMathNode {
	return &displayMathNode{
		mathNodePayload: newMathNodePayload(nil, true),
	}
}

func (node *displayMathNode) Kind() ast.NodeKind { return displayMathNodeKind }

// Display math owns its raw source and must not be reparsed as Markdown.
func (node *displayMathNode) IsRaw() bool { return true }

func (node *displayMathNode) Dump(source []byte, level int) {
	ast.DumpHelper(node, source, level, map[string]string{"display": "true"}, nil)
}

func (node *displayMathNode) appendBody(line []byte) {
	if node.oversized {
		return
	}
	remaining := maximumMathExpressionBytes + 1 - len(node.body)
	if remaining <= 0 {
		node.oversized = true
		return
	}
	if len(line) > remaining {
		node.body = append(node.body, line[:remaining]...)
		node.oversized = true
		return
	}
	node.body = append(node.body, line...)
}

type inlineMathParser struct{}

func (inlineMathParser) Trigger() []byte { return []byte{'$'} }

func (inlineMathParser) Parse(_ ast.Node, reader text.Reader, parseContext parser.Context) ast.Node {
	line, segment := reader.PeekLine()
	if len(line) == 0 || line[0] != '$' {
		return nil
	}

	state := mathState(parseContext)
	if len(line) > 1 && line[1] == '$' {
		state.reject(errMalformedMath)
		return consumeMathLiteral(reader, segment, 2)
	}

	for index := 1; index < len(line); {
		if err := state.ctx.Err(); err != nil {
			state.reject(err)
			return consumeMathLiteral(reader, segment, 1)
		}
		switch line[index] {
		case '\n', '\r':
			state.reject(errMalformedMath)
			return consumeMathLiteral(reader, segment, 1)
		case '\\':
			if index+1 < len(line) {
				index += 2
			} else {
				index++
			}
			continue
		case '$':
			if index+1 < len(line) && line[index+1] == '$' {
				state.reject(errMalformedMath)
				return consumeMathLiteral(reader, segment, 1)
			}
			expression, ok := state.accept(line[1:index])
			reader.Advance(index + 1)
			if !ok {
				return ast.NewTextSegment(segment.WithStop(segment.Start + index + 1))
			}
			return newInlineMathNode(expression)
		default:
			index++
		}

		if index-1 > maximumMathExpressionBytes {
			state.reject(errMathLimitExceeded)
			return consumeMathLiteral(reader, segment, 1)
		}
	}

	state.reject(errMalformedMath)
	return consumeMathLiteral(reader, segment, 1)
}

func consumeMathLiteral(reader text.Reader, segment text.Segment, length int) ast.Node {
	if length < 1 {
		length = 1
	}
	reader.Advance(length)
	return ast.NewTextSegment(segment.WithStop(segment.Start + length))
}

type displayMathParser struct{}

func (displayMathParser) Trigger() []byte { return []byte{'$'} }

func (displayMathParser) Open(
	_ ast.Node,
	reader text.Reader,
	parseContext parser.Context,
) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	offset := parseContext.BlockOffset()
	if offset < 0 || offset+2 > len(line) || line[offset] != '$' || line[offset+1] != '$' {
		return nil, parser.NoChildren
	}

	state := mathState(parseContext)
	node := newDisplayMathNode()
	content := trimMarkdownLineRightSpace(line[offset:])
	switch {
	case bytes.Equal(content, []byte("$$")):
		// The body is collected by Continue until an exact standalone closer.
	case len(content) >= 4 && bytes.HasSuffix(content, []byte("$$")):
		raw := content[2 : len(content)-2]
		if containsUnescapedDollar(raw) {
			state.reject(errMalformedMath)
		} else if expression, ok := state.accept(raw); ok {
			node.mathNodePayload = newMathNodePayload(expression, true)
		}
		node.closed = true
	default:
		state.reject(errMalformedMath)
		node.closed = true
	}
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

func (displayMathParser) Continue(
	block ast.Node,
	reader text.Reader,
	parseContext parser.Context,
) parser.State {
	node := block.(*displayMathNode)
	if node.closed {
		return parser.Close
	}

	line, _ := reader.PeekLine()
	if line == nil {
		return parser.Close
	}
	state := mathState(parseContext)
	if err := state.ctx.Err(); err != nil {
		state.reject(err)
	}

	if standaloneDisplayCloser(line, reader.LineOffset()) {
		expression, ok := state.accept(node.body)
		if ok {
			node.mathNodePayload = newMathNodePayload(expression, true)
		}
		node.closed = true
		reader.AdvanceToEOL()
		return parser.Close
	}

	if containsUnescapedDollar(line) {
		state.reject(errMalformedMath)
	}
	node.appendBody(line)
	if node.oversized {
		state.reject(errMathLimitExceeded)
	}
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func (displayMathParser) Close(block ast.Node, _ text.Reader, parseContext parser.Context) {
	node := block.(*displayMathNode)
	if !node.closed {
		mathState(parseContext).reject(errMalformedMath)
	}
}

func (displayMathParser) CanInterruptParagraph() bool { return true }

func (displayMathParser) CanAcceptIndentedLine() bool { return false }

func trimMarkdownLineRightSpace(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	return bytes.TrimRight(line, " \t")
}

func standaloneDisplayCloser(line []byte, lineOffset int) bool {
	width, position := util.IndentWidth(line, lineOffset)
	if width > 3 || position >= len(line) {
		return false
	}
	return bytes.Equal(trimMarkdownLineRightSpace(line[position:]), []byte("$$"))
}

func containsUnescapedDollar(value []byte) bool {
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		if value[index] == '$' {
			return true
		}
	}
	return false
}

type mathHTMLRenderer struct{}

func (mathHTMLRenderer) RegisterFuncs(register goldmarkrenderer.NodeRendererFuncRegisterer) {
	register.Register(inlineMathNodeKind, renderMathNode)
	register.Register(displayMathNodeKind, renderMathNode)
}

func renderMathNode(
	writer util.BufWriter,
	_ []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	math, ok := node.(mathASTNode)
	if !ok {
		return ast.WalkStop, errors.New("content render: invalid math AST node")
	}

	class := "math-inline"
	if math.displayMath() {
		class = "math-display"
	}
	if _, err := writer.WriteString(`<span class="` + class + `">`); err != nil {
		return ast.WalkStop, err
	}
	mathML := math.renderedMathML()
	if len(mathML) == 0 {
		return ast.WalkStop, errInvalidMathRenderer
	}
	if _, err := writer.Write(mathML); err != nil {
		return ast.WalkStop, err
	}
	if _, err := writer.WriteString("</span>"); err != nil {
		return ast.WalkStop, err
	}
	if math.displayMath() {
		if err := writer.WriteByte('\n'); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkContinue, nil
}

type mathExtension struct{}

func (mathExtension) Extend(markdown goldmark.Markdown) {
	markdown.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(displayMathParser{}, 650)),
		parser.WithInlineParsers(util.Prioritized(inlineMathParser{}, 550)),
	)
	markdown.Renderer().AddOptions(goldmarkrenderer.WithNodeRenderers(
		util.Prioritized(mathHTMLRenderer{}, 550),
	))
}

// prepareMathNodes renders math expressions in document order without
// recursion and publishes results only after every node succeeds. This keeps a
// failed engine call from leaving a partially prepared AST.
func prepareMathNodes(
	ctx context.Context,
	root ast.Node,
	render func(context.Context, mathRenderInput) ([]byte, error),
) error {
	if ctx == nil || root == nil || render == nil {
		return errInvalidMathRenderer
	}

	type preparedMath struct {
		node   mathASTNode
		mathML []byte
	}
	prepared := make([]preparedMath, 0, 8)
	totalBytes := 0
	stack := []ast.Node{root}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]

		if math, ok := node.(mathASTNode); ok {
			input := mathRenderInput{
				Expression: append([]byte(nil), math.mathExpression()...),
				Display:    math.displayMath(),
			}
			mathML, err := render(ctx, input)
			if err != nil {
				return fmt.Errorf("content render: render math node: %w", err)
			}
			if len(mathML) == 0 ||
				len(mathML) > maximumMathMLFragmentBytes ||
				len(mathML) > DefaultMaxRenderedHTMLBytes-totalBytes {
				return errMathLimitExceeded
			}
			totalBytes += len(mathML)
			prepared = append(prepared, preparedMath{
				node:   math,
				mathML: append([]byte(nil), mathML...),
			})
		}

		for child := node.LastChild(); child != nil; child = child.PreviousSibling() {
			stack = append(stack, child)
		}
	}

	for _, result := range prepared {
		result.node.setRenderedMathML(result.mathML)
	}
	return nil
}
