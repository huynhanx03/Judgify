package judging

import (
	"errors"
	"regexp"
	"sort"
	"strings"
)

const (
	RuntimeCatalogVersion   = 2
	MaximumCatalogRuntimes  = 32
	maximumRuntimeKeyBytes  = 64
	maximumDisplayNameBytes = 80
)

var runtimeKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

var ErrInvalidRuntimeCatalog = errors.New("judge runtime catalog is invalid")

type Language string

const (
	LanguageCPP    Language = "cpp"
	LanguagePython Language = "python"
	LanguageJava   Language = "java"
	LanguageGo     Language = "go"
)

// LanguageDescriptor is the transport-safe part of a judge runtime. Compiler
// commands and sandbox internals deliberately stay worker-only.
type LanguageDescriptor struct {
	ID             Language `json:"id"`
	SourceFilename string   `json:"source_filename"`
	FileExtensions []string `json:"file_extensions"`
	Compiled       bool     `json:"compiled"`
}

// RuntimeIdentity is the browser-safe part read from the active release. It
// deliberately excludes compiler, image, command and supervisor internals.
type RuntimeIdentity struct {
	Key         string
	Language    string
	DisplayName string
}

// RuntimeDescriptor is one server-activated runtime option. Language controls
// editor/file behavior while Key is the opaque value submitted back to intake.
type RuntimeDescriptor struct {
	Key            string   `json:"runtime_key"`
	Language       Language `json:"language"`
	DisplayName    string   `json:"display_name"`
	SourceFilename string   `json:"source_filename"`
	FileExtensions []string `json:"file_extensions"`
	Compiled       bool     `json:"compiled"`
}

// RuntimeCatalog tells clients exactly which immutable runtime identifiers the
// current release accepts. The version changes only when its contract changes.
type RuntimeCatalog struct {
	Version                int                 `json:"version"`
	MaximumSourceCodeBytes int                 `json:"maximum_source_code_bytes"`
	Runtimes               []RuntimeDescriptor `json:"runtimes"`
}

var languageCatalog = [...]LanguageDescriptor{
	{
		ID:             LanguageCPP,
		SourceFilename: "main.cpp",
		FileExtensions: []string{".cpp", ".cc", ".cxx", ".c"},
		Compiled:       true,
	},
	{
		ID:             LanguagePython,
		SourceFilename: "main.py",
		FileExtensions: []string{".py"},
	},
	{
		ID:             LanguageJava,
		SourceFilename: "Main.java",
		FileExtensions: []string{".java"},
		Compiled:       true,
	},
	{
		ID:             LanguageGo,
		SourceFilename: "main.go",
		FileExtensions: []string{".go"},
		Compiled:       true,
	},
}

func SupportedLanguages() []LanguageDescriptor {
	result := make([]LanguageDescriptor, len(languageCatalog))
	for index, descriptor := range languageCatalog {
		result[index] = descriptor
		result[index].FileExtensions = append([]string(nil), descriptor.FileExtensions...)
	}
	return result
}

func LanguageValues() []string {
	result := make([]string, len(languageCatalog))
	for index, descriptor := range languageCatalog {
		result[index] = string(descriptor.ID)
	}
	return result
}

func ParseLanguage(value string) (Language, bool) {
	normalized := Language(strings.TrimSpace(value))
	for _, descriptor := range languageCatalog {
		if descriptor.ID == normalized {
			return normalized, true
		}
	}
	return "", false
}

func NewRuntimeCatalog(identities []RuntimeIdentity) (RuntimeCatalog, error) {
	if len(identities) == 0 || len(identities) > MaximumCatalogRuntimes {
		return RuntimeCatalog{}, ErrInvalidRuntimeCatalog
	}
	sorted := append([]RuntimeIdentity(nil), identities...)
	sort.Slice(sorted, func(left int, right int) bool {
		return sorted[left].Key < sorted[right].Key
	})
	runtimes := make([]RuntimeDescriptor, 0, len(sorted))
	seen := make(map[string]RuntimeIdentity, len(sorted))
	for _, identity := range sorted {
		language, ok := ParseLanguage(identity.Language)
		metadata, metadataExists := languageMetadata(language)
		if !ok || !metadataExists ||
			string(language) != identity.Language ||
			len(identity.Key) == 0 ||
			len(identity.Key) > maximumRuntimeKeyBytes ||
			!runtimeKeyPattern.MatchString(identity.Key) ||
			identity.DisplayName == "" ||
			identity.DisplayName != strings.TrimSpace(identity.DisplayName) ||
			len(identity.DisplayName) > maximumDisplayNameBytes {
			return RuntimeCatalog{}, ErrInvalidRuntimeCatalog
		}
		if previous, exists := seen[identity.Key]; exists {
			if previous != identity {
				return RuntimeCatalog{}, ErrInvalidRuntimeCatalog
			}
			continue
		}
		seen[identity.Key] = identity
		runtimes = append(runtimes, RuntimeDescriptor{
			Key:            identity.Key,
			Language:       language,
			DisplayName:    identity.DisplayName,
			SourceFilename: metadata.SourceFilename,
			FileExtensions: append([]string(nil), metadata.FileExtensions...),
			Compiled:       metadata.Compiled,
		})
	}
	return RuntimeCatalog{
		Version:                RuntimeCatalogVersion,
		MaximumSourceCodeBytes: MaximumSourceCodeBytes,
		Runtimes:               runtimes,
	}, nil
}

func languageMetadata(language Language) (LanguageDescriptor, bool) {
	for _, descriptor := range languageCatalog {
		if descriptor.ID == language {
			return descriptor, true
		}
	}
	return LanguageDescriptor{}, false
}
