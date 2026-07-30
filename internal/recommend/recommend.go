// Package recommend translates resolved machine capabilities into explainable
// kernel symbol recommendations.
package recommend

import (
	"fmt"
	"sort"

	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/kernel"
)

type Binding struct {
	Capability string
	Symbol     kernel.Symbol
	State      kernel.State
	Detail     string
}

type Action string

const (
	ActionKeep    Action = "keep"
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
	ActionChange  Action = "change"
)

type Recommendation struct {
	Capability  string
	Disposition domain.Disposition
	Symbol      kernel.Symbol
	Current     *kernel.State
	Desired     kernel.State
	Action      Action
	Detail      string
	Evidence    []domain.Evidence
}

// Kernel converts resolved capability decisions into deterministic symbol
// recommendations. Invalid or incomplete bindings fail atomically.
func Kernel(
	config kernel.Config,
	decisions []domain.Decision,
	bindings []Binding,
) ([]Recommendation, error) {
	byCapability := make(map[string][]Binding)
	seenBindings := make(map[string]bool)
	for index, binding := range bindings {
		if binding.Capability == "" || binding.Symbol == "" || binding.Detail == "" {
			return nil, fmt.Errorf("binding %d is incomplete", index)
		}
		if _, err := kernel.ParseState(binding.State.ConfigValue()); err != nil {
			return nil, fmt.Errorf("binding %d state: %w", index, err)
		}
		key := binding.Capability + "\x00" + string(binding.Symbol)
		if seenBindings[key] {
			return nil, fmt.Errorf(
				"binding %d duplicates %s for capability %q",
				index, binding.Symbol.String(), binding.Capability,
			)
		}
		seenBindings[key] = true
		byCapability[binding.Capability] = append(byCapability[binding.Capability], binding)
	}

	ordered := append([]domain.Decision(nil), decisions...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].Capability < ordered[right].Capability
	})
	var result []Recommendation
	for _, decision := range ordered {
		selected := append([]Binding(nil), byCapability[decision.Capability]...)
		if len(selected) == 0 {
			return nil, fmt.Errorf("capability %q has no kernel binding", decision.Capability)
		}
		sort.SliceStable(selected, func(left, right int) bool {
			return selected[left].Symbol < selected[right].Symbol
		})
		for _, binding := range selected {
			desired := binding.State
			if decision.Disposition == domain.Prohibited {
				desired = kernel.No()
			}
			recommendation := Recommendation{
				Capability: decision.Capability, Disposition: decision.Disposition,
				Symbol: binding.Symbol, Desired: desired, Detail: binding.Detail,
				Evidence: append([]domain.Evidence(nil), decision.Evidence...),
			}
			if entry, found := config.Get(binding.Symbol); found {
				current := entry.State
				recommendation.Current = &current
			}
			recommendation.Action = action(recommendation.Current, desired)
			result = append(result, recommendation)
		}
	}
	return result, nil
}

func action(current *kernel.State, desired kernel.State) Action {
	if current != nil && satisfies(*current, desired) {
		return ActionKeep
	}
	if desired.Kind == kernel.StateNo {
		return ActionDisable
	}
	if current == nil || current.Kind == kernel.StateNo {
		return ActionEnable
	}
	return ActionChange
}

func satisfies(current, desired kernel.State) bool {
	if current == desired {
		return true
	}
	return desired.Kind == kernel.StateModule && current.Kind == kernel.StateYes
}
