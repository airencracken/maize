package recommend

import (
	"fmt"
	"sort"

	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/kernel"
)

// PackageKernelPolicy translates active structured ebuild/eclass requirements
// into direct symbol recommendations. Contradictory requirements are retained
// so candidate generation fails rather than hiding the conflict.
func PackageKernelPolicy(
	config kernel.Config,
	policy []maizegentoo.PackageKernelRequirement,
) ([]Recommendation, error) {
	var result []Recommendation
	for index, requirement := range policy {
		if !requirement.Active {
			continue
		}
		symbol, err := kernel.ParseSymbol(requirement.Symbol)
		if err != nil {
			return nil, fmt.Errorf("package kernel requirement %d: %w", index, err)
		}
		desired := kernel.Module()
		if requirement.Expectation == maizegentoo.KernelDisabled {
			desired = kernel.No()
		} else if entry, found := config.Get(symbol); found &&
			(entry.State.Kind == kernel.StateYes || entry.State.Kind == kernel.StateModule) {
			desired = entry.State
		}
		disposition := domain.Required
		if requirement.Severity == maizegentoo.KernelWarning {
			disposition = domain.Recommended
		}
		item := Recommendation{
			Capability: "package.kernel-policy", Disposition: disposition,
			Symbol: symbol, Desired: desired,
			Detail: "structured " + requirement.Origin + " kernel requirement",
			Evidence: []domain.Evidence{{
				Kind: domain.SourcePackage, Source: requirement.Provenance.Source,
				Detail: requirement.Provenance.Detail, Confidence: domain.Certain,
			}},
		}
		if entry, found := config.Get(symbol); found {
			current := entry.State
			item.Current = &current
		}
		item.Action = action(item.Current, desired)
		result = append(result, item)
	}
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Symbol != result[right].Symbol {
			return result[left].Symbol < result[right].Symbol
		}
		return result[left].Desired.ConfigValue() < result[right].Desired.ConfigValue()
	})
	return result, nil
}
