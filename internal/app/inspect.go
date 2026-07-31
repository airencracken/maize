package app

import (
	"context"
	"fmt"
	"io"
	"time"

	shared "github.com/airencracken/gentooling"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
	"github.com/airencracken/maize/internal/resolve"
)

const InspectSchema = "maize.inspect/v2"

type Inspection struct {
	Schema              string
	ConfigSource        kernel.ConfigSource
	CurrentConfig       kernel.Config
	Hardware            hardware.Inventory
	Repositories        []maizegentoo.RepositoryEvidence
	SnapshotConsistency maizegentoo.SnapshotConsistency
	DynamicKernelPolicy []maizegentoo.DynamicKernelPolicy
	CandidateIssues     int
	InstalledCount      int
	WorldSelections     []maizegentoo.SelectionEvidence
	SystemSelections    []maizegentoo.SelectionEvidence
	Recommendations     []recommend.Recommendation
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
	return inspectWithInputs(ctx, paths, environment, config, kernel.ConfigSource{
		Path: configPath, Origin: kernel.ConfigExplicit,
	}, hardware.Inventory{Schema: 1}, maizegentoo.SnapshotLocked, "")
}

// InspectSystem discovers the most authoritative kernel configuration and
// walks current sysfs hardware before evaluating Gentoo package requirements.
func InspectSystem(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	configPaths kernel.ConfigPaths,
	hardwarePaths hardware.SystemPaths,
	observedAt time.Time,
	consistency maizegentoo.SnapshotConsistency,
) (Inspection, error) {
	return InspectSystemForKernel(ctx, paths, environment, configPaths, hardwarePaths, observedAt, consistency, "")
}

func InspectSystemForKernel(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	configPaths kernel.ConfigPaths,
	hardwarePaths hardware.SystemPaths,
	observedAt time.Time,
	consistency maizegentoo.SnapshotConsistency,
	targetKernelRelease string,
) (Inspection, error) {
	config, source, err := kernel.LoadConfig(ctx, configPaths)
	if err != nil {
		return Inspection{}, err
	}
	inventory, err := hardware.Collect(ctx, hardwarePaths, hardware.CollectOptions{
		ObservedAt: observedAt,
	})
	if err != nil {
		return Inspection{}, err
	}
	if targetKernelRelease == "" {
		targetKernelRelease = source.RunningRelease
	}
	return inspectWithInputs(ctx, paths, environment, config, source, inventory, consistency, targetKernelRelease)
}

func inspectWithInputs(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
	config kernel.Config,
	configSource kernel.ConfigSource,
	inventory hardware.Inventory,
	consistency maizegentoo.SnapshotConsistency,
	kernelRelease string,
) (Inspection, error) {
	snapshot, err := maizegentoo.ReadSystemSnapshotForKernel(
		ctx, paths, environment, 3, consistency, kernelRelease,
	)
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
	packagePolicy, err := recommend.PackageKernelPolicy(config, snapshot.KernelPolicy)
	if err != nil {
		return Inspection{}, err
	}
	recommendations = append(recommendations, packagePolicy...)
	return Inspection{
		Schema: InspectSchema, ConfigSource: configSource, Hardware: inventory,
		CurrentConfig:       config,
		Repositories:        append([]maizegentoo.RepositoryEvidence(nil), snapshot.Repositories...),
		SnapshotConsistency: snapshot.Consistency,
		DynamicKernelPolicy: append([]maizegentoo.DynamicKernelPolicy(nil), snapshot.DynamicKernelPolicy...),
		CandidateIssues:     snapshot.CandidateIssues,
		InstalledCount:      len(snapshot.Installed.Packages),
		WorldSelections:     append([]maizegentoo.SelectionEvidence(nil), snapshot.Selections.World...),
		SystemSelections:    append([]maizegentoo.SelectionEvidence(nil), snapshot.Selections.System...),
		Recommendations:     recommendations,
	}, nil
}
