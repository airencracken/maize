package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
)

func TestHelpListsEveryCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code %d, want 0", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	for _, command := range []string{"inspect", "generate", "migrate", "check", "impact", "observe"} {
		if !strings.Contains(stdout.String(), "maize "+command) {
			t.Errorf("help does not list %q", command)
		}
	}
}

func TestEveryDeclaredCommandExists(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"inspect":  {"Usage:", "kernel", "Gentoo", "--config", "--format", "Exit status:"},
		"generate": {"Usage:", "--kernel-tree", "--output", "--experimental-best-guess", "--experimental-minimize", "olddefconfig", "Exit status:"},
		"migrate":  {"Usage:", "newest installed kernel source", "--old-kconfig", "--new-kconfig", "--old-config", "--new-config", "Exit status:"},
		"check":    {"Usage:", "hardware and package requirements", "--config", "known kernel changes required", "unresolved package policy"},
		"impact":   {"Usage:", "Required:", "--config PATH", "read-only", "Exit status:"},
		"observe":  {"Usage:", "--output", "sysfs hardware", "loaded modules", "atomically", "Exit status:"},
	}
	for command, expected := range tests {
		command, expected := command, expected
		t.Run(command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			args := []string{command, "--help"}
			exitCode := run(args, &stdout, &stderr)
			if exitCode != 0 || stdout.Len() != 0 {
				t.Fatalf("%s route: exit %d, stderr %q", command, exitCode, stderr.String())
			}
			for _, text := range expected {
				if !strings.Contains(stderr.String(), text) {
					t.Errorf("help missing %q:\n%s", text, stderr.String())
				}
			}
		})
	}
}

func TestShortHelpMatchesLongHelpForEverySubcommand(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"inspect", "generate", "migrate", "check", "impact", "observe"} {
		var long, short bytes.Buffer
		if code := run([]string{command, "--help"}, io.Discard, &long); code != 0 {
			t.Fatalf("%s --help exit %d", command, code)
		}
		if code := run([]string{command, "-h"}, io.Discard, &short); code != 0 {
			t.Fatalf("%s -h exit %d", command, code)
		}
		if long.String() != short.String() {
			t.Errorf("%s short and long help differ", command)
		}
	}
}

func TestGenerateHelpIsIndependentOfOptionOrderAndHostState(t *testing.T) {
	t.Parallel()
	var standard, reordered bytes.Buffer
	if code := run([]string{"generate", "--help"}, io.Discard, &standard); code != 0 {
		t.Fatalf("standard help exit %d", code)
	}
	if code := run([]string{"generate", "--kernel-tree", "/missing", "--help", "--output", "/forbidden"}, io.Discard, &reordered); code != 0 {
		t.Fatalf("reordered help exit %d: %s", code, reordered.String())
	}
	if standard.String() != reordered.String() {
		t.Fatal("generate help depends on option order")
	}
}

func TestInspectRejectsInvalidArgumentsBeforeReadingHost(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"inspect", "--format", "yaml"},
		{"inspect", "--color", "sometimes"},
		{"inspect", "--snapshot-consistency", "eventual"},
		{"inspect", "--repository", "missing-separator"},
		{"inspect", "--repository", "bad/name=/tmp"},
		{"inspect", "--repository", "same=/one", "--repository", "same=/two"},
		{"inspect", "positional"},
	}
	for _, args := range tests {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if exitCode := run(args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("%v exit code %d, want 2: %s", args, exitCode, stderr.String())
		}
	}
}

