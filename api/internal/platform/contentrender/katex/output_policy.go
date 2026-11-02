package katex

import (
	"bytes"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const (
	maximumMathMLDimension       = 20
	maximumMathMLDimensionTokens = 256
	maximumMathMLDimensionTotal  = 256
	maximumMathMLScale           = 4
)

var (
	mathMLNumber = regexp.MustCompile(
		`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`,
	)
	mathMLDimensionAttributes = map[string]struct{}{
		"width":         {},
		"height":        {},
		"depth":         {},
		"lspace":        {},
		"rspace":        {},
		"voffset":       {},
		"minsize":       {},
		"maxsize":       {},
		"linethickness": {},
		"mathsize":      {},
		"rowspacing":    {},
		"columnspacing": {},
	}
	mathMLSizeFactors = map[string]float64{
		"0.5em": 0.5, "0.6em": 0.6, "0.7em": 0.7, "0.8em": 0.8,
		"0.9em": 0.9, "1em": 1, "1.2em": 1.2, "1.44em": 1.44,
		"1.728em": 1.728, "2.074em": 2.074, "2.488em": 2.488,
	}
	mpaddedWidthValues = map[string]float64{
		"0": 0, "0px": 0, "+0.6em": 0.6, "+6pt": 0.6,
	}
	mpaddedHeightValues = map[string]float64{"0px": 0, "+6pt": 0.6}
	mpaddedDepthValues  = map[string]float64{"0px": 0}
	mpaddedLSpaceValues = map[string]float64{
		"0.3em": 0.3, "3pt": 0.3, "-1width": 1, "-0.5width": 0.5,
	}
	mpaddedVOffsetValues = map[string]float64{"3pt": 0.3}
	operatorLSpaceValues = map[string]float64{
		"0em": 0, "0.05em": 0.05, "0.22em": 0.22,
	}
	operatorRSpaceValues = map[string]float64{
		"0em": 0, "0.05em": 0.05, "0.17em": 0.17, "0.22em": 0.22,
	}
	operatorMinSizeValues = map[string]float64{
		"1.2em": 1.2, "1.8em": 1.8, "2.4em": 2.4, "3em": 3, "3.0em": 3,
	}
	operatorMaxSizeValues = map[string]float64{
		"1.2em": 1.2, "1.8em": 1.8, "2.4em": 2.4, "3em": 3,
	}
	tableWidthValues = map[string]float64{"100%": 1}
	cellWidthValues  = map[string]float64{"50%": 0.5}
	rowSpacingValues = map[string]float64{
		"0.1em": 0.1, "0.16em": 0.16, "0.25em": 0.25, "0.36em": 0.36,
	}
	columnSpacingValues = map[string]float64{
		"0em": 0, "0.2778em": 0.2778, "0.5em": 0.5, "1em": 1,
	}
)

type mathMLOutputFrame struct {
	element string
	scale   float64
}

type mathMLDimensionEvaluation struct {
	magnitudes  []float64
	scaleFactor float64
	tokens      int
}

// validateOutputResourceBounds checks the exact element/attribute pairs
// emitted by the pinned KaTeX MathML backend. KaTeX's maxSize option does not
// clamp every negative or raw dimension, and per-token checks alone do not
// prevent nested scale or aggregate spacing amplification.
func validateOutputResourceBounds(rendered []byte) error {
	tokenizer := html.NewTokenizer(bytes.NewReader(rendered))
	frames := make([]mathMLOutputFrame, 0, 16)
	dimensionTokens := 0
	totalMagnitude := float64(0)

	for {
		switch tokenType := tokenizer.Next(); tokenType {
		case html.ErrorToken:
			if !errors.Is(tokenizer.Err(), io.EOF) || len(frames) != 0 {
				return ErrUnsafeOutput
			}
			return nil

		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			scale := 1.0
			if len(frames) > 0 {
				scale = frames[len(frames)-1].scale
			}

			for index, attribute := range token.Attr {
				if _, dimension := mathMLDimensionAttributes[attribute.Key]; !dimension {
					continue
				}
				if attribute.Namespace != "" {
					return ErrUnsafeOutput
				}
				for previous := 0; previous < index; previous++ {
					if token.Attr[previous].Key == attribute.Key {
						return ErrUnsafeOutput
					}
				}
				if attribute.Key != "mathsize" {
					continue
				}
				evaluation, ok := evaluateMathMLDimension(token.Data, attribute.Key, attribute.Val)
				if !ok || evaluation.scaleFactor == 0 {
					return ErrUnsafeOutput
				}
				dimensionTokens += evaluation.tokens
				if dimensionTokens > maximumMathMLDimensionTokens {
					return ErrUnsafeOutput
				}
				scale *= evaluation.scaleFactor
				if math.IsNaN(scale) || math.IsInf(scale, 0) || scale > maximumMathMLScale {
					return ErrUnsafeOutput
				}
			}

			for _, attribute := range token.Attr {
				if _, dimension := mathMLDimensionAttributes[attribute.Key]; !dimension ||
					attribute.Key == "mathsize" {
					continue
				}
				evaluation, ok := evaluateMathMLDimension(token.Data, attribute.Key, attribute.Val)
				if !ok || evaluation.scaleFactor != 0 {
					return ErrUnsafeOutput
				}
				dimensionTokens += evaluation.tokens
				if dimensionTokens > maximumMathMLDimensionTokens {
					return ErrUnsafeOutput
				}
				for _, magnitude := range evaluation.magnitudes {
					effective := magnitude * scale
					if math.IsNaN(effective) || math.IsInf(effective, 0) ||
						effective > maximumMathMLDimension {
						return ErrUnsafeOutput
					}
					totalMagnitude += effective
					if totalMagnitude > maximumMathMLDimensionTotal {
						return ErrUnsafeOutput
					}
				}
			}

			if tokenType == html.StartTagToken {
				frames = append(frames, mathMLOutputFrame{
					element: token.Data,
					scale:   scale,
				})
			}

		case html.EndTagToken:
			token := tokenizer.Token()
			if len(frames) == 0 || frames[len(frames)-1].element != token.Data {
				return ErrUnsafeOutput
			}
			frames = frames[:len(frames)-1]
		}
	}
}

func evaluateMathMLDimension(element, attribute, value string) (mathMLDimensionEvaluation, bool) {
	switch element {
	case "mspace":
		if attribute == "width" || attribute == "height" {
			return boundedUnitDimension(value, "em", true)
		}

	case "mpadded":
		switch attribute {
		case "width":
			return fixedDimension(value, mpaddedWidthValues)
		case "height":
			if evaluation, ok := boundedUnitDimension(value, "em", true); ok {
				return evaluation, true
			}
			return fixedDimension(value, mpaddedHeightValues)
		case "depth":
			if evaluation, ok := boundedUnitDimension(value, "em", false); ok {
				return evaluation, true
			}
			return fixedDimension(value, mpaddedDepthValues)
		case "lspace":
			return fixedDimension(value, mpaddedLSpaceValues)
		case "voffset":
			if evaluation, ok := boundedUnitDimension(value, "em", true); ok {
				return evaluation, true
			}
			if evaluation, ok := boundedUnitDimension(value, "ex", true); ok {
				return evaluation, true
			}
			return fixedDimension(value, mpaddedVOffsetValues)
		}

	case "mo":
		switch attribute {
		case "lspace":
			return fixedDimension(value, operatorLSpaceValues)
		case "rspace":
			return fixedDimension(value, operatorRSpaceValues)
		case "minsize":
			return fixedDimension(value, operatorMinSizeValues)
		case "maxsize":
			return fixedDimension(value, operatorMaxSizeValues)
		}

	case "mfrac":
		if attribute == "linethickness" {
			if value == "0px" {
				return mathMLDimensionEvaluation{magnitudes: []float64{0}, tokens: 1}, true
			}
			return boundedUnitDimension(value, "em", false)
		}

	case "mstyle":
		if attribute == "mathsize" {
			factor, ok := mathMLSizeFactors[value]
			if !ok {
				return mathMLDimensionEvaluation{}, false
			}
			return mathMLDimensionEvaluation{scaleFactor: factor, tokens: 1}, true
		}

	case "mtable":
		switch attribute {
		case "width":
			return fixedDimension(value, tableWidthValues)
		case "rowspacing":
			return fixedDimension(value, rowSpacingValues)
		case "columnspacing":
			return evaluateColumnSpacing(value)
		}

	case "mtd":
		if attribute == "width" {
			return fixedDimension(value, cellWidthValues)
		}
	}

	return mathMLDimensionEvaluation{}, false
}

func boundedUnitDimension(value, unit string, signed bool) (mathMLDimensionEvaluation, bool) {
	if !strings.HasSuffix(value, unit) {
		return mathMLDimensionEvaluation{}, false
	}
	numberText := strings.TrimSuffix(value, unit)
	if !mathMLNumber.MatchString(numberText) {
		return mathMLDimensionEvaluation{}, false
	}
	number, err := strconv.ParseFloat(numberText, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) ||
		(!signed && number < 0) || math.Abs(number) > maximumMathMLDimension {
		return mathMLDimensionEvaluation{}, false
	}
	return mathMLDimensionEvaluation{
		magnitudes: []float64{math.Abs(number)},
		tokens:     1,
	}, true
}

func fixedDimension(value string, allowed map[string]float64) (mathMLDimensionEvaluation, bool) {
	magnitude, ok := allowed[value]
	if !ok {
		return mathMLDimensionEvaluation{}, false
	}
	return mathMLDimensionEvaluation{
		magnitudes: []float64{magnitude},
		tokens:     1,
	}, true
}

func evaluateColumnSpacing(value string) (mathMLDimensionEvaluation, bool) {
	if value == "" {
		return mathMLDimensionEvaluation{}, true
	}
	tokens := strings.Split(value, " ")
	if len(tokens) == 1 {
		return fixedDimension(value, columnSpacingValues)
	}
	if len(tokens) > maximumMathMLDimensionTokens {
		return mathMLDimensionEvaluation{}, false
	}
	magnitudes := make([]float64, len(tokens))
	for index, token := range tokens {
		expected := "0em"
		if index%2 == 1 {
			expected = "1em"
			magnitudes[index] = 1
		}
		if token != expected {
			return mathMLDimensionEvaluation{}, false
		}
	}
	return mathMLDimensionEvaluation{
		magnitudes: magnitudes,
		tokens:     len(tokens),
	}, true
}
