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

type SnapshotConsistency string

const (
	SnapshotLocked     SnapshotConsistency = "locked-and-stabilized"
	SnapshotStabilized SnapshotConsistency = "stabilized-lockless"
)

type RepositoryEvidence struct {
	Name       string
	Location   string
	Priority   int
	Main       bool
	Masters    []string
	Provenance domain.Provenance
}

type SystemSnapshotEvidence struct {
	Installed    shared.InstalledInventory
	Config       EffectiveConfigEvidence
	Repositories []RepositoryEvidence
	Selections   SelectionsEvidence
	Consistency  SnapshotConsistency
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
	return ReadSystemSnapshotWithConsistency(
		ctx, paths, environment, attempts, SnapshotLocked,
	)
}

func ReadSystemSnapshotWithConsistency(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	attempts int,
	consistency SnapshotConsistency,
) (SystemSnapshotEvidence, error) {
	sharedConsistency, err := sharedSnapshotConsistency(consistency)
	if err != nil {
		return SystemSnapshotEvidence{}, err
	}
	snapshot, err := shared.ReadSystemSnapshot(ctx, paths, shared.SnapshotOptions{
		Installed: shared.InstalledOptions{
			Integrity:       shared.RequireComplete,
			IncludeContents: false,
		},
		Config: shared.ConfigOptions{
			Environment: append([]string(nil), environment...),
		},
		Attempts: attempts, Consistency: sharedConsistency,
	})
	if err != nil {
		return SystemSnapshotEvidence{}, err
	}
	return SystemSnapshotEvidence{
		Installed: snapshot.Installed, Config: configEvidence(snapshot.Config),
		Repositories: repositoriesEvidence(snapshot.Repositories),
		Selections:   selectionsEvidence(snapshot.Selections),
		Consistency:  consistencyEvidence(snapshot.Consistency),
	}, nil
}

func sharedSnapshotConsistency(value SnapshotConsistency) (shared.SnapshotConsistency, error) {
	switch value {
	case SnapshotLocked:
		return shared.LockedAndStabilized, nil
	case SnapshotStabilized:
		return shared.StabilizedLockless, nil
	default:
		return 0, fmt.Errorf("invalid snapshot consistency %q", value)
	}
}

func consistencyEvidence(value shared.SnapshotConsistency) SnapshotConsistency {
	if value == shared.StabilizedLockless {
		return SnapshotStabilized
	}
	return SnapshotLocked
}

func repositoriesEvidence(repositories []shared.Repository) []RepositoryEvidence {
	result := make([]RepositoryEvidence, 0, len(repositories))
	for _, repository := range repositories {
		detail := "repository configuration"
		source := repository.Source.Path
		kind := domain.SourceConfig
		if source == "" {
			source = "caller"
			kind = domain.SourceOperator
			detail = "explicit repository path"
		}
		if repository.Source.Line > 0 {
			detail = fmt.Sprintf("%s at line %d", detail, repository.Source.Line)
		}
		result = append(result, RepositoryEvidence{
			Name: repository.Name, Location: repository.Location,
			Priority: repository.Priority, Main: repository.Main,
			Masters: append([]string(nil), repository.Masters...),
			Provenance: domain.Provenance{
				Kind: kind, Source: source, Detail: detail,
			},
		})
	}
	return result
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
