package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/recommend"
)

type inspectJSON struct {
	Schema           string               `json:"schema"`
	InstalledCount   int                  `json:"installed_count"`
	WorldSelections  []selectionJSON      `json:"world_selections"`
	SystemSelections []selectionJSON      `json:"system_selections"`
	Recommendations  []recommendationJSON `json:"recommendations"`
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
		writer, "Maize inspection\nInstalled packages: %d\nWorld selections: %d\nSystem selections: %d\nKernel recommendations: %d\n",
		inspection.InstalledCount, len(inspection.WorldSelections),
		len(inspection.SystemSelections), len(inspection.Recommendations),
	); err != nil {
		return err
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
		WorldSelections:  make([]selectionJSON, 0, len(inspection.WorldSelections)),
		SystemSelections: make([]selectionJSON, 0, len(inspection.SystemSelections)),
		Recommendations:  make([]recommendationJSON, 0, len(inspection.Recommendations)),
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
