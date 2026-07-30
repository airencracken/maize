package kernel_test

import (
	"slices"
	"testing"

	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/kernel"
)

func TestCompareRealPreemptionKconfigVersions(t *testing.T) {
	t.Parallel()

	oldCatalog := parseKconfigFixture(t, "testdata/Kconfig.preempt-6.0.5")
	newCatalog := parseKconfigFixture(t, "testdata/Kconfig.preempt-7.1.5")
	oldConfig := parseConfigFixture(t, "testdata/config-5.10.76-gentoo-r1-x86_64")
	newConfig := parseConfigFixture(t, "testdata/config-6.0.5-gentoo-x86_64")

	changes := kernel.Compare(oldCatalog, newCatalog, oldConfig, newConfig)
	lazy := findChange(t, changes, "PREEMPT_LAZY")
	if !slices.Contains(lazy.Kinds, kernel.ChangeAdded) {
		t.Fatalf("PREEMPT_LAZY changes = %v", lazy.Kinds)
	}
	dynamic := findChange(t, changes, "PREEMPT_DYNAMIC")
	if !slices.Contains(dynamic.Kinds, kernel.ChangeDependencies) ||
		!slices.Contains(dynamic.Kinds, kernel.ChangeValue) {
		t.Fatalf("PREEMPT_DYNAMIC changes = %v", dynamic.Kinds)
	}
	if dynamic.Explanation.Confidence != domain.Certain ||
		len(dynamic.Explanation.Provenance) != 2 {
		t.Fatalf("migration explanation lacks provenance: %#v", dynamic.Explanation)
	}
}

func TestCompareDoesNotReportUnchangedSymbols(t *testing.T) {
	t.Parallel()

	catalog, err := kernel.NewCatalog(kernel.Definition{
		Symbol: mustSymbol(t, "EXT4_FS"), Type: kernel.TypeTristate,
	})
	if err != nil {
		t.Fatal(err)
	}
	config := parseConfigFixture(t, "testdata/config-6.0.5-gentoo-x86_64")
	if changes := kernel.Compare(catalog, catalog, config, config); len(changes) != 0 {
		t.Fatalf("unchanged inputs produced changes: %#v", changes)
	}
}

func findChange(t *testing.T, changes []kernel.Change, symbol string) kernel.Change {
	t.Helper()
	want := mustSymbol(t, symbol)
	for _, change := range changes {
		if change.Symbol == want {
			return change
		}
	}
	t.Fatalf("change for %s not found in %#v", symbol, changes)
	return kernel.Change{}
}
