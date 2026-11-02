package bootstrapadmin

import (
	"strings"
	"testing"
)

func TestParseInputAcceptsStandardInputAndEnvironment(t *testing.T) {
	t.Parallel()

	input, err := ParseInput(
		[]string{"--username", "rootadmin", "--password-stdin"},
		func(string) string { return "" },
		strings.NewReader("correct horse battery staple\n"),
	)
	if err != nil {
		t.Fatalf("ParseInput() error = %v", err)
	}
	if input.Username != "rootadmin" {
		t.Fatalf("Username = %q, want rootadmin", input.Username)
	}
	if string(input.Password) != "correct horse battery staple" {
		t.Fatal("Password was not read from standard input")
	}

	input, err = ParseInput(nil, func(key string) string {
		switch key {
		case EnvUsername:
			return "operator"
		case EnvPassword:
			return "another correct password"
		default:
			return ""
		}
	}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseInput() environment error = %v", err)
	}
	if input.Username != "operator" || string(input.Password) != "another correct password" {
		t.Fatalf("environment input = %#v", input)
	}
}

func TestParseInputReadsPasswordFromStandardInputWithoutPuttingItInErrors(t *testing.T) {
	t.Parallel()

	const secret = "correct horse battery staple"
	input, err := ParseInput(
		[]string{"--username", "operator", "--password-stdin"},
		func(string) string { return "" },
		strings.NewReader(secret+"\n"),
	)
	if err != nil {
		t.Fatalf("ParseInput() error = %v", err)
	}
	if string(input.Password) != secret {
		t.Fatal("Password was not read from standard input")
	}

	_, err = ParseInput(
		[]string{"--username", "bad user"},
		func(key string) string {
			if key == EnvPassword {
				return secret
			}
			return ""
		},
		strings.NewReader(""),
	)
	if err == nil {
		t.Fatal("ParseInput() error = nil, want invalid username")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("ParseInput() exposed password in error: %v", err)
	}
}

func TestParseInputRejectsAmbiguousOrWeakCredentials(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		args   []string
		getenv func(string) string
		stdin  string
	}{
		{
			args: []string{"--username", "operator"},
			getenv: func(key string) string {
				if key == EnvPassword {
					return "short"
				}
				return ""
			},
		},
		{
			args: []string{"--username", "operator", "--password-stdin"},
			getenv: func(key string) string {
				if key == EnvPassword {
					return "correct horse battery staple"
				}
				return ""
			},
			stdin: "correct horse battery staple\n",
		},
		{
			args: []string{"--username", "a"},
			getenv: func(key string) string {
				if key == EnvPassword {
					return "correct horse battery staple"
				}
				return ""
			},
		},
		{
			args: []string{"--username", strings.Repeat("a", 65)},
			getenv: func(key string) string {
				if key == EnvPassword {
					return "correct horse battery staple"
				}
				return ""
			},
		},
		{
			args:   []string{"--username", "operator", "--password", "must-not-appear-in-argv"},
			getenv: func(string) string { return "" },
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(strings.Join(testCase.args[:2], " "), func(t *testing.T) {
			t.Parallel()
			if _, err := ParseInput(testCase.args, testCase.getenv, strings.NewReader(testCase.stdin)); err == nil {
				t.Fatal("ParseInput() error = nil")
			}
		})
	}
}

func TestParseInputUsesArgon2idPasswordLimit(t *testing.T) {
	t.Parallel()

	secret := strings.Repeat("a", 256)
	input, err := ParseInput(
		[]string{"--username", "operator", "--password-stdin"},
		func(string) string { return "" },
		strings.NewReader(secret+"\n"),
	)
	if err != nil {
		t.Fatalf("ParseInput() error = %v", err)
	}
	if string(input.Password) != secret {
		t.Fatal("ParseInput() changed the password")
	}
}
