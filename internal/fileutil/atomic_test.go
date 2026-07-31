package fileutil_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/airencracken/maize/internal/fileutil"
)

func TestWriteAtomicReplacesDestinationOnlyAfterSuccessfulWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := fileutil.WriteAtomic(path, 0o644, func(writer io.Writer) error {
		if _, writeErr := writer.Write([]byte("partial")); writeErr != nil {
			return writeErr
		}
		return errors.New("injected failure")
	})
	if err == nil {
		t.Fatal("injected failure ignored")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("failed write changed destination to %q", data)
	}

	if err := fileutil.WriteAtomic(path, 0o644, func(writer io.Writer) error {
		_, writeErr := writer.Write([]byte("new"))
		return writeErr
	}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("successful write = %q", data)
	}
}

func TestWriteAtomicRejectsMissingPath(t *testing.T) {
	t.Parallel()

	if err := fileutil.WriteAtomic("", 0o644, func(io.Writer) error {
		return nil
	}); err == nil {
		t.Fatal("empty path accepted")
	}
}
