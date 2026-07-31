package kernel_test

import (
	"testing"

	"github.com/airencracken/maize/internal/kernel"
)

func TestClassifyMigrationChangeCoversOperatorConsequences(t *testing.T) {
	t.Parallel()

	yes, module, no := kernel.Yes(), kernel.Module(), kernel.No()
	stringA, _ := kernel.ParseState(`"old"`)
	stringB, _ := kernel.ParseState(`"new"`)
	tests := []struct {
		name   string
		change kernel.Change
		want   kernel.MigrationImpact
	}{
		{"lost", kernel.Change{Before: &yes, After: &no}, kernel.ImpactLostCapability},
		{"removed", kernel.Change{Before: &module}, kernel.ImpactLostCapability},
		{"enabled", kernel.Change{Before: &no, After: &yes}, kernel.ImpactNewlyEnabled},
		{"new enabled", kernel.Change{After: &module}, kernel.ImpactNewlyEnabled},
		{"weakened", kernel.Change{Before: &yes, After: &module}, kernel.ImpactBuiltinToModule},
		{"strengthened", kernel.Change{Before: &module, After: &yes}, kernel.ImpactModuleToBuiltin},
		{"value", kernel.Change{Before: &stringA, After: &stringB}, kernel.ImpactValueChanged},
		{"removed value", kernel.Change{Before: &stringA}, kernel.ImpactValueChanged},
		{"definition", kernel.Change{Before: &yes, After: &yes, Kinds: []kernel.ChangeKind{kernel.ChangeDependencies}}, kernel.ImpactDefinitionChange},
		{"new disabled", kernel.Change{After: &no, Kinds: []kernel.ChangeKind{kernel.ChangeValue}}, kernel.ImpactInactiveChurn},
		{"removed disabled", kernel.Change{Before: &no, Kinds: []kernel.ChangeKind{kernel.ChangeValue}}, kernel.ImpactInactiveChurn},
		{"inactive definition", kernel.Change{Before: &no, Kinds: []kernel.ChangeKind{kernel.ChangeDependencies}}, kernel.ImpactInactiveChurn},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := kernel.ClassifyMigrationChange(test.change); got != test.want {
				t.Fatalf("impact = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSummarizeMigrationAccountsForEveryChange(t *testing.T) {
	t.Parallel()

	yes, no := kernel.Yes(), kernel.No()
	changes := []kernel.Change{
		{Before: &yes, After: &no},
		{Before: &no, After: &yes},
		{After: &no, Kinds: []kernel.ChangeKind{kernel.ChangeValue}},
	}
	summary := kernel.SummarizeMigration(changes)
	accounted := summary.LostCapabilities + summary.NewlyEnabled +
		summary.BuiltinToModule + summary.ModuleToBuiltin + summary.ValueChanged +
		summary.DefinitionChanged + summary.InactiveChurnHidden
	if summary.Total != len(changes) || accounted != summary.Total {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestConfigRelevantChangesDropsInactiveDefinitionOnlyChurn(t *testing.T) {
	t.Parallel()

	yes, no := kernel.Yes(), kernel.No()
	changes := []kernel.Change{
		{Before: &no, After: &no, Kinds: []kernel.ChangeKind{kernel.ChangeDependencies}},
		{Before: &yes, After: &yes, Kinds: []kernel.ChangeKind{kernel.ChangeDependencies}},
		{Before: &no, After: &no, Kinds: []kernel.ChangeKind{kernel.ChangeValue}},
	}
	filtered := kernel.ConfigRelevantChanges(changes)
	if len(filtered) != 2 || filtered[0].Before == nil || filtered[0].Before.Kind != kernel.StateYes {
		t.Fatalf("filtered = %#v", filtered)
	}
}
