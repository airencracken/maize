package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/report"
	"github.com/airencracken/maize/internal/terminal"
)

func TestHardwareJSONUsesVersionedOwnedContract(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := report.HardwareJSON(&output, hardware.Inventory{Schema: 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"schema": "maize.hardware/v1"`) ||
		!strings.Contains(output.String(), `"devices": []`) {
		t.Fatalf("hardware JSON:\n%s", output.String())
	}
}

func TestMigrationReportsAreDeterministic(t *testing.T) {
	t.Parallel()

	symbol, _ := kernel.ParseSymbol("TEST")
	before, after := kernel.No(), kernel.Yes()
	changes := []kernel.Change{{
		Symbol: symbol, Kinds: []kernel.ChangeKind{kernel.ChangeValue},
		Before: &before, After: &after,
	}}
	var first bytes.Buffer
	if err := report.MigrationJSON(&first, changes); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := report.MigrationJSON(&second, changes); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() ||
		!strings.Contains(first.String(), `"schema": "maize.migration/v1"`) {
		t.Fatalf("migration JSON:\n%s\n%s", first.String(), second.String())
	}
}

func TestContextualMigrationReportsIdentifyComparedKernels(t *testing.T) {
	t.Parallel()

	context := report.MigrationContext{
		RunningRelease: "6.9.12-gentoo", RunningConfig: "/proc/config.gz",
		TargetRelease: "6.10.2-gentoo", TargetTree: "/usr/src/linux-6.10.2-gentoo",
	}
	var text bytes.Buffer
	if err := report.MigrationTextWithContext(
		&text, context, nil, terminal.Style{},
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "Running kernel: 6.9.12-gentoo") ||
		!strings.Contains(text.String(), "Target kernel: 6.10.2-gentoo") {
		t.Fatalf("migration text:\n%s", text.String())
	}
	var document bytes.Buffer
	if err := report.MigrationJSONWithContext(&document, context, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document.String(), `"schema": "maize.migration/v2"`) ||
		!strings.Contains(document.String(), `"target_release": "6.10.2-gentoo"`) {
		t.Fatalf("migration JSON:\n%s", document.String())
	}
}