func TestGenerateRejectsDuplicateExplicitPathsBeforeReadingHost(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"generate", "--output", "one", "--output", "two", "--kernel-tree", "/usr/src/linux"},
		{"generate", "--output", "candidate.config", "--kernel-tree", "one", "--kernel-tree", "two"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v exit %d, stdout %q, stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestGenerateDiscoversNewestKernelTreeAndAcceptsDirectoryOutput(t *testing.T) {
	t.Parallel()
	root := commandFixture(t)
	config := filepath.Join(root, "kernel.config")
	commandWrite(t, config, "CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\nCONFIG_SECCOMP_FILTER=y\n")
	commandKernelSourceFixture(t, root, "6.12.1-gentoo", "olddefconfig:\n\t@test -f \"$(KCONFIG_CONFIG)\"\n")
	newest := commandKernelSourceFixture(t, root, "7.1.5-gentoo", "olddefconfig:\n\t@test -f \"$(KCONFIG_CONFIG)\"\n")
	outputDirectory := filepath.Join(root, "output")
	if err := os.Mkdir(outputDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate", "--root", root, "--config", config, "--output", outputDirectory}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), newest) || !strings.Contains(stdout.String(), "7.1.5-gentoo") {
		t.Fatalf("source selection not reported: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(outputDirectory, "maize.config")); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationOutputDefaultsAndDirectoryResolution(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	got, err := generationOutputPath(directory)
	if err != nil || got != filepath.Join(directory, "maize.config") {
		t.Fatalf("directory output = %q, %v", got, err)
	}
	got, err = generationOutputPath("maize.config")
	if err != nil || got != "maize.config" {
		t.Fatalf("default output = %q, %v", got, err)
	}
}

func TestExperimentalBestGuessSwitchRejectsValuesAndDuplicates(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--experimental-best-guess=true"},
		{"--experimental-best-guess", "--experimental-best-guess"},
	} {
		found, remaining, err := takeSwitch(args, "--experimental-best-guess")
		if err == nil || found || remaining != nil {
			t.Fatalf("takeSwitch(%v) = %v, %v, %v", args, found, remaining, err)
		}
	}
}

func TestInspectCommandRunsEndToEndAgainstAlternateRoot(t *testing.T) {
	t.Parallel()

	root := commandFixture(t)
	config := filepath.Join(root, "kernel.config")
	if err := os.WriteFile(config, []byte(
		"CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\n"+
			"# CONFIG_SECCOMP_FILTER is not set\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"inspect", "--root", root, "--config", config, "--format", "json"},
		&stdout, &stderr,
	)
	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr.String())
	}
	for _, expected := range []string{
		`"schema": "maize.inspect/v2"`,
		`"symbol": "CONFIG_SECCOMP_FILTER"`,
		`"action": "enable"`,
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestGenerateCheckImpactAndObserveRunAgainstAlternateRoot(t *testing.T) {
	t.Parallel()

	root := commandFixture(t)
	config := filepath.Join(root, "kernel.config")
	commandWrite(t, config,
		"CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\n"+
			"# CONFIG_SECCOMP_FILTER is not set\n")
	output := filepath.Join(root, "generated.config")
	kernelTree := commandKernelTreeFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"generate", "--root", root, "--config", config, "--output", output,
		"--kernel-tree", kernelTree,
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("generate exit %d, stderr %q", code, stderr.String())
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "CONFIG_SECCOMP_FILTER=y") {
		t.Fatalf("generated config:\n%s", generated)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"check", "--root", root, "--config", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("generated check exit %d, stdout %q, stderr %q",
			code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"check", "--root", root, "--config", config}, &stdout, &stderr)
	if code != 3 || !strings.Contains(stdout.String(), "CONFIG_SECCOMP_FILTER") {
		t.Fatalf("check exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"impact", "--root", root, "--config", config}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "CONFIG_SECCOMP_FILTER") {
		t.Fatalf("impact exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	inventory := filepath.Join(root, "hardware.json")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"observe", "--root", root, "--output", inventory}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("observe exit %d, stderr %q", code, stderr.String())
	}
	data, err := os.ReadFile(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schema": "maize.hardware/v1"`) {
		t.Fatalf("hardware inventory:\n%s", data)
	}
}

func TestGenerateValidationFailureDoesNotCreateOutput(t *testing.T) {
	t.Parallel()

	root := commandFixture(t)
	config := filepath.Join(root, "kernel.config")
	commandWrite(t, config,
		"CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\n"+
			"CONFIG_SECCOMP_FILTER=y\n")
	output := filepath.Join(root, "must-not-exist.config")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"generate", "--root", root, "--config", config, "--output", output,
		"--kernel-tree", filepath.Join(root, "missing-kernel"),
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "generate validation:") {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output must not be created, stat error = %v", err)
	}
}

func TestGenerateRefusesIncompleteDynamicPackageKernelPolicyAtomically(t *testing.T) {
	t.Parallel()

	root := commandFixture(t)
	repository := filepath.Join(root, "var", "db", "repos", "gentoo")
	commandWrite(t,
		filepath.Join(repository, "metadata", "md5-cache", "app-containers", "docker-28.3.2"),
		"EAPI=8\nSLOT=0\nIUSE=seccomp apparmor\n_eclasses_=linux-info digest\n",
	)
	commandWrite(t,
		filepath.Join(repository, "app-containers", "docker", "docker-28.3.2.ebuild"),
		"CONFIG_CHECK=\"${RUNTIME_SYMBOL}\"\npkg_setup() { check_extra_config; }\n",
	)
	config := filepath.Join(root, "kernel.config")
	commandWrite(t, config,
		"CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\n"+
			"CONFIG_SECCOMP_FILTER=y\n")
	output := filepath.Join(root, "must-not-exist.config")
	kernelTree := commandKernelTreeFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"check", "--root", root, "--config", config,
		"--repository", "gentoo=" + repository, "--color", "never",
	}, &stdout, &stderr)
	if code != 4 ||
		!strings.Contains(stdout.String(), "Package kernel policy: incomplete") ||
		!strings.Contains(stdout.String(), "Use --verbose") ||
		strings.Contains(stdout.String(), "${RUNTIME_SYMBOL}") {
		t.Fatalf("check exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"generate", "--root", root, "--config", config, "--output", output,
		"--repository", "gentoo=" + repository,
		"--kernel-tree", kernelTree,
	}, &stdout, &stderr)
	if code != 1 ||
		!strings.Contains(stderr.String(), "dynamic package kernel policies across") ||
		!strings.Contains(stderr.String(), "app-containers/docker-28.3.2") ||
		!strings.Contains(stderr.String(), "shell") {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output must not be created, stat error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"generate", "--root", root, "--config", config, "--output", output,
		"--repository", "gentoo=" + repository, "--kernel-tree", kernelTree, "--verbose",
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "${RUNTIME_SYMBOL}") ||
		!strings.Contains(stderr.String(), "docker-28.3.2.ebuild") {
		t.Fatalf("verbose generate exit %d, stderr %q", code, stderr.String())
	}
}

func TestMigrateCommandReportsSemanticChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	oldKconfig := filepath.Join(root, "old.Kconfig")
	newKconfig := filepath.Join(root, "new.Kconfig")
	oldConfig := filepath.Join(root, "old.config")
	newConfig := filepath.Join(root, "new.config")
	commandWrite(t, oldKconfig, "config TEST\n\tbool \"Old\"\n")
	commandWrite(t, newKconfig, "config TEST\n\tbool \"New\"\n")
	commandWrite(t, oldConfig, "# CONFIG_TEST is not set\n")
	commandWrite(t, newConfig, "CONFIG_TEST=y\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"migrate", "--old-kconfig", oldKconfig, "--new-kconfig", newKconfig,
		"--old-config", oldConfig, "--new-config", newConfig, "--format", "json",
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 ||
		!strings.Contains(stdout.String(), `"schema": "maize.migration/v1"`) ||
		!strings.Contains(stdout.String(), `"CONFIG_TEST"`) {
		t.Fatalf("migrate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestMigrateDefaultsFromRunningKernelToLatestInstalledSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	commandWrite(t, filepath.Join(root, "proc", "sys", "kernel", "osrelease"), "6.9.12-gentoo\n")
	commandWrite(t, filepath.Join(root, "proc", "config.gz"),
		"CONFIG_KEEP=y\n# CONFIG_NEW is not set\n")
	commandKernelSourceFixture(t, root, "6.9.12-gentoo",
		"olddefconfig:\n\t@test -f \"$(KCONFIG_CONFIG)\"\n")
	target := commandKernelSourceFixture(t, root, "6.10.2-gentoo",
		"olddefconfig:\n\t@printf 'CONFIG_KEEP=y\\nCONFIG_NEW=y\\n' > \"$(KCONFIG_CONFIG)\"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"migrate", "--root", root, "--format", "json", "--color", "never",
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 ||
		!strings.Contains(stdout.String(), `"schema": "maize.migration/v4"`) ||
		!strings.Contains(stdout.String(), `"consumer_evidence": "unavailable:`) ||
		!strings.Contains(stdout.String(), `"inactive_churn"`) ||
		!strings.Contains(stdout.String(), `"running_release": "6.9.12-gentoo"`) ||
		!strings.Contains(stdout.String(), `"target_release": "6.10.2-gentoo"`) ||
		!strings.Contains(stdout.String(), `"target_tree": "`+target+`"`) ||
		!strings.Contains(stdout.String(), `"symbol": "CONFIG_NEW"`) {
		t.Fatalf("migrate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestMigrationRecommendationReasonsRetainPurposeAndConsumerEvidence(t *testing.T) {
	t.Parallel()

	symbol, _ := kernel.ParseSymbol("DM_CRYPT")
	reasons := migrationRecommendationReasons([]recommend.Recommendation{{
		Symbol: symbol, Detail: "provide device-mapper encryption",
		Evidence: []domain.Evidence{{
			Source: "sys-fs/cryptsetup-2.8.6-r2", Detail: "cryptsetup is installed",
		}},
	}})
	if len(reasons[symbol]) != 2 ||
		!strings.Contains(strings.Join(reasons[symbol], "\n"), "cryptsetup is installed") ||
		!strings.Contains(strings.Join(reasons[symbol], "\n"), "device-mapper encryption") {
		t.Fatalf("reasons = %#v", reasons)
	}
}

func TestMigrationConsumerRelevanceKeepsValuesAndKnownDefinitions(t *testing.T) {
	t.Parallel()

	valueSymbol, _ := kernel.ParseSymbol("VALUE")
	knownSymbol, _ := kernel.ParseSymbol("KNOWN")
	unknownSymbol, _ := kernel.ParseSymbol("UNKNOWN")
	changes := []kernel.Change{
		{Symbol: valueSymbol, Kinds: []kernel.ChangeKind{kernel.ChangeValue}},
		{Symbol: knownSymbol, Kinds: []kernel.ChangeKind{kernel.ChangeDependencies}},
		{Symbol: unknownSymbol, Kinds: []kernel.ChangeKind{kernel.ChangeDependencies}},
	}
	filtered := migrationConsumerRelevantChanges(
		changes, map[kernel.Symbol][]string{knownSymbol: {"package needs it"}},
	)
	if len(filtered) != 2 || filtered[0].Symbol != valueSymbol ||
		filtered[1].Symbol != knownSymbol {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestMigrateRejectsPartialExplicitArtifactsBeforeReadingHost(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"migrate", "--old-config", "old.config", "--new-config", "new.config",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "all four explicit") {
		t.Fatalf("migrate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func commandFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys"), 0o755); err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(root, "var", "db", "repos", "gentoo")
	profile := filepath.Join(repository, "profiles", "default", "linux", "amd64")
	commandWrite(t, filepath.Join(repository, "profiles", "base", "packages"), "*sys-apps/baselayout\n")
	commandWrite(t, filepath.Join(profile, "parent"), "../../../base\n")
	commandWrite(t, filepath.Join(profile, "make.defaults"), "ARCH=\"amd64\"\n")
	active := filepath.Join(root, "etc", "portage", "make.profile")
	if err := os.MkdirAll(filepath.Dir(active), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(profile, active); err != nil {
		t.Fatal(err)
	}
	commandWrite(t, filepath.Join(root, "etc", "portage", "make.conf"), "USE=\"\"\n")
	commandWrite(t, filepath.Join(root, "usr", "share", "portage", "config", "make.globals"), "USE=\"\"\n")
	commandWrite(t, filepath.Join(root, "var", "lib", "portage", "world"), "app-containers/docker\n")
	vdb := filepath.Join(root, "var", "db", "pkg", "app-containers", "docker-28.3.2")
	for name, value := range map[string]string{
		"CONTENTS": "", "EAPI": "8", "SLOT": "0", "repository": "gentoo",
		"IUSE": "seccomp apparmor", "USE": "seccomp",
	} {
		commandWrite(t, filepath.Join(vdb, name), value)
	}
	return root
}

func commandWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commandKernelTreeFixture(t *testing.T) string {
	t.Helper()

	tree := t.TempDir()
	commandWrite(t, filepath.Join(tree, "Kconfig"), "mainmenu \"Fixture\"\n")
	commandWrite(t, filepath.Join(tree, "Makefile"),
		"olddefconfig:\n\t@test -f \"$(KCONFIG_CONFIG)\"\n")
	return tree
}

func commandKernelSourceFixture(t *testing.T, root, release, makefile string) string {
	t.Helper()

	tree := filepath.Join(root, "usr", "src", "linux-"+release)
	commandWrite(t, filepath.Join(tree, "Kconfig"), "mainmenu \"Fixture\"\n")
	commandWrite(t, filepath.Join(tree, "Makefile"), makefile)
	return tree
}

func TestUnknownCommandIsRejected(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"../../hostile"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit code %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
