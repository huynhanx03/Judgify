package contentrender

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var (
	errOutputTooLarge = errors.New("content render: rendered output exceeds the byte limit")
	errNestingTooDeep = errors.New("content render: rendered output nesting exceeds the limit")
)

type sanitizer struct {
	policy *bluemonday.Policy
}

func newSanitizer() *sanitizer {
	policy := bluemonday.NewPolicy()
	policy.AllowElements(
		"p", "br", "hr", "blockquote",
		"ul", "ol", "li",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"strong", "em", "del", "code", "pre",
		"table", "thead", "tbody", "tr", "th", "td",
		"a", "span",
	)
	allowReviewedMathML(policy)
	policy.AllowAttrs("title").OnElements("a")
	policy.AllowAttrs("href").Matching(regexp.MustCompile(
		`(?i)^(?:https://[^\x00-\x20\\]+|/(?:[^/\\\x00-\x20][^\\\x00-\x20]*)?|[?#][^\\\x00-\x20]*|(?:\./|\.\./)?[^/\\\x00-\x20][^\\\x00-\x20]*)$`,
	)).OnElements("a")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(
		`^(language-[A-Za-z0-9_+.-]{1,32}|math-inline|math-display|katex)$`,
	)).OnElements("code", "span")
	policy.AllowAttrs("class").
		Matching(regexp.MustCompile(`^(?:mtr-glue|mml-eqn-num)$`)).
		OnElements("mtd")
	policy.AllowAttrs("align").Matching(regexp.MustCompile(`^(left|center|right)$`)).OnElements("th", "td")
	policy.AllowRelativeURLs(true)
	policy.AllowURLSchemeWithCustomPolicy("https", func(parsed *url.URL) bool {
		return parsed != nil && parsed.Opaque == "" && parsed.User == nil && parsed.Hostname() != ""
	})
	policy.RequireParseableURLs(true)
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	return &sanitizer{policy: policy}
}

