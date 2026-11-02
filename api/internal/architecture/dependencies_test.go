package architecture_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestRootCatalogsUseOneFilePerDomain(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	expected := map[string][]string{
		"ports": {
			"identity.go", "problem.go", "submission.go", "contest.go",
			"cultivation.go", "material.go", "notification.go", "judge.go", "platform.go",
		},
		"constant": {
			"identity.go", "problem.go", "submission.go", "contest.go",
			"cultivation.go", "material.go", "notification.go", "judge.go", "platform.go",
		},
	}
	for catalog, files := range expected {
		t.Run(catalog, func(t *testing.T) {
			entries, err := os.ReadDir(filepath.Join(root, "internal", catalog))
			if err != nil {
				t.Fatalf("read %s catalog: %v", catalog, err)
			}
			actual := make([]string, 0, len(entries))
			for _, entry := range entries {
				if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" && !strings.HasSuffix(entry.Name(), "_test.go") {
					actual = append(actual, entry.Name())
				}
			}
			sort.Strings(actual)
			sortedExpected := slices.Clone(files)
			sort.Strings(sortedExpected)
			if !slices.Equal(actual, sortedExpected) {
				t.Fatalf("%s catalog files = %v, want %v", catalog, actual, sortedExpected)
			}
		})
	}
}

func TestIdentityPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "identity", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Identity port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Identity port catalog: %v", err)
	}
}

func TestProblemPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "problem", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Problem port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Problem port catalog: %v", err)
	}
}

func TestSubmissionPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "submission", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Submission port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Submission port catalog: %v", err)
	}
}

func TestContestPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "contest", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Contest port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Contest port catalog: %v", err)
	}
}

func TestCultivationPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "cultivation", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Cultivation port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Cultivation port catalog: %v", err)
	}
}

func TestMaterialPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "material", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Material port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Material port catalog: %v", err)
	}
}

func TestNotificationPortCatalogLivesAtRoot(t *testing.T) {
	t.Parallel()
	legacy := filepath.Join(moduleRoot(t), "internal", "notification", "ports")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("legacy Notification port catalog still exists at %s", legacy)
		}
		t.Fatalf("inspect legacy Notification port catalog: %v", err)
	}
}

func TestDrivenDBUsesFlatFeatureFiles(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	for _, domain := range []string{
		"identity", "problem", "submission", "contest", "cultivation", "material", "notification",
	} {
		for _, legacyFolder := range []string{"builder", "mapper"} {
			path := filepath.Join(root, "internal", domain, "adapters", "driven", "db", legacyFolder)
			if _, err := os.Stat(path); err == nil {
				t.Fatalf("%s must use flat feature files; legacy %s folder still exists", domain, legacyFolder)
			} else if !os.IsNotExist(err) {
				t.Fatalf("inspect %s: %v", path, err)
			}
		}
	}
}

func TestBusinessDomainMocksLiveInTestutil(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	for _, domain := range []string{
		"identity", "problem", "submission", "contest", "cultivation", "material", "notification",
	} {
		path := filepath.Join(root, "internal", domain, "mocks")
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s mocks must live under internal/testutil/mocks, not %s", domain, path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", path, err)
		}
	}
}

func TestBusinessDomainCompositionLivesAtRoot(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	for _, domain := range []string{
		"identity", "problem", "submission", "contest", "cultivation", "material", "notification",
	} {
		path := filepath.Join(root, "internal", domain, "di")
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s composition must live under internal/di, not %s", domain, path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", path, err)
		}
	}
}

func TestBusinessDomainsDoNotUseLegacyCoreLayers(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	legacyPaths := []string{
		filepath.Join(root, "internal", "identity", "core", "entity"),
		filepath.Join(root, "internal", "identity", "core", "service"),
	}
	for _, domain := range []string{
		"identity", "problem", "submission", "contest", "cultivation", "material", "notification",
	} {
		legacyPaths = append(legacyPaths,
			filepath.Join(root, "internal", domain, "core", "dto"),
			filepath.Join(root, "internal", domain, "core", "mapper"),
		)
	}
	for _, legacy := range legacyPaths {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy core layer still exists: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", legacy, err)
		}
	}
}

