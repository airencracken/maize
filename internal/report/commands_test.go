package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/report"
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
