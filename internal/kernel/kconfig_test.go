package kernel_test

import (
	"os"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/kernel"
)

func TestParseRealKconfigExcerpts(t *testing.T) {
	t.Parallel()

	catalog := parseKconfigFixture(t, "testdata/Kconfig.preempt-7.1.5")
	definition, found := catalog.Get(mustSymbol(t, "PREEMPT_LAZY"))
	if !found {
		t.Fatal("PREEMPT_LAZY not parsed")
	}
	if definition.Type != kernel.TypeBool ||
		definition.Prompt != "Scheduler controlled preemption model" ||
		len(definition.DependsOn) != 2 ||
		len(definition.Selects) != 1 ||
		!strings.Contains(definition.Help, "scheduler driven") {
		t.Fatalf("definition lost semantics: %#v", definition)
	}
}

func TestParseKconfigRejectsDuplicateDefinitions(t *testing.T) {
	t.Parallel()

	_, err := kernel.ParseKconfig("duplicate", strings.NewReader(
		"config TEST\n\tbool\nconfig TEST\n\ttristate\n",
	))
	if err == nil {
		t.Fatal("duplicate Kconfig definition accepted")
	}
}

func TestCatalogDefinitionsAreDeterministic(t *testing.T) {
	t.Parallel()

	catalog, err := kernel.ParseKconfig("fixture", strings.NewReader(
		"config ZED\n\tbool\nconfig ALPHA\n\ttristate\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	definitions := catalog.Definitions()
	if len(definitions) != 2 || definitions[0].Symbol.String() != "CONFIG_ALPHA" {
		t.Fatalf("definitions not sorted: %#v", definitions)
	}
}

func TestCatalogReturnsOwnedDefinitions(t *testing.T) {
	t.Parallel()

	symbol := mustSymbol(t, "TEST")
	catalog, err := kernel.NewCatalog(kernel.Definition{
		Symbol: symbol, Type: kernel.TypeBool, DependsOn: []string{"FIRST"},
	})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := catalog.Get(symbol)
	if !found {
		t.Fatal("definition not found")
	}
	definition.DependsOn[0] = "MUTATED"
	again, _ := catalog.Get(symbol)
	if again.DependsOn[0] != "FIRST" {
		t.Fatalf("returned definition aliases catalog: %#v", again)
	}
}

func parseKconfigFixture(t *testing.T, path string) kernel.Catalog {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	catalog, err := kernel.ParseKconfig(path, file)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
