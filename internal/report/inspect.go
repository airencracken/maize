package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/recommend"
	"github.com/airencracken/maize/internal/terminal"
)

type inspectJSON struct {
	Schema              string               `json:"schema"`
	ConfigSource        configSourceJSON     `json:"config_source"`
	Hardware            hardwareJSON         `json:"hardware"`
	SnapshotConsistency string               `json:"snapshot_consistency"`
	Repositories        []repositoryJSON     `json:"repositories"`
	CandidateIssues     int                  `json:"candidate_issues"`
	DynamicKernelPolicy []dynamicPolicyJSON  `json:"dynamic_kernel_policy"`
	InstalledCount      int                  `json:"installed_count"`
	WorldSelections     []selectionJSON      `json:"world_selections"`
	SystemSelections    []selectionJSON      `json:"system_selections"`
	Recommendations     []recommendationJSON `json:"recommendations"`
}

type dynamicPolicyJSON struct {
	Package    string `json:"package"`
	Expression string `json:"expression"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
	Detail     string `json:"detail"`
}

type repositoryJSON struct {
	Name     string   `json:"name"`
	Location string   `json:"location"`
	Priority int      `json:"priority"`
	Main     bool     `json:"main"`
	Masters  []string `json:"masters"`
	Source   string   `json:"source"`
	Detail   string   `json:"detail"`
}

type configSourceJSON struct {
	Path           string `json:"path"`
	Origin         string `json:"origin"`
	RunningRelease string `json:"running_release,omitempty"`
	Compressed     bool   `json:"compressed"`
}

type hardwareJSON struct {
	Schema  uint         `json:"schema"`
	Devices []deviceJSON `json:"devices"`
}

type deviceJSON struct {
	Bus        string           `json:"bus"`
	Address    string           `json:"address"`
	Vendor     string           `json:"vendor,omitempty"`
	Product    string           `json:"product,omitempty"`
	Class      string           `json:"class,omitempty"`
	Name       string           `json:"name,omitempty"`
	Driver     string           `json:"driver,omitempty"`
	Modules    []string         `json:"modules"`
	Firmware   []string         `json:"firmware"`
	Presence   string           `json:"presence"`
	Provenance []provenanceJSON `json:"provenance"`
}

type provenanceJSON struct {
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	Detail     string `json:"detail"`
	ObservedAt string `json:"observed_at,omitempty"`
}

type selectionJSON struct {
	Value  string `json:"value"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Line   string `json:"detail"`
}

type recommendationJSON struct {
	Capability  string         `json:"capability"`
	Disposition string         `json:"disposition"`
	Symbol      string         `json:"symbol"`
	Current     *string        `json:"current"`
	Desired     string         `json:"desired"`
	Action      string         `json:"action"`
	Detail      string         `json:"detail"`
	Evidence    []evidenceJSON `json:"evidence"`
}

type evidenceJSON struct {
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	Detail     string `json:"detail"`
	Confidence string `json:"confidence"`
}

func InspectionText(writer io.Writer, inspection app.Inspection) error {
	return InspectionTextWithOptions(writer, inspection, TextOptions{})
}

func InspectionTextStyled(
	writer io.Writer,
	inspection app.Inspection,
	style terminal.Style,
) error {
	return InspectionTextWithOptions(writer, inspection, TextOptions{Style: style})
}

type TextOptions struct {
	Style   terminal.Style
	Verbose bool
}

