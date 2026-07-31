package kernel

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxConfigSize = 64 * 1024 * 1024

type ConfigOrigin string

const (
	ConfigExplicit      ConfigOrigin = "explicit"
	ConfigRunningKernel ConfigOrigin = "running-kernel"
	ConfigBoot          ConfigOrigin = "boot"
	ConfigKernelSource  ConfigOrigin = "kernel-source"
)

type ConfigPaths struct {
	Explicit     string
	ProcConfig   string
	ProcRelease  string
	Boot         string
	KernelSource string
}

func DefaultConfigPaths(root string) ConfigPaths {
	clean := filepath.Clean(root)
	return ConfigPaths{
		ProcConfig:   filepath.Join(clean, "proc", "config.gz"),
		ProcRelease:  filepath.Join(clean, "proc", "sys", "kernel", "osrelease"),
		Boot:         filepath.Join(clean, "boot"),
		KernelSource: filepath.Join(clean, "usr", "src", "linux"),
	}
}

type ConfigSource struct {
	Path           string
	Origin         ConfigOrigin
	RunningRelease string
	Compressed     bool
}

// LoadConfig selects and parses an existing kernel configuration. Automatic
// discovery prefers the running kernel, then /boot, then the selected source
// tree. Inputs are size-bounded and gzip is detected by its file signature.
func LoadConfig(
	ctx context.Context,
	paths ConfigPaths,
) (Config, ConfigSource, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, ConfigSource{}, err
	}
	if paths.Explicit != "" {
		config, compressed, err := loadConfigFile(ctx, paths.Explicit)
		if err != nil {
			return Config{}, ConfigSource{}, err
		}
		return config, ConfigSource{
			Path: paths.Explicit, Origin: ConfigExplicit, Compressed: compressed,
		}, nil
	}

	release, err := readSmallText(paths.ProcRelease, 4096)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, ConfigSource{}, fmt.Errorf("read running kernel release: %w", err)
	}
	candidates := []struct {
		path   string
		origin ConfigOrigin
	}{
		{paths.ProcConfig, ConfigRunningKernel},
	}
	if release != "" && paths.Boot != "" {
		candidates = append(candidates, struct {
			path   string
			origin ConfigOrigin
		}{filepath.Join(paths.Boot, "config-"+release), ConfigBoot})
	}
	if paths.KernelSource != "" {
		candidates = append(candidates, struct {
			path   string
			origin ConfigOrigin
		}{filepath.Join(paths.KernelSource, ".config"), ConfigKernelSource})
	}
	for _, candidate := range candidates {
		if candidate.path == "" {
			continue
		}
		config, compressed, loadErr := loadConfigFile(ctx, candidate.path)
		if errors.Is(loadErr, fs.ErrNotExist) {
			continue
		}
		if loadErr != nil {
			return Config{}, ConfigSource{}, loadErr
		}
		return config, ConfigSource{
			Path: candidate.path, Origin: candidate.origin,
			RunningRelease: release, Compressed: compressed,
		}, nil
	}
	return Config{}, ConfigSource{}, errors.New("no kernel configuration source found")
}

func loadConfigFile(ctx context.Context, path string) (Config, bool, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, false, fmt.Errorf("open kernel configuration %q: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return Config{}, false, fmt.Errorf("read kernel configuration %q: %w", path, err)
	}
	if len(data) > maxConfigSize {
		return Config{}, false, fmt.Errorf("kernel configuration %q exceeds %d bytes", path, maxConfigSize)
	}
	if err := ctx.Err(); err != nil {
		return Config{}, false, err
	}
	compressed := len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
	if compressed {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return Config{}, false, fmt.Errorf("decompress kernel configuration %q: %w", path, err)
		}
		decompressed, readErr := io.ReadAll(io.LimitReader(reader, maxConfigSize+1))
		closeErr := reader.Close()
		if readErr != nil {
			return Config{}, false, fmt.Errorf("decompress kernel configuration %q: %w", path, readErr)
		}
		if closeErr != nil {
			return Config{}, false, fmt.Errorf("close compressed kernel configuration %q: %w", path, closeErr)
		}
		if len(decompressed) > maxConfigSize {
			return Config{}, false, fmt.Errorf("decompressed kernel configuration %q exceeds %d bytes", path, maxConfigSize)
		}
		data = decompressed
	}
	if err := ctx.Err(); err != nil {
		return Config{}, false, err
	}
	config, err := ParseConfig(path, bytes.NewReader(data))
	if err != nil {
		return Config{}, false, err
	}
	return config, compressed, nil
}

func readSmallText(path string, limit int64) (string, error) {
	if path == "" {
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit {
		return "", fmt.Errorf("%q exceeds %d bytes", path, limit)
	}
	return strings.TrimSpace(string(data)), nil
}
