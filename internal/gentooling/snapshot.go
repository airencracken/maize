package gentooling

import (
	"context"
	"fmt"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

type SelectionKind string

const (
	SelectionPackage SelectionKind = "package"
	SelectionSet     SelectionKind = "set"
)

type SelectionEvidence struct {
	Value      string
	Kind       SelectionKind
	Provenance domain.Provenance
}

type SelectionsEvidence struct {
	World  []SelectionEvidence
	System []SelectionEvidence
}

type SystemSnapshotEvidence struct {
	Installed  shared.InstalledInventory
	Config     EffectiveConfigEvidence
	Selections SelectionsEvidence
}

// ReadSystemSnapshot obtains one mutually consistent view of installed
// packages, effective Portage policy, and world/system selections. It uses
// strict installed-package evidence and excludes CONTENTS payloads.
func ReadSystemSnapshot(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	attempts int,
) (SystemSnapshotEvidence, error) {
	snapshot, err := shared.ReadSystemSnapshot(ctx, paths, shared.SnapshotOptions{
		Installed: shared.InstalledOptions{
			Integrity:       shared.RequireComplete,
			IncludeContents: false,
		},
		Config: shared.ConfigOptions{
			Environment: append([]string(nil), environment...),
		},
		Attempts: attempts,
	})
	if err != nil {
		return SystemSnapshotEvidence{}, err
	}
	return SystemSnapshotEvidence{
		Installed:  snapshot.Installed,
		Config:     configEvidence(snapshot.Config),
		Selections: selectionsEvidence(snapshot.Selections),
	}, nil
}

func selectionsEvidence(selections shared.Selections) SelectionsEvidence {
	return SelectionsEvidence{
		World:  translateSelections(selections.World, false),
		System: translateSelections(selections.System, true),
	}
}

func translateSelections(selections []shared.Selection, profile bool) []SelectionEvidence {
	result := make([]SelectionEvidence, 0, len(selections))
	for _, selection := range selections {
		kind := SelectionPackage
		if selection.Kind == shared.SetSelection {
			kind = SelectionSet
		}
		sourceKind := domain.SourceConfig
		detail := "world selection"
		if profile {
			sourceKind = domain.SourceProfile
			detail = "effective system selection"
		}
		if selection.Source.Line > 0 {
			detail = fmt.Sprintf("%s at line %d", detail, selection.Source.Line)
		}
		result = append(result, SelectionEvidence{
			Value: selection.Value,
			Kind:  kind,
			Provenance: domain.Provenance{
				Kind: sourceKind, Source: selection.Source.Path, Detail: detail,
			},
		})
	}
	return result
}