// allowReviewedMathML admits only the finite presentation-only subset emitted
// by the pinned KaTeX MathML backend. In particular, mglyph, SVG, style,
// href/src, event, id, data-* and arbitrary class attributes remain denied.
func allowReviewedMathML(policy *bluemonday.Policy) {
	elements := []string{
		"math", "annotation", "semantics",
		"mtext", "mn", "mo", "mi", "mspace",
		"mover", "munder", "munderover", "msup", "msub", "msubsup",
		"mfrac", "mroot", "msqrt",
		"mtable", "mtr", "mtd", "mlabeledtr",
		"mrow", "menclose", "mstyle", "mpadded", "mphantom",
	}
	policy.AllowElements(elements...)
	policy.AllowNoAttrs().OnElements(elements...)

	policy.AllowAttrs("xmlns").
		Matching(regexp.MustCompile(`^http://www\.w3\.org/1998/Math/MathML$`)).
		OnElements("math")
	policy.AllowAttrs("display").
		Matching(regexp.MustCompile(`^block$`)).
		OnElements("math")
	policy.AllowAttrs("encoding").
		Matching(regexp.MustCompile(`^application/x-tex$`)).
		OnElements("annotation")

	booleanValue := regexp.MustCompile(`^(?:true|false)$`)
	policy.AllowAttrs(
		"accent", "accentunder", "displaystyle", "fence", "largeop",
		"separator", "stretchy",
	).Matching(booleanValue).OnElements(elements...)
	policy.AllowAttrs("linebreak").
		Matching(regexp.MustCompile(`^newline$`)).
		OnElements(elements...)
	policy.AllowAttrs("scriptlevel").
		Matching(regexp.MustCompile(`^(?:0|1|2)$`)).
		OnElements(elements...)
	policy.AllowAttrs("mathvariant").
		Matching(regexp.MustCompile(
			`^(?:bold|bold-italic|bold-sans-serif|double-struck|fraktur|italic|monospace|normal|sans-serif|sans-serif-bold-italic|sans-serif-italic|script)$`,
		)).
		OnElements(elements...)
	policy.AllowAttrs("mathcolor", "mathbackground").
		Matching(regexp.MustCompile(`^(?:#[A-Fa-f0-9]{3,4}|#[A-Fa-f0-9]{6}|#[A-Fa-f0-9]{8}|[A-Za-z]{1,32})$`)).
		OnElements(elements...)

	// Duplicate the engine's exact element/attribute grammar as a final HTML
	// trust-boundary defense. Do not widen these to global MathML attributes:
	// the same syntactically valid length can have very different semantics on
	// another element.
	boundedMagnitude := `(?:0*(?:[0-9](?:\.[0-9]*)?|1[0-9](?:\.[0-9]*)?|20(?:\.0*)?)|\.[0-9]+)`
	signedEM := `[+-]?` + boundedMagnitude + `em`
	nonnegativeEM := boundedMagnitude + `em`
	signedEX := `[+-]?` + boundedMagnitude + `ex`

	policy.AllowAttrs("width", "height").
		Matching(regexp.MustCompile(`^` + signedEM + `$`)).
		OnElements("mspace")
	policy.AllowAttrs("width").
		Matching(regexp.MustCompile(`^(?:0|0px|\+0\.6em|\+6pt)$`)).
		OnElements("mpadded")
	policy.AllowAttrs("height").
		Matching(regexp.MustCompile(`^(?:0px|\+6pt|` + signedEM + `)$`)).
		OnElements("mpadded")
	policy.AllowAttrs("depth").
		Matching(regexp.MustCompile(`^(?:0px|` + nonnegativeEM + `)$`)).
		OnElements("mpadded")
	policy.AllowAttrs("lspace").
		Matching(regexp.MustCompile(`^(?:0\.3em|3pt|-1width|-0\.5width)$`)).
		OnElements("mpadded")
	policy.AllowAttrs("voffset").
		Matching(regexp.MustCompile(`^(?:3pt|` + signedEM + `|` + signedEX + `)$`)).
		OnElements("mpadded")

	policy.AllowAttrs("lspace").
		Matching(regexp.MustCompile(`^(?:0em|0\.05em|0\.22em)$`)).
		OnElements("mo")
	policy.AllowAttrs("rspace").
		Matching(regexp.MustCompile(`^(?:0em|0\.05em|0\.17em|0\.22em)$`)).
		OnElements("mo")
	policy.AllowAttrs("minsize").
		Matching(regexp.MustCompile(`^(?:1\.2em|1\.8em|2\.4em|3em|3\.0em)$`)).
		OnElements("mo")
	policy.AllowAttrs("maxsize").
		Matching(regexp.MustCompile(`^(?:1\.2em|1\.8em|2\.4em|3em)$`)).
		OnElements("mo")
	policy.AllowAttrs("linethickness").
		Matching(regexp.MustCompile(`^(?:0px|` + nonnegativeEM + `)$`)).
		OnElements("mfrac")
	policy.AllowAttrs("mathsize").
		Matching(regexp.MustCompile(
			`^(?:0\.5em|0\.6em|0\.7em|0\.8em|0\.9em|1em|1\.2em|1\.44em|1\.728em|2\.074em|2\.488em)$`,
		)).
		OnElements("mstyle")
	policy.AllowAttrs("rowspacing").
		Matching(regexp.MustCompile(`^(?:0\.1em|0\.16em|0\.25em|0\.36em)$`)).
		OnElements("mtable")
	policy.AllowAttrs("columnspacing").
		Matching(regexp.MustCompile(
			`^(?:0em|0\.2778em|0\.5em|1em|0em 1em(?: 0em 1em)*(?: 0em)?)$`,
		)).
		OnElements("mtable")
	policy.AllowAttrs("width").
		Matching(regexp.MustCompile(`^100%$`)).
		OnElements("mtable")
	policy.AllowAttrs("width").
		Matching(regexp.MustCompile(`^50%$`)).
		OnElements("mtd")
	policy.AllowAttrs("columnalign").
		Matching(regexp.MustCompile(`^(?:left|center|right)(?: (?:left|center|right))*$`)).
		OnElements("mtable")
	policy.AllowAttrs("columnlines", "rowlines").
		Matching(regexp.MustCompile(`^(?:none|solid|dashed)(?: (?:none|solid|dashed))*$`)).
		OnElements("mtable")
	policy.AllowAttrs("notation").
		Matching(regexp.MustCompile(
			`^(?:updiagonalstrike|downdiagonalstrike|phasorangle|horizontalstrike|box|actuarial|top|bottom|left|right)(?: (?:updiagonalstrike|downdiagonalstrike|phasorangle|horizontalstrike|box|actuarial|top|bottom|left|right))*$`,
		)).
		OnElements("menclose")
}

