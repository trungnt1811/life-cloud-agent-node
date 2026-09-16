package domain_test

import (
	"path/filepath"
	"testing"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/archtest"
)

func TestDomainProductionCodeDoesNotImportOuterLayers(t *testing.T) {
	archtest.AssertProductionImports(t, archtest.ImportPolicy{
		RootDir: archtest.CallerDir(t),
		AllowedFirstPartyPrefixes: []string{
			"constants",
			"internal/domain",
			"internal/platform/logger",
		},
		ForbiddenFirstPartyPrefixes: archtest.CoreForbiddenFirstPartyPrefixes(),
		AllowedExternalPrefixes: []string{
			"github.com/google/uuid",
		},
		ForbiddenExternalPrefixes: archtest.FrameworkAndPersistenceExternalPrefixes(),
		ForbiddenStdlibPrefixes:   archtest.FrameworkAndPersistenceStdlibPrefixes(),
	})
}

func TestUsecaseProductionCodeDoesNotImportRuntimeOrOuterLayers(t *testing.T) {
	repoRoot := codebaseRoot(t)
	archtest.AssertProductionImports(t, archtest.ImportPolicy{
		RootDir: filepath.Join(repoRoot, "internal", "domain", "usecases"),
		AllowedFirstPartyPrefixes: []string{
			"constants",
			"internal/domain",
			"internal/platform/logger",
		},
		ForbiddenFirstPartyPrefixes: archtest.CoreForbiddenFirstPartyPrefixes(),
		ForbiddenExternalPrefixes:   archtest.FrameworkAndPersistenceExternalPrefixes(),
		ForbiddenStdlibPrefixes:     archtest.FrameworkAndPersistenceStdlibPrefixes(),
	})
}

func TestAdaptersProductionCodeDoesNotImportDelivery(t *testing.T) {
	repoRoot := codebaseRoot(t)
	archtest.AssertProductionImports(t, archtest.ImportPolicy{
		RootDir: filepath.Join(repoRoot, "internal", "adapters"),
		ForbiddenFirstPartyPrefixes: []string{
			"internal/delivery",
			"internal/di",
			"internal/server",
		},
	})
}

func TestDeliveryProductionCodeDoesNotImportPersistenceAdapters(t *testing.T) {
	repoRoot := codebaseRoot(t)
	archtest.AssertProductionImports(t, archtest.ImportPolicy{
		RootDir: filepath.Join(repoRoot, "internal", "delivery"),
		ForbiddenFirstPartyPrefixes: []string{
			"internal/adapters/postgres",
			"internal/adapters/repositories",
			"infrastructures",
		},
	})
}

func TestCodebaseDoesNotUseHandwrittenTestDoubles(t *testing.T) {
	archtest.AssertNoHandwrittenTestDoubles(t, archtest.TestDoublePolicy{
		RootDir: codebaseRoot(t),
		ExcludeDirs: []string{
			"docs",
			"mocks",
			"testdata",
		},
	})
}

func TestCodebaseGoFilesStayReviewable(t *testing.T) {
	archtest.AssertGoFileSizes(t, archtest.GoFileSizePolicy{
		RootDir:            codebaseRoot(t),
		MaxProductionLines: 500,
		MaxProductionBytes: 25 * 1024,
		MaxTestLines:       1000,
		ExcludeDirs: []string{
			"docs",
			"gen",
			"mocks",
		},
	})
}

func codebaseRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(archtest.CallerDir(t), "..", ".."))
}
