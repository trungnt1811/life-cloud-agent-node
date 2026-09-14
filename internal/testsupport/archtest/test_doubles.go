package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDoublePolicy describes naming rules for handwritten test doubles.
type TestDoublePolicy struct {
	RootDir string

	ForbiddenNameFragments []string
	AllowedTypeNames       []string
	ExcludeDirs            []string
	ExcludeFiles           []string
	IgnoredPathFragments   []string
}

// AssertNoHandwrittenTestDoubles fails when test code declares handwritten fake/stub/mock/spy types.
func AssertNoHandwrittenTestDoubles(tb testing.TB, policy TestDoublePolicy) {
	tb.Helper()

	if strings.TrimSpace(policy.RootDir) == "" {
		tb.Fatal("archtest test double policy requires RootDir")
	}

	moduleRoot, _ := findGoModule(tb, policy.RootDir)
	excludedDirs := stringSet(policy.ExcludeDirs)
	excludedFiles := stringSet(policy.ExcludeFiles)
	allowedTypes := stringSet(policy.AllowedTypeNames)
	forbiddenFragments := normalizedForbiddenNameFragments(policy.ForbiddenNameFragments)

	var violations []string
	err := filepath.WalkDir(policy.RootDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == dirVendor || name == dirGit || excludedDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, "_test.go") || excludedFiles[name] {
			return nil
		}
		if ignoredProductionImportPath(path, policy.IgnoredPathFragments) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isGeneratedGoFile(content) {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil || allowedTypes[typeSpec.Name.Name] {
					continue
				}
				if matched := matchingNameFragment(typeSpec.Name.Name, forbiddenFragments); matched != "" {
					position := fset.Position(typeSpec.Name.Pos())
					position.Filename = displayPath(moduleRoot, position.Filename)
					violations = append(violations, fmt.Sprintf("%s declares handwritten test double type %q matching %q", position, typeSpec.Name.Name, matched))
				}
			}
		}
		return nil
	})
	if err != nil {
		tb.Fatalf("walk test double policy root: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		tb.Fatalf("handwritten test double violations:\n%s", strings.Join(violations, "\n"))
	}
}

func normalizedForbiddenNameFragments(values []string) []string {
	if len(values) == 0 {
		values = []string{"fake", "stub", "mock", "spy"}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func matchingNameFragment(name string, fragments []string) string {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	for _, fragment := range fragments {
		if strings.Contains(normalizedName, fragment) {
			return fragment
		}
	}
	return ""
}

func isGeneratedGoFile(content []byte) bool {
	header := string(content)
	if len(header) > 2048 {
		header = header[:2048]
	}
	return strings.Contains(header, "Code generated") && strings.Contains(header, "DO NOT EDIT")
}
