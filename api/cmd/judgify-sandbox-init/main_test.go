package main

import "testing"

func TestValidExecutableRejectsShellAndNonCanonicalPaths(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"python3",
		"/bin/sh",
		"/usr/bin/env",
		"/workspace/../bin/main",
		"/workspace\\main",
		"",
	} {
		if validExecutable(value) {
			t.Fatalf("validExecutable(%q) = true", value)
		}
	}
	for _, value := range []string{
		"/usr/bin/python3",
		"/usr/bin/g++",
		"/workspace/artifact/main",
	} {
		if !validExecutable(value) {
			t.Fatalf("validExecutable(%q) = false", value)
		}
	}
}
