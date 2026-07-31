package kernel

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxKconfigFiles    = 8192
	maxKconfigFileSize = 8 * 1024 * 1024
)

// LoadKconfigCatalog reads explanatory symbol definitions from a target
// kernel tree. Kconfig itself remains authoritative for evaluation; this
// catalog is used only for prompts, help, and source provenance.
func LoadKconfigCatalog(ctx context.Context, tree string) (Catalog, error) {
	resolved, err := validateKernelTree(tree)
	if err != nil {
		return Catalog{}, err
	}
	architecture, err := hostKernelArchitecture()
	if err != nil {
		return Catalog{}, err
	}
	result := Catalog{definitions: make(map[Symbol]Definition)}
	files := 0
	err = filepath.WalkDir(resolved, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "Documentation" ||
				entry.Name() == "tools" {
				return filepath.SkipDir
			}
			relative, relErr := filepath.Rel(resolved, path)
			if relErr != nil {
				return relErr
			}
			parts := strings.Split(relative, string(filepath.Separator))
			if len(parts) == 2 && parts[0] == "arch" && parts[1] != architecture {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 ||
			!(entry.Name() == "Kconfig" || strings.HasPrefix(entry.Name(), "Kconfig.")) {
			return nil
		}
		files++
		if files > maxKconfigFiles {
			return fmt.Errorf("kernel tree exceeds %d Kconfig files", maxKconfigFiles)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		limited := io.LimitReader(file, maxKconfigFileSize+1)
		definitions, parseErr := parseKconfigDefinitions(path, limited)
		closeErr := file.Close()
		if parseErr != nil {
			return parseErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, definition := range definitions {
			if previous, found := result.definitions[definition.Symbol]; found {
				result.definitions[definition.Symbol] = mergeDefinitions(previous, definition)
			} else {
				result.definitions[definition.Symbol] = cloneDefinition(definition)
			}
		}
		return nil
	})
	if err != nil {
		return Catalog{}, fmt.Errorf("read target Kconfig catalog: %w", err)
	}
	if files == 0 {
		return Catalog{}, fmt.Errorf("kernel tree %q contains no Kconfig files", resolved)
	}
	return result, nil
}

func mergeDefinitions(first, second Definition) Definition {
	result := cloneDefinition(first)
	if result.Type == TypeUnknown {
		result.Type = second.Type
	}
	if result.Prompt == "" && second.Prompt != "" {
		result.Prompt = second.Prompt
		result.Location = second.Location
	}
	if result.Help == "" && second.Help != "" {
		result.Help = second.Help
	}
	result.DependsOn = appendUnique(result.DependsOn, second.DependsOn...)
	result.Defaults = appendUnique(result.Defaults, second.Defaults...)
	result.Selects = appendUnique(result.Selects, second.Selects...)
	result.Implies = appendUnique(result.Implies, second.Implies...)
	return result
}

func appendUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		found := false
		for _, value := range values {
			if value == addition {
				found = true
				break
			}
		}
		if !found {
			values = append(values, addition)
		}
	}
	return values
}
