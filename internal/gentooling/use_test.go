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

func TestReadPackageUseEvaluatesPolicyWithProvenance(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	installed := shared.InstalledPackage{
		ID: shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
		DeclaredUse: []shared.UseDeclaration{
			{Name: "feature", Default: shared.UseDefaultEnabled},
			{Name: "local"},
		},
	}

	evidence, err := maizegentoo.ReadPackageUse(
		context.Background(), paths, []string{"USE=feature"}, installed, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Stable || evidence.Package.CPV() != "app-misc/example-1" {
		t.Fatalf("package context = %#v", evidence)
	}
	if len(evidence.Decisions) != 2 {
		t.Fatalf("decisions = %#v", evidence.Decisions)
	}
	feature := evidence.Decisions[0]
	if feature.Name != "feature" || !feature.Enabled ||
		feature.Default != maizegentoo.UseDefaultEnabled ||
		len(feature.Evidence) != 4 {
		t.Fatalf("feature decision = %#v", feature)
	}
	if feature.Evidence[0].Kind != "iuse-default" ||
		feature.Evidence[0].Provenance.Kind != domain.SourcePackage ||
		feature.Evidence[1].Kind != "user-package-use" ||
		filepath.Base(feature.Evidence[1].Provenance.Source) != "10-base" ||
		feature.Evidence[3].Kind != "command-use" ||
		feature.Evidence[3].Provenance.Kind != domain.SourceOperator {
		t.Fatalf("feature provenance = %#v", feature.Evidence)
	}
	if local := evidence.Decisions[1]; local.Name != "local" || !local.Enabled {
		t.Fatalf("local decision = %#v", local)
	}
}

func TestReadPackageUseReturnsOwnedDeterministicEvidence(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	installed := shared.InstalledPackage{
		ID:          shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
		DeclaredUse: []shared.UseDeclaration{{Name: "zeta"}, {Name: "alpha"}},
	}
	first, err := maizegentoo.ReadPackageUse(context.Background(), paths, nil, installed, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := maizegentoo.ReadPackageUse(context.Background(), paths, nil, installed, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) ||
		len(first.Decisions) != 2 ||
		first.Decisions[0].Name != "alpha" ||
		first.Decisions[1].Name != "zeta" {
		t.Fatalf("nondeterministic decisions:\nfirst  %#v\nsecond %#v", first, second)
	}
	first.Decisions[0].Name = "mutated"
	if second.Decisions[0].Name == "mutated" {
		t.Fatal("results alias each other")
	}
}

func TestReadPackageUsePreservesErrorsAtomically(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	installed := shared.InstalledPackage{
		ID:          shared.PackageID{Category: "app-misc", Name: "example", Version: "1"},
		DeclaredUse: []shared.UseDeclaration{{Name: "duplicate"}, {Name: "duplicate"}},
	}
	evidence, err := maizegentoo.ReadPackageUse(context.Background(), paths, nil, installed, false)
	if !errors.Is(err, shared.ErrInvalidData) {
		t.Fatalf("error %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.PackageUseEvidence{}) {
		t.Fatalf("error returned partial evidence: %#v", evidence)
	}

	if err := os.Remove(paths.MakeGlobals); err != nil {
		t.Fatal(err)
	}
	evidence, err = maizegentoo.ReadPackageUse(context.Background(), paths, nil, installed, false)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error %v, want os.ErrNotExist", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.PackageUseEvidence{}) {
		t.Fatalf("filesystem error returned partial evidence: %#v", evidence)
	}
}

func TestReadPackageUseHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	evidence, err := maizegentoo.ReadPackageUse(
		ctx, effectiveConfigFixture(t), nil,
		shared.InstalledPackage{ID: shared.PackageID{
			Category: "app-misc", Name: "example", Version: "1",
		}}, false,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(evidence, maizegentoo.PackageUseEvidence{}) {
		t.Fatalf("cancellation returned partial evidence: %#v", evidence)
	}
}
