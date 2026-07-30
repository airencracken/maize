package gentooling

import (
	"context"
	"fmt"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

type UseDefault string

const (
	UseDefaultUnspecified UseDefault = ""
	UseDefaultEnabled     UseDefault = "+"
	UseDefaultDisabled    UseDefault = "-"
)

type UsePolicyEvidence struct {
	Enabled    bool
	Kind       string
	Layer      ConfigLayer
	Provenance domain.Provenance
}

type UseDecision struct {
	Name     string
	Enabled  bool
	Default  UseDefault
	Forced   bool
	Masked   bool
	Evidence []UsePolicyEvidence
}

type PackageUseEvidence struct {
	Package   shared.PackageID
	Stable    bool
	Decisions []UseDecision
}

// ReadPackageUse reads one effective configuration snapshot and evaluates its
// package-specific USE policy. Stable is explicit until Gentooling supplies
// keyword/visibility policy.
func ReadPackageUse(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	installed shared.InstalledPackage,
	stable bool,
) (PackageUseEvidence, error) {
	config, err := shared.ReadEffectiveConfig(ctx, paths, shared.ConfigOptions{
		Environment: append([]string(nil), environment...),
	})
	if err != nil {
		return PackageUseEvidence{}, err
	}
	evaluation, err := config.EvaluateUse(ctx, shared.PackageContext{
		ID:          installed.ID,
		DeclaredUse: append([]shared.UseDeclaration(nil), installed.DeclaredUse...),
		Stable:      stable,
	})
	if err != nil {
		return PackageUseEvidence{}, err
	}
	return packageUseEvidence(evaluation, stable), nil
}

func packageUseEvidence(evaluation shared.UseEvaluation, stable bool) PackageUseEvidence {
	result := PackageUseEvidence{
		Package: evaluation.Package,
		Stable:  stable,
	}
	for _, decision := range evaluation.Decisions {
		translated := UseDecision{
			Name: decision.Name, Enabled: decision.Enabled,
			Default: useDefault(decision.Default),
			Forced:  decision.Forced, Masked: decision.Masked,
		}
		for _, evidence := range decision.Evidence {
			layer := ConfigLayer(evidence.Layer)
			source := evidence.Source
			provenance := configProvenance(source, layer, fmt.Sprintf("effective USE input %s", evidence.Kind))
			if evidence.Kind == "iuse-default" {
				provenance = domain.Provenance{
					Kind: domain.SourcePackage, Source: evaluation.Package.CPV(),
					Detail: "IUSE default",
				}
			}
			translated.Evidence = append(translated.Evidence, UsePolicyEvidence{
				Enabled: evidence.Enabled, Kind: evidence.Kind, Layer: layer,
				Provenance: provenance,
			})
		}
		result.Decisions = append(result.Decisions, translated)
	}
	return result
}

func useDefault(value shared.UseDefault) UseDefault {
	switch value {
	case shared.UseDefaultEnabled:
		return UseDefaultEnabled
	case shared.UseDefaultDisabled:
		return UseDefaultDisabled
	default:
		return UseDefaultUnspecified
	}
}
