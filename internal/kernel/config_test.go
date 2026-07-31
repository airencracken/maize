package kernel_test

import (
	"os"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/kernel"
)

func TestParseRealGentooConfigs(t *testing.T) {
	t.Parallel()

	oldConfig := parseConfigFixture(t, "testdata/config-5.10.76-gentoo-r1-x86_64")
	newConfig := parseConfigFixture(t, "testdata/config-6.0.5-gentoo-x86_64")

	preempt := mustSymbol(t, "PREEMPT")
	if entry, found := oldConfig.Get(preempt); !found || entry.State != kernel.No() {
		t.Fatalf("old PREEMPT = %#v, %v", entry, found)
	}
	dynamic := mustSymbol(t, "PREEMPT_DYNAMIC")
	if _, found := oldConfig.Get(dynamic); found {
		t.Fatal("PREEMPT_DYNAMIC unexpectedly present in 5.10 fixture")
	}
	if entry, found := newConfig.Get(dynamic); !found || entry.State != kernel.Yes() {
		t.Fatalf("new PREEMPT_DYNAMIC = %#v, %v", entry, found)
	}
	compiler := mustSymbol(t, "CC_VERSION_TEXT")
	entry, found := newConfig.Get(compiler)
	if !found || entry.State.Kind != kernel.StateString ||
		!strings.Contains(entry.State.Value, "Gentoo 14.2.1") {
		t.Fatalf("compiler string lost: %#v", entry)
	}
}

func TestParseConfigRejectsDuplicateMalformedAndOversizedInput(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"duplicate":  "CONFIG_TEST=y\n# CONFIG_TEST is not set\n",
		"malformed":  "CONFIG_TEST\n",
		"hostile":    "CONFIG_GOOD=y\n../../escape\n",
		"bad value":  "CONFIG_TEST=maybe\n",
		"bad symbol": "CONFIG_bad.symbol=y\n",
	}
	for name, input := range tests {
		input := input
		t.Run(name, func(t *testing.T) {
			if _, err := kernel.ParseConfig(name, strings.NewReader(input)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}

	oversized := "CONFIG_TEST=\"" + strings.Repeat("x", 4*1024*1024+1) + "\"\n"
	if _, err := kernel.ParseConfig("oversized", strings.NewReader(oversized)); err == nil {
		t.Fatal("oversized line accepted")
	}
}

func TestParseConfigAcceptsMixedCaseKernelSymbol(t *testing.T) {
	t.Parallel()

	config, err := kernel.ParseConfig(
		"real.config",
		strings.NewReader("# CONFIG_SCSI_DC395x is not set\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	symbol := mustSymbol(t, "CONFIG_SCSI_DC395x")
	entry, found := config.Get(symbol)
	if !found || entry.State != kernel.No() {
		t.Fatalf("entry = %#v, found %v", entry, found)
	}
}

func TestConfigEntriesAreDeterministicAndOwned(t *testing.T) {
	t.Parallel()

	config, err := kernel.ParseConfig("fixture", strings.NewReader(
		"CONFIG_ZED=m\nCONFIG_ALPHA=0x2A\nCONFIG_COUNT=42\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	entries := config.Entries()
	if len(entries) != 3 || entries[0].Symbol.String() != "CONFIG_ALPHA" {
		t.Fatalf("entries not sorted: %#v", entries)
	}
	entries[0].State.Value = "mutated"
	again := config.Entries()
	if again[0].State.Value != "0x2a" {
		t.Fatalf("returned entries alias internal state: %#v", again[0])
	}
}

func TestConfigWithStatesAndWriteAreDeterministic(t *testing.T) {
	t.Parallel()

	config, err := kernel.ParseConfig("fixture", strings.NewReader(
		"CONFIG_ZED=m\n# CONFIG_ALPHA is not set\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	alpha := mustSymbol(t, "ALPHA")
	candidate, err := config.WithStates(map[kernel.Symbol]kernel.State{
		alpha: kernel.Yes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := candidate.Write(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "CONFIG_ALPHA=y\nCONFIG_ZED=m\n" {
		t.Fatalf("canonical config:\n%s", output.String())
	}
	if entry, _ := config.Get(alpha); entry.State != kernel.No() {
		t.Fatalf("input config mutated: %#v", entry)
	}
}

func FuzzParseConfigNeverPanics(f *testing.F) {
	f.Add("CONFIG_EXT4_FS=y\n")
	f.Add("# CONFIG_SECURITY_LANDLOCK is not set\n")
	f.Add("CONFIG_LOCALVERSION=\"-gentoo\"\n")
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = kernel.ParseConfig("fuzz", strings.NewReader(input))
	})
}

func parseConfigFixture(t *testing.T, path string) kernel.Config {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, err := kernel.ParseConfig(path, file)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func mustSymbol(t *testing.T, value string) kernel.Symbol {
	t.Helper()
	symbol, err := kernel.ParseSymbol(value)
	if err != nil {
		t.Fatal(err)
	}
	return symbol
}
