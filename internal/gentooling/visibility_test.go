package gentooling_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
)

func TestReadProspectivePackageDerivesStableUsePolicyFromVisibility(t *testing.T) {
	t.Parallel()

	paths := prospectiveFixture(t)
	evidence, err := maizegentoo.ReadProspectivePackage(
		context.Background(), paths, nil, maizegentoo.ProspectivePackage{
			ID:       shared.PackageID{Category: "app-misc", Name: "stable", Version: "1"},
			Keywords: []string{"amd64"},
			DeclaredUse: []shared.UseDeclaration{
				{Name: "stableonly"},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Visibility.Visible || !evidence.Visibility.Stable ||
		evidence.Visibility.Status != maizegentoo.VisibilityVisible {
		t.Fatalf("visibility = %#v", evidence.Visibility)
	}
	if !evidence.Use.Stable || len(evidence.Use.Decisions) != 1 ||
		!evidence.Use.Decisions[0].Enabled ||
		!evidence.Use.Decisions[0].Forced ||
		evidence.Use.Decisions[0].Evidence[0].Kind != "profile-stable-force" {
		t.Fatalf("stable USE policy = %#v", evidence.Use)
	}
}

func TestReadProspectivePackageEvaluatesTestingKeywordPolicyWithProvenance(t *testing.T) {
	t.Parallel()

	paths := prospectiveFixture(t)
	evidence, err := maizegentoo.ReadProspectivePackage(
		context.Background(), paths, nil, maizegentoo.ProspectivePackage{
			ID:       shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
			Keywords: []string{"~amd64"},
			DeclaredUse: []shared.UseDeclaration{
				{Name: "stableonly"},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	visibility := evidence.Visibility
	if !visibility.Visible || visibility.Stable ||
		visibility.Status != maizegentoo.VisibilityVisible ||
		!reflect.DeepEqual(visibility.AcceptedKeywords, []string{"amd64", "~amd64"}) {
		t.Fatalf("visibility = %#v", visibility)
	}
	if len(visibility.Evidence) != 1 ||
		visibility.Evidence[0].Kind != "package-accept-keywords" ||
		visibility.Evidence[0].Provenance.Kind != domain.SourceConfig ||
		filepath.Base(visibility.Evidence[0].Provenance.Source) != "package.accept_keywords" {
		t.Fatalf("visibility provenance = %#v", visibility.Evidence)
	}
	if evidence.Use.Stable || evidence.Use.Decisions[0].Enabled {
		t.Fatalf("testing package received stable-only USE policy: %#v", evidence.Use)
	}
}

func TestReadProspectivePackageReturnsOrdinaryRejectionAsEvidence(t *testing.T) {
	t.Parallel()

	paths := prospectiveFixture(t)
	evidence, err := maizegentoo.ReadProspectivePackage(
		context.Background(), paths, nil, maizegentoo.ProspectivePackage{
			ID:       shared.PackageID{Category: "app-misc", Name: "masked", Version: "1"},
			Keywords: []string{"amd64"},
		},
	)
	if err != nil {
		t.Fatalf("ordinary mask returned error: %v", err)
	}
	if evidence.Visibility.Visible ||
		evidence.Visibility.Status != maizegentoo.VisibilityPackageMasked ||
		len(evidence.Visibility.Evidence) != 1 ||
		evidence.Visibility.Evidence[0].Reason != "broken on current kernels" {
		t.Fatalf("mask evidence = %#v", evidence.Visibility)
	}

	unsupported, err := maizegentoo.ReadProspectivePackage(
		context.Background(), paths, nil, maizegentoo.ProspectivePackage{
			ID:       shared.PackageID{Category: "app-misc", Name: "other", Version: "1"},
			Keywords: []string{"arm64"},
		},
	)
	if err != nil {
		t.Fatalf("unsupported architecture returned error: %v", err)
	}
	if unsupported.Visibility.Visible ||
		unsupported.Visibility.Status != maizegentoo.VisibilityUnsupportedArchitecture {
		t.Fatalf("unsupported evidence = %#v", unsupported.Visibility)
	}
}

func TestReadProspectivePackagePreservesErrorsAtomically(t *testing.T) {
	t.Parallel()

	paths := prospectiveFixture(t)
	candidate := maizegentoo.ProspectivePackage{
		ID:       shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
		Keywords: []string{"invalid keyword"},
	}
	evidence, err := maizegentoo.ReadProspectivePackage(context.Background(), paths, nil, candidate)
	if !errors.Is(err, shared.ErrInvalidData) {
		t.Fatalf("error %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.ProspectivePackageEvidence{}) {
		t.Fatalf("invalid evidence returned partial result: %#v", evidence)
	}

	candidate.Keywords = []string{"amd64"}
	candidate.DeclaredUse = []shared.UseDeclaration{{Name: "same"}, {Name: "same"}}
	evidence, err = maizegentoo.ReadProspectivePackage(context.Background(), paths, nil, candidate)
	if !errors.Is(err, shared.ErrInvalidData) {
		t.Fatalf("USE error %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.ProspectivePackageEvidence{}) {
		t.Fatalf("invalid USE returned partial result: %#v", evidence)
	}
}

func TestReadProspectivePackageHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	evidence, err := maizegentoo.ReadProspectivePackage(
		ctx, prospectiveFixture(t), nil, maizegentoo.ProspectivePackage{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.ProspectivePackageEvidence{}) {
		t.Fatalf("cancellation returned partial result: %#v", evidence)
	}
}

func prospectiveFixture(t *testing.T) shared.SystemPaths {
	t.Helper()

	paths := effectiveConfigFixture(t)
	leaf, err := filepath.EvalSymlinks(paths.ActiveProfile)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, leaf, "use.stable.force", "stableonly\n")
	writeFile(t, paths.ConfigRoot, "package.accept_keywords", "app-misc/example\n")
	writeFile(t, filepath.Join(paths.Repositories[0].Path, "profiles"), "package.mask",
		"# broken on current kernels\napp-misc/masked\n")
	return paths
}

func TestReadProspectivePackageRejectsSymlinkedVisibilityPolicy(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	target := filepath.Join(t.TempDir(), "keywords")
	if err := os.WriteFile(target, []byte("app-misc/example ~amd64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.ConfigRoot, "package.accept_keywords")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	evidence, err := maizegentoo.ReadProspectivePackage(
		context.Background(), paths, nil, maizegentoo.ProspectivePackage{
			ID:       shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
			Keywords: []string{"amd64"},
		},
	)
	if !errors.Is(err, shared.ErrInvalidData) {
		t.Fatalf("error %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.ProspectivePackageEvidence{}) {
		t.Fatalf("symlink error returned partial evidence: %#v", evidence)
	}
}
