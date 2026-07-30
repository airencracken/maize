package recommend_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
)

func TestKernelProducesDeterministicActionsAndPreservesEvidence(t *testing.T) {
	t.Parallel()

	config, err := kernel.ParseConfig("old.config", strings.NewReader(
		"CONFIG_ALPHA=y\n# CONFIG_BETA is not set\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	evidence := domain.Evidence{
		Kind: domain.SourcePackage, Source: "app-misc/example-1",
		Detail: "example is installed", Confidence: domain.High,
	}
	decisions := []domain.Decision{
		{Capability: "second", Disposition: domain.Required, Evidence: []domain.Evidence{evidence}},
		{Capability: "first", Disposition: domain.Recommended, Evidence: []domain.Evidence{evidence}},
	}
	alpha, _ := kernel.ParseSymbol("ALPHA")
	beta, _ := kernel.ParseSymbol("BETA")
	bindings := []recommend.Binding{
		{Capability: "second", Symbol: beta, State: kernel.Yes(), Detail: "enable beta"},
		{Capability: "first", Symbol: alpha, State: kernel.Module(), Detail: "provide alpha"},
	}

	got, err := recommend.Kernel(config, decisions, bindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0].Capability != "first" || got[0].Action != recommend.ActionKeep ||
		got[1].Capability != "second" || got[1].Action != recommend.ActionEnable ||
		!reflect.DeepEqual(got[1].Evidence, []domain.Evidence{evidence}) {
		t.Fatalf("recommendations = %#v", got)
	}

	reversed, err := recommend.Kernel(config,
		[]domain.Decision{decisions[1], decisions[0]},
		[]recommend.Binding{bindings[1], bindings[0]},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, reversed) {
		t.Fatalf("input order changed result:\nfirst  %#v\nsecond %#v", got, reversed)
	}
}

func TestKernelProhibitionDisablesBoundSymbol(t *testing.T) {
	t.Parallel()

	config, err := kernel.ParseConfig("old.config", strings.NewReader("CONFIG_ALPHA=y\n"))
	if err != nil {
		t.Fatal(err)
	}
	alpha, _ := kernel.ParseSymbol("ALPHA")
	got, err := recommend.Kernel(config, []domain.Decision{{
		Capability: "feature", Disposition: domain.Prohibited,
	}}, []recommend.Binding{{
		Capability: "feature", Symbol: alpha, State: kernel.Yes(), Detail: "feature switch",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Desired != kernel.No() ||
		got[0].Action != recommend.ActionDisable {
		t.Fatalf("recommendations = %#v", got)
	}
}

func TestKernelRejectsMissingAndInvalidBindingsAtomically(t *testing.T) {
	t.Parallel()

	decisions := []domain.Decision{{Capability: "known"}, {Capability: "missing"}}
	symbol, _ := kernel.ParseSymbol("KNOWN")
	got, err := recommend.Kernel(kernel.Config{}, decisions, []recommend.Binding{{
		Capability: "known", Symbol: symbol, State: kernel.Yes(), Detail: "known binding",
	}})
	if err == nil || got != nil {
		t.Fatalf("missing binding returned %#v, %v", got, err)
	}

	got, err = recommend.Kernel(kernel.Config{}, nil, []recommend.Binding{{}})
	if err == nil || got != nil {
		t.Fatalf("invalid binding returned %#v, %v", got, err)
	}

	duplicate := recommend.Binding{
		Capability: "known", Symbol: symbol, State: kernel.Yes(), Detail: "known binding",
	}
	got, err = recommend.Kernel(kernel.Config{}, nil, []recommend.Binding{duplicate, duplicate})
	if err == nil || got != nil {
		t.Fatalf("duplicate binding returned %#v, %v", got, err)
	}
}

func TestBuiltInCatalogIsInternallyConsistent(t *testing.T) {
	t.Parallel()

	for index, rule := range recommend.PackageRules() {
		if err := rule.Validate(); err != nil {
			t.Fatalf("rule %d: %v", index, err)
		}
	}
	if _, err := recommend.Kernel(kernel.Config{}, nil, recommend.KernelBindings()); err != nil {
		t.Fatalf("bindings: %v", err)
	}
}
