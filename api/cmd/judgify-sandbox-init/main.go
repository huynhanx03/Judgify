// judgify-sandbox-init is the trusted container bootstrap. It holds the
// contestant process behind a fixed handshake until the host supervisor has
// resolved the fresh cgroup and captured its accounting baseline.
package main

import (
	"crypto/subtle"
	"io"
	"os"
	"path"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

var startMagic = []byte("JUDGIFY1")

func main() {
	arguments := os.Args[1:]
	if len(arguments) < 2 || arguments[0] != "--" ||
		!validExecutable(arguments[1]) {
		os.Exit(78)
	}
	for _, argument := range arguments[1:] {
		if argument == "" || len(argument) > 4096 ||
			!utf8.ValidString(argument) || containsControl(argument) {
			os.Exit(78)
		}
	}
	handshake := make([]byte, len(startMagic))
	if _, err := io.ReadFull(os.Stdin, handshake); err != nil ||
		subtle.ConstantTimeCompare(handshake, startMagic) != 1 {
		os.Exit(78)
	}
	if err := syscall.Exec(arguments[1], arguments[1:], os.Environ()); err != nil {
		os.Exit(127)
	}
}

func validExecutable(value string) bool {
	if !path.IsAbs(value) || path.Clean(value) != value ||
		strings.Contains(value, "\\") {
		return false
	}
	switch path.Base(value) {
	case "ash", "bash", "busybox", "dash", "env", "sh", "zsh":
		return false
	default:
		return true
	}
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return true
		}
	}
	return false
}
