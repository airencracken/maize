package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/terminal"
)

type MigrationContext struct {
	RunningRelease   string `json:"running_release"`
	RunningConfig    string `json:"running_config"`
	TargetRelease    string `json:"target_release"`
	TargetTree       string `json:"target_tree"`
	ConsumerEvidence string `json:"consumer_evidence,omitempty"`
}

func HardwareJSON(writer io.Writer, inventory hardware.Inventory) error {
	document := struct {
		Schema   string       `json:"schema"`
		Hardware hardwareJSON `json:"hardware"`
	}{
		Schema: "maize.hardware/v1", Hardware: hardwareDocument(inventory),
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func MigrationText(writer io.Writer, changes []kernel.Change) error {
	return MigrationTextWithOptions(writer, changes, MigrationTextOptions{Verbose: true})
}

type MigrationTextOptions struct {
	Style            terminal.Style
	Verbose          bool
	Reasons          map[kernel.Symbol][]string
	EvidenceComplete bool
}

const defaultMigrationGroupLimit = 12

func MigrationTextWithContext(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
	style terminal.Style,
) error {
	return MigrationTextWithContextOptions(
		writer, context, changes, MigrationTextOptions{Style: style},
	)
}

func MigrationTextWithContextOptions(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
	options MigrationTextOptions,
) error {
	style := options.Style
	if _, err := fmt.Fprintf(
		writer,
		"%s %s\n%s %s\n%s %s\n%s %s\n",
		style.Bold("Running kernel:"), style.Cyan(context.RunningRelease),
		style.Bold("Running config:"), style.Cyan(context.RunningConfig),
		style.Bold("Target kernel:"), style.Cyan(context.TargetRelease),
		style.Bold("Target source:"), style.Cyan(context.TargetTree),
	); err != nil {
		return err
	}
	if context.ConsumerEvidence != "" {
		if _, err := fmt.Fprintf(
			writer, "%s %s\n",
			style.Bold("Consumer evidence:"), context.ConsumerEvidence,
		); err != nil {
			return err
		}
	}
	return MigrationTextWithOptions(writer, changes, options)
}

func MigrationTextStyled(
	writer io.Writer,
	changes []kernel.Change,
	style terminal.Style,
) error {
	return MigrationTextWithOptions(
		writer, changes, MigrationTextOptions{Style: style, Verbose: true},
	)
}

func MigrationTextWithOptions(
	writer io.Writer,
	changes []kernel.Change,
	options MigrationTextOptions,
) error {
	style := options.Style
	summary := kernel.SummarizeMigration(changes)
	orderedChanges := migrationChangesByConsumer(changes, options.Reasons)
	inactiveLabel := "inactive churn hidden:"
	if options.Verbose {
		inactiveLabel = "inactive churn:"
	}
	if _, err := fmt.Fprintf(
		writer,
		"%s %s\n  %s %s\n  %s %s\n  %s %s\n  %s %s\n  %s %s\n  %s %s\n  %s %s\n",
		style.BoldCyan("Kernel migration differences:"), style.Cyan(fmt.Sprint(summary.Total)),
		style.BoldRed("lost capabilities:"), style.Red(fmt.Sprint(summary.LostCapabilities)),
		style.BoldYellow("built-in to module:"), style.Yellow(fmt.Sprint(summary.BuiltinToModule)),
		style.BoldGreen("module to built-in:"), style.Green(fmt.Sprint(summary.ModuleToBuiltin)),
		style.BoldGreen("newly enabled:"), style.Green(fmt.Sprint(summary.NewlyEnabled)),
		style.Bold("other value changes:"), style.Cyan(fmt.Sprint(summary.ValueChanged)),
		style.Bold("Kconfig definition changes:"), style.Cyan(fmt.Sprint(summary.DefinitionChanged)),
		style.Bold(inactiveLabel), style.Cyan(fmt.Sprint(summary.InactiveChurnHidden)),
	); err != nil {
		return err
	}
	for _, impact := range migrationImpactOrder(options.Verbose) {
		count := migrationImpactCount(summary, impact)
		if count == 0 {
			continue
		}
		if _, err := fmt.Fprintf(writer, "\n%s\n", style.Bold(migrationImpactHeading(impact))); err != nil {
			return err
		}
		written := 0
		limit := defaultMigrationGroupLimit
		if options.Verbose {
			limit = count
		}
		for _, change := range orderedChanges {
			if kernel.ClassifyMigrationChange(change) != impact {
				continue
			}
			if written >= limit {
				continue
			}
			if _, err := fmt.Fprintf(
				writer, "  %s: %s -> %s\n    %s\n",
				migrationSymbolStyle(style, impact, change.Symbol.String()),
				migrationState(change.Before), migrationState(change.After),
				migrationImpactExplanation(impact, change),
			); err != nil {
				return err
			}
			if change.Purpose != "" {
				if _, err := fmt.Fprintf(writer, "    %s %s\n", style.Bold("Purpose:"), change.Purpose); err != nil {
					return err
				}
			}
			reasons := options.Reasons[change.Symbol]
			for _, reason := range reasons {
				if _, err := fmt.Fprintf(
					writer, "    %s %s\n", style.Bold("Current-system reason:"), reason,
				); err != nil {
					return err
				}
			}
			if len(reasons) == 0 && options.EvidenceComplete &&
				(impact == kernel.ImpactLostCapability || impact == kernel.ImpactBuiltinToModule) {
				if _, err := fmt.Fprintln(
					writer, "    No current package or hardware requirement is known to Maize.",
				); err != nil {
					return err
				}
			}
			if options.Verbose {
				if help := compactMigrationHelp(change.Help); help != "" && help != change.Purpose {
					if _, err := fmt.Fprintf(writer, "    %s %s\n", style.Bold("Kconfig help:"), help); err != nil {
						return err
					}
				}
				for _, provenance := range change.Explanation.Provenance {
					if _, err := fmt.Fprintf(
						writer, "    %s %s (%s)\n",
						style.Bold("Kconfig source:"), provenance.Source, provenance.Detail,
					); err != nil {
						return err
					}
				}
			}
			written++
		}
		if written < count {
			if _, err := fmt.Fprintf(
				writer, "  ... %d more in this category; use --verbose to show all\n",
				count-written,
			); err != nil {
				return err
			}
		}
	}
	if !options.Verbose && summary.InactiveChurnHidden != 0 {
		_, err := fmt.Fprintln(writer, "\nUse --verbose to include inactive symbol churn.")
		return err
	}
	return nil
}

func migrationChangesByConsumer(
	changes []kernel.Change,
	reasons map[kernel.Symbol][]string,
) []kernel.Change {
	result := append([]kernel.Change(nil), changes...)
	sort.SliceStable(result, func(left, right int) bool {
		leftKnown := len(reasons[result[left].Symbol]) != 0
		rightKnown := len(reasons[result[right].Symbol]) != 0
		if leftKnown != rightKnown {
			return leftKnown
		}
		return result[left].Symbol < result[right].Symbol
	})
	return result
}

func compactMigrationHelp(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	const limit = 500
	if len(value) > limit {
		return value[:limit] + "..."
	}
	return value
}

func migrationImpactOrder(verbose bool) []kernel.MigrationImpact {
	result := []kernel.MigrationImpact{
		kernel.ImpactLostCapability, kernel.ImpactBuiltinToModule,
		kernel.ImpactModuleToBuiltin, kernel.ImpactNewlyEnabled,
		kernel.ImpactValueChanged, kernel.ImpactDefinitionChange,
	}
	if verbose {
		result = append(result, kernel.ImpactInactiveChurn)
	}
	return result
}

func migrationImpactCount(summary kernel.MigrationSummary, impact kernel.MigrationImpact) int {
	switch impact {
	case kernel.ImpactLostCapability:
		return summary.LostCapabilities
	case kernel.ImpactNewlyEnabled:
		return summary.NewlyEnabled
	case kernel.ImpactBuiltinToModule:
		return summary.BuiltinToModule
	case kernel.ImpactModuleToBuiltin:
		return summary.ModuleToBuiltin
	case kernel.ImpactValueChanged:
		return summary.ValueChanged
	case kernel.ImpactDefinitionChange:
		return summary.DefinitionChanged
	default:
		return summary.InactiveChurnHidden
	}
}

func migrationImpactHeading(impact kernel.MigrationImpact) string {
	switch impact {
	case kernel.ImpactLostCapability:
		return "Capabilities disabled or removed"
	case kernel.ImpactNewlyEnabled:
		return "New target defaults enabled"
	case kernel.ImpactBuiltinToModule:
		return "Built-in capabilities changed to modules"
	case kernel.ImpactModuleToBuiltin:
		return "Modules changed to built-in"
	case kernel.ImpactValueChanged:
		return "Other changed values"
	case kernel.ImpactDefinitionChange:
		return "Kconfig definitions changed"
	default:
		return "Inactive symbol churn"
	}
}

func migrationState(state *kernel.State) string {
	if state == nil {
		return "absent"
	}
	return state.ConfigValue()
}

func migrationImpactExplanation(impact kernel.MigrationImpact, change kernel.Change) string {
	switch impact {
	case kernel.ImpactLostCapability:
		if change.After == nil {
			return "enabled in the running kernel; this symbol is absent from the target result"
		}
		return "enabled in the running kernel but unavailable or disabled in the target result"
	case kernel.ImpactNewlyEnabled:
		if change.Before == nil {
			return "new target symbol enabled by target Kconfig"
		}
		return "disabled or absent in the running kernel and enabled by the target"
	case kernel.ImpactBuiltinToModule:
		return "target Kconfig weakened built-in support to a loadable module"
	case kernel.ImpactModuleToBuiltin:
		return "target Kconfig promoted a module to built-in support"
	case kernel.ImpactValueChanged:
		return "target Kconfig changed a non-tristate value"
	case kernel.ImpactDefinitionChange:
		return "the symbol's Kconfig definition changed"
	default:
		return "disabled symbol was added or removed without enabling a capability"
	}
}

func migrationSymbolStyle(style terminal.Style, impact kernel.MigrationImpact, symbol string) string {
	switch impact {
	case kernel.ImpactLostCapability, kernel.ImpactBuiltinToModule:
		return style.Red(symbol)
	case kernel.ImpactNewlyEnabled, kernel.ImpactModuleToBuiltin:
		return style.Green(symbol)
	default:
		return style.Cyan(symbol)
	}
}

func MigrationJSON(writer io.Writer, changes []kernel.Change) error {
	return migrationJSON(writer, "maize.migration/v1", nil, changes, false, nil, false)
}

func MigrationJSONWithContext(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
) error {
	return migrationJSON(writer, "maize.migration/v2", &context, changes, false, nil, false)
}

func MigrationJSONPrioritized(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
) error {
	return migrationJSON(writer, "maize.migration/v3", &context, changes, true, nil, false)
}

func MigrationJSONExplained(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
	reasons map[kernel.Symbol][]string,
) error {
	return migrationJSON(writer, "maize.migration/v4", &context, changes, true, reasons, true)
}

func migrationJSON(
	writer io.Writer,
	schema string,
	context *MigrationContext,
	changes []kernel.Change,
	prioritized bool,
	reasons map[kernel.Symbol][]string,
	explained bool,
) error {
	type sourceJSON struct {
		Path   string `json:"path"`
		Detail string `json:"detail"`
	}
	type changeJSON struct {
		Symbol  string       `json:"symbol"`
		Kinds   []string     `json:"kinds"`
		Before  *string      `json:"before"`
		After   *string      `json:"after"`
		Summary string       `json:"summary"`
		Impact  string       `json:"impact,omitempty"`
		Purpose string       `json:"purpose,omitempty"`
		Help    string       `json:"help,omitempty"`
		Reasons []string     `json:"current_system_reasons,omitempty"`
		Sources []sourceJSON `json:"kconfig_sources,omitempty"`
	}
	type summaryJSON struct {
		Total               int `json:"total"`
		LostCapabilities    int `json:"lost_capabilities"`
		NewlyEnabled        int `json:"newly_enabled"`
		BuiltinToModule     int `json:"builtin_to_module"`
		ModuleToBuiltin     int `json:"module_to_builtin"`
		ValueChanged        int `json:"value_changed"`
		DefinitionChanged   int `json:"definition_changed"`
		InactiveChurnHidden int `json:"inactive_churn"`
	}
	document := struct {
		Schema  string            `json:"schema"`
		Context *MigrationContext `json:"context,omitempty"`
		Summary *summaryJSON      `json:"summary,omitempty"`
		Changes []changeJSON      `json:"changes"`
	}{
		Schema:  schema,
		Context: context,
		Changes: make([]changeJSON, 0, len(changes)),
	}
	if prioritized {
		summary := kernel.SummarizeMigration(changes)
		document.Summary = &summaryJSON{
			Total: summary.Total, LostCapabilities: summary.LostCapabilities,
			NewlyEnabled: summary.NewlyEnabled, BuiltinToModule: summary.BuiltinToModule,
			ModuleToBuiltin: summary.ModuleToBuiltin, ValueChanged: summary.ValueChanged,
			DefinitionChanged:   summary.DefinitionChanged,
			InactiveChurnHidden: summary.InactiveChurnHidden,
		}
	}
	for _, change := range changes {
		item := changeJSON{
			Symbol: change.Symbol.String(), Summary: change.Explanation.Summary,
		}
		if prioritized {
			item.Impact = string(kernel.ClassifyMigrationChange(change))
		}
		if explained {
			item.Purpose = change.Purpose
			item.Help = change.Help
			item.Reasons = append([]string{}, reasons[change.Symbol]...)
			item.Sources = make([]sourceJSON, 0, len(change.Explanation.Provenance))
			for _, provenance := range change.Explanation.Provenance {
				item.Sources = append(item.Sources, sourceJSON{
					Path: provenance.Source, Detail: provenance.Detail,
				})
			}
		}
		for _, kind := range change.Kinds {
			item.Kinds = append(item.Kinds, string(kind))
		}
		if change.Before != nil {
			value := change.Before.ConfigValue()
			item.Before = &value
		}
		if change.After != nil {
			value := change.After.ConfigValue()
			item.After = &value
		}
		document.Changes = append(document.Changes, item)
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}
