package app

import (
	"fmt"
	"strings"

	"github.com/airencracken/maize/internal/domain"
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

// ValidateRequiredRecommendations ensures target Kconfig did not discard or
// weaken a required Maize decision. A built-in result satisfies a requested
// module because it provides at least the requested capability.
func ValidateRequiredRecommendations(
	validation kernel.TargetValidation,
	recommendations []recommend.Recommendation,
) error {
	var unsatisfied []string
	for _, item := range recommendations {
		if item.Disposition != domain.Required {
			continue
		}
		entry, found := validation.Config.Get(item.Symbol)
		if found && stateSatisfies(entry.State, item.Desired) {
			continue
		}
		resolved := kernel.No()
		if found {
			resolved = entry.State
		}
		unsatisfied = append(unsatisfied, fmt.Sprintf(
			"%s requested %s but target resolved %s",
			item.Symbol.String(), item.Desired.ConfigValue(), resolved.ConfigValue(),
		))
	}
	if len(unsatisfied) != 0 {
		return fmt.Errorf(
			"target Kconfig rejected required decisions: %s",
			strings.Join(unsatisfied, "; "),
		)
	}
	return nil
}

func stateSatisfies(current, desired kernel.State) bool {
	return current == desired ||
		(desired.Kind == kernel.StateModule && current.Kind == kernel.StateYes)
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
