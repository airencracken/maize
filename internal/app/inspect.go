package app

import (
	"context"
	"fmt"
	"io"

	shared "github.com/airencracken/gentooling"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
	"github.com/airencracken/maize/internal/resolve"
)

const InspectSchema = "maize.inspect/v1"

type Inspection struct {
	Schema           string
	InstalledCount   int
	WorldSelections  []maizegentoo.SelectionEvidence
	SystemSelections []maizegentoo.SelectionEvidence
	Recommendations  []recommend.Recommendation
}

// Inspect runs the first read-only Maize pipeline from a consistent Gentoo
// snapshot and an existing kernel configuration to explained recommendations.
func Inspect(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	configPath string,
	configReader io.Reader,
) (Inspection, error) {
	if configReader == nil {
		return Inspection{}, fmt.Errorf("kernel configuration reader is required")
	}
	config, err := kernel.ParseConfig(configPath, configReader)
	if err != nil {
		return Inspection{}, err
	}
	snapshot, err := maizegentoo.ReadSystemSnapshot(ctx, paths, environment, 3)
	if err != nil {
		return Inspection{}, err
	}
	requirements, err := maizegentoo.Requirements(snapshot.Installed, recommend.PackageRules())
	if err != nil {
		return Inspection{}, err
	}
	decisions, err := resolve.Requirements(requirements)
	if err != nil {
		return Inspection{}, err
	}
	recommendations, err := recommend.Kernel(config, decisions, recommend.KernelBindings())
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{
		Schema: InspectSchema, InstalledCount: len(snapshot.Installed.Packages),
		WorldSelections:  append([]maizegentoo.SelectionEvidence(nil), snapshot.Selections.World...),
		SystemSelections: append([]maizegentoo.SelectionEvidence(nil), snapshot.Selections.System...),
		Recommendations:  recommendations,
	}, nil
}
