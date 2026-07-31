package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type validationRunner func(context.Context, string, string, []string, []string) error

func (run validationRunner) Run(
	ctx context.Context,
	directory string,
	executable string,
	args []string,
	environment []string,
) error {
	return run(ctx, directory, executable, args, environment)
}

func TestValidateTargetUsesIsolatedOutputAndReportsResolution(t *testing.T) {
	t.Parallel()

	tree := kernelTreeFixture(t)
	candidate, err := ParseConfig("candidate", strings.NewReader(
		"CONFIG_AVAILABLE=y\nCONFIG_REMOVED=y\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	result, err := validateTarget(
		context.Background(), tree, candidate,
		validationRunner(func(
			_ context.Context,
			directory string,
			executable string,
			args []string,
			environment []string,
		) error {
			if directory != tree || executable != "make" || environment != nil ||
				len(args) != 6 || args[0] != "-s" ||
				args[1] != "-C" || args[2] != tree || args[5] != "olddefconfig" {
				t.Fatalf("runner directory %q, executable %q, args %#v, env %#v",
					directory, executable, args, environment)
			}
			output := strings.TrimPrefix(args[3], "O=")
			configPath := strings.TrimPrefix(args[4], "KCONFIG_CONFIG=")
			if output == tree || filepath.Dir(configPath) != output {
				t.Fatalf("validation was not isolated: output %q, config %q", output, configPath)
			}
			return os.WriteFile(configPath, []byte(
				"CONFIG_AVAILABLE=m\n# CONFIG_NEW is not set\n",
			), 0o600)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changes) != 2 ||
		result.Changes[0].Symbol.String() != "CONFIG_AVAILABLE" ||
		result.Changes[0].Resolved != Module() ||
		result.Changes[1].Symbol.String() != "CONFIG_REMOVED" ||
		result.Changes[1].Resolved != No() {
		t.Fatalf("changes = %#v", result.Changes)
	}
	if _, err := os.Stat(filepath.Join(tree, ".config")); !os.IsNotExist(err) {
		t.Fatalf("source tree was modified: %v", err)
	}
}

func TestValidateTargetPrefersTargetConfWithoutWritingSourceTree(t *testing.T) {
	t.Parallel()

	tree := kernelTreeFixture(t)
	conf := filepath.Join(tree, "scripts", "kconfig", "conf")
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	candidate, err := ParseConfig("candidate", strings.NewReader("CONFIG_TEST=y\n"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := validateTarget(
		context.Background(), tree, candidate,
		validationRunner(func(
			_ context.Context,
			directory string,
			executable string,
			args []string,
			environment []string,
		) error {
			if directory != tree || executable != conf ||
				!reflect.DeepEqual(args, []string{"--olddefconfig", filepath.Join(tree, "Kconfig")}) ||
				len(environment) != 6 ||
				!strings.HasPrefix(environment[0], "KCONFIG_CONFIG=") ||
				environment[1] != "srctree="+tree {
				t.Fatalf("directory %q, executable %q, args %#v, env %#v",
					directory, executable, args, environment)
			}
			configPath := strings.TrimPrefix(environment[0], "KCONFIG_CONFIG=")
			return os.WriteFile(configPath, []byte("CONFIG_TEST=y\n"), 0o600)
		}),
	)
	if err != nil || len(result.Changes) != 0 {
		t.Fatalf("result %#v, error %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(tree, ".config")); !os.IsNotExist(err) {
		t.Fatalf("source tree was modified: %v", err)
	}
}

func TestValidateTargetRejectsInvalidInputsAtomically(t *testing.T) {
	t.Parallel()

	candidate, err := ParseConfig("candidate", strings.NewReader("CONFIG_TEST=y\n"))
	if err != nil {
		t.Fatal(err)
	}
	for name, tree := range map[string]string{
		"empty": "", "missing": filepath.Join(t.TempDir(), "missing"),
		"not directory": fixtureFile(t, "not-tree"),
		"incomplete":    t.TempDir(),
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			result, validateErr := validateTarget(
				context.Background(), tree, candidate,
				validationRunner(func(context.Context, string, string, []string, []string) error {
					called = true
					return nil
				}),
			)
			if validateErr == nil || called || !reflect.DeepEqual(result, TargetValidation{}) {
				t.Fatalf("result %#v, error %v, runner called %v", result, validateErr, called)
			}
		})
	}
}

func TestValidateTargetPreservesRunnerFailureAndCancellationAtomically(t *testing.T) {
	t.Parallel()

	candidate, err := ParseConfig("candidate", strings.NewReader("CONFIG_TEST=y\n"))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("olddefconfig failed")
	result, err := validateTarget(
		context.Background(), kernelTreeFixture(t), candidate,
		validationRunner(func(context.Context, string, string, []string, []string) error {
			return sentinel
		}),
	)
	if !errors.Is(err, sentinel) || !reflect.DeepEqual(result, TargetValidation{}) {
		t.Fatalf("result %#v, error %v", result, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = validateTarget(
		ctx, kernelTreeFixture(t), candidate,
		validationRunner(func(context.Context, string, string, []string, []string) error {
			t.Fatal("runner called for canceled context")
			return nil
		}),
	)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, TargetValidation{}) {
		t.Fatalf("canceled result %#v, error %v", result, err)
	}
}

func TestLimitedBufferBoundsHostileCommandOutput(t *testing.T) {
	t.Parallel()

	buffer := &limitedBuffer{remaining: 8}
	input := []byte("0123456789abcdef")
	written, err := buffer.Write(input)
	if err != nil || written != len(input) ||
		buffer.String() != "01234567\n[output truncated]" {
		t.Fatalf("write = %d, %v; output %q", written, err, buffer.String())
	}
}

func kernelTreeFixture(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "Kconfig"), []byte("mainmenu \"Fixture\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "Makefile"), []byte("olddefconfig:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return tree
}

func fixtureFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
