package gentooling

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

// Rule maps reviewed package state to a stable Maize capability. Atom is a
// Gentoo package atom. UseFlag, when present, requires that the
// recorded installed package was built with that flag enabled.
type Rule struct {
	Atom        string
	UseFlag     string
	Capability  string
	Disposition domain.Disposition
	Confidence  domain.Confidence
	Detail      string
}

func (r Rule) Validate() error {
	if _, err := gentooling.ParseAtom(r.Atom); err != nil {
		return fmt.Errorf("invalid package atom %q: %w", r.Atom, err)
	}
	if r.UseFlag != "" && !validUseFlag(r.UseFlag) {
		return fmt.Errorf("invalid USE flag %q", r.UseFlag)
	}
	probe := domain.Requirement{
		Capability:  r.Capability,
		Disposition: r.Disposition,
		Evidence: domain.Evidence{
			Kind:       domain.SourcePackage,
			Source:     r.Atom,
			Detail:     r.Detail,
			Confidence: r.Confidence,
		},
	}
	if err := probe.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}
	return nil
}

// ReadInstalled obtains strict package evidence without CONTENTS payloads.
// Any interrupted, corrupt, unreadable, invalid, or concurrently changing VDB
// evidence is returned with gentooling.ErrIncompleteEvidence.
func ReadInstalled(
	ctx context.Context,
	paths gentooling.SystemPaths,
) (gentooling.InstalledInventory, error) {
	return gentooling.ReadInstalled(ctx, paths, gentooling.InstalledOptions{
		Integrity:       gentooling.RequireComplete,
		IncludeContents: false,
	})
}

// Requirements translates installed package state through reviewed rules.
// Invalid rules fail atomically: no partial requirement set is returned.
func Requirements(
	inventory gentooling.InstalledInventory,
	rules []Rule,
) ([]domain.Requirement, error) {
	for index, rule := range rules {
		if err := rule.Validate(); err != nil {
			return nil, fmt.Errorf("rule %d: %w", index, err)
		}
	}

	packages := append([]gentooling.InstalledPackage(nil), inventory.Packages...)
	slices.SortFunc(packages, func(left, right gentooling.InstalledPackage) int {
		return strings.Compare(packageIdentity(left), packageIdentity(right))
	})

	orderedRules := append([]Rule(nil), rules...)
	slices.SortFunc(orderedRules, compareRule)

	var requirements []domain.Requirement
	for _, installed := range packages {
		for _, rule := range orderedRules {
			atom, err := gentooling.ParseAtom(rule.Atom)
			if err != nil {
				return nil, fmt.Errorf("validated atom %q: %w", rule.Atom, err)
			}
			matches, err := atom.Matches(installed.ID, installedUseState(installed))
			if err != nil {
				return nil, fmt.Errorf("match atom %q against %s: %w", rule.Atom, installed.ID.CPV(), err)
			}
			if !matches || !useEnabled(installed, rule.UseFlag) {
				continue
			}
			source := installed.ID.CPV()
			kind := domain.SourcePackage
			if rule.UseFlag != "" {
				source += "[" + rule.UseFlag + "]"
				kind = domain.SourceUseFlag
			}
			requirements = append(requirements, domain.Requirement{
				Capability:  rule.Capability,
				Disposition: rule.Disposition,
				Evidence: domain.Evidence{
					Kind:       kind,
					Source:     source,
					Detail:     rule.Detail,
					Confidence: rule.Confidence,
				},
			})
		}
	}
	return requirements, nil
}

func useEnabled(installed gentooling.InstalledPackage, required string) bool {
	if required == "" {
		return true
	}
	return slices.Contains(installed.EnabledUse, required)
}

func packageIdentity(pkg gentooling.InstalledPackage) string {
	return pkg.ID.CPV() + "\x00" + pkg.ID.Slot + "\x00" + pkg.ID.Repository
}

func compareRule(left, right Rule) int {
	for _, comparison := range []int{
		strings.Compare(left.Atom, right.Atom),
		strings.Compare(left.UseFlag, right.UseFlag),
		strings.Compare(left.Capability, right.Capability),
		strings.Compare(string(left.Disposition), string(right.Disposition)),
		strings.Compare(string(left.Confidence), string(right.Confidence)),
		strings.Compare(left.Detail, right.Detail),
	} {
		if comparison != 0 {
			return comparison
		}
	}
	return 0
}

func installedUseState(installed gentooling.InstalledPackage) gentooling.UseState {
	state := gentooling.UseState{
		Enabled:  make(map[string]bool, len(installed.EnabledUse)),
		Declared: make(map[string]bool, len(installed.DeclaredUse)),
	}
	for _, name := range installed.EnabledUse {
		state.Enabled[name] = true
	}
	for _, declaration := range installed.DeclaredUse {
		state.Declared[declaration.Name] = true
	}
	return state
}

func validUseFlag(value string) bool {
	return validIdentifier(value) && value != "." && value != ".."
}

func validIdentifier(value string) bool {
	if value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || r == '+' || r == '_' || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return value != ""
}
