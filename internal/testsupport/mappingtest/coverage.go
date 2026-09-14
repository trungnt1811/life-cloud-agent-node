package mappingtest

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// CoverageOption customizes field coverage assertions for intentional gaps.
type CoverageOption func(*coverageOptions)

type coverageOptions struct {
	sourceIgnore map[string]struct{}
	targetIgnore map[string]struct{}
}

// IgnoreSource excludes source fields that intentionally have no target mapping.
func IgnoreSource(fields ...string) CoverageOption {
	return func(opts *coverageOptions) {
		for _, field := range fields {
			opts.sourceIgnore[field] = struct{}{}
		}
	}
}

// IgnoreTarget excludes target fields that intentionally have no source mapping.
func IgnoreTarget(fields ...string) CoverageOption {
	return func(opts *coverageOptions) {
		for _, field := range fields {
			opts.targetIgnore[field] = struct{}{}
		}
	}
}

// AssertSameNamedFieldCoverage requires every exported leaf field to map by name.
func AssertSameNamedFieldCoverage(tb testing.TB, source, target any, opts ...CoverageOption) {
	tb.Helper()

	sourceFields := exportedLeafFields(reflect.TypeOf(source), "")
	mappings := make(map[string]string, len(sourceFields))
	for _, field := range sourceFields {
		mappings[field] = field
	}
	AssertFieldCoverage(tb, source, target, mappings, opts...)
}

// AssertFieldCoverage requires explicit mappings to cover every exported leaf field.
func AssertFieldCoverage(tb testing.TB, source, target any, mappings map[string]string, opts ...CoverageOption) {
	tb.Helper()

	options := coverageOptions{
		sourceIgnore: map[string]struct{}{},
		targetIgnore: map[string]struct{}{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	sourceFields := fieldSet(exportedLeafFields(reflect.TypeOf(source), ""))
	targetFields := fieldSet(exportedLeafFields(reflect.TypeOf(target), ""))
	mappedSources := make(map[string]struct{}, len(mappings))
	mappedTargets := make(map[string]struct{}, len(mappings))
	for sourceField, targetField := range mappings {
		mappedSources[sourceField] = struct{}{}
		mappedTargets[targetField] = struct{}{}
	}

	requireNoUnknownMappings(tb, "source", mappedSources, sourceFields)
	requireNoUnknownMappings(tb, "target", mappedTargets, targetFields)
	requireNoMissingMappings(tb, "source", sourceFields, mappedSources, options.sourceIgnore)
	requireNoMissingMappings(tb, "target", targetFields, mappedTargets, options.targetIgnore)
}

func exportedLeafFields(valueType reflect.Type, prefix string) []string {
	valueType = dereferenceType(valueType)
	if valueType.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]string, 0, valueType.NumField())
	for index := 0; index < valueType.NumField(); index++ {
		field := valueType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		name := field.Name
		if prefix != "" {
			name = prefix + "." + name
		}
		if shouldDescend(field.Type) {
			fields = append(fields, exportedLeafFields(field.Type, name)...)
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func shouldDescend(fieldType reflect.Type) bool {
	fieldType = dereferenceType(fieldType)
	return fieldType.Kind() == reflect.Struct && strings.HasSuffix(fieldType.Name(), "Record")
}

func dereferenceType(valueType reflect.Type) reflect.Type {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	return valueType
}

func fieldSet(fields []string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[field] = struct{}{}
	}
	return result
}

func requireNoUnknownMappings(tb testing.TB, side string, mapped, available map[string]struct{}) {
	tb.Helper()

	var unknown []string
	for field := range mapped {
		if _, ok := available[field]; !ok {
			unknown = append(unknown, field)
		}
	}
	if len(unknown) == 0 {
		return
	}
	sort.Strings(unknown)
	tb.Fatalf("mapper references unknown %s fields: %s", side, strings.Join(unknown, ", "))
}

func requireNoMissingMappings(tb testing.TB, side string, available, mapped, ignored map[string]struct{}) {
	tb.Helper()

	var missing []string
	for field := range available {
		if _, ok := mapped[field]; ok {
			continue
		}
		if _, ok := ignored[field]; ok {
			continue
		}
		missing = append(missing, field)
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	tb.Fatalf("mapper does not cover %s fields: %s", side, strings.Join(missing, ", "))
}
