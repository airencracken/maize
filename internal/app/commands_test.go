package app_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
)

func TestCandidateConfigAppliesRecommendationsWithoutMutatingInput(t *testing.T) {
	t.Parallel()

	current, err := kernel.ParseConfig("current", strings.NewReader(
		"# CONFIG_ALPHA is not set\nCONFIG_KEEP=y\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	alpha, _ := kernel.ParseSymbol("ALPHA")
	inspection := app.Inspection{
		CurrentConfig: current,
		Recommendations: []recommend.Recommendation{{
			Symbol: alpha, Desired: kernel.Yes(),
		}},
	}
	candidate, err := app.CandidateConfig(inspection)
	if err != nil {
		t.Fatal(err)
	}
	if entry, _ := candidate.Get(alpha); entry.State != kernel.Yes() {
		t.Fatalf("candidate alpha = %#v", entry)
	}
	if entry, _ := current.Get(alpha); entry.State != kernel.No() {
		t.Fatalf("input mutated: %#v", entry)
	}
}

func TestCandidateConfigRejectsConflictingSymbolAtomically(t *testing.T) {
	t.Parallel()

	alpha, _ := kernel.ParseSymbol("ALPHA")
	candidate, err := app.CandidateConfig(app.Inspection{
		Recommendations: []recommend.Recommendation{
			{Symbol: alpha, Desired: kernel.Yes()},
			{Symbol: alpha, Desired: kernel.No()},
		},
	})
	if err == nil || !reflect.DeepEqual(candidate, kernel.Config{}) {
		t.Fatalf("conflict returned %#v, %v", candidate, err)
	}
}

func TestValidateRequiredRecommendationsAcceptsExactAndStrongerStates(t *testing.T) {
	t.Parallel()

	module, _ := kernel.ParseSymbol("MODULE")
	exact, _ := kernel.ParseSymbol("EXACT")
	config, err := kernel.ParseConfig("validated", strings.NewReader(
		"CONFIG_MODULE=y\nCONFIG_EXACT=y\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	err = app.ValidateRequiredRecommendations(
		kernel.TargetValidation{Config: config},
		[]recommend.Recommendation{
			{Symbol: module, Desired: kernel.Module(), Disposition: domain.Required},
			{Symbol: exact, Desired: kernel.Yes(), Disposition: domain.Required},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateRequiredRecommendationsReportsEveryRejectedDecision(t *testing.T) {
	t.Parallel()

	alpha, _ := kernel.ParseSymbol("ALPHA")
	missing, _ := kernel.ParseSymbol("MISSING")
	optional, _ := kernel.ParseSymbol("OPTIONAL")
	config, err := kernel.ParseConfig("validated", strings.NewReader(
		"# CONFIG_ALPHA is not set\n# CONFIG_OPTIONAL is not set\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	err = app.ValidateRequiredRecommendations(
		kernel.TargetValidation{Config: config},
		[]recommend.Recommendation{
			{Symbol: alpha, Desired: kernel.Yes(), Disposition: domain.Required},
			{Symbol: missing, Desired: kernel.Module(), Disposition: domain.Required},
			{Symbol: optional, Desired: kernel.Yes(), Disposition: domain.Recommended},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_ALPHA requested y") ||
		!strings.Contains(err.Error(), "CONFIG_MISSING requested m") ||
		strings.Contains(err.Error(), "CONFIG_OPTIONAL") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnsatisfiedFiltersKeepAndOptionalPolicy(t *testing.T) {
	t.Parallel()

	items := []recommend.Recommendation{
		{Capability: "required", Disposition: domain.Required, Action: recommend.ActionEnable},
		{Capability: "recommended", Disposition: domain.Recommended, Action: recommend.ActionChange},
		{Capability: "satisfied", Disposition: domain.Required, Action: recommend.ActionKeep},
	}
	inspection := app.Inspection{Recommendations: items}
	if got := app.Unsatisfied(inspection, true); len(got) != 1 ||
		got[0].Capability != "required" {
		t.Fatalf("required unsatisfied = %#v", got)
	}
	if got := app.Unsatisfied(inspection, false); len(got) != 2 {
		t.Fatalf("all unsatisfied = %#v", got)
	}
}
