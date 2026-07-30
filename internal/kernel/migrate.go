package kernel

import (
	"reflect"
	"sort"

	"github.com/airencracken/maize/internal/domain"
)

type ChangeKind string

const (
	ChangeAdded             ChangeKind = "added"
	ChangeRemoved           ChangeKind = "removed"
	ChangeValue             ChangeKind = "value"
	ChangeType              ChangeKind = "type"
	ChangePrompt            ChangeKind = "prompt"
	ChangeDependencies      ChangeKind = "dependencies"
	ChangeDefaults          ChangeKind = "defaults"
	ChangeReverseDependency ChangeKind = "reverse-dependencies"
)

type Change struct {
	Symbol      Symbol
	Kinds       []ChangeKind
	Before      *State
	After       *State
	Explanation domain.Explanation
}

func Compare(oldCatalog, newCatalog Catalog, oldConfig, newConfig Config) []Change {
	symbols := make(map[Symbol]bool)
	for symbol := range oldCatalog.definitions {
		symbols[symbol] = true
	}
	for symbol := range newCatalog.definitions {
		symbols[symbol] = true
	}
	for symbol := range oldConfig.entries {
		symbols[symbol] = true
	}
	for symbol := range newConfig.entries {
		symbols[symbol] = true
	}
	ordered := make([]Symbol, 0, len(symbols))
	for symbol := range symbols {
		ordered = append(ordered, symbol)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	var changes []Change
	for _, symbol := range ordered {
		oldDefinition, inOldCatalog := oldCatalog.Get(symbol)
		newDefinition, inNewCatalog := newCatalog.Get(symbol)
		oldEntry, inOldConfig := oldConfig.Get(symbol)
		newEntry, inNewConfig := newConfig.Get(symbol)
		change := Change{Symbol: symbol}
		if inOldConfig {
			value := oldEntry.State
			change.Before = &value
		}
		if inNewConfig {
			value := newEntry.State
			change.After = &value
		}

		switch {
		case inOldCatalog && !inNewCatalog:
			change.Kinds = append(change.Kinds, ChangeRemoved)
		case !inOldCatalog && inNewCatalog:
			change.Kinds = append(change.Kinds, ChangeAdded)
		case inOldCatalog && inNewCatalog:
			if oldDefinition.Type != newDefinition.Type {
				change.Kinds = append(change.Kinds, ChangeType)
			}
			if oldDefinition.Prompt != newDefinition.Prompt {
				change.Kinds = append(change.Kinds, ChangePrompt)
			}
			if !reflect.DeepEqual(oldDefinition.DependsOn, newDefinition.DependsOn) {
				change.Kinds = append(change.Kinds, ChangeDependencies)
			}
			if !reflect.DeepEqual(oldDefinition.Defaults, newDefinition.Defaults) {
				change.Kinds = append(change.Kinds, ChangeDefaults)
			}
			if !reflect.DeepEqual(oldDefinition.Selects, newDefinition.Selects) ||
				!reflect.DeepEqual(oldDefinition.Implies, newDefinition.Implies) {
				change.Kinds = append(change.Kinds, ChangeReverseDependency)
			}
		}
		if inOldConfig != inNewConfig ||
			inOldConfig && inNewConfig && oldEntry.State != newEntry.State {
			change.Kinds = append(change.Kinds, ChangeValue)
		}
		if len(change.Kinds) == 0 {
			continue
		}
		change.Explanation = explainChange(change, oldDefinition, newDefinition)
		changes = append(changes, change)
	}
	return changes
}

func explainChange(change Change, oldDefinition, newDefinition Definition) domain.Explanation {
	summary := change.Symbol.String() + " changed"
	confidence := domain.Certain
	if len(change.Kinds) == 1 && change.Kinds[0] == ChangeAdded {
		summary = change.Symbol.String() + " was added"
	} else if len(change.Kinds) == 1 && change.Kinds[0] == ChangeRemoved {
		summary = change.Symbol.String() + " was removed"
	}
	provenance := make([]domain.Provenance, 0, 2)
	for _, definition := range []Definition{oldDefinition, newDefinition} {
		if definition.Location.Path == "" {
			continue
		}
		provenance = append(provenance, domain.Provenance{
			Kind: domain.SourceKernel, Source: definition.Location.Path,
			Detail: "Kconfig definition at line " + itoa(definition.Location.Line),
		})
	}
	return domain.Explanation{Summary: summary, Confidence: confidence, Provenance: provenance}
}

func itoa(value int) string {
	if value == 0 {
		return "unknown"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
