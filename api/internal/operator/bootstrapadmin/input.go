// Package bootstrapadmin implements the finite, operator-only initialization
// of an otherwise empty, already-migrated Judgify database.
package bootstrapadmin

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"unicode/utf8"

	commonpassword "github.com/huynhanx03/go-common/pkg/security/password"
)

const (
	EnvUsername = "JUDGIFY_BOOTSTRAP_ADMIN_USERNAME"
	EnvPassword = "JUDGIFY_BOOTSTRAP_ADMIN_PASSWORD"

	maxUsernameBytes = 50
	minPasswordBytes = 8
)

var (
	ErrInvalidInput  = errors.New("bootstrap-admin: invalid input")
	usernamePattern  = regexp.MustCompile(`\A[a-zA-Z0-9]{3,50}\z`)
	maxPasswordBytes = commonpassword.DefaultPolicy().MaxPasswordBytes
)

// Input is the validated, ephemeral bootstrap credential. Callers must not
// log or retain Password after Bootstrap returns.
type Input struct {
	Username string
	Password []byte
}

// Clear overwrites the owned password buffer. Callers should defer it as soon
// as ParseInput succeeds; Bootstrap also clears its shared backing buffer.
func (input *Input) Clear() {
	if input == nil {
		return
	}
	for index := range input.Password {
		input.Password[index] = 0
	}
	input.Password = nil
}

// ParseInput resolves the username from a flag or environment and the secret
// exclusively from standard input or environment. Secrets are deliberately
// rejected as command arguments because process arguments are observable.
// No validation error contains a supplied secret.
func ParseInput(args []string, getenv func(string) string, stdin io.Reader) (Input, error) {
	if getenv == nil {
		return Input{}, fmt.Errorf("%w: environment reader", ErrInvalidInput)
	}

	flags := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	username := flags.String("username", "", "initial administrator username")
	passwordStdin := flags.Bool("password-stdin", false, "read password from standard input")
	if err := flags.Parse(args); err != nil {
		return Input{}, fmt.Errorf("%w: command flags", ErrInvalidInput)
	}
	if flags.NArg() != 0 {
		return Input{}, fmt.Errorf("%w: unexpected arguments", ErrInvalidInput)
	}

	resolvedUsername := *username
	if resolvedUsername == "" {
		resolvedUsername = getenv(EnvUsername)
	}
	var resolvedPassword []byte
	if *passwordStdin {
		if getenv(EnvPassword) != "" {
			return Input{}, fmt.Errorf("%w: password source", ErrInvalidInput)
		}
		var err error
		resolvedPassword, err = readPasswordLine(stdin)
		if err != nil {
			return Input{}, err
		}
	} else {
		resolvedPassword = []byte(getenv(EnvPassword))
	}

	input := Input{Username: resolvedUsername, Password: resolvedPassword}
	if !validUsername(input.Username) {
		input.Clear()
		return Input{}, fmt.Errorf("%w: username", ErrInvalidInput)
	}
	if !validPassword(input.Password) {
		input.Clear()
		return Input{}, fmt.Errorf("%w: password", ErrInvalidInput)
	}
	return input, nil
}

func readPasswordLine(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: password input", ErrInvalidInput)
	}
	data, err := io.ReadAll(io.LimitReader(reader, int64(maxPasswordBytes+2)))
	if err != nil {
		return nil, fmt.Errorf("%w: password input", ErrInvalidInput)
	}
	data = bytes.TrimSuffix(data, []byte{'\n'})
	data = bytes.TrimSuffix(data, []byte{'\r'})
	if len(data) > maxPasswordBytes {
		for index := range data {
			data[index] = 0
		}
		return nil, fmt.Errorf("%w: password", ErrInvalidInput)
	}
	return data, nil
}

func validUsername(value string) bool {
	return len(value) <= maxUsernameBytes && usernamePattern.MatchString(value)
}

func validPassword(value []byte) bool {
	if !utf8.Valid(value) || len(value) < minPasswordBytes || len(value) > maxPasswordBytes {
		return false
	}
	for len(value) > 0 {
		runeValue, size := utf8.DecodeRune(value)
		if runeValue < 0x20 || runeValue == 0x7f {
			return false
		}
		value = value[size:]
	}
	return true
}
