package queryfields

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationSeedMatchesSchemaV1FieldCodes(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	scriptPath := filepath.Clean(filepath.Join(
		filepath.Dir(thisFile),
		"..", "..", "adapters", "postgres", "scripts",
		"03_create_enabled_query_fields_table.sql",
	))
	content, err := os.ReadFile(scriptPath)
	require.NoError(t, err)

	matches := regexp.MustCompile(`\('([A-Z0-9]+)',\s*FALSE`).FindAllStringSubmatch(string(content), -1)
	require.NotEmpty(t, matches)

	seeded := make([]string, 0, len(matches))
	for _, match := range matches {
		seeded = append(seeded, match[1])
	}
	require.Equal(t, SchemaV1FieldCodes, seeded,
		"migration seed must stay identical to SchemaV1FieldCodes; update both together")
}
