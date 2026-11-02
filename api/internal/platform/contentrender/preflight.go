package contentrender

import (
	"bytes"
	"context"
	"errors"
)

const preflightContextCheckInterval = 4 << 10

// preflightMarkdown places a hard, allocation-free ceiling on block-container
// prefixes before Goldmark constructs and recursively walks an AST. The exact
// semantic nesting check still runs on the completed AST; this pass exists to
// make pathological one-line inputs safe to hand to the parser.
func preflightMarkdown(ctx context.Context, source []byte, maximumContainers int) error {
	if ctx == nil || maximumContainers < 1 {
		return errors.New("content render: invalid Markdown preflight")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var fence markdownFence
	for lineStart := 0; lineStart <= len(source); {
		lineEnd := lineStart
		for lineEnd < len(source) && source[lineEnd] != '\n' {
			if lineEnd%preflightContextCheckInterval == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			lineEnd++
		}

		line := source[lineStart:lineEnd]
		if fence.marker != 0 {
			if content, ok := fence.content(line); ok {
				if fence.closes(content) {
					fence = markdownFence{}
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if lineEnd == len(source) {
					return nil
				}
				lineStart = lineEnd + 1
				continue
			}
			// The line no longer belongs to the containers that opened the
			// fence. CommonMark has ended that container-scoped fence, so this
			// same physical line must be checked again as ordinary Markdown.
			fence = markdownFence{}
		}

		contentOffset, containers := markdownContainerPrefixDetails(line)
		if len(containers) > maximumContainers {
			return errNestingTooDeep
		}
		content := line[contentOffset:]
		if opener, ok := newMarkdownFence(content); ok {
			opener.containers = append([]markdownContainer(nil), containers...)
			fence = opener
		}

		if err := ctx.Err(); err != nil {
			return err
		}
		if lineEnd == len(source) {
			return nil
		}
		lineStart = lineEnd + 1
	}
	return nil
}

type markdownContainer struct {
	kind               byte
	continuationIndent int
}

func markdownContainerPrefixDetails(line []byte) (offset int, containers []markdownContainer) {
	for {
		containerStart := offset
		offset = skipAtMostThreeSpaces(line, offset)
		if offset >= len(line) {
			return offset, containers
		}

		if line[offset] == '>' {
			containers = append(containers, markdownContainer{kind: '>'})
			offset++
			if offset < len(line) && (line[offset] == ' ' || line[offset] == '\t') {
				offset++
			}
			continue
		}

		markerLength := markdownListMarkerLength(line[offset:])
		if markerLength == 0 {
			return offset, containers
		}
		offset += markerLength
		offset = skipListPadding(line, offset)
		containers = append(containers, markdownContainer{
			kind:               'l',
			continuationIndent: offset - containerStart,
		})
	}
}

func skipAtMostThreeSpaces(line []byte, offset int) int {
	for skipped := 0; skipped < 3 && offset < len(line) && line[offset] == ' '; skipped++ {
		offset++
	}
	return offset
}

func skipListPadding(line []byte, offset int) int {
	if offset >= len(line) || (line[offset] != ' ' && line[offset] != '\t') {
		return offset
	}
	if line[offset] == '\t' {
		return offset + 1
	}

	end := offset
	for end < len(line) && line[end] == ' ' && end-offset < 5 {
		end++
	}
	if end-offset <= 4 {
		return end
	}
	// CommonMark treats padding wider than four columns as one column, leaving
	// the remainder as indented content rather than another list container.
	return offset + 1
}

func markdownListMarkerLength(value []byte) int {
	if len(value) == 0 {
		return 0
	}
	if value[0] == '-' || value[0] == '+' || value[0] == '*' {
		if len(value) == 1 || value[1] == ' ' || value[1] == '\t' {
			return 1
		}
		return 0
	}

	digits := 0
	for digits < len(value) && digits < 9 && value[digits] >= '0' && value[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits >= len(value) || (value[digits] != '.' && value[digits] != ')') {
		return 0
	}
	end := digits + 1
	if end == len(value) || value[end] == ' ' || value[end] == '\t' {
		return end
	}
	return 0
}

type markdownFence struct {
	marker     byte
	length     int
	containers []markdownContainer
}

func newMarkdownFence(content []byte) (markdownFence, bool) {
	if len(content) < 3 || (content[0] != '`' && content[0] != '~') {
		return markdownFence{}, false
	}
	length := markerRunLength(content, content[0])
	if length < 3 {
		return markdownFence{}, false
	}
	if content[0] == '`' && bytes.IndexByte(content[length:], '`') >= 0 {
		return markdownFence{}, false
	}
	return markdownFence{marker: content[0], length: length}, true
}

func (fence markdownFence) closes(content []byte) bool {
	if fence.marker == 0 || len(content) < fence.length || content[0] != fence.marker {
		return false
	}
	length := markerRunLength(content, fence.marker)
	return length >= fence.length && whitespaceOnly(content[length:])
}

// content removes only the exact outer containers that own this fence. It
// deliberately does not call markdownContainerPrefix on the remainder:
// blockquote/list-looking bytes inside fenced code are literal code.
func (fence markdownFence) content(line []byte) ([]byte, bool) {
	offset := 0
	for _, container := range fence.containers {
		switch container.kind {
		case '>':
			offset = skipAtMostThreeSpaces(line, offset)
			if offset >= len(line) || line[offset] != '>' {
				return nil, false
			}
			offset++
			if offset < len(line) && (line[offset] == ' ' || line[offset] == '\t') {
				offset++
			}
		case 'l':
			next, ok := consumeMarkdownIndent(line, offset, container.continuationIndent)
			if !ok {
				return nil, false
			}
			offset = next
		default:
			return nil, false
		}
	}
	offset = skipAtMostThreeSpaces(line, offset)
	return line[offset:], true
}

func consumeMarkdownIndent(line []byte, offset, columns int) (int, bool) {
	if columns < 1 {
		return offset, false
	}
	consumed := 0
	for offset < len(line) && consumed < columns {
		switch line[offset] {
		case ' ':
			offset++
			consumed++
		case '\t':
			offset++
			consumed += 4 - consumed%4
		default:
			return offset, false
		}
	}
	return offset, consumed >= columns
}

func markerRunLength(content []byte, marker byte) int {
	length := 0
	for length < len(content) && content[length] == marker {
		length++
	}
	return length
}

func whitespaceOnly(value []byte) bool {
	for _, character := range value {
		if character != ' ' && character != '\t' {
			return false
		}
	}
	return true
}
