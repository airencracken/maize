package gentooling

import (
	"context"
	"fmt"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

type VisibilityStatus string

const (
	VisibilityVisible                 VisibilityStatus = "visible"
	VisibilityPackageMasked           VisibilityStatus = "package-masked"
	VisibilityKeywordMasked           VisibilityStatus = "keyword-masked"
	VisibilityUnsupportedArchitecture VisibilityStatus = "unsupported-architecture"
)

type VisibilityPolicyEvidence struct {
	Kind       string
	Value      string
	Enabled    bool
	Layer      ConfigLayer
	Reason     string
	Provenance domain.Provenance
}

type PackageVisibilityEvidence struct {
	Package          shared.PackageID
	Visible          bool
	Stable           bool
	Status           VisibilityStatus
	Architecture     string
	PackageKeywords  []string
	AcceptedKeywords []string
	Evidence         []VisibilityPolicyEvidence
}

type ProspectivePackage struct {
	ID          shared.PackageID
	Keywords    []string
	DeclaredUse []shared.UseDeclaration
}

type ProspectivePackageEvidence struct {
	Visibility PackageVisibilityEvidence
	Use        PackageUseEvidence
}

// ReadProspectivePackage evaluates visibility and USE policy from the same
// effective configuration. Visibility determines whether stable-only USE
// policy applies; callers do not supply or infer stability.
func ReadProspectivePackage(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	candidate ProspectivePackage,
) (ProspectivePackageEvidence, error) {
	config, err := shared.ReadEffectiveConfig(ctx, paths, shared.ConfigOptions{
		Environment: append([]string(nil), environment...),
	})
	if err != nil {
		return ProspectivePackageEvidence{}, err
	}
	visibility, err := config.EvaluateVisibility(ctx, shared.PackageVisibilityContext{
		ID:       candidate.ID,
		Keywords: append([]string(nil), candidate.Keywords...),
	})
	if err != nil {
		return ProspectivePackageEvidence{}, err
	}
	use, err := config.EvaluateUse(ctx, shared.PackageContext{
		ID:          candidate.ID,
		DeclaredUse: append([]shared.UseDeclaration(nil), candidate.DeclaredUse...),
		Stable:      visibility.Stable,
	})
	if err != nil {
		return ProspectivePackageEvidence{}, err
	}
	return ProspectivePackageEvidence{
		Visibility: packageVisibilityEvidence(visibility),
		Use:        packageUseEvidence(use, visibility.Stable),
	}, nil
}

func packageVisibilityEvidence(result shared.VisibilityResult) PackageVisibilityEvidence {
	evidence := PackageVisibilityEvidence{
		Package: result.Package, Visible: result.Visible, Stable: result.Stable,
		Status:           visibilityStatus(result.Status),
		Architecture:     result.Architecture,
		PackageKeywords:  append([]string(nil), result.PackageKeywords...),
		AcceptedKeywords: append([]string(nil), result.AcceptedKeywords...),
	}
	for _, input := range result.Evidence {
		layer := ConfigLayer(input.Layer)
		provenance := configProvenance(
			input.Source, layer, fmt.Sprintf("visibility policy input %s", input.Kind),
		)
		evidence.Evidence = append(evidence.Evidence, VisibilityPolicyEvidence{
			Kind: input.Kind, Value: input.Value, Enabled: input.Enabled,
			Layer: layer, Reason: input.Reason, Provenance: provenance,
		})
	}
	return evidence
}

func visibilityStatus(status shared.VisibilityStatus) VisibilityStatus {
	switch status {
	case shared.VisibilityPackageMasked:
		return VisibilityPackageMasked
	case shared.VisibilityKeywordMasked:
		return VisibilityKeywordMasked
	case shared.VisibilityUnsupportedArchitecture:
		return VisibilityUnsupportedArchitecture
	default:
		return VisibilityVisible
	}
}
