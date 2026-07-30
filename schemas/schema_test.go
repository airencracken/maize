package schemas_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestInspectSchemaIsValidAndRequiresPublicContractFields(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("maize.inspect.v1.schema.json")
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
	expected := []string{
		"schema", "installed_count", "world_selections",
		"system_selections", "recommendations",
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
