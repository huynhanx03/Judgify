package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/huynhanx03/judgify/internal/operator/bootstrapadmin"
)

func TestRunWithRejectsInvalidInputWithoutInvokingBootstrapOrEchoingSecret(t *testing.T) {
	t.Parallel()

	const secret = "correct horse battery staple"
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	exitCode := runWith(
		context.Background(),
		[]string{"--username", "bad user"},
		strings.NewReader(""),
		&stdout,
		&stderr,
		func(key string) string {
			if key == bootstrapadmin.EnvPassword {
				return secret
			}
			return ""
		},
		func(context.Context, bootstrapadmin.Input) (uuid.UUID, error) {
			called = true
			return uuid.New(), nil
		},
	)
	if exitCode != 2 {
		t.Fatalf("runWith() exit = %d, want 2; stderr=%s", exitCode, stderr.String())
	}
	if called {
		t.Fatal("bootstrap function was called for invalid input")
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("secret leaked to command output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunWithReportsOnlyAdministratorID(t *testing.T) {
	t.Parallel()

	const secret = "correct horse battery staple"
	administratorID := uuid.New()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runWith(
		context.Background(),
		[]string{"--username", "operator", "--password-stdin"},
		strings.NewReader(secret+"\n"),
		&stdout,
		&stderr,
		func(string) string { return "" },
		func(_ context.Context, input bootstrapadmin.Input) (uuid.UUID, error) {
			if input.Username != "operator" || string(input.Password) != secret {
				t.Fatalf("input = %#v", input)
			}
			return administratorID, nil
		},
	)
	if exitCode != 0 {
		t.Fatalf("runWith() exit = %d, want 0; stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), administratorID.String()) {
		t.Fatalf("stdout = %q, want administrator ID", stdout.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("secret leaked to command output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunWithClearsPasswordBufferAfterBootstrap(t *testing.T) {
	t.Parallel()

	const secret = "correct horse battery staple"
	var captured []byte
	exitCode := runWith(
		context.Background(),
		[]string{"--username", "operator", "--password-stdin"},
		strings.NewReader(secret+"\n"),
		&bytes.Buffer{},
		&bytes.Buffer{},
		func(string) string { return "" },
		func(_ context.Context, input bootstrapadmin.Input) (uuid.UUID, error) {
			captured = input.Password
			return uuid.New(), nil
		},
	)
	if exitCode != 0 {
		t.Fatalf("runWith() exit = %d, want 0", exitCode)
	}
	for index, value := range captured {
		if value != 0 {
			t.Fatalf("password byte %d was not cleared", index)
		}
	}
}
