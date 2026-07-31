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

func TestPrioritizedMigrationReportCapsDefaultGroupsAndPreservesVerboseAudit(t *testing.T) {
	t.Parallel()

	symbol, _ := kernel.ParseSymbol("REMOVED")
	yes := kernel.Yes()
	changes := make([]kernel.Change, 13)
	for index := range changes {
		changes[index] = kernel.Change{Symbol: symbol, Before: &yes}
	}
	var concise bytes.Buffer
	if err := report.MigrationTextWithOptions(
		&concise, changes, report.MigrationTextOptions{},
	); err != nil {
		t.Fatal(err)
	}
	if strings.Count(concise.String(), "CONFIG_REMOVED:") != 12 ||
		!strings.Contains(concise.String(), "1 more in this category") {
		t.Fatalf("concise report:\n%s", concise.String())
	}
	var verbose bytes.Buffer
	if err := report.MigrationTextWithOptions(
		&verbose, changes, report.MigrationTextOptions{Verbose: true},
	); err != nil {
		t.Fatal(err)
	}
	if strings.Count(verbose.String(), "CONFIG_REMOVED:") != 13 ||
		strings.Contains(verbose.String(), "more in this category") {
		t.Fatalf("verbose report:\n%s", verbose.String())
	}
}

func TestPrioritizedMigrationJSONIncludesSummaryAndImpact(t *testing.T) {
	t.Parallel()

	symbol, _ := kernel.ParseSymbol("REMOVED")
	yes := kernel.Yes()
	context := report.MigrationContext{
		RunningRelease: "6.9", RunningConfig: "/proc/config.gz",
		TargetRelease: "6.10", TargetTree: "/usr/src/linux-6.10",
	}
	var output bytes.Buffer
	if err := report.MigrationJSONPrioritized(
		&output, context, []kernel.Change{{Symbol: symbol, Before: &yes}},
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"schema": "maize.migration/v3"`) ||
		!strings.Contains(output.String(), `"lost_capabilities": 1`) ||
		!strings.Contains(output.String(), `"impact": "lost-capability"`) {
		t.Fatalf("prioritized JSON:\n%s", output.String())
	}
}
