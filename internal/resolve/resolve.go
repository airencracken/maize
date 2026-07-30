package resolve

import (
	"errors"
	"fmt"
	"sort"

	"github.com/airencracken/maize/internal/domain"
)

var ErrConflict = errors.New("requirements conflict")

type ConflictError struct {
	Conflicts []domain.Conflict
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%v: %d capability conflicts", ErrConflict, len(e.Conflicts))
}

func (e *ConflictError) Unwrap() error {
	return ErrConflict
}

// Requirements converts validated requirements into a deterministic decision
// set. Resolution is atomic: validation errors and conflicts return no
// decisions.
func Requirements(requirements []domain.Requirement) ([]domain.Decision, error) {
	grouped := make(map[string][]domain.Requirement)
	for i, requirement := range requirements {
		if err := requirement.Validate(); err != nil {
			return nil, fmt.Errorf("requirement %d: %w", i, err)
		}
		grouped[requirement.Capability] = append(grouped[requirement.Capability], requirement)
	}

	capabilities := make([]string, 0, len(grouped))
	for capability := range grouped {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)

	conflicts := findConflicts(capabilities, grouped)
	if len(conflicts) != 0 {
		return nil, &ConflictError{Conflicts: conflicts}
	}

	decisions := make([]domain.Decision, 0, len(capabilities))
	for _, capability := range capabilities {
		requirementsForCapability := grouped[capability]
		sortRequirements(requirementsForCapability)

		evidence := make([]domain.Evidence, 0, len(requirementsForCapability))
		for _, requirement := range requirementsForCapability {
			evidence = append(evidence, requirement.Evidence)
		}
		decisions = append(decisions, domain.Decision{
			Capability:  capability,
			Disposition: strongestDisposition(requirementsForCapability),
			Evidence:    evidence,
		})
	}
	return decisions, nil
}

func findConflicts(capabilities []string, grouped map[string][]domain.Requirement) []domain.Conflict {
	var conflicts []domain.Conflict
	for _, capability := range capabilities {
		requirements := grouped[capability]
		hasRequired := false
		hasProhibited := false
		for _, requirement := range requirements {
			hasRequired = hasRequired || requirement.Disposition == domain.Required
			hasProhibited = hasProhibited || requirement.Disposition == domain.Prohibited
		}
		if !hasRequired || !hasProhibited {
			continue
		}

		sortRequirements(requirements)
		conflict := domain.Conflict{
			Capability:   capability,
			Dispositions: []domain.Disposition{domain.Required, domain.Prohibited},
			Evidence:     make([]domain.Evidence, 0, len(requirements)),
		}
		for _, requirement := range requirements {
			conflict.Evidence = append(conflict.Evidence, requirement.Evidence)
		}
		conflicts = append(conflicts, conflict)
	}
	return conflicts
}

func strongestDisposition(requirements []domain.Requirement) domain.Disposition {
	rank := map[domain.Disposition]int{
		domain.Optional:    0,
		domain.Recommended: 1,
		domain.Required:    2,
		domain.Prohibited:  3,
	}
	result := domain.Optional
	for _, requirement := range requirements {
		if rank[requirement.Disposition] > rank[result] {
			result = requirement.Disposition
		}
	}
	return result
}

func sortRequirements(requirements []domain.Requirement) {
	sort.SliceStable(requirements, func(i, j int) bool {
		left := requirements[i]
		right := requirements[j]
		if left.Disposition != right.Disposition {
			return left.Disposition < right.Disposition
		}
		if left.Evidence.Kind != right.Evidence.Kind {
			return left.Evidence.Kind < right.Evidence.Kind
		}
		if left.Evidence.Source != right.Evidence.Source {
			return left.Evidence.Source < right.Evidence.Source
		}
		return left.Evidence.Detail < right.Evidence.Detail
	})
}
