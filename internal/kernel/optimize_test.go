package kernel

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExperimentalBestGuessUsesObservedModulesAndPreservesPolicyKconfig(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeOptimizationFixture(t, filepath.Join(tree, "Kconfig"),
		"config KEEP\n\ttristate \"keep\"\nsource \"drivers/Kconfig\"\n")
	writeOptimizationFixture(t, filepath.Join(tree, "drivers", "Kconfig"),
		"config DROP\n\ttristate \"drop\"\n")
	writeOptimizationFixture(t, filepath.Join(tree, "Makefile"), "all:\n\t@true\n")
	script := `use strict;
open(my $modules, '<', $ENV{'LSMOD'}) or die "lsmod";
my $data = do { local $/; <$modules> };
die "missing observed module" unless $data =~ /^iwlwifi /m;
die "missing policy preservation" unless $ENV{'LMC_KEEP'} =~ /Kconfig/;
print "CONFIG_KEEP=m\n# CONFIG_DROP is not set\n";
`
	writeOptimizationFixture(t, filepath.Join(tree, "scripts", "kconfig", "streamline_config.pl"), script)
	baseline, err := ParseConfig("baseline", strings.NewReader("CONFIG_KEEP=m\nCONFIG_DROP=m\n"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadKconfigCatalog(context.Background(), tree)
	if err != nil {
		t.Fatal(err)
	}
	if patterns := preservedKconfigPatterns(tree, catalog, []Symbol{"KEEP"}); len(patterns) == 0 {
		t.Fatal("protected symbol did not produce a Kconfig preservation pattern")
	}
	result, err := ExperimentalOptimize(context.Background(), tree, baseline, []Symbol{"KEEP"}, []string{"iwlwifi", "iwlwifi", "../hostile"}, OptimizeMinimize)
	if err != nil {
		t.Fatal(err)
	}
	if result.DisabledModules != 1 || !reflect.DeepEqual(result.ObservedModules, []string{"iwlwifi"}) {
		t.Fatalf("result = %#v", result)
	}
	entry, found := result.Config.Get("DROP")
	if !found || entry.State.Kind != StateNo {
		t.Fatalf("drop = %#v, %v", entry, found)
	}
}

func TestExperimentalBestGuessFailsAtomicallyForMissingMachineryAndCancellation(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeOptimizationFixture(t, filepath.Join(tree, "Kconfig"), "config TEST\n\tbool \"test\"\n")
	writeOptimizationFixture(t, filepath.Join(tree, "Makefile"), "all:\n\t@true\n")
	config, err := ParseConfig("baseline", strings.NewReader("CONFIG_TEST=y\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExperimentalOptimize(context.Background(), tree, config, nil, nil, OptimizeMinimize); err == nil || !strings.Contains(err.Error(), "streamline_config.pl") {
		t.Fatalf("missing machinery error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExperimentalOptimize(ctx, tree, config, nil, nil, OptimizeMinimize); err == nil {
		t.Fatal("cancelled optimization succeeded")
	}
	if _, err := ExperimentalOptimize(context.Background(), tree, config, nil, nil, "hostile"); err == nil {
		t.Fatal("unknown strategy succeeded")
	}
}

func TestNormalizeModulesRejectsAdversarialNamesAndSorts(t *testing.T) {
	t.Parallel()
	got := normalizeModules([]string{" zfs ", "bad/name", "two words", "zfs", "ext4", ""})
	want := []string{"ext4", "zfs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("modules = %v, want %v", got, want)
	}
}

func TestBestGuessHasFallbackFamiliesThatMinimizeOmits(t *testing.T) {
	t.Parallel()
	if len(bestGuessFallbackPatterns()) != 4 {
		t.Fatal("best-guess fallback families changed unexpectedly")
	}
	if OptimizeBestGuess == OptimizeMinimize {
		t.Fatal("optimization strategies alias")
	}
}

func writeOptimizationFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