func InspectionTextWithOptions(
	writer io.Writer,
	inspection app.Inspection,
	options TextOptions,
) error {
	style := options.Style
	if _, err := fmt.Fprintf(
		writer, "%s\n%s %s (%s)\n%s %s\n%s %s\n%s %s\n%s %s\n%s %s\n%s %s\n%s %s\n%s %s\n%s %s\n",
		style.BoldCyan("Maize inspection"),
		style.Bold("Kernel config:"), style.Cyan(inspection.ConfigSource.Path), inspection.ConfigSource.Origin,
		style.Bold("Hardware devices:"), style.Cyan(fmt.Sprint(len(inspection.Hardware.Devices))),
		style.Bold("Snapshot consistency:"), style.Cyan(string(inspection.SnapshotConsistency)),
		style.Bold("Repositories:"), style.Cyan(fmt.Sprint(len(inspection.Repositories))),
		style.Bold("Candidate issues:"), countStyle(style, inspection.CandidateIssues),
		style.Bold("Dynamic kernel policies:"), countStyle(style, len(inspection.DynamicKernelPolicy)),
		style.Bold("Installed packages:"), style.Cyan(fmt.Sprint(inspection.InstalledCount)),
		style.Bold("World selections:"), style.Cyan(fmt.Sprint(len(inspection.WorldSelections))),
		style.Bold("System selections:"), style.Cyan(fmt.Sprint(len(inspection.SystemSelections))),
		style.Bold("Kernel recommendations:"), style.Cyan(fmt.Sprint(len(inspection.Recommendations))),
	); err != nil {
		return err
	}
	if err := writeDynamicPolicySummary(writer, inspection, options); err != nil {
		return err
	}
	for _, item := range inspection.Recommendations {
		current := "missing"
		if item.Current != nil {
			current = item.Current.ConfigValue()
		}
		if _, err := fmt.Fprintf(
			writer, "\n%s: %s -> %s (%s, %s)\n",
			style.Cyan(item.Symbol.String()), current,
			recommendationState(style, item), recommendationAction(style, item),
			item.Disposition,
		); err != nil {
			return err
		}
		if !evidenceRepeatsDetail(item) {
			if _, err := fmt.Fprintf(writer, "  %s\n", item.Detail); err != nil {
				return err
			}
		}
		for _, evidence := range item.Evidence {
			if _, err := fmt.Fprintf(writer, "  because %s", evidence.Detail); err != nil {
				return err
			}
			if evidence.Source != "" {
				if _, err := fmt.Fprintf(writer, " [%s]", style.Magenta(evidence.Source)); err != nil {
					return err
				}
			}
			if options.Verbose {
				if _, err := fmt.Fprintf(
					writer, " [%s]", style.Cyan(string(evidence.Confidence)),
				); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if options.Verbose {
			for _, provenance := range item.Provenance {
				if _, err := fmt.Fprintf(
					writer, "  source %s: %s\n",
					style.Magenta(provenance.Source), provenance.Detail,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func evidenceRepeatsDetail(item recommend.Recommendation) bool {
	return len(item.Evidence) == 1 && item.Detail != "" &&
		item.Evidence[0].Detail == item.Detail
}

func writeDynamicPolicySummary(
	writer io.Writer,
	inspection app.Inspection,
	options TextOptions,
) error {
	if len(inspection.DynamicKernelPolicy) == 0 {
		_, err := fmt.Fprintf(
			writer, "%s %s\n",
			options.Style.Bold("Package kernel policy:"), options.Style.Green("complete"),
		)
		return err
	}
	packages := make(map[string]bool)
	dispatch := 0
	for _, dynamic := range inspection.DynamicKernelPolicy {
		packages[dynamic.Package.CPV()] = true
		if dynamic.Expression == "inherit linux-info" {
			dispatch++
		}
	}
	expressions := len(inspection.DynamicKernelPolicy) - dispatch
	if _, err := fmt.Fprintf(
		writer, "%s %s\n  %d unresolved findings across %d packages\n  %d runtime dispatch markers; %d unevaluated expressions\n",
		options.Style.Bold("Package kernel policy:"),
		options.Style.BoldYellow("incomplete"),
		len(inspection.DynamicKernelPolicy), len(packages), dispatch, expressions,
	); err != nil {
		return err
	}
	if !options.Verbose {
		_, err := fmt.Fprintln(
			writer, "  Use --verbose to show every unresolved package-policy finding.",
		)
		return err
	}
	for _, dynamic := range inspection.DynamicKernelPolicy {
		if _, err := fmt.Fprintf(
			writer, "  %s %s: %s (%s)\n",
			options.Style.BoldYellow("unresolved"),
			options.Style.Magenta(dynamic.Package.CPV()),
			options.Style.Yellow(dynamic.Expression), dynamic.Reason,
		); err != nil {
			return err
		}
	}
	return nil
}

func countStyle(style terminal.Style, count int) string {
	value := fmt.Sprint(count)
	if count != 0 {
		return style.Yellow(value)
	}
	return style.Green(value)
}

func recommendationState(style terminal.Style, item recommend.Recommendation) string {
	value := item.Desired.ConfigValue()
	switch item.Action {
	case recommend.ActionKeep:
		return style.Green(value)
	case recommend.ActionDisable:
		return style.Red(value)
	default:
		return style.Yellow(value)
	}
}

func recommendationAction(style terminal.Style, item recommend.Recommendation) string {
	value := string(item.Action)
	switch item.Action {
	case recommend.ActionKeep:
		return style.Green(value)
	case recommend.ActionDisable:
		return style.Red(value)
	default:
		if item.Disposition == domain.Required {
			return style.BoldYellow(value)
		}
		return style.Yellow(value)
	}
}

func InspectionJSON(writer io.Writer, inspection app.Inspection) error {
	document := inspectJSON{
		Schema: inspection.Schema, InstalledCount: inspection.InstalledCount,
		ConfigSource: configSourceJSON{
			Path: inspection.ConfigSource.Path, Origin: string(inspection.ConfigSource.Origin),
			RunningRelease: inspection.ConfigSource.RunningRelease,
			Compressed:     inspection.ConfigSource.Compressed,
		},
		Hardware:            hardwareDocument(inspection.Hardware),
		SnapshotConsistency: string(inspection.SnapshotConsistency),
		Repositories:        make([]repositoryJSON, 0, len(inspection.Repositories)),
		CandidateIssues:     inspection.CandidateIssues,
		DynamicKernelPolicy: make(
			[]dynamicPolicyJSON, 0, len(inspection.DynamicKernelPolicy),
		),
		WorldSelections:  make([]selectionJSON, 0, len(inspection.WorldSelections)),
		SystemSelections: make([]selectionJSON, 0, len(inspection.SystemSelections)),
		Recommendations:  make([]recommendationJSON, 0, len(inspection.Recommendations)),
	}
	for _, dynamic := range inspection.DynamicKernelPolicy {
		document.DynamicKernelPolicy = append(document.DynamicKernelPolicy, dynamicPolicyJSON{
			Package: dynamic.Package.CPV(), Expression: dynamic.Expression,
			Reason: dynamic.Reason, Source: dynamic.Provenance.Source,
			Detail: dynamic.Provenance.Detail,
		})
	}
	for _, repository := range inspection.Repositories {
		document.Repositories = append(document.Repositories, repositoryJSON{
			Name: repository.Name, Location: repository.Location,
			Priority: repository.Priority, Main: repository.Main,
			Masters: append([]string{}, repository.Masters...),
			Source:  repository.Provenance.Source, Detail: repository.Provenance.Detail,
		})
	}
	for _, selection := range inspection.WorldSelections {
		document.WorldSelections = append(document.WorldSelections, selectionJSON{
			Value: selection.Value, Kind: string(selection.Kind),
			Source: selection.Provenance.Source, Line: selection.Provenance.Detail,
		})
	}
	for _, selection := range inspection.SystemSelections {
		document.SystemSelections = append(document.SystemSelections, selectionJSON{
			Value: selection.Value, Kind: string(selection.Kind),
			Source: selection.Provenance.Source, Line: selection.Provenance.Detail,
		})
	}
	for _, item := range inspection.Recommendations {
		document.Recommendations = append(
			document.Recommendations, recommendationDocument(item),
		)
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}

func hardwareDocument(inventory hardware.Inventory) hardwareJSON {
	result := hardwareJSON{
		Schema: inventory.Schema, Devices: make([]deviceJSON, 0, len(inventory.Devices)),
	}
	for _, device := range inventory.Devices {
		translated := deviceJSON{
			Bus: string(device.Bus), Address: device.ID.Address,
			Vendor: device.ID.Vendor, Product: device.ID.Product, Class: device.ID.Class,
			Name: device.Name, Driver: device.Driver,
			Modules:    append([]string{}, device.Modules...),
			Firmware:   append([]string{}, device.Firmware...),
			Presence:   string(device.Presence),
			Provenance: make([]provenanceJSON, 0, len(device.Provenance)),
		}
		for _, provenance := range device.Provenance {
			observedAt := ""
			if !provenance.ObservedAt.IsZero() {
				observedAt = provenance.ObservedAt.UTC().Format("2006-01-02T15:04:05.999999999Z")
			}
			translated.Provenance = append(translated.Provenance, provenanceJSON{
				Kind: string(provenance.Kind), Source: provenance.Source,
				Detail: provenance.Detail, ObservedAt: observedAt,
			})
		}
		result.Devices = append(result.Devices, translated)
	}
	return result
}

func recommendationDocument(item recommend.Recommendation) recommendationJSON {
	result := recommendationJSON{
		Capability: item.Capability, Disposition: string(item.Disposition),
		Symbol: item.Symbol.String(), Desired: item.Desired.ConfigValue(),
		Action: string(item.Action), Detail: item.Detail,
		Evidence: make([]evidenceJSON, 0, len(item.Evidence)),
	}
	if item.Current != nil {
		value := item.Current.ConfigValue()
		result.Current = &value
	}
	for _, evidence := range item.Evidence {
		result.Evidence = append(result.Evidence, evidenceDocument(evidence))
	}
	return result
}

func evidenceDocument(evidence domain.Evidence) evidenceJSON {
	return evidenceJSON{
		Kind: string(evidence.Kind), Source: evidence.Source,
		Detail: evidence.Detail, Confidence: string(evidence.Confidence),
	}
}
