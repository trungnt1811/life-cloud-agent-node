package archtest

import "testing"

func TestImportViolationMatchesPackageBoundaries(t *testing.T) {
	policy := ImportPolicy{
		ForbiddenFirstPartyPrefixes: []string{"internal/adapters"},
		ForbiddenStdlibPrefixes:     []string{"net/http"},
	}

	assertForbiddenImport(t, "github.com/acme/app/internal/adapters/repositories", "github.com/acme/app", policy)
	assertForbiddenImport(t, "net/http/httptest", "github.com/acme/app", policy)
	assertAllowedImport(t, "github.com/acme/app/internal/adaptersx/repositories", "github.com/acme/app", policy)
	assertAllowedImport(t, "net/httpx", "github.com/acme/app", policy)
}

func TestImportViolationRequiresExternalAllowlist(t *testing.T) {
	policy := ImportPolicy{
		AllowedExternalPrefixes: []string{"github.com/google/uuid"},
	}

	assertAllowedImport(t, "github.com/google/uuid", "github.com/acme/app", policy)
	assertAllowedImport(t, "github.com/google/uuid/internal", "github.com/acme/app", policy)
	assertForbiddenImport(t, "github.com/google/uuidx", "github.com/acme/app", policy)
	assertForbiddenImport(t, "github.com/acme/other", "github.com/acme/app", policy)
}

func assertAllowedImport(t *testing.T, importPath, modulePath string, policy ImportPolicy) {
	t.Helper()

	if reason, forbidden := importViolation(importPath, modulePath, policy); forbidden {
		t.Fatalf("expected %q to be allowed, got violation: %s", importPath, reason)
	}
}

func assertForbiddenImport(t *testing.T, importPath, modulePath string, policy ImportPolicy) {
	t.Helper()

	if _, forbidden := importViolation(importPath, modulePath, policy); !forbidden {
		t.Fatalf("expected %q to be forbidden", importPath)
	}
}
