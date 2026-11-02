package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RuntimeKeyCPP17   = "cpp17"
	RuntimeKeyPython3 = "python3"
	RuntimeKeyJava17  = "java17"
	RuntimeKeyGo      = "go"
)

type ArgumentKind string

const (
	ArgumentLiteral      ArgumentKind = "literal"
	ArgumentWorkerPath   ArgumentKind = "worker_path"
	ArgumentNumericLimit ArgumentKind = "numeric_limit"
)

// CommandArgument is either trusted literal data or a named value resolved by
// the worker. It deliberately has no user-text substitution variant.
type CommandArgument struct {
	Kind   ArgumentKind
	Value  string
	Prefix string
	Suffix string
}

type EnvironmentVariable struct {
	Name  string
	Value string
}

// CommandTemplate is a direct argv contract. Executable is never a shell and
// arguments are rendered independently, never concatenated into a command.
type CommandTemplate struct {
	Executable       string
	WorkingDirectory CommandArgument
	Arguments        []CommandArgument
	Environment      []EnvironmentVariable
}

type RuntimeArtifact struct {
	Path         string `json:"path"`
	Executable   bool   `json:"executable"`
	MaximumBytes int64  `json:"maximum_bytes"`
	MaximumFiles int    `json:"maximum_files"`
	Required     bool   `json:"required"`
}

type RuntimeWorkspaceValue struct {
	Name string
	Path string
}

type RuntimeWorkspaceFile struct {
	Path    string
	Content string
}

type RuntimeWorkspace struct {
	SourcePath   string
	Values       []RuntimeWorkspaceValue
	TrustedFiles []RuntimeWorkspaceFile
}

// RuntimeProfile identifies one versioned, code-owned runtime contract.
type RuntimeProfile struct {
	ID                      uuid.UUID
	Key                     string
	Language                string
	DisplayName             string
	ProfileVersion          string
	CompilerVersion         string
	RuntimeBootstrapVersion string
	ImageDigest             string
	CompileCommandID        string
	RunCommandID            string
	Compile                 CommandTemplate
	Run                     CommandTemplate
	DefaultCompileTimeout   time.Duration
	ManifestVersion         string
	ManifestChecksum        string
	Artifacts               []RuntimeArtifact
	Workspace               RuntimeWorkspace
	Active                  bool
}
