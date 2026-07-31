package kernel_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/airencracken/maize/internal/kernel"
)

func TestValidateTargetAgainstInstalledKernelTree(t *testing.T) {
	tree := os.Getenv("MAIZE_TEST_KERNEL_TREE")
	if tree == "" {
		t.Skip("MAIZE_TEST_KERNEL_TREE is not set")
	}
	configPath := os.Getenv("MAIZE_TEST_KERNEL_CONFIG")
	if configPath == "" {
		configPath = filepath.Join(tree, ".config")
	}
	config, _, err := kernel.LoadConfig(
		context.Background(), kernel.ConfigPaths{Explicit: configPath},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := kernel.ValidateTarget(context.Background(), tree, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Config.Entries()) == 0 {
		t.Fatal("target validation returned an empty configuration")
	}
}

func TestLoadCatalogFromInstalledKernelTree(t *testing.T) {
	tree := os.Getenv("MAIZE_TEST_KERNEL_TREE")
	if tree == "" {
		t.Skip("MAIZE_TEST_KERNEL_TREE is not set")
	}
	catalog, err := kernel.LoadKconfigCatalog(context.Background(), tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Definitions()) < 1000 {
		t.Fatalf("installed kernel catalog has only %d definitions", len(catalog.Definitions()))
	}
}
