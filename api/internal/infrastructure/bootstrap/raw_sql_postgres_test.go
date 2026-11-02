package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/huynhanx03/judgify/internal/testutil/rawsql"
)

func TestEveryRawStatement(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	rawsql.PrepareAll(t, os.Getenv(rawsql.DSNEnvironment), filepath.Dir(file))
}
