package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
)

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
	if _, err := fmt.Fprintf(writer, "Kernel migration changes: %d\n", len(changes)); err != nil {
		return err
	}
	for _, change := range changes {
		if _, err := fmt.Fprintf(
			writer, "%s: %v\n  %s\n",
			change.Symbol.String(), change.Kinds, change.Explanation.Summary,
		); err != nil {
			return err
		}
	}
	return nil
}

func MigrationJSON(writer io.Writer, changes []kernel.Change) error {
	type changeJSON struct {
		Symbol  string   `json:"symbol"`
		Kinds   []string `json:"kinds"`
		Before  *string  `json:"before"`
		After   *string  `json:"after"`
		Summary string   `json:"summary"`
	}
	document := struct {
		Schema  string       `json:"schema"`
		Changes []changeJSON `json:"changes"`
	}{
		Schema:  "maize.migration/v1",
		Changes: make([]changeJSON, 0, len(changes)),
	}
	for _, change := range changes {
		item := changeJSON{
			Symbol: change.Symbol.String(), Summary: change.Explanation.Summary,
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
