package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/terminal"
)

type MigrationContext struct {
	RunningRelease string `json:"running_release"`
	RunningConfig  string `json:"running_config"`
	TargetRelease  string `json:"target_release"`
	TargetTree     string `json:"target_tree"`
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
	return MigrationTextStyled(writer, changes, terminal.Style{})
}

func MigrationTextWithContext(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
	style terminal.Style,
) error {
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
	return MigrationTextStyled(writer, changes, style)
}

func MigrationTextStyled(
	writer io.Writer,
	changes []kernel.Change,
	style terminal.Style,
) error {
	if _, err := fmt.Fprintf(
		writer, "%s %s\n",
		style.BoldCyan("Kernel migration changes:"), style.Cyan(fmt.Sprint(len(changes))),
	); err != nil {
		return err
	}
	for _, change := range changes {
		if _, err := fmt.Fprintf(
			writer, "%s: %v\n  %s\n",
			style.Cyan(change.Symbol.String()), change.Kinds, change.Explanation.Summary,
		); err != nil {
			return err
		}
	}
	return nil
}

func MigrationJSON(writer io.Writer, changes []kernel.Change) error {
	return migrationJSON(writer, "maize.migration/v1", nil, changes)
}

func MigrationJSONWithContext(
	writer io.Writer,
	context MigrationContext,
	changes []kernel.Change,
) error {
	return migrationJSON(writer, "maize.migration/v2", &context, changes)
}

func migrationJSON(
	writer io.Writer,
	schema string,
	context *MigrationContext,
	changes []kernel.Change,
) error {
	type changeJSON struct {
		Symbol  string   `json:"symbol"`
		Kinds   []string `json:"kinds"`
		Before  *string  `json:"before"`
		After   *string  `json:"after"`
		Summary string   `json:"summary"`
	}
	document := struct {
		Schema  string            `json:"schema"`
		Context *MigrationContext `json:"context,omitempty"`
		Changes []changeJSON      `json:"changes"`
	}{
		Schema:  schema,
		Context: context,
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
