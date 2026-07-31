package gentooling

import (
	"context"
	"fmt"
	"slices"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

type KernelExpectation string

const (
	KernelEnabled  KernelExpectation = "enabled"
	KernelDisabled KernelExpectation = "disabled"
)

type KernelSeverity string

const (
	KernelFatal   KernelSeverity = "fatal"
	KernelWarning KernelSeverity = "warning"
)

type KernelUseCondition struct {
	Flag    string
	Enabled bool
}

type PackageKernelRequirement struct {
	Package     shared.PackageID
	Symbol      string
	Expectation KernelExpectation
	Severity    KernelSeverity
	Conditions  []KernelUseCondition
	Active      bool
	Function    string
	Origin      string
	Provenance  domain.Provenance
}

type DynamicKernelPolicy struct {
	Package    shared.PackageID
	Expression string
	Reason     string
	Conditions []KernelUseCondition
	Function   string
	Origin     string
	Provenance domain.Provenance
}

type PackageKernelPolicy struct {
	Package      shared.PackageID
	Requirements []PackageKernelRequirement
	Dynamic      []DynamicKernelPolicy
	Invocations  int
}

// ReadPackageKernelPolicy reads conservative structured kernel requirements
// through Gentooling and evaluates only their explicit USE conditions against
// caller-supplied package state. Shell-dependent evidence remains dynamic.
func ReadPackageKernelPolicy(
	ctx context.Context,
	candidate shared.RepositoryCandidate,
	repositories []shared.Repository,
	enabledUse []string,
	requireComplete bool,
) (PackageKernelPolicy, error) {
	integrity := shared.AllowPartial
	if requireComplete {
		integrity = shared.RequireComplete
	}
	set, err := shared.ReadKernelRequirements(
		ctx, candidate, repositories,
		shared.KernelRequirementOptions{Integrity: integrity},
	)
	if err != nil {
		return PackageKernelPolicy{}, err
	}
	result := PackageKernelPolicy{
		Package: set.Package, Invocations: len(set.Invocations),
	}
	for _, requirement := range set.Requirements {
		conditions := kernelConditions(requirement.Conditions)
		result.Requirements = append(result.Requirements, PackageKernelRequirement{
			Package: set.Package, Symbol: requirement.Symbol,
			Expectation: kernelExpectation(requirement.Expectation),
			Severity:    kernelSeverity(requirement.Severity),
			Conditions:  conditions,
			Active:      conditionsSatisfied(conditions, enabledUse),
			Function:    requirement.Function, Origin: requirement.Origin,
			Provenance: kernelPolicyProvenance(
				requirement.Source, requirement.Origin, "structured kernel requirement",
			),
		})
	}
	for _, dynamic := range set.Dynamic {
		conditions := kernelConditions(dynamic.Conditions)
		result.Dynamic = append(result.Dynamic, DynamicKernelPolicy{
			Package: set.Package, Expression: dynamic.Expression, Reason: dynamic.Reason,
			Conditions: conditions, Function: dynamic.Function, Origin: dynamic.Origin,
			Provenance: kernelPolicyProvenance(
				dynamic.Source, dynamic.Origin, "dynamic kernel policy",
			),
		})
	}
	return result, nil
}

func kernelConditions(input []shared.UseCondition) []KernelUseCondition {
	result := make([]KernelUseCondition, 0, len(input))
	for _, condition := range input {
		result = append(result, KernelUseCondition{
			Flag: condition.Flag, Enabled: condition.Enabled,
		})
	}
	return result
}

func conditionsSatisfied(conditions []KernelUseCondition, enabledUse []string) bool {
	for _, condition := range conditions {
		if slices.Contains(enabledUse, condition.Flag) != condition.Enabled {
			return false
		}
	}
	return true
}

func kernelExpectation(value shared.KernelConfigExpectation) KernelExpectation {
	if value == shared.KernelConfigDisabled {
		return KernelDisabled
	}
	return KernelEnabled
}

func kernelSeverity(value shared.KernelRequirementSeverity) KernelSeverity {
	if value == shared.KernelRequirementWarning {
		return KernelWarning
	}
	return KernelFatal
}

func kernelPolicyProvenance(source shared.PolicySource, origin, detail string) domain.Provenance {
	if source.Line > 0 {
		detail = fmt.Sprintf("%s at line %d", detail, source.Line)
	}
	if origin != "" {
		detail += " from " + origin
	}
	return domain.Provenance{
		Kind: domain.SourcePackage, Source: source.Path, Detail: detail,
	}
}

type InstalledModulePackage struct {
	Package      shared.PackageID
	TargetKernel string
	NeedsRebuild bool
	RebuildState string
	Modules      []string
	Evidence     []string
}

// ReadInstalledModulePackages delegates VDB ownership and inherited-eclass
// classification to Gentooling. TargetKernelRelease is always explicit.
func ReadInstalledModulePackages(
	ctx context.Context,
	paths shared.SystemPaths,
	targetKernelRelease string,
) ([]InstalledModulePackage, error) {
	inventory, err := shared.ReadInstalledKernelModules(
		ctx, paths, shared.InstalledKernelModuleOptions{
			Integrity: shared.RequireComplete, TargetKernelRelease: targetKernelRelease,
		},
	)
	if err != nil {
		return nil, err
	}
	result := make([]InstalledModulePackage, 0, len(inventory.Packages))
	for _, installed := range inventory.Packages {
		item := InstalledModulePackage{
			Package: installed.Package, TargetKernel: installed.TargetKernel,
			NeedsRebuild: installed.NeedsRebuild, RebuildState: string(installed.Rebuild),
		}
		for _, module := range installed.Modules {
			item.Modules = append(item.Modules, module.Path)
		}
		for _, evidence := range installed.Evidence {
			item.Evidence = append(item.Evidence, string(evidence.Kind)+":"+evidence.Value)
		}
		result = append(result, item)
	}
	return result, nil
}