func TestHTTPAdaptersUseFeatureRouteFiles(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "handler.go" {
			return nil
		}
		if strings.Contains(filepath.ToSlash(path), "/adapters/driver/http/") {
			return fmt.Errorf("HTTP adapter must use a feature route file, not %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProductionCodeDoesNotImportTestUtilities(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	violations := make([]string, 0)
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), modulePath+"/internal/testutil") {
			violations = append(violations, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect production imports: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("production code imports internal/testutil:\n%s", strings.Join(violations, "\n"))
	}
}

func TestMigratedApplicationContractsDoNotCarryHTTPMetadata(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	contracts := []string{
		filepath.Join(root, "internal", "problem", "contract", "difficulty.go"),
		filepath.Join(root, "internal", "problem", "contract", "tag.go"),
		filepath.Join(root, "internal", "problem", "contract", "test_case.go"),
		filepath.Join(root, "internal", "problem", "contract", "problem.go"),
		filepath.Join(root, "internal", "material", "contract", "material_category.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "element.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "level.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "rank.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "rarity.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "trait.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "user_trait.go"),
		filepath.Join(root, "internal", "cultivation", "contract", "ranking.go"),
		filepath.Join(root, "internal", "contest", "contract", "standing.go"),
		filepath.Join(root, "internal", "contest", "contract", "rating.go"),
		filepath.Join(root, "internal", "contest", "contract", "content.go"),
		filepath.Join(root, "internal", "contest", "contract", "contest.go"),
		filepath.Join(root, "internal", "submission", "contract", "submission.go"),
		filepath.Join(root, "internal", "notification", "contract", "notification.go"),
		filepath.Join(root, "internal", "identity", "contract", "common.go"),
		filepath.Join(root, "internal", "identity", "contract", "public_profile.go"),
		filepath.Join(root, "internal", "identity", "contract", "session.go"),
		filepath.Join(root, "internal", "identity", "contract", "user_lifecycle.go"),
		filepath.Join(root, "internal", "identity", "contract", "authentication.go"),
		filepath.Join(root, "internal", "identity", "contract", "role.go"),
		filepath.Join(root, "internal", "identity", "contract", "authorization.go"),
		filepath.Join(root, "internal", "identity", "contract", "attribute_definition.go"),
		filepath.Join(root, "internal", "identity", "contract", "user.go"),
		filepath.Join(root, "internal", "identity", "contract", "profile_schema.go"),
		filepath.Join(root, "internal", "identity", "contract", "user_invitation.go"),
	}
	for _, path := range contracts {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, marker := range []string{"`json:", "`uri:", "`form:", "`validate:"} {
			if strings.Contains(string(content), marker) {
				t.Fatalf("application contract %s contains HTTP metadata %q", path, marker)
			}
		}
	}
}

func TestContractHTTPMetadataIsRestrictedToExplicitIntegrationSchemas(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	allowed := map[string]struct{}{
		"identity/contract/events.go":                    {},
		"contest/contract/events.go":                     {},
		"cultivation/contract/ranking_reconciliation.go": {},
		"cultivation/contract/solve_reward.go":           {},
		"cultivation/contract/trait_offer.go":            {},
		"material/contract/events.go":                    {},
		"material/contract/material.go":                  {},
		"notification/contract/campaign_dto.go":          {},
		"notification/contract/campaign_event.go":        {},
		"notification/contract/notification_event.go":    {},
		"problem/contract/events.go":                     {},
		"submission/contract/finalization.go":            {},
		"submission/contract/intake.go":                  {},
	}

	for _, domain := range []string{"identity", "contest", "cultivation", "material", "notification", "problem", "submission"} {
		matches, err := filepath.Glob(filepath.Join(root, "internal", domain, "contract", "*.go"))
		if err != nil {
			t.Fatalf("glob %s contracts: %v", domain, err)
		}
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if !strings.Contains(string(content), "`json:") && !strings.Contains(string(content), "`uri:") && !strings.Contains(string(content), "`form:") && !strings.Contains(string(content), "`validate:") {
				continue
			}
			relative, err := filepath.Rel(filepath.Join(root, "internal"), path)
			if err != nil {
				t.Fatalf("relativize %s: %v", path, err)
			}
			if _, ok := allowed[filepath.ToSlash(relative)]; !ok {
				t.Fatalf("contract %s carries HTTP metadata without explicit integration-schema allowance", relative)
			}
		}
	}
}

