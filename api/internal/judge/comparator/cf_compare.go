package comparator

import "strings"

// CompareOutputCF compares actual vs expected output using Codeforces rules:
// - Trim trailing whitespace on each line
// - Trim trailing empty lines
// - Compare line by line
func CompareOutputCF(actual, expected string) bool {
	actualLines := normalizeLines(actual)
	expectedLines := normalizeLines(expected)

	if len(actualLines) != len(expectedLines) {
		return false
	}

	for i := range actualLines {
		if actualLines[i] != expectedLines[i] {
			return false
		}
	}
	return true
}

// normalizeLines splits output into lines, trims trailing spaces per line,
// and removes trailing empty lines.
func normalizeLines(s string) []string {
	lines := strings.Split(s, "\n")

	// Trim trailing whitespace on each line
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}

	// Remove trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}
