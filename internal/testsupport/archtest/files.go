package archtest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// GoFileSizePolicy describes reviewability budgets for Go source files.
type GoFileSizePolicy struct {
	RootDir string

	MaxProductionLines int
	MaxProductionBytes int
	MaxTestLines       int
	ExcludeDirs        []string
	ExcludeFiles       []string
}

func AssertGoFileSizes(tb testing.TB, policy GoFileSizePolicy) {
	tb.Helper()

	if strings.TrimSpace(policy.RootDir) == "" {
		tb.Fatal("archtest file size policy requires RootDir")
	}

	moduleRoot, _ := findGoModule(tb, policy.RootDir)
	excludedDirs := stringSet(policy.ExcludeDirs)
	excludedFiles := stringSet(policy.ExcludeFiles)
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
		if !strings.HasSuffix(name, ".go") || excludedFiles[name] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		display := displayPath(moduleRoot, path)
		lineCount := strings.Count(string(content), "\n") + 1
		if strings.HasSuffix(name, "_test.go") {
			if policy.MaxTestLines > 0 && lineCount > policy.MaxTestLines {
				violations = append(violations, fmt.Sprintf("%s has %d lines; split tests by behavior before growing beyond %d lines", display, lineCount, policy.MaxTestLines))
			}
			return nil
		}
		if policy.MaxProductionLines > 0 && lineCount > policy.MaxProductionLines {
			violations = append(violations, fmt.Sprintf("%s has %d lines; split production code by concern before growing beyond %d lines", display, lineCount, policy.MaxProductionLines))
		}
		if policy.MaxProductionBytes > 0 && len(content) > policy.MaxProductionBytes {
			violations = append(violations, fmt.Sprintf("%s has %d bytes; split production code by concern before growing beyond %d bytes", display, len(content), policy.MaxProductionBytes))
		}
		return nil
	})
	if err != nil {
		tb.Fatalf("walk file size policy root: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		tb.Fatalf("file size budget violations:\n%s", strings.Join(violations, "\n"))
	}
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized != "" {
			out[normalized] = true
		}
	}
	return out
}
