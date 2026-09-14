package archtest

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// ImportPolicy describes direct-import boundaries for production Go files.
type ImportPolicy struct {
	RootDir string

	AllowedFirstPartyPrefixes   []string
	ForbiddenFirstPartyPrefixes []string
	AllowedExternalPrefixes     []string
	ForbiddenExternalPrefixes   []string
	ForbiddenStdlibPrefixes     []string
	IgnoredPathFragments        []string
}

func CoreForbiddenFirstPartyPrefixes() []string {
	return []string{
		"conf",
		"infrastructures",
		"internal/adapters",
		"internal/configmapper",
		"internal/delivery",
		"internal/di",
		"internal/pkg",
		"internal/server",
		"internal/workers",
	}
}

func FrameworkAndPersistenceExternalPrefixes() []string {
	return []string{
		"github.com/gin-gonic/gin",
		"gorm.io",
	}
}

func FrameworkAndPersistenceStdlibPrefixes() []string {
	return []string{
		"database/sql",
		"net/http",
	}
}

func CallerDir(tb testing.TB) string {
	tb.Helper()

	_, currentFile, _, ok := runtime.Caller(1)
	if !ok {
		tb.Fatal("resolve caller file")
	}
	return filepath.Dir(currentFile)
}

func AssertProductionImports(tb testing.TB, policy ImportPolicy) {
	tb.Helper()

	if strings.TrimSpace(policy.RootDir) == "" {
		tb.Fatal("archtest import policy requires RootDir")
	}

	moduleRoot, modulePath := findGoModule(tb, policy.RootDir)
	var violations []string

	err := filepath.WalkDir(policy.RootDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == dirVendor || entry.Name() == dirGit {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		if ignoredProductionImportPath(path, policy.IgnoredPathFragments) {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imported := range file.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			reason, forbidden := importViolation(importPath, modulePath, policy)
			if !forbidden {
				continue
			}
			position := fset.Position(imported.Pos())
			position.Filename = displayPath(moduleRoot, position.Filename)
			violations = append(violations, fmt.Sprintf("%s imports %q: %s", position, importPath, reason))
		}
		return nil
	})
	if err != nil {
		tb.Fatalf("walk architecture boundary root: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		tb.Fatalf("architecture import boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}

func ignoredProductionImportPath(path string, ignoredFragments []string) bool {
	if len(ignoredFragments) == 0 {
		return false
	}
	normalizedPath := filepath.ToSlash(path)
	for _, fragment := range ignoredFragments {
		normalizedFragment := filepath.ToSlash(strings.TrimSpace(fragment))
		if normalizedFragment != "" && strings.Contains(normalizedPath, normalizedFragment) {
			return true
		}
	}
	return false
}

func importViolation(importPath, modulePath string, policy ImportPolicy) (string, bool) {
	if rel, ok := firstPartyImport(importPath, modulePath); ok {
		if matched := matchingPackagePrefix(rel, policy.ForbiddenFirstPartyPrefixes); matched != "" {
			return fmt.Sprintf("forbidden first-party layer %q", matched), true
		}
		if len(policy.AllowedFirstPartyPrefixes) > 0 && matchingPackagePrefix(rel, policy.AllowedFirstPartyPrefixes) == "" {
			return "first-party package is not in this layer's allowlist", true
		}
		return "", false
	}

	if isStdlibImport(importPath) {
		if matched := matchingPackagePrefix(importPath, policy.ForbiddenStdlibPrefixes); matched != "" {
			return fmt.Sprintf("forbidden stdlib package %q", matched), true
		}
		return "", false
	}

	if matched := matchingPackagePrefix(importPath, policy.ForbiddenExternalPrefixes); matched != "" {
		return fmt.Sprintf("forbidden external package %q", matched), true
	}
	if len(policy.AllowedExternalPrefixes) > 0 && matchingPackagePrefix(importPath, policy.AllowedExternalPrefixes) == "" {
		return "external package is not in this layer's allowlist", true
	}
	return "", false
}

func firstPartyImport(importPath, modulePath string) (string, bool) {
	if importPath == modulePath {
		return ".", true
	}
	prefix := modulePath + "/"
	if strings.HasPrefix(importPath, prefix) {
		return strings.TrimPrefix(importPath, prefix), true
	}
	return "", false
}

func isStdlibImport(importPath string) bool {
	firstSegment, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(firstSegment, ".")
}

func matchingPackagePrefix(importPath string, prefixes []string) string {
	for _, prefix := range prefixes {
		normalizedImport := normalizePackagePath(importPath)
		normalizedPrefix := normalizePackagePath(prefix)
		if normalizedPrefix == "" {
			continue
		}
		if normalizedImport == normalizedPrefix || strings.HasPrefix(normalizedImport, normalizedPrefix+"/") {
			return normalizedPrefix
		}
	}
	return ""
}

func normalizePackagePath(value string) string {
	return strings.Trim(strings.TrimSpace(value), "/")
}

func findGoModule(tb testing.TB, startDir string) (string, string) {
	tb.Helper()

	dir, err := filepath.Abs(startDir)
	if err != nil {
		tb.Fatalf("resolve module search dir: %v", err)
	}
	for {
		goModPath := filepath.Join(dir, "go.mod")
		content, err := os.ReadFile(goModPath)
		if err == nil {
			modulePath := parseModulePath(content)
			if modulePath == "" {
				tb.Fatalf("%s does not declare a module path", goModPath)
			}
			return dir, modulePath
		}
		if !os.IsNotExist(err) {
			tb.Fatalf("read %s: %v", goModPath, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatalf("could not find go.mod from %s", startDir)
		}
		dir = parent
	}
}

func parseModulePath(content []byte) string {
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}

func displayPath(moduleRoot, path string) string {
	relative, err := filepath.Rel(moduleRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
