package kernel

import (
	"reflect"
	"sort"
	"strings"

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
	Purpose     string
	Help        string
	Explanation domain.Explanation
}

type MigrationImpact string

const (
	ImpactLostCapability   MigrationImpact = "lost-capability"
	ImpactNewlyEnabled     MigrationImpact = "newly-enabled"
	ImpactBuiltinToModule  MigrationImpact = "builtin-to-module"
	ImpactModuleToBuiltin  MigrationImpact = "module-to-builtin"
	ImpactValueChanged     MigrationImpact = "value-changed"
	ImpactDefinitionChange MigrationImpact = "definition-changed"
	ImpactInactiveChurn    MigrationImpact = "inactive-churn"
)

type MigrationSummary struct {
	Total               int
	LostCapabilities    int
	NewlyEnabled        int
	BuiltinToModule     int
	ModuleToBuiltin     int
	ValueChanged        int
	DefinitionChanged   int
	InactiveChurnHidden int
}

func SummarizeMigration(changes []Change) MigrationSummary {
	result := MigrationSummary{Total: len(changes)}
	for _, change := range changes {
		switch ClassifyMigrationChange(change) {
		case ImpactLostCapability:
			result.LostCapabilities++
		case ImpactNewlyEnabled:
			result.NewlyEnabled++
		case ImpactBuiltinToModule:
			result.BuiltinToModule++
		case ImpactModuleToBuiltin:
			result.ModuleToBuiltin++
		case ImpactValueChanged:
			result.ValueChanged++
		case ImpactDefinitionChange:
			result.DefinitionChanged++
		case ImpactInactiveChurn:
			result.InactiveChurnHidden++
		}
	}
	return result
}

// ConfigRelevantChanges retains value changes and definition changes for
// symbols enabled in either configuration. Definition-only churn for inactive
// symbols is not part of a running-config migration.
func ConfigRelevantChanges(changes []Change) []Change {
	result := make([]Change, 0, len(changes))
	for _, change := range changes {
		if containsChangeKind(change.Kinds, ChangeValue) ||
			stateEnabled(change.Before) || stateEnabled(change.After) {
			result = append(result, change)
		}
	}
	return result
}

func containsChangeKind(kinds []ChangeKind, expected ChangeKind) bool {
	for _, kind := range kinds {
		if kind == expected {
			return true
		}
	}
	return false
}

func stateEnabled(state *State) bool {
	return state != nil && (state.Kind == StateYes || state.Kind == StateModule)
}

func ClassifyMigrationChange(change Change) MigrationImpact {
	before, beforeEnabled := tristateValue(change.Before)
	after, afterEnabled := tristateValue(change.After)
	switch {
	case beforeEnabled && !afterEnabled:
		return ImpactLostCapability
	case !beforeEnabled && afterEnabled:
		return ImpactNewlyEnabled
	case before == StateYes && after == StateModule:
		return ImpactBuiltinToModule
	case before == StateModule && after == StateYes:
		return ImpactModuleToBuiltin
	case !beforeEnabled && !afterEnabled &&
		!nonTristateMaterial(change.Before) && !nonTristateMaterial(change.After):
		return ImpactInactiveChurn
	case nonTristateMaterial(change.Before) || nonTristateMaterial(change.After):
		return ImpactValueChanged
	case hasDefinitionChange(change.Kinds):
		return ImpactDefinitionChange
	default:
		return ImpactInactiveChurn
	}
}

func nonTristateMaterial(state *State) bool {
	return state != nil && state.Kind != StateNo && state.Kind != StateYes &&
		state.Kind != StateModule
}

func tristateValue(state *State) (StateKind, bool) {
	if state == nil {
		return StateNo, false
	}
	return state.Kind, state.Kind == StateYes || state.Kind == StateModule
}

func hasDefinitionChange(kinds []ChangeKind) bool {
	for _, kind := range kinds {
		if kind != ChangeValue {
			return true
		}
	}
	return false
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
		change.Purpose, change.Help = migrationPurpose(oldDefinition, newDefinition)
		change.Explanation = explainChange(change, oldDefinition, newDefinition)
		changes = append(changes, change)
	}
	return changes
}

func migrationPurpose(oldDefinition, newDefinition Definition) (string, string) {
	selected := newDefinition
	if selected.Prompt == "" && selected.Help == "" {
		selected = oldDefinition
	}
	purpose := selected.Prompt
	help := selected.Help
	if purpose == "" {
		purpose = firstSentence(help)
	}
	return purpose, help
}

func firstSentence(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if end := strings.Index(value, ". "); end >= 0 {
		return value[:end+1]
	}
	return value
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
