package recommend

import (
	"fmt"
	"sort"
	"strings"

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
			Detail: packageKernelExplanation(requirement, symbol),
			Evidence: []domain.Evidence{{
				Kind: domain.SourcePackage, Source: requirement.Package.CPV(),
				Detail:     packageKernelExplanation(requirement, symbol),
				Confidence: domain.Certain,
			}},
			Provenance: []domain.Provenance{requirement.Provenance},
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

func packageKernelExplanation(
	requirement maizegentoo.PackageKernelRequirement,
	symbol kernel.Symbol,
) string {
	action := "requires"
	if requirement.Severity == maizegentoo.KernelWarning {
		action = "recommends"
	}
	state := "enabled"
	if requirement.Expectation == maizegentoo.KernelDisabled {
		state = "disabled"
	}
	pkg := requirement.Package.CPV()
	if pkg == "" {
		pkg = "installed package"
	}
	explanation := fmt.Sprintf(
		"%s explicitly %s %s to be %s",
		pkg, action, symbol.String(), state,
	)
	if len(requirement.Conditions) != 0 {
		var conditions []string
		for _, condition := range requirement.Conditions {
			value := "enabled"
			if !condition.Enabled {
				value = "disabled"
			}
			conditions = append(conditions, fmt.Sprintf("USE=%s is %s", condition.Flag, value))
		}
		explanation += " when " + strings.Join(conditions, " and ")
	}
	if requirement.Function != "" {
		explanation += " during " + requirement.Function
	}
	return explanation
}
