package gentooling

import (
	"context"
	"fmt"

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
	policy, err := EvaluatePackageKernelPolicy(ctx, candidate, repositories, shared.KernelRequirementContext{
		Phase: "pkg_setup", MergeType: shared.MergeSource, InstalledUSE: enabledUse,
	})
	if err != nil {
		return PackageKernelPolicy{}, err
	}
	if requireComplete && len(policy.Dynamic) != 0 {
		return PackageKernelPolicy{}, fmt.Errorf("incomplete kernel requirements for %s", candidate.ID.CPV())
	}
	return policy, nil
}

func EvaluatePackageKernelPolicy(ctx context.Context, candidate shared.RepositoryCandidate, repositories []shared.Repository, evaluation shared.KernelRequirementContext) (PackageKernelPolicy, error) {
	set, err := shared.EvaluateKernelRequirements(ctx, candidate, repositories, evaluation)
	if err != nil {
		return PackageKernelPolicy{}, err
	}
	result := PackageKernelPolicy{Package: set.Package}
	invocations := make(map[string]bool)
	for _, requirement := range set.Requirements {
		if requirement.Invocation.Function != "" {
			invocations[fmt.Sprintf("%s:%d:%s", requirement.Invocation.Source.Path, requirement.Invocation.Source.Line, requirement.Invocation.Function)] = true
		}
		conditions := kernelConditions(requirement.Conditions)
		result.Requirements = append(result.Requirements, PackageKernelRequirement{
			Package: set.Package, Symbol: requirement.Symbol,
			Expectation: kernelExpectation(requirement.Expectation),
			Severity:    kernelSeverity(requirement.Severity),
			Conditions:  conditions,
			Active:      requirement.Applicability == shared.Applicable,
			Function:    requirement.Invocation.Function, Origin: requirement.Origin,
			Provenance: kernelPolicyProvenance(
				requirement.Source, requirement.Origin, "structured kernel requirement",
			),
		})
	}
	for _, unresolved := range set.Unresolved {
		if !unresolved.Blocking {
			continue
		}
		if unresolved.Invocation.Function != "" {
			invocations[fmt.Sprintf("%s:%d:%s", unresolved.Invocation.Source.Path, unresolved.Invocation.Source.Line, unresolved.Invocation.Function)] = true
		}
		conditions := kernelConditions(unresolved.Conditions)
		result.Dynamic = append(result.Dynamic, DynamicKernelPolicy{
			Package: set.Package, Expression: unresolved.DeveloperText, Reason: unresolved.OperatorText,
			Conditions: conditions, Function: unresolved.Invocation.Function, Origin: unresolved.Origin,
			Provenance: kernelPolicyProvenance(
				unresolved.Source, unresolved.Origin, "unresolved active kernel requirement",
			),
		})
	}
	result.Invocations = len(invocations)
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