const modulePath = "github.com/huynhanx03/judgify"

type dependencyRule struct {
	fromPrefix string
	imports    []string
}

var forbiddenDependencies = []dependencyRule{
	{
		fromPrefix: modulePath + "/internal/identity",
		imports: []string{
			"/internal/problem/",
			"/internal/submission/",
			"/internal/contest/",
			"/internal/cultivation/",
			"/internal/material/",
		},
	},
	{
		fromPrefix: modulePath + "/internal/problem",
		imports: []string{
			"/internal/identity/adapters/",
			"/internal/submission/adapters/",
			"/internal/contest/adapters/",
		},
	},
	{
		fromPrefix: modulePath + "/internal/submission",
		imports: []string{
			"/internal/identity/adapters/",
			"/internal/problem/adapters/",
			"/internal/contest/adapters/",
		},
	},
	{
		fromPrefix: modulePath + "/internal/contest",
		imports: []string{
			"/internal/identity/adapters/",
			"/internal/problem/adapters/",
			"/internal/submission/adapters/",
		},
	},
}

func TestModuleDependencies(t *testing.T) {
	pkgs := loadPackages(t, "./internal/...")
	violations := make([]string, 0)

	for _, pkg := range pkgs {
		for _, rule := range forbiddenDependencies {
			if !strings.HasPrefix(pkg.PkgPath, rule.fromPrefix) {
				continue
			}

			for importedPath := range pkg.Imports {
				for _, fragment := range rule.imports {
					if strings.Contains(importedPath, fragment) {
						violations = append(violations, formatViolation("module", pkg.PkgPath, importedPath))
					}
				}
			}
		}
	}

	assertViolationBaseline(t, "module", violations)
}

func loadPackages(t *testing.T, patterns ...string) []*packages.Package {
	t.Helper()

	cfg := &packages.Config{
		Dir:  moduleRoot(t),
		Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) == 0 {
			continue
		}

		messages := make([]string, 0, len(pkg.Errors))
		for _, pkgErr := range pkg.Errors {
			messages = append(messages, pkgErr.Error())
		}
		sort.Strings(messages)
		t.Fatalf("load package %s:\n%s", pkg.PkgPath, strings.Join(messages, "\n"))
	}

	return pkgs
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	return root
}

func formatViolation(kind, source, imported string) string {
	return fmt.Sprintf("%s|%s|%s", kind, source, imported)
}

func assertViolationBaseline(t *testing.T, kind string, observed []string) {
	t.Helper()

	sort.Strings(observed)
	baselinePath := filepath.Join("testdata", "known_violations.txt")
	file, err := os.Open(baselinePath)
	if err != nil {
		t.Fatalf(
			"read architecture baseline %s: %v\nobserved %s violations:\n%s",
			baselinePath,
			err,
			kind,
			strings.Join(observed, "\n"),
		)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close architecture baseline: %v", closeErr)
		}
	}()

	all := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		all = append(all, line)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read architecture baseline: %v", err)
	}

	sorted := slices.Clone(all)
	sort.Strings(sorted)
	if !slices.Equal(all, sorted) {
		t.Fatalf("architecture baseline must be sorted:\n%s", strings.Join(sorted, "\n"))
	}

	if os.Getenv("JUDGIFY_ARCH_STRICT") == "1" && len(all) > 0 {
		t.Fatalf("strict architecture check rejects known violations:\n%s", strings.Join(all, "\n"))
	}

	prefix := kind + "|"
	expected := make([]string, 0)
	for _, line := range all {
		if strings.HasPrefix(line, prefix) {
			expected = append(expected, line)
		}
	}

	if !slices.Equal(expected, observed) {
		t.Fatalf(
			"%s architecture violations changed\nexpected baseline:\n%s\nobserved:\n%s",
			kind,
			strings.Join(expected, "\n"),
			strings.Join(observed, "\n"),
		)
	}
}
