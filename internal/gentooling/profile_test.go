package gentooling_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
)

func TestReadProfilePreservesOrderedLayersAndPolicyProvenance(t *testing.T) {
	t.Parallel()

	paths, base, leaf := profileFixture(t)
	evidence, err := maizegentoo.ReadProfile(context.Background(), paths)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if evidence.ActivePath != leaf || len(evidence.Layers) != 2 ||
		evidence.Layers[0].Path != base || evidence.Layers[1].Path != leaf {
		t.Fatalf("profile order lost: %#v", evidence)
	}
	if evidence.Layers[0].MakeDefaults["ARCH"] != "amd64" {
		t.Fatalf("make.defaults lost: %#v", evidence.Layers[0].MakeDefaults)
	}

	var packagePolicy maizegentoo.ProfilePolicy
	for _, policy := range evidence.Layers[1].Policies {
		if policy.Kind == maizegentoo.PolicyPackageUse {
			packagePolicy = policy
			break
		}
	}
	if packagePolicy.Value != "app-containers/docker" ||
		len(packagePolicy.Flags) != 1 || packagePolicy.Flags[0] != "seccomp" ||
		packagePolicy.Provenance.Kind != domain.SourceProfile ||
		packagePolicy.Provenance.Source != filepath.Join(leaf, "package.use") {
		t.Fatalf("package policy provenance lost: %#v", packagePolicy)
	}
}

func TestReadProfileReturnsOwnedEvidence(t *testing.T) {
	t.Parallel()

	paths, _, _ := profileFixture(t)
	evidence, err := maizegentoo.ReadProfile(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Directories[0] = "mutated"
	evidence.Layers[0].MakeDefaults["ARCH"] = "mutated"
	evidence.Layers[0].Policies[0].Value = "mutated"

	again, err := maizegentoo.ReadProfile(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if again.Directories[0] == "mutated" ||
		again.Layers[0].MakeDefaults["ARCH"] == "mutated" ||
		again.Layers[0].Policies[0].Value == "mutated" {
		t.Fatalf("profile evidence aliases prior result: %#v", again)
	}
}

func TestReadProfilePreservesTypedErrorsAndCancellation(t *testing.T) {
	t.Parallel()

	paths, base, _ := profileFixture(t)
	writeFile(t, base, "parent", "../default/linux/amd64\n")
	_, err := maizegentoo.ReadProfile(context.Background(), paths)
	if !errors.Is(err, shared.ErrProfileCycle) {
		t.Fatalf("error %v, want ErrProfileCycle", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = maizegentoo.ReadProfile(ctx, paths)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
}

func TestProfilePolicyConstantsRemainDistinct(t *testing.T) {
	t.Parallel()

	values := []maizegentoo.ProfilePolicyKind{
		maizegentoo.PolicySystem,
		maizegentoo.PolicyPackageProvided,
		maizegentoo.PolicyUseForce,
		maizegentoo.PolicyUseMask,
		maizegentoo.PolicyUseStableForce,
		maizegentoo.PolicyUseStableMask,
		maizegentoo.PolicyPackageUse,
		maizegentoo.PolicyPackageUseForce,
		maizegentoo.PolicyPackageUseMask,
		maizegentoo.PolicyPackageUseStableForce,
		maizegentoo.PolicyPackageUseStableMask,
	}
	seen := make(map[maizegentoo.ProfilePolicyKind]bool)
	for _, value := range values {
		if value == "" || seen[value] {
			t.Fatalf("invalid or duplicate policy kind %q", value)
		}
		seen[value] = true
	}
}

func profileFixture(t *testing.T) (shared.SystemPaths, string, string) {
	t.Helper()

	root := t.TempDir()
	repository := filepath.Join(root, "repos", "gentoo")
	base := filepath.Join(repository, "profiles", "base")
	leaf := filepath.Join(repository, "profiles", "default", "linux", "amd64")
	writeFile(t, base, "make.defaults", "ARCH=\"amd64\"\n")
	writeFile(t, base, "packages", "*sys-apps/baselayout\n")
	writeFile(t, leaf, "parent", "../../../base\n")
	writeFile(t, leaf, "package.use", "app-containers/docker seccomp\n")
	active := filepath.Join(root, "etc", "portage", "make.profile")
	if err := os.MkdirAll(filepath.Dir(active), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(leaf, active); err != nil {
		t.Fatal(err)
	}
	return shared.SystemPaths{
		ActiveProfile: active,
		Repositories:  []shared.RepositoryPath{{Name: "gentoo", Path: repository}},
	}, base, leaf
}

func writeFile(t *testing.T, directory, name, value string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
