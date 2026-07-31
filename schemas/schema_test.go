package schemas_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestInspectSchemaIsValidAndRequiresPublicContractFields(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path     string
		required []string
	}{
		{"maize.inspect.v1.schema.json", []string{
			"schema", "installed_count", "world_selections",
			"system_selections", "recommendations",
		}},
		{"maize.inspect.v2.schema.json", []string{
			"schema", "config_source", "hardware", "snapshot_consistency",
			"repositories", "installed_count",
			"world_selections", "system_selections", "recommendations",
		}},
		{"maize.hardware.v1.schema.json", []string{"schema", "hardware"}},
		{"maize.migration.v1.schema.json", []string{"schema", "changes"}},
		{"maize.migration.v2.schema.json", []string{"schema", "context", "changes"}},
	} {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			assertSchema(t, test.path, test.required)
		})
	}
}

func assertSchema(t *testing.T, path string, expected []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Schema     string                     `json:"$schema"`
		Type       string                     `json:"type"`
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("invalid JSON schema: %v", err)
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" ||
		schema.Type != "object" || !reflect.DeepEqual(schema.Required, expected) {
		t.Fatalf("schema contract = %#v", schema)
	}
	for _, name := range expected {
		if _, found := schema.Properties[name]; !found {
			t.Errorf("required property %q has no schema", name)
		}
	}
}
