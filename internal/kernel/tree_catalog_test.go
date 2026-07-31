package kernel_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/kernel"
)

func TestLoadKconfigCatalogCollectsPurposeAcrossTree(t *testing.T) {
	t.Parallel()

	tree := t.TempDir()
	writeKconfigTreeFile(t, tree, "Makefile", "fixture\n")
	writeKconfigTreeFile(t, tree, "Kconfig", "source \"drivers/Kconfig\"\n")
	writeKconfigTreeFile(t, tree, "drivers/Kconfig", `
config EXAMPLE
	tristate "Example peripheral support"
	help
	  Supports the example peripheral used by test hardware.
`)
	writeKconfigTreeFile(t, tree, "arch/foreign/Kconfig", `
config FOREIGN_ONLY
	bool "Foreign architecture option"
`)
	catalog, err := kernel.LoadKconfigCatalog(context.Background(), tree)
	if err != nil {
		t.Fatal(err)
	}
	symbol, _ := kernel.ParseSymbol("EXAMPLE")
	definition, found := catalog.Get(symbol)
	if !found || definition.Prompt != "Example peripheral support" ||
		!strings.Contains(definition.Help, "test hardware") ||
		definition.Location.Path != filepath.Join(tree, "drivers", "Kconfig") {
		t.Fatalf("definition = %#v, found %v", definition, found)
	}
	foreign, _ := kernel.ParseSymbol("FOREIGN_ONLY")
	if _, found := catalog.Get(foreign); found {
		t.Fatal("foreign architecture definition was included")
	}
}

func TestLoadKconfigCatalogRejectsCancellationAndMalformedInputAtomically(t *testing.T) {
	t.Parallel()

	tree := t.TempDir()
	writeKconfigTreeFile(t, tree, "Makefile", "fixture\n")
	writeKconfigTreeFile(t, tree, "Kconfig", "config ../../HOSTILE\n\tbool\n")
	catalog, err := kernel.LoadKconfigCatalog(context.Background(), tree)
	if err == nil || !reflect.DeepEqual(catalog, kernel.Catalog{}) {
		t.Fatalf("catalog %#v, error %v", catalog, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	catalog, err = kernel.LoadKconfigCatalog(ctx, tree)
	if err == nil || !reflect.DeepEqual(catalog, kernel.Catalog{}) {
		t.Fatalf("canceled catalog %#v, error %v", catalog, err)
	}
}

func writeKconfigTreeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
