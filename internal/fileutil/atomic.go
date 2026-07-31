package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteAtomic writes a new regular file beside path, syncs it, and renames it
// into place. A failed write leaves the destination unchanged.
func WriteAtomic(path string, mode os.FileMode, write func(io.Writer) error) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	directory := filepath.Dir(filepath.Clean(path))
	file, err := os.CreateTemp(directory, ".maize-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(temporary)
	}
	if err := file.Chmod(mode); err != nil {
		cleanup()
		return err
	}
	if err := write(file); err != nil {
		cleanup()
		return err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
