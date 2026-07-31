package kernel

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const validationOutputLimit = 1024 * 1024

type ValidationChange struct {
	Symbol    Symbol
	Requested State
	Resolved  State
}

type TargetValidation struct {
	Config  Config
	Changes []ValidationChange
}

type commandRunner interface {
	Run(context.Context, string, string, []string, []string) error
}

type execRunner struct{}

// ValidateTarget runs the target kernel's olddefconfig implementation against
// an isolated copy of candidate. It never writes into the source tree.
func ValidateTarget(ctx context.Context, tree string, candidate Config) (TargetValidation, error) {
	return validateTarget(ctx, tree, candidate, execRunner{})
}

func validateTarget(
	ctx context.Context,
	tree string,
	candidate Config,
	runner commandRunner,
) (TargetValidation, error) {
	if err := ctx.Err(); err != nil {
		return TargetValidation{}, err
	}
	resolvedTree, err := validateKernelTree(tree)
	if err != nil {
		return TargetValidation{}, err
	}
	output, err := os.MkdirTemp("", "maize-kconfig-")
	if err != nil {
		return TargetValidation{}, fmt.Errorf("create isolated Kconfig output: %w", err)
	}
	defer os.RemoveAll(output)

	configPath := filepath.Join(output, ".config")
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return TargetValidation{}, fmt.Errorf("create candidate config: %w", err)
	}
	writeErr := candidate.Write(file)
	closeErr := file.Close()
	if writeErr != nil {
		return TargetValidation{}, fmt.Errorf("write candidate config: %w", writeErr)
	}
	if closeErr != nil {
		return TargetValidation{}, fmt.Errorf("close candidate config: %w", closeErr)
	}

	executable := filepath.Join(resolvedTree, "scripts", "kconfig", "conf")
	args := []string{"--olddefconfig", filepath.Join(resolvedTree, "Kconfig")}
	architecture, err := hostKernelArchitecture()
	if err != nil {
		return TargetValidation{}, err
	}
	environment := []string{
		"KCONFIG_CONFIG=" + configPath,
		"srctree=" + resolvedTree,
		"ARCH=" + architecture,
		"SRCARCH=" + architecture,
		"CC=" + environmentDefault("CC", "cc"),
		"LD=" + environmentDefault("LD", "ld"),
	}
	if info, statErr := os.Stat(executable); statErr != nil || !info.Mode().IsRegular() ||
		info.Mode().Perm()&0o111 == 0 {
		executable = "make"
		args = []string{
			"-s", "-C", resolvedTree, "O=" + output,
			"KCONFIG_CONFIG=" + configPath, "olddefconfig",
		}
		environment = nil
	}
	if err := runner.Run(ctx, resolvedTree, executable, args, environment); err != nil {
		return TargetValidation{}, fmt.Errorf("target olddefconfig: %w", err)
	}
	validatedFile, err := os.Open(configPath)
	if err != nil {
		return TargetValidation{}, fmt.Errorf("open validated config: %w", err)
	}
	validated, parseErr := ParseConfig(configPath, validatedFile)
	closeErr = validatedFile.Close()
	if parseErr != nil {
		return TargetValidation{}, parseErr
	}
	if closeErr != nil {
		return TargetValidation{}, fmt.Errorf("close validated config: %w", closeErr)
	}
	return TargetValidation{
		Config: validated, Changes: validationChanges(candidate, validated),
	}, nil
}

func hostKernelArchitecture() (string, error) {
	switch runtime.GOARCH {
	case "386", "amd64":
		return "x86", nil
	case "arm", "arm64", "mips":
		return runtime.GOARCH, nil
	case "riscv64":
		return "riscv", nil
	case "loong64":
		return "loongarch", nil
	case "ppc64", "ppc64le":
		return "powerpc", nil
	case "s390x":
		return "s390", nil
	default:
		return "", fmt.Errorf("cannot infer target kernel architecture from %s", runtime.GOARCH)
	}
}

func environmentDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func validateKernelTree(tree string) (string, error) {
	if strings.TrimSpace(tree) == "" {
		return "", fmt.Errorf("target kernel tree is required")
	}
	resolved, err := filepath.EvalSymlinks(tree)
	if err != nil {
		return "", fmt.Errorf("resolve target kernel tree %q: %w", tree, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve target kernel tree %q: %w", tree, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect target kernel tree %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("target kernel tree %q is not a directory", resolved)
	}
	for _, name := range []string{"Kconfig", "Makefile"} {
		info, statErr := os.Stat(filepath.Join(resolved, name))
		if statErr != nil {
			return "", fmt.Errorf("target kernel tree has no %s: %w", name, statErr)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("target kernel tree %s is not a regular file", name)
		}
	}
	return resolved, nil
}

func validationChanges(candidate, validated Config) []ValidationChange {
	var result []ValidationChange
	for _, requested := range candidate.Entries() {
		resolved := No()
		if entry, found := validated.Get(requested.Symbol); found {
			resolved = entry.State
		}
		if resolved != requested.State {
			result = append(result, ValidationChange{
				Symbol: requested.Symbol, Requested: requested.State, Resolved: resolved,
			})
		}
	}
	return result
}

func (execRunner) Run(
	ctx context.Context,
	directory string,
	executable string,
	args []string,
	environment []string,
) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), environment...)
	output := &limitedBuffer{remaining: validationOutputLimit}
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(output.String())
		if detail != "" {
			return fmt.Errorf("%w: %s", err, detail)
		}
		return err
	}
	return nil
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int
	truncated bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	if len(value) > b.remaining {
		value = value[:b.remaining]
		b.truncated = true
	}
	b.remaining -= len(value)
	_, _ = b.buffer.Write(value)
	return original, nil
}

func (b *limitedBuffer) String() string {
	if b.truncated {
		return b.buffer.String() + "\n[output truncated]"
	}
	return b.buffer.String()
}
