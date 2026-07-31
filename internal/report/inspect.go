package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/recommend"
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
	if _, err := fmt.Fprintf(
		writer, "Maize inspection\nKernel config: %s (%s)\nHardware devices: %d\nSnapshot consistency: %s\nRepositories: %d\nCandidate issues: %d\nDynamic kernel policies: %d\nInstalled packages: %d\nWorld selections: %d\nSystem selections: %d\nKernel recommendations: %d\n",
		inspection.ConfigSource.Path, inspection.ConfigSource.Origin,
		len(inspection.Hardware.Devices), inspection.SnapshotConsistency,
		len(inspection.Repositories), inspection.CandidateIssues,
		len(inspection.DynamicKernelPolicy), inspection.InstalledCount, len(inspection.WorldSelections),
		len(inspection.SystemSelections), len(inspection.Recommendations),
	); err != nil {
		return err
	}
	for _, dynamic := range inspection.DynamicKernelPolicy {
		if _, err := fmt.Fprintf(
			writer, "warning: dynamic kernel policy for %s: %s (%s)\n",
			dynamic.Package.CPV(), dynamic.Expression, dynamic.Reason,
		); err != nil {
			return err
		}
	}
	for _, item := range inspection.Recommendations {
		current := "missing"
		if item.Current != nil {
			current = item.Current.ConfigValue()
		}
		if _, err := fmt.Fprintf(
			writer, "\n%s: %s -> %s (%s, %s)\n  %s\n",
			item.Symbol.String(), current, item.Desired.ConfigValue(),
			item.Action, item.Disposition, item.Detail,
		); err != nil {
			return err
		}
		for _, evidence := range item.Evidence {
			if _, err := fmt.Fprintf(
				writer, "  because %s: %s [%s]\n",
				evidence.Source, evidence.Detail, evidence.Confidence,
			); err != nil {
				return err
			}
		}
	}
	return nil
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
