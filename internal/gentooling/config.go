package gentooling

import (
	"context"
	"fmt"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
)

type ConfigLayer string

const (
	ConfigGlobal  ConfigLayer = "global"
	ConfigProfile ConfigLayer = "profile"
	ConfigUser    ConfigLayer = "user"
	ConfigCommand ConfigLayer = "command"
)

type UseChange struct {
	Name       string
	Enabled    bool
	Layer      ConfigLayer
	Provenance domain.Provenance
}

type EffectiveConfigEvidence struct {
	Variables         map[string]string
	Profile           *ProfileEvidence
	UseChanges        []UseChange
	UserPackagePolicy []ProfilePolicy
	UseExpand         []string
	UseExpandHidden   []string
	UseExpandImplicit []string
}

// ReadEffectiveConfig reads package-manager truth through Gentooling.
// Environment is explicit command input; nil means disk-only evaluation.
func ReadEffectiveConfig(
	ctx context.Context,
	paths shared.SystemPaths,
	environment []string,
) (EffectiveConfigEvidence, error) {
	config, err := shared.ReadEffectiveConfig(ctx, paths, shared.ConfigOptions{
		Environment: append([]string(nil), environment...),
	})
	if err != nil {
		return EffectiveConfigEvidence{}, err
	}
	return configEvidence(config), nil
}

func configEvidence(config shared.EffectiveConfig) EffectiveConfigEvidence {
	result := EffectiveConfigEvidence{
		Variables:         cloneMap(config.Variables),
		UseExpand:         append([]string(nil), config.UseExpand...),
		UseExpandHidden:   append([]string(nil), config.UseExpandHidden...),
		UseExpandImplicit: append([]string(nil), config.UseExpandImplicit...),
	}
	if config.Profile != nil {
		profile := profileEvidence(*config.Profile)
		result.Profile = &profile
	}
	appendChanges := func(changes []shared.FlagChange) {
		for _, change := range changes {
			layer := ConfigLayer(change.Layer)
			result.UseChanges = append(result.UseChanges, UseChange{
				Name: change.Name, Enabled: change.Enabled, Layer: layer,
				Provenance: configProvenance(change.Source, layer, "effective USE policy"),
			})
		}
	}
	appendChanges(config.ProfileUse)
	appendChanges(config.UserUse)
	appendChanges(config.CommandUse)
	for _, rule := range config.UserPackageUse {
		result.UserPackagePolicy = append(result.UserPackagePolicy, ProfilePolicy{
			Kind: PolicyPackageUse, Value: rule.Atom,
			Flags:      append([]string(nil), rule.Flags...),
			Provenance: configProvenance(rule.Source, ConfigUser, "user package.use policy"),
		})
	}
	return result
}

func configProvenance(source shared.PolicySource, layer ConfigLayer, detail string) domain.Provenance {
	if source.Line > 0 {
		detail = fmt.Sprintf("%s at line %d", detail, source.Line)
	}
	kind := domain.SourceConfig
	if layer == ConfigProfile {
		kind = domain.SourceProfile
	} else if layer == ConfigCommand {
		kind = domain.SourceOperator
	}
	return domain.Provenance{
		Kind: kind, Source: source.Path, Detail: detail,
	}
}