func (value *sanitizer) sanitize(ctx context.Context, input []byte, maximumBytes, maximumDepth int) ([]byte, error) {
	output := &boundedBuffer{ctx: ctx, maximumBytes: maximumBytes}
	inputReader := &contextReader{ctx: ctx, reader: bytes.NewReader(input)}
	if err := value.policy.SanitizeReaderToWriter(inputReader, output); err != nil {
		if errors.Is(err, errOutputTooLarge) {
			return nil, errOutputTooLarge
		}
		return nil, err
	}
	if output.Len() > maximumBytes {
		return nil, errOutputTooLarge
	}
	result := append([]byte(nil), output.Bytes()...)
	exceeds, err := exceedsHTMLNestingDepth(ctx, result, maximumDepth)
	if err != nil {
		return nil, err
	}
	if exceeds {
		return nil, errNestingTooDeep
	}
	return result, nil
}

type boundedBuffer struct {
	bytes.Buffer
	ctx          context.Context
	maximumBytes int
	err          error
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	if err := buffer.prepareWrite(len(value)); err != nil {
		return 0, err
	}
	return buffer.Buffer.Write(value)
}

func (buffer *boundedBuffer) WriteString(value string) (int, error) {
	if err := buffer.prepareWrite(len(value)); err != nil {
		return 0, err
	}
	return buffer.Buffer.WriteString(value)
}

func (buffer *boundedBuffer) WriteByte(value byte) error {
	if err := buffer.prepareWrite(1); err != nil {
		return err
	}
	return buffer.Buffer.WriteByte(value)
}

func (buffer *boundedBuffer) WriteRune(value rune) (int, error) {
	return buffer.WriteString(string(value))
}

func (buffer *boundedBuffer) Available() int {
	remaining := buffer.maximumBytes - buffer.Len()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (buffer *boundedBuffer) Buffered() int { return buffer.Len() }

func (buffer *boundedBuffer) Flush() error {
	if buffer.err != nil {
		return buffer.err
	}
	if buffer.ctx != nil {
		buffer.err = buffer.ctx.Err()
	}
	return buffer.err
}

func (buffer *boundedBuffer) prepareWrite(length int) error {
	if buffer.err != nil {
		return buffer.err
	}
	if buffer.ctx != nil {
		if err := buffer.ctx.Err(); err != nil {
			buffer.err = err
			return err
		}
	}
	if length < 0 || length > buffer.maximumBytes-buffer.Len() {
		buffer.err = errOutputTooLarge
		return buffer.err
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(destination []byte) (int, error) {
	if reader.ctx != nil {
		if err := reader.ctx.Err(); err != nil {
			return 0, err
		}
	}
	return reader.reader.Read(destination)
}

func exceedsHTMLNestingDepth(ctx context.Context, document []byte, maximumDepth int) (bool, error) {
	if maximumDepth < 1 {
		return true, nil
	}
	tokenizer := html.NewTokenizer(bytes.NewReader(document))
	depth := 0
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		switch tokenizer.Next() {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return false, nil
			}
			return false, tokenizer.Err()
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if htmlVoidElement(string(name)) {
				continue
			}
			depth++
			if depth > maximumDepth {
				return true, nil
			}
		case html.EndTagToken:
			if depth > 0 {
				depth--
			}
		}
	}
}

func htmlVoidElement(name string) bool {
	switch name {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}
