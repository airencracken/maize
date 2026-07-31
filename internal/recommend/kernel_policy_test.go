package recommend_test

import (
	"strings"
	"testing"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
)

func TestPackageKernelPolicyTranslatesActiveStaticRequirements(t *testing.T) {
	t.Parallel()

	config, err := kernel.ParseConfig("config", strings.NewReader(
		"# CONFIG_MODULES is not set\nCONFIG_MODVERSIONS=y\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	policy := []maizegentoo.PackageKernelRequirement{
		{
			Package: shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
			Symbol:  "MODULES", Expectation: maizegentoo.KernelEnabled,
			Severity: maizegentoo.KernelFatal, Active: true, Origin: "ebuild",
			Conditions: []maizegentoo.KernelUseCondition{{Flag: "modules", Enabled: true}},
			Function:   "pkg_setup",
			Provenance: domain.Provenance{
				Kind: domain.SourcePackage, Source: "/repo/pkg.ebuild", Detail: "line 2",
			},
		},
		{
			Package: shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
			Symbol:  "MODVERSIONS", Expectation: maizegentoo.KernelDisabled,
			Severity: maizegentoo.KernelWarning, Active: true, Origin: "eclass:test",
			Provenance: domain.Provenance{
				Kind: domain.SourcePackage, Source: "/repo/test.eclass", Detail: "line 3",
			},
		},
		{Symbol: "IGNORED", Active: false},
	}
	got, err := recommend.PackageKernelPolicy(config, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0].Symbol.String() != "CONFIG_MODULES" ||
		got[0].Action != recommend.ActionEnable ||
		got[0].Desired != kernel.Module() ||
		got[0].Disposition != domain.Required ||
		!strings.Contains(got[0].Detail, "app-misc/example-1 explicitly requires CONFIG_MODULES") ||
		!strings.Contains(got[0].Detail, "USE=modules is enabled during pkg_setup") ||
		got[0].Evidence[0].Source != "app-misc/example-1" ||
		got[0].Provenance[0].Source != "/repo/pkg.ebuild" ||
		got[1].Symbol.String() != "CONFIG_MODVERSIONS" ||
		got[1].Desired != kernel.No() ||
		got[1].Disposition != domain.Recommended {
		t.Fatalf("recommendations = %#v", got)
	}
}

func TestPackageKernelPolicyRejectsInvalidSymbolAtomically(t *testing.T) {
	t.Parallel()

	got, err := recommend.PackageKernelPolicy(kernel.Config{}, []maizegentoo.PackageKernelRequirement{
		{Symbol: "GOOD", Active: true},
		{Symbol: "../HOSTILE", Active: true},
	})
	if err == nil || got != nil {
		t.Fatalf("invalid symbol returned %#v, %v", got, err)
	}
}
