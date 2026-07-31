package kernel_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/airencracken/maize/internal/kernel"
)

func TestLoadConfigReadsProcConfigGzipBySignature(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := kernel.DefaultConfigPaths(root)
	writeKernelFile(t, paths.ProcRelease, []byte("6.12.1-gentoo\n"))
	writeGzip(t, paths.ProcConfig, []byte(
		"CONFIG_SECCOMP=y\n# CONFIG_SCSI_DC395x is not set\n",
	))
	config, source, err := kernel.LoadConfig(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if source.Origin != kernel.ConfigRunningKernel || !source.Compressed ||
		source.RunningRelease != "6.12.1-gentoo" ||
		source.Path != paths.ProcConfig {
		t.Fatalf("source = %#v", source)
	}
	symbol, _ := kernel.ParseSymbol("SCSI_DC395x")
	if entry, found := config.Get(symbol); !found || entry.State != kernel.No() {
		t.Fatalf("mixed-case entry = %#v, %v", entry, found)
	}
}

func TestLoadConfigFallsBackFromProcToBootThenSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := kernel.DefaultConfigPaths(root)
	writeKernelFile(t, paths.ProcRelease, []byte("6.12.1\n"))
	boot := filepath.Join(paths.Boot, "config-6.12.1")
	writeKernelFile(t, boot, []byte("CONFIG_ALPHA=y\n"))

	_, source, err := kernel.LoadConfig(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if source.Origin != kernel.ConfigBoot || source.Path != boot {
		t.Fatalf("boot source = %#v", source)
	}

	if err := os.Remove(boot); err != nil {
		t.Fatal(err)
	}
	sourceConfig := filepath.Join(paths.KernelSource, ".config")
	writeKernelFile(t, sourceConfig, []byte("CONFIG_ALPHA=m\n"))
	_, source, err = kernel.LoadConfig(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if source.Origin != kernel.ConfigKernelSource || source.Path != sourceConfig {
		t.Fatalf("source-tree source = %#v", source)
	}
}

func TestLoadConfigExplicitPathDoesNotFallBack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := kernel.DefaultConfigPaths(root)
	paths.Explicit = filepath.Join(root, "missing")
	writeGzip(t, paths.ProcConfig, []byte("CONFIG_ALPHA=y\n"))
	config, source, err := kernel.LoadConfig(context.Background(), paths)
	if err == nil ||
		!reflect.DeepEqual(config, kernel.Config{}) ||
		!reflect.DeepEqual(source, kernel.ConfigSource{}) {
		t.Fatalf("explicit missing returned %#v, %#v, %v", config, source, err)
	}
}

func TestLoadConfigRejectsCorruptGzipAndMalformedConfigAtomically(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := kernel.DefaultConfigPaths(root)
	paths.Explicit = filepath.Join(root, "corrupt.gz")
	writeKernelFile(t, paths.Explicit, []byte{0x1f, 0x8b, 0x00})
	config, source, err := kernel.LoadConfig(context.Background(), paths)
	if err == nil ||
		!reflect.DeepEqual(config, kernel.Config{}) ||
		!reflect.DeepEqual(source, kernel.ConfigSource{}) {
		t.Fatalf("corrupt gzip returned %#v, %#v, %v", config, source, err)
	}

	writeKernelFile(t, paths.Explicit, []byte("not a kernel config\n"))
	config, source, err = kernel.LoadConfig(context.Background(), paths)
	if err == nil ||
		!reflect.DeepEqual(config, kernel.Config{}) ||
		!reflect.DeepEqual(source, kernel.ConfigSource{}) {
		t.Fatalf("malformed config returned %#v, %#v, %v", config, source, err)
	}
}

func TestLoadConfigHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	config, source, err := kernel.LoadConfig(ctx, kernel.DefaultConfigPaths(t.TempDir()))
	if !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(config, kernel.Config{}) ||
		!reflect.DeepEqual(source, kernel.ConfigSource{}) {
		t.Fatalf("cancellation returned %#v, %#v, %v", config, source, err)
	}
}

func writeGzip(t *testing.T, path string, value []byte) {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(value); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writeKernelFile(t, path, compressed.Bytes())
}

func writeKernelFile(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, value, 0o644); err != nil {
		t.Fatal(err)
	}
}
