package gentooling

import (
	"context"
	"fmt"
	"path/filepath"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

type ProfilePolicyKind string

const (
	PolicySystem                ProfilePolicyKind = "system"
	PolicyPackageProvided       ProfilePolicyKind = "package-provided"
	PolicyUseForce              ProfilePolicyKind = "use-force"
	PolicyUseMask               ProfilePolicyKind = "use-mask"
	PolicyUseStableForce        ProfilePolicyKind = "use-stable-force"
	PolicyUseStableMask         ProfilePolicyKind = "use-stable-mask"
	PolicyPackageUse            ProfilePolicyKind = "package-use"
	PolicyPackageUseForce       ProfilePolicyKind = "package-use-force"
	PolicyPackageUseMask        ProfilePolicyKind = "package-use-mask"
	PolicyPackageUseStableForce ProfilePolicyKind = "package-use-stable-force"
	PolicyPackageUseStableMask  ProfilePolicyKind = "package-use-stable-mask"
)

type ProfilePolicy struct {
	Kind       ProfilePolicyKind
	Value      string
	Flags      []string
	Provenance domain.Provenance
}

type ProfileLayer struct {
	Path         string
	Parents      []string
	MakeDefaults map[string]string
	Policies     []ProfilePolicy
}

type ProfileEvidence struct {
	ActivePath  string
	Directories []string
	Layers      []ProfileLayer
}

// ReadProfile loads Gentoo profile truth through Gentooling and translates it
// into Maize evidence. It does not evaluate final package USE state.
func ReadProfile(
	ctx context.Context,
	paths shared.SystemPaths,
) (ProfileEvidence, error) {
	profile, err := shared.ReadProfile(ctx, paths)
	if err != nil {
		return ProfileEvidence{}, err
	}
	return profileEvidence(profile), nil
}

func profileEvidence(profile shared.Profile) ProfileEvidence {
	result := ProfileEvidence{
		ActivePath:  profile.ActivePath,
		Directories: append([]string(nil), profile.Directories...),
		Layers:      make([]ProfileLayer, 0, len(profile.Layers)),
	}
	for _, layer := range profile.Layers {
		translated := ProfileLayer{
			Path:         layer.Path,
			Parents:      append([]string(nil), layer.Parents...),
			MakeDefaults: cloneMap(layer.MakeDefaults),
		}
		appendValues := func(kind ProfilePolicyKind, values []string, filename string) {
			for _, value := range values {
				translated.Policies = append(translated.Policies, ProfilePolicy{
					Kind: kind, Value: value,
					Provenance: domain.Provenance{
						Kind: domain.SourceProfile, Source: filepath.Join(layer.Path, filename),
						Detail: "profile policy from layer " + layer.Path,
					},
				})
			}
		}
		appendRules := func(kind ProfilePolicyKind, rules []shared.PackageFlagRule) {
			for _, rule := range rules {
				translated.Policies = append(translated.Policies, ProfilePolicy{
					Kind: kind, Value: rule.Atom, Flags: append([]string(nil), rule.Flags...),
					Provenance: domain.Provenance{
						Kind: domain.SourceProfile, Source: rule.Source.Path,
						Detail: fmt.Sprintf("profile package policy at line %d", rule.Source.Line),
					},
				})
			}
		}
		appendValues(PolicySystem, layer.System, "packages")
		appendValues(PolicyPackageProvided, layer.PackageProvided, "package.provided")
		appendValues(PolicyUseForce, layer.UseForce, "use.force")
		appendValues(PolicyUseMask, layer.UseMask, "use.mask")
		appendValues(PolicyUseStableForce, layer.UseStableForce, "use.stable.force")
		appendValues(PolicyUseStableMask, layer.UseStableMask, "use.stable.mask")
		appendRules(PolicyPackageUse, layer.PackageUse)
		appendRules(PolicyPackageUseForce, layer.PackageUseForce)
		appendRules(PolicyPackageUseMask, layer.PackageUseMask)
		appendRules(PolicyPackageUseStableForce, layer.PackageUseStableForce)
		appendRules(PolicyPackageUseStableMask, layer.PackageUseStableMask)
		result.Layers = append(result.Layers, translated)
	}
	return result
}

func cloneMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}
