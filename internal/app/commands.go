package app

import (
	"fmt"

	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
)

// CandidateConfig applies every resolved recommendation to an owned copy of
// the inspected configuration. Target Kconfig validation is a later gate.
func CandidateConfig(inspection Inspection) (kernel.Config, error) {
	states := make(map[kernel.Symbol]kernel.State, len(inspection.Recommendations))
	for _, item := range inspection.Recommendations {
		if previous, exists := states[item.Symbol]; exists && previous != item.Desired {
			return kernel.Config{}, fmt.Errorf(
				"conflicting desired states for %s", item.Symbol.String(),
			)
		}
		states[item.Symbol] = item.Desired
	}
	return inspection.CurrentConfig.WithStates(states)
}

func Unsatisfied(inspection Inspection, requiredOnly bool) []recommend.Recommendation {
	var result []recommend.Recommendation
	for _, item := range inspection.Recommendations {
		if item.Action == recommend.ActionKeep {
			continue
		}
		if requiredOnly && item.Disposition != "required" {
			continue
		}
		result = append(result, item)
	}
	return result
}
