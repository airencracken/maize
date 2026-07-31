package gentooling_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	shared "github.com/airencracken/gentooling"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
)

func TestReadPackageKernelPolicyPreservesStaticDynamicAndUseEvidence(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	ebuild := filepath.Join(repository, "app-misc", "example", "example-1.ebuild")
	writeFile(t, filepath.Dir(ebuild), filepath.Base(ebuild), `
CONFIG_CHECK="MODULES ~MODVERSIONS"
pkg_setup() {
	if use seccomp ; then
		CONFIG_CHECK+=" SECCOMP_FILTER"
	fi
	check_extra_config
}
`)
	candidate := shared.RepositoryCandidate{
		ID: shared.PackageID{
			Category: "app-misc", Name: "example", Version: "1", Repository: "test",
		},
		Inherited: []string{"linux-info"},
	}
	policy, err := maizegentoo.ReadPackageKernelPolicy(
		context.Background(), candidate,
		[]shared.Repository{{Name: "test", Location: repository}},
		[]string{"seccomp"}, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Package.CPV() != "app-misc/example-1" ||
		len(policy.Requirements) != 3 || policy.Invocations != 1 ||
		len(policy.Dynamic) == 0 {
		t.Fatalf("policy = %#v", policy)
	}
	var conditional maizegentoo.PackageKernelRequirement
	for _, requirement := range policy.Requirements {
		if requirement.Symbol == "SECCOMP_FILTER" {
			conditional = requirement
		}
	}
	if !conditional.Active || len(conditional.Conditions) != 1 ||
		conditional.Provenance.Source != ebuild {
		t.Fatalf("conditional requirement = %#v", conditional)
	}
}

func TestReadPackageKernelPolicyStrictlyRejectsDynamicEvidenceAtomically(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	ebuild := filepath.Join(repository, "app-misc", "example", "example-1.ebuild")
	writeFile(t, filepath.Dir(ebuild), filepath.Base(ebuild), `
CONFIG_CHECK="${RUNTIME_SYMBOL}"
pkg_setup() { check_extra_config; }
`)
	candidate := shared.RepositoryCandidate{ID: shared.PackageID{
		Category: "app-misc", Name: "example", Version: "1", Repository: "test",
	}}
	policy, err := maizegentoo.ReadPackageKernelPolicy(
		context.Background(), candidate,
		[]shared.Repository{{Name: "test", Location: repository}}, nil, true,
	)
	if err == nil || !reflect.DeepEqual(policy, maizegentoo.PackageKernelPolicy{}) {
		t.Fatalf("strict dynamic policy returned %#v, %v", policy, err)
	}
}

func TestReadInstalledModulePackagesFindsTargetRebuilds(t *testing.T) {
	t.Parallel()

	vdb := t.TempDir()
	writeInstalled(t, vdb, "sys-fs/zfs-kmod-2.3", map[string]string{
		"INHERITED": "linux-info linux-mod-r1",
		"CONTENTS":  "obj /lib/modules/6.12/extra/zfs.ko.zst hash 1\n",
	})
	packages, err := maizegentoo.ReadInstalledModulePackages(
		context.Background(), shared.SystemPaths{VDB: vdb}, "6.13",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || !packages[0].NeedsRebuild ||
		packages[0].RebuildState != "target_missing" ||
		len(packages[0].Modules) != 1 {
		t.Fatalf("module packages = %#v", packages)
	}
}

func TestReadInstalledModulePackagesRejectsHostileTargetAtomically(t *testing.T) {
	t.Parallel()

	packages, err := maizegentoo.ReadInstalledModulePackages(
		context.Background(), shared.SystemPaths{VDB: t.TempDir()}, "../escape",
	)
	if err == nil || packages != nil {
		t.Fatalf("hostile target returned %#v, %v", packages, err)
	}
}

func TestReadInstalledModulePackagesDoesNotNeedLiveModulesTree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vdb := filepath.Join(root, "vdb")
	writeInstalled(t, vdb, "sys-kernel/module-1", map[string]string{
		"INHERITED": "linux-mod-r1",
		"CONTENTS":  "",
	})
	if err := os.RemoveAll(filepath.Join(root, "lib", "modules")); err != nil {
		t.Fatal(err)
	}
	packages, err := maizegentoo.ReadInstalledModulePackages(
		context.Background(), shared.SystemPaths{VDB: vdb}, "",
	)
	if err != nil || len(packages) != 1 ||
		packages[0].RebuildState != "no_module_artifacts" {
		t.Fatalf("module packages = %#v, %v", packages, err)
	}
}
