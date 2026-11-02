package katex

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"sync"

	"github.com/dop251/goja"
)

const (
	// Version is the exact KaTeX runtime embedded into the API binary.
	Version = "0.16.44"

	// AssetSHA256 authenticates assets/katex.min.js before it is compiled.
	AssetSHA256 = "781acf302f284be795aea416fe3566a98a8db8023fa6a61cacea5cd4715bef51"
)

//go:embed assets/katex.min.js
var katexSource string

type programs struct {
	katex     *goja.Program
	bootstrap *goja.Program
}

var sharedPrograms = sync.OnceValues(compileEmbeddedPrograms)

func compileEmbeddedPrograms() (*programs, error) {
	digest := sha256.Sum256([]byte(katexSource))
	if hex.EncodeToString(digest[:]) != AssetSHA256 {
		return nil, ErrAssetIntegrity
	}

	katexProgram, err := goja.Compile("katex.min.js", katexSource, false)
	if err != nil {
		return nil, ErrInternal
	}
	bootstrapProgram, err := goja.Compile(
		"katex-bootstrap.js",
		bootstrapSource,
		true,
	)
	if err != nil {
		return nil, ErrInternal
	}
	return &programs{
		katex:     katexProgram,
		bootstrap: bootstrapProgram,
	}, nil
}
