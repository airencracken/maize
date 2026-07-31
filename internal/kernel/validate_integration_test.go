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
	configPath := filepath.Join(tree, ".config")
	file, err := os.Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config, parseErr := kernel.ParseConfig(configPath, file)
	closeErr := file.Close()
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	result, err := kernel.ValidateTarget(context.Background(), tree, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Config.Entries()) == 0 {
		t.Fatal("target validation returned an empty configuration")
	}
}
