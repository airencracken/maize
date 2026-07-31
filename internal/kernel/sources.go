package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type SourceTree struct {
	Path           string
	Release        string
	RunningRelease bool
}

type SourceInventory struct {
	RunningRelease string
	Trees          []SourceTree
	Target         SourceTree
}

// DiscoverSourceTrees finds versioned Linux source trees below ROOT/usr/src
// and selects the newest kernel release. The unversioned linux symlink is not
// treated as a separate installed source.
func DiscoverSourceTrees(root, runningRelease string) (SourceInventory, error) {
	sourceRoot := filepath.Join(filepath.Clean(root), "usr", "src")
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return SourceInventory{}, fmt.Errorf("read installed kernel sources %q: %w", sourceRoot, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return SourceInventory{}, fmt.Errorf("resolve installed kernel source root %q: %w", sourceRoot, err)
	}
	result := SourceInventory{RunningRelease: strings.TrimSpace(runningRelease)}
	seen := make(map[string]bool)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "linux-") {
			continue
		}
		path := filepath.Join(sourceRoot, entry.Name())
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil || !pathWithin(resolvedRoot, resolved) || seen[resolved] {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return SourceInventory{}, fmt.Errorf("inspect installed kernel source %q: %w", path, statErr)
		}
		if !info.IsDir() || !validSourceTree(path) {
			continue
		}
		seen[resolved] = true
		release := sourceRelease(path, strings.TrimPrefix(entry.Name(), "linux-"))
		if release == "" {
			continue
		}
		result.Trees = append(result.Trees, SourceTree{
			Path: path, Release: release,
			RunningRelease: sameKernelRelease(release, result.RunningRelease),
		})
	}
	if len(result.Trees) == 0 {
		return SourceInventory{}, fmt.Errorf("no installed kernel source trees found in %q", sourceRoot)
	}
	for left := 1; left < len(result.Trees); left++ {
		for right := left; right > 0 &&
			compareKernelRelease(result.Trees[right].Release, result.Trees[right-1].Release) < 0; right-- {
			result.Trees[right], result.Trees[right-1] = result.Trees[right-1], result.Trees[right]
		}
	}
	result.Target = result.Trees[len(result.Trees)-1]
	return result, nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validSourceTree(path string) bool {
	for _, name := range []string{"Kconfig", "Makefile"} {
		info, err := os.Stat(filepath.Join(path, name))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func sourceRelease(path, fallback string) string {
	data, err := os.ReadFile(filepath.Join(path, "include", "config", "kernel.release"))
	if err == nil {
		if release := strings.TrimSpace(string(data)); release != "" && !strings.ContainsAny(release, "/\x00") {
			return release
		}
	}
	return fallback
}

func sameKernelRelease(left, right string) bool {
	return strings.TrimSpace(left) != "" && strings.TrimSpace(left) == strings.TrimSpace(right)
}

type versionToken struct {
	number bool
	value  string
}

func compareKernelRelease(left, right string) int {
	leftTokens := kernelVersionTokens(left)
	rightTokens := kernelVersionTokens(right)
	limit := len(leftTokens)
	if len(rightTokens) < limit {
		limit = len(rightTokens)
	}
	for index := 0; index < limit; index++ {
		leftToken, rightToken := leftTokens[index], rightTokens[index]
		if leftToken.number && rightToken.number {
			leftNumber, _ := strconv.ParseUint(strings.TrimLeft(leftToken.value, "0"), 10, 64)
			rightNumber, _ := strconv.ParseUint(strings.TrimLeft(rightToken.value, "0"), 10, 64)
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
			continue
		}
		if leftToken.number != rightToken.number {
			if leftToken.number {
				return 1
			}
			return -1
		}
		leftRank, rightRank := kernelSuffixRank(leftToken.value), kernelSuffixRank(rightToken.value)
		if leftRank != rightRank {
			if leftRank < rightRank {
				return -1
			}
			return 1
		}
		if leftToken.value < rightToken.value {
			return -1
		}
		if leftToken.value > rightToken.value {
			return 1
		}
	}
	if len(leftTokens) == len(rightTokens) {
		return 0
	}
	if len(leftTokens) > limit && leftTokens[limit].value == "rc" {
		return -1
	}
	if len(rightTokens) > limit && rightTokens[limit].value == "rc" {
		return 1
	}
	if len(leftTokens) < len(rightTokens) {
		return -1
	}
	return 1
}

func kernelVersionTokens(value string) []versionToken {
	var result []versionToken
	for index := 0; index < len(value); {
		r := rune(value[index])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			index++
			continue
		}
		number := unicode.IsDigit(r)
		end := index + 1
		for end < len(value) && unicode.IsDigit(rune(value[end])) == number &&
			(unicode.IsLetter(rune(value[end])) || unicode.IsDigit(rune(value[end]))) {
			end++
		}
		result = append(result, versionToken{number: number, value: value[index:end]})
		index = end
	}
	return result
}

func kernelSuffixRank(value string) int {
	switch value {
	case "rc":
		return 0
	case "gentoo":
		return 2
	default:
		return 1
	}
}
