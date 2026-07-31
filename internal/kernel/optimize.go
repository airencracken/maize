package kernel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// OptimizationResult is an experimental local-module optimization proposal.
// The caller must still validate it with the target Kconfig implementation.
type OptimizationResult struct {
	Config          Config
	DisabledModules int
	ObservedModules []string
}

type OptimizationStrategy string

const (
	OptimizeBestGuess OptimizationStrategy = "best-guess"
	OptimizeMinimize  OptimizationStrategy = "minimize"
)

// ExperimentalBestGuess uses the target kernel's module-to-Kconfig knowledge
// without modifying the kernel tree. It deliberately preserves every Kconfig
// file containing a known package recommendation.
func ExperimentalOptimize(ctx context.Context, tree string, baseline Config, protected []Symbol, modules []string, strategy OptimizationStrategy) (OptimizationResult, error) {
	if strategy != OptimizeBestGuess && strategy != OptimizeMinimize {
		return OptimizationResult{}, fmt.Errorf("unknown experimental optimization strategy %q", strategy)
	}
	resolved, err := validateKernelTree(tree)
	if err != nil {
		return OptimizationResult{}, err
	}
	script := filepath.Join(resolved, "scripts", "kconfig", "streamline_config.pl")
	if info, statErr := os.Stat(script); statErr != nil || !info.Mode().IsRegular() {
		return OptimizationResult{}, fmt.Errorf("target kernel has no scripts/kconfig/streamline_config.pl")
	}
	catalog, err := LoadKconfigCatalog(ctx, resolved)
	if err != nil {
		return OptimizationResult{}, err
	}
	preserved := preservedKconfigPatterns(resolved, catalog, protected)
	if strategy == OptimizeBestGuess {
		// Retain common removable-device, network, filesystem, and audio families.
		// Minimize intentionally omits this safety buffer.
		preserved = append(preserved, bestGuessFallbackPatterns()...)
	}
	modules = normalizeModules(modules)

	directory, err := os.MkdirTemp("", "maize-optimize-")
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("create isolated optimizer directory: %w", err)
	}
	defer os.RemoveAll(directory)
	configPath := filepath.Join(directory, ".config")
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("create optimizer config: %w", err)
	}
	writeErr := baseline.Write(file)
	closeErr := file.Close()
	if writeErr != nil {
		return OptimizationResult{}, fmt.Errorf("write optimizer config: %w", writeErr)
	}
	if closeErr != nil {
		return OptimizationResult{}, fmt.Errorf("close optimizer config: %w", closeErr)
	}
	lsmod := filepath.Join(directory, "lsmod")
	var moduleData strings.Builder
	moduleData.WriteString("Module Size Used by\n")
	for _, module := range modules {
		fmt.Fprintf(&moduleData, "%s 0 0\n", module)
	}
	if err := os.WriteFile(lsmod, []byte(moduleData.String()), 0o600); err != nil {
		return OptimizationResult{}, fmt.Errorf("write module evidence: %w", err)
	}

	command := exec.CommandContext(ctx, "perl", script, "--localmodconfig", resolved, "Kconfig")
	command.Dir = directory
	command.Env = append(os.Environ(), "LSMOD="+lsmod, "LMC_KEEP="+strings.Join(preserved, ":"))
	output := &limitedBuffer{remaining: validationOutputLimit}
	diagnostics := &limitedBuffer{remaining: validationOutputLimit}
	command.Stdout, command.Stderr = output, diagnostics
	if err := command.Run(); err != nil {
		return OptimizationResult{}, fmt.Errorf("target localmodconfig: %w: %s", err, strings.TrimSpace(diagnostics.String()))
	}
	optimized, err := ParseConfig("experimental-best-guess", strings.NewReader(output.String()))
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("parse target localmodconfig output: %w", err)
	}
	disabled := 0
	for _, before := range baseline.Entries() {
		if before.State.Kind != StateModule {
			continue
		}
		after, found := optimized.Get(before.Symbol)
		if !found || after.State.Kind == StateNo {
			disabled++
		}
	}
	return OptimizationResult{Config: optimized, DisabledModules: disabled, ObservedModules: modules}, nil
}

func bestGuessFallbackPatterns() []string {
	return []string{"drivers/(hid|input|net|usb)/", "fs/", "net/", "sound/"}
}

func normalizeModules(modules []string) []string {
	seen := make(map[string]bool)
	for _, module := range modules {
		module = strings.TrimSpace(module)
		if module != "" && !strings.ContainsAny(module, " \t\r\n/") {
			seen[module] = true
		}
	}
	result := make([]string, 0, len(seen))
	for module := range seen {
		result = append(result, module)
	}
	sort.Strings(result)
	return result
}

func preservedKconfigPatterns(tree string, catalog Catalog, protected []Symbol) []string {
	seen := make(map[string]bool)
	for _, symbol := range protected {
		definition, found := catalog.Get(symbol)
		if !found {
			continue
		}
		relative, err := filepath.Rel(tree, definition.Location.Path)
		if err == nil && relative != "." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			seen[regexp.QuoteMeta(filepath.ToSlash(relative))] = true
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
